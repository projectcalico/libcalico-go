// Copyright (c) 2016-2017 Tigera, Inc. All rights reserved.

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v3

import (
	"net"
	"reflect"
	"regexp"
	"strings"

	api "github.com/projectcalico/libcalico-go/lib/apis/v3"
	"github.com/projectcalico/libcalico-go/lib/errors"
	cnet "github.com/projectcalico/libcalico-go/lib/net"
	"github.com/projectcalico/libcalico-go/lib/numorstring"
	"github.com/projectcalico/libcalico-go/lib/selector"
	log "github.com/sirupsen/logrus"

	"github.com/projectcalico/libcalico-go/lib/backend/model"
	"gopkg.in/go-playground/validator.v8"
)

var validate *validator.Validate

var (
	labelRegex            = regexp.MustCompile("^[a-zA-Z0-9_./-]{1,512}$")
	labelValueRegex       = regexp.MustCompile("^[a-zA-Z0-9]?([a-zA-Z0-9_.-]{0,61}[a-zA-Z0-9])?$")
	nameRegex             = regexp.MustCompile("^[a-zA-Z0-9_.-]{1,128}$")
	labelsToApplyValRegex = regexp.MustCompile("^[a-zA-Z0-9_.-]{0,128}$")
	namespacedNameRegex   = regexp.MustCompile(`^[a-zA-Z0-9_./-]{1,128}$`)
	interfaceRegex        = regexp.MustCompile("^[a-zA-Z0-9_-]{1,15}$")
	actionRegex           = regexp.MustCompile("^(Allow|Deny|Log|Pass)$")
	protocolRegex         = regexp.MustCompile("^(TCP|UDP|ICMP|ICMPv6|SCTP|UDPLite)$")
	ipipModeRegex         = regexp.MustCompile("^(Always|CrossSubnet|Never)$")
	felixLogLevel         = regexp.MustCompile("^(Debug|Info|Warning|Error|Fatal)$")
	datastoreType         = regexp.MustCompile("^(etcdv3|kubernetes)$")
	dropAcceptReturnRegex = regexp.MustCompile("^(Drop|Accept|Return)$")
	acceptReturnRegex     = regexp.MustCompile("^(Accept|Return)$")
	reasonString          = "Reason: "
	poolSmallIPv4         = "IP pool size is too small (min /26) for use with Calico IPAM"
	poolSmallIPv6         = "IP pool size is too small (min /122) for use with Calico IPAM"
	poolUnstictCIDR       = "IP pool CIDR is not strictly masked"
	overlapsV4LinkLocal   = "IP pool range overlaps with IPv4 Link Local range 169.254.0.0/16"
	overlapsV6LinkLocal   = "IP pool range overlaps with IPv6 Link Local range fe80::/10"
	protocolPortsMsg      = "rules that specify ports must set protocol to TCP or UDP"

	ipv4LinkLocalNet = net.IPNet{
		IP:   net.ParseIP("169.254.0.0"),
		Mask: net.CIDRMask(16, 32),
	}

	ipv6LinkLocalNet = net.IPNet{
		IP:   net.ParseIP("fe80::"),
		Mask: net.CIDRMask(10, 128),
	}
)

// Validate is used to validate the supplied structure according to the
// registered field and structure validators.
func Validate(current interface{}) error {
	err := validate.Struct(current)
	if err == nil {
		return nil
	}

	// current.(v1.ObjectMetaAccessor).GetObjectMeta().GetName()

	verr := errors.ErrorValidation{}
	for _, f := range err.(validator.ValidationErrors) {
		verr.ErroredFields = append(verr.ErroredFields,
			errors.ErroredField{
				Name:   f.Name,
				Value:  f.Value,
				Reason: extractReason(f.Tag),
			})
	}
	return verr
}

func init() {
	// Initialise static data.
	config := &validator.Config{TagName: "validate", FieldNameTag: "json"}
	validate = validator.New(config)

	// Register field validators.
	registerFieldValidator("action", validateAction)
	registerFieldValidator("interface", validateInterface)
	registerFieldValidator("datastoreType", validateDatastoreType)
	registerFieldValidator("name", validateName)
	registerFieldValidator("namespacedName", validateNamespacedName)
	registerFieldValidator("selector", validateSelector)
	registerFieldValidator("tag", validateTag)
	registerFieldValidator("labels", validateLabels)
	registerFieldValidator("labelsToApply", validateLabelsToApply)
	registerFieldValidator("labelsToApply", validateLabelsToApply)
	registerFieldValidator("ipVersion", validateIPVersion)
	registerFieldValidator("ipIpMode", validateIPIPMode)
	registerFieldValidator("policyType", validatePolicyType)
	registerFieldValidator("bgpLogLevel", validateBGPLogLevel)
	registerFieldValidator("felixLogLevel", validateFelixLogLevel)
	registerFieldValidator("dropAcceptReturn", validateFelixEtoHAction)
	registerFieldValidator("acceptReturn", validateAcceptReturn)
	registerFieldValidator("ipv4", validateIPv4orCIDRAddress)
	registerFieldValidator("ipv6", validateIPv6orCIDRAddress)
	registerFieldValidator("ip", validateIPorCIDRAddress)

	// Register struct validators.
	// Shared types.
	registerStructValidator(validateProtocol, numorstring.Protocol{})
	registerStructValidator(validatePort, numorstring.Port{})

	// Frontend API types.
	registerStructValidator(validateIPNAT, api.IPNAT{})
	registerStructValidator(validateWorkloadEndpointSpec, api.WorkloadEndpointSpec{})
	registerStructValidator(validateHostEndpointSpec, api.HostEndpointSpec{})
	registerStructValidator(validateIPPool, api.IPPool{})
	registerStructValidator(validateICMPFields, api.ICMPFields{})
	registerStructValidator(validateRule, api.Rule{})
	registerStructValidator(validateEndpointPort, api.EndpointPort{})
	registerStructValidator(validateNodeSpec, api.NodeSpec{})
	registerStructValidator(validateGlobalNetworkPolicySpec, api.GlobalNetworkPolicySpec{})
	registerStructValidator(validateNetworkPolicySpec, api.NetworkPolicySpec{})
	registerStructValidator(validateProtoPort, api.ProtoPort{})
	registerStructValidator(validateBGPPeerSpec, api.BGPPeerSpec{})

	// Backend model types.
	registerStructValidator(validateBackendRule, model.Rule{})
	registerStructValidator(validateBackendEndpointPort, model.EndpointPort{})
}

// reason returns the provided error reason prefixed with an identifier that
// allows the string to be used as the field tag in the validator and then
// re-extracted as the reason when the validator returns a field error.
func reason(r string) string {
	return reasonString + r
}

// extractReason extracts the error reason from the field tag in a validator
// field error (if there is one).
func extractReason(tag string) string {
	if strings.HasPrefix(tag, reasonString) {
		return strings.TrimPrefix(tag, reasonString)
	}
	return ""
}

func registerFieldValidator(key string, fn validator.Func) {
	validate.RegisterValidation(key, fn)
}

func registerStructValidator(fn validator.StructLevelFunc, t ...interface{}) {
	validate.RegisterStructValidation(fn, t...)
}

func validateAction(v *validator.Validate, topStruct reflect.Value, currentStructOrField reflect.Value, field reflect.Value, fieldType reflect.Type, fieldKind reflect.Kind, param string) bool {
	s := field.String()
	log.Debugf("Validate action: %s", s)
	return actionRegex.MatchString(s)
}

func validateInterface(v *validator.Validate, topStruct reflect.Value, currentStructOrField reflect.Value, field reflect.Value, fieldType reflect.Type, fieldKind reflect.Kind, param string) bool {
	s := field.String()
	log.Debugf("Validate interface: %s", s)
	return interfaceRegex.MatchString(s)
}

func validateDatastoreType(v *validator.Validate, topStruct reflect.Value, currentStructOrField reflect.Value, field reflect.Value, fieldType reflect.Type, fieldKind reflect.Kind, param string) bool {
	s := field.String()
	log.Debugf("Validate Datastore Type: %s", s)
	return datastoreType.MatchString(s)
}

func validateName(v *validator.Validate, topStruct reflect.Value, currentStructOrField reflect.Value, field reflect.Value, fieldType reflect.Type, fieldKind reflect.Kind, param string) bool {
	s := field.String()
	log.Debugf("Validate name: %s", s)
	return nameRegex.MatchString(s)
}

func validateNamespacedName(v *validator.Validate, topStruct reflect.Value, currentStructOrField reflect.Value, field reflect.Value, fieldType reflect.Type, fieldKind reflect.Kind, param string) bool {
	s := field.String()
	log.Debugf("Validate namespacedName: %s", s)
	return namespacedNameRegex.MatchString(s)
}

func validateIPVersion(v *validator.Validate, topStruct reflect.Value, currentStructOrField reflect.Value, field reflect.Value, fieldType reflect.Type, fieldKind reflect.Kind, param string) bool {
	ver := field.Int()
	log.Debugf("Validate ip version: %d", ver)
	return ver == 4 || ver == 6
}

func validateIPIPMode(v *validator.Validate, topStruct reflect.Value, currentStructOrField reflect.Value, field reflect.Value, fieldType reflect.Type, fieldKind reflect.Kind, param string) bool {
	s := field.String()
	log.Debugf("Validate IPIP Mode: %s", s)
	return ipipModeRegex.MatchString(s)
}

func validateBGPLogLevel(v *validator.Validate, topStruct reflect.Value, currentStructOrField reflect.Value, field reflect.Value, fieldType reflect.Type, fieldKind reflect.Kind, param string) bool {
	s := field.String()
	log.Debugf("Validate BGP log level: %s", s)
	return felixLogLevel.MatchString(s)
}

func validateFelixLogLevel(v *validator.Validate, topStruct reflect.Value, currentStructOrField reflect.Value, field reflect.Value, fieldType reflect.Type, fieldKind reflect.Kind, param string) bool {
	s := field.String()
	log.Debugf("Validate Felix log level: %s", s)
	return felixLogLevel.MatchString(s)
}

func validateFelixEtoHAction(v *validator.Validate, topStruct reflect.Value, currentStructOrField reflect.Value, field reflect.Value, fieldType reflect.Type, fieldKind reflect.Kind, param string) bool {
	s := field.String()
	log.Debugf("Validate Felix DefaultEndpointToHostAction: %s", s)
	return dropAcceptReturnRegex.MatchString(s)
}

func validateAcceptReturn(v *validator.Validate, topStruct reflect.Value, currentStructOrField reflect.Value, field reflect.Value, fieldType reflect.Type, fieldKind reflect.Kind, param string) bool {
	s := field.String()
	log.Debugf("Validate Accept Return Action: %s", s)
	return acceptReturnRegex.MatchString(s)
}

func validateSelector(v *validator.Validate, topStruct reflect.Value, currentStructOrField reflect.Value, field reflect.Value, fieldType reflect.Type, fieldKind reflect.Kind, param string) bool {
	s := field.String()
	log.Debugf("Validate selector: %s", s)

	// We use the selector parser to validate a selector string.
	_, err := selector.Parse(s)
	if err != nil {
		log.Debugf("Selector %#v was invalid: %v", s, err)
		return false
	}
	return true
}

func validateTag(v *validator.Validate, topStruct reflect.Value, currentStructOrField reflect.Value, field reflect.Value, fieldType reflect.Type, fieldKind reflect.Kind, param string) bool {
	s := field.String()
	log.Debugf("Validate tag: %s", s)
	return nameRegex.MatchString(s)
}

func validateLabels(v *validator.Validate, topStruct reflect.Value, currentStructOrField reflect.Value, field reflect.Value, fieldType reflect.Type, fieldKind reflect.Kind, param string) bool {
	l := field.Interface().(map[string]string)
	log.Debugf("Validate labels: %s", l)
	for k, v := range l {
		if !labelRegex.MatchString(k) || !labelValueRegex.MatchString(v) {
			return false
		}
	}
	return true
}

func validateLabelsToApply(v *validator.Validate, topStruct reflect.Value, currentStructOrField reflect.Value, field reflect.Value, fieldType reflect.Type, fieldKind reflect.Kind, param string) bool {
	lta := field.Interface().(map[string]string)
	log.Debugf("Validate LabelsToApply: %s", lta)
	for k, v := range lta {
		if !nameRegex.MatchString(k) || !labelsToApplyValRegex.MatchString(v) {
			return false
		}
	}
	return true
}

func validatePolicyType(v *validator.Validate, topStruct reflect.Value, currentStructOrField reflect.Value, field reflect.Value, fieldType reflect.Type, fieldKind reflect.Kind, param string) bool {
	s := field.String()
	log.Debugf("Validate policy type: %s", s)
	if s == string(api.PolicyTypeIngress) || s == string(api.PolicyTypeEgress) {
		return true
	}
	return false
}

func validateProtocol(v *validator.Validate, structLevel *validator.StructLevel) {
	p := structLevel.CurrentStruct.Interface().(numorstring.Protocol)
	log.Debugf("Validate protocol: %v %s %d", p.Type, p.StrVal, p.NumVal)

	// The protocol field may be an integer 1-255 (i.e. not 0), or one of the valid protocol
	// names.
	if num, err := p.NumValue(); err == nil {
		if num == 0 {
			structLevel.ReportError(reflect.ValueOf(p.NumVal),
				"Protocol", "", reason("protocol number invalid"))
		}
	} else if !protocolRegex.MatchString(p.String()) {
		structLevel.ReportError(reflect.ValueOf(p.String()),
			"Protocol", "", reason("protocol name invalid"))
	}
}

func validateIPv4orCIDRAddress(v *validator.Validate, topStruct reflect.Value, currentStructOrField reflect.Value, field reflect.Value, fieldType reflect.Type, fieldKind reflect.Kind, param string) bool {
	ipAddr := field.String()
	log.Debugf("Validate IPv4 address: %s", ipAddr)
	ipa, _, err := cnet.ParseCIDROrIP(ipAddr)
	if err != nil {
		return false
	}

	return ipa.Version() == 4
}

func validateIPv6orCIDRAddress(v *validator.Validate, topStruct reflect.Value, currentStructOrField reflect.Value, field reflect.Value, fieldType reflect.Type, fieldKind reflect.Kind, param string) bool {
	ipAddr := field.String()
	log.Debugf("Validate IPv6 address: %s", ipAddr)
	ipa, _, err := cnet.ParseCIDROrIP(ipAddr)
	if err != nil {
		return false
	}

	return ipa.Version() == 6
}

func validateIPorCIDRAddress(v *validator.Validate, topStruct reflect.Value, currentStructOrField reflect.Value, field reflect.Value, fieldType reflect.Type, fieldKind reflect.Kind, param string) bool {
	ipAddr := field.String()
	log.Debugf("Validate IP address: %s", ipAddr)
	_, _, err := cnet.ParseCIDROrIP(ipAddr)
	return err == nil
}

func validatePort(v *validator.Validate, structLevel *validator.StructLevel) {
	p := structLevel.CurrentStruct.Interface().(numorstring.Port)

	// Check that the port range is in the correct order.  The YAML parsing also checks this,
	// but this protects against misuse of the programmatic API.
	log.Debugf("Validate port: %v", p)
	if p.MinPort > p.MaxPort {
		structLevel.ReportError(reflect.ValueOf(p.MaxPort),
			"Port", "", reason("port range invalid"))
	}

	if p.PortName != "" {
		if p.MinPort != 0 || p.MaxPort != 0 {
			structLevel.ReportError(reflect.ValueOf(p.PortName),
				"Port", "", reason("named port invalid, if name is specified, min and max should be 0"))
		}
	} else if p.MinPort < 1 || p.MaxPort < 1 {
		structLevel.ReportError(reflect.ValueOf(p.MaxPort),
			"Port", "", reason("port range invalid, port number must be between 0 and 65536"))
	}
}

func validateIPNAT(v *validator.Validate, structLevel *validator.StructLevel) {
	i := structLevel.CurrentStruct.Interface().(api.IPNAT)
	log.Debugf("Internal IP: %s; External IP: %s", i.InternalIP, i.ExternalIP)

	iip, _, err := cnet.ParseCIDROrIP(i.InternalIP)
	if err != nil {
		structLevel.ReportError(reflect.ValueOf(i.ExternalIP),
			"InternalIP", "", reason("invalid IP address"))
	}

	eip, _, err := cnet.ParseCIDROrIP(i.ExternalIP)
	if err != nil {
		structLevel.ReportError(reflect.ValueOf(i.ExternalIP),
			"InternalIP", "", reason("invalid IP address"))
	}

	// An IPNAT must have both the internal and external IP versions the same.
	if iip.Version() != eip.Version() {
		structLevel.ReportError(reflect.ValueOf(i.ExternalIP),
			"ExternalIP", "", reason("mismatched IP versions"))
	}
}

func validateWorkloadEndpointSpec(v *validator.Validate, structLevel *validator.StructLevel) {
	w := structLevel.CurrentStruct.Interface().(api.WorkloadEndpointSpec)

	// The configured networks only support /32 (for IPv4) and /128 (for IPv6) at present.
	for _, netw := range w.IPNetworks {
		_, nw, err := cnet.ParseCIDROrIP(netw)
		if err != nil {
			structLevel.ReportError(reflect.ValueOf(netw),
				"IPNetworks", "", reason("invalid CIDR"))
		}

		ones, bits := nw.Mask.Size()
		if bits != ones {
			structLevel.ReportError(reflect.ValueOf(w.IPNetworks),
				"IPNetworks", "", reason("IP network contains multiple addresses"))
		}
	}

	_, v4gw, err := cnet.ParseCIDROrIP(w.IPv4Gateway)
	if err != nil {
		structLevel.ReportError(reflect.ValueOf(w.IPv4Gateway),
			"IPv4Gateway", "", reason("invalid CIDR"))
	}

	_, v6gw, err := cnet.ParseCIDROrIP(w.IPv6Gateway)
	if err != nil {
		structLevel.ReportError(reflect.ValueOf(w.IPv6Gateway),
			"IPv6Gateway", "", reason("invalid CIDR"))
	}

	if v4gw.IP != nil && v4gw.Version() != 4 {
		structLevel.ReportError(reflect.ValueOf(w.IPv4Gateway),
			"IPv4Gateway", "", reason("invalid IPv4 gateway address specified"))
	}

	if v6gw.IP != nil && v6gw.Version() != 6 {
		structLevel.ReportError(reflect.ValueOf(w.IPv6Gateway),
			"IPv6Gateway", "", reason("invalid IPv6 gateway address specified"))
	}

	// If NATs have been specified, then they should each be within the configured networks of
	// the endpoint.
	if len(w.IPNATs) > 0 {
		valid := false
		for _, nat := range w.IPNATs {
			_, natCidr, err := cnet.ParseCIDROrIP(nat.InternalIP)
			if err != nil {
				structLevel.ReportError(reflect.ValueOf(nat.InternalIP),
					"IPNATs", "", reason("invalid InternalIP CIDR"))
			}
			// Check each NAT to ensure it is within the configured networks.  If any
			// are not then exit without further checks.
			valid = false
			for _, cidr := range w.IPNetworks {
				_, nw, err := cnet.ParseCIDROrIP(cidr)
				if err != nil {
					structLevel.ReportError(reflect.ValueOf(cidr),
						"IPNetworks", "", reason("invalid CIDR"))
				}

				if nw.Contains(natCidr.IP) {
					valid = true
					break
				}
			}
			if !valid {
				break
			}
		}

		if !valid {
			structLevel.ReportError(reflect.ValueOf(w.IPNATs),
				"IPNATs", "", reason("NAT is not in the endpoint networks"))
		}
	}
}

func validateHostEndpointSpec(v *validator.Validate, structLevel *validator.StructLevel) {
	h := structLevel.CurrentStruct.Interface().(api.HostEndpointSpec)

	// A host endpoint must have an interface name and/or some expected IPs specified.
	if h.InterfaceName == "" && len(h.ExpectedIPs) == 0 {
		structLevel.ReportError(reflect.ValueOf(h.InterfaceName),
			"InterfaceName", "", reason("no interface or expected IPs have been specified"))
	}
}

func validateIPPool(v *validator.Validate, structLevel *validator.StructLevel) {
	pool := structLevel.CurrentStruct.Interface().(api.IPPool)

	// Spec.CIDR field must not be empty.
	if pool.Spec.CIDR == "" {
		structLevel.ReportError(reflect.ValueOf(pool.Spec.CIDR),
			"IPPool.Spec.CIDR", "", reason("IPPool CIDR must be specified"))
	}

	// Make sure the CIDR is parsable.
	ipAddr, cidr, err := cnet.ParseCIDR(pool.Spec.CIDR)
	if err != nil {
		structLevel.ReportError(reflect.ValueOf(pool.Spec.CIDR),
			"IPPool.Spec.CIDR", "", reason("IPPool CIDR must be a valid subnet"))
	}

	// Normalize the CIDR before persisting.
	pool.Spec.CIDR = cidr.String()

	// IPIP cannot be enabled for IPv6.
	if cidr.Version() == 6 && pool.Spec.IPIPMode != api.IPIPModeNever {
		structLevel.ReportError(reflect.ValueOf(pool.Spec.IPIPMode),
			"IPPool.Spec.IPIPMode", "", reason("IPIPMode other than 'Never' is not supported on an IPv6 IP pool"))
	}

	// The Calico IPAM places restrictions on the minimum IP pool size.  If
	// the ippool is enabled, check that the pool is at least the minimum size.
	if !pool.Spec.Disabled {
		ones, bits := cidr.Mask.Size()
		log.Debugf("Pool CIDR: %s, num bits: %d", cidr.String(), bits-ones)
		if bits-ones < 6 {
			if cidr.Version() == 4 {
				structLevel.ReportError(reflect.ValueOf(pool.Spec.CIDR),
					"IPPool.Spec.CIDR", "", reason(poolSmallIPv4))
			} else {
				structLevel.ReportError(reflect.ValueOf(pool.Spec.CIDR),
					"IPPool.Spec.CIDR", "", reason(poolSmallIPv6))
			}
		}
	}

	// The Calico CIDR should be strictly masked
	log.Debugf("IPPool CIDR: %s, Masked IP: %d", pool.Spec.CIDR, cidr.IP)
	if cidr.IP.String() != ipAddr.String() {
		structLevel.ReportError(reflect.ValueOf(pool.Spec.CIDR),
			"IPPool.Spec.CIDR", "", reason(poolUnstictCIDR))
	}

	// IPv4 link local subnet.
	ipv4LinkLocalNet := net.IPNet{
		IP:   net.ParseIP("169.254.0.0"),
		Mask: net.CIDRMask(16, 32),
	}
	// IPv6 link local subnet.
	ipv6LinkLocalNet := net.IPNet{
		IP:   net.ParseIP("fe80::"),
		Mask: net.CIDRMask(10, 128),
	}

	// IP Pool CIDR cannot overlap with IPv4 or IPv6 link local address range.
	if cidr.Version() == 4 && cidr.IsNetOverlap(ipv4LinkLocalNet) {
		structLevel.ReportError(reflect.ValueOf(pool.Spec.CIDR),
			"IPPool.Spec.CIDR", "", reason(overlapsV4LinkLocal))
	}

	if cidr.Version() == 6 && cidr.IsNetOverlap(ipv6LinkLocalNet) {
		structLevel.ReportError(reflect.ValueOf(pool.Spec.CIDR),
			"IPPool.Spec.CIDR", "", reason(overlapsV6LinkLocal))
	}
}

func validateICMPFields(v *validator.Validate, structLevel *validator.StructLevel) {
	icmp := structLevel.CurrentStruct.Interface().(api.ICMPFields)

	// Due to Kernel limitations, ICMP code must always be specified with a type.
	if icmp.Code != nil && icmp.Type == nil {
		structLevel.ReportError(reflect.ValueOf(icmp.Code),
			"Code", "", reason("ICMP code specified without an ICMP type"))
	}
}

func validateRule(v *validator.Validate, structLevel *validator.StructLevel) {
	rule := structLevel.CurrentStruct.Interface().(api.Rule)

	// If the protocol is neither tcp (6) nor udp (17) check that the port values have not
	// been specified.
	if rule.Protocol == nil || !rule.Protocol.SupportsPorts() {
		if len(rule.Source.Ports) > 0 {
			structLevel.ReportError(reflect.ValueOf(rule.Source.Ports),
				"Source.Ports", "", reason(protocolPortsMsg))
		}
		if len(rule.Source.NotPorts) > 0 {
			structLevel.ReportError(reflect.ValueOf(rule.Source.NotPorts),
				"Source.NotPorts", "", reason(protocolPortsMsg))
		}

		if len(rule.Destination.Ports) > 0 {
			structLevel.ReportError(reflect.ValueOf(rule.Destination.Ports),
				"Destination.Ports", "", reason(protocolPortsMsg))
		}
		if len(rule.Destination.NotPorts) > 0 {
			structLevel.ReportError(reflect.ValueOf(rule.Destination.NotPorts),
				"Destination.NotPorts", "", reason(protocolPortsMsg))
		}
	}

	var seenV4, seenV6 bool

	scanNets := func(nets []string, fieldName string) {
		var v4, v6 bool
		for _, n := range nets {
			_, cidr, err := cnet.ParseCIDR(n)
			if err != nil {
				structLevel.ReportError(reflect.ValueOf(n), fieldName,
					"", reason("invalid CIDR"))
			} else {
				v4 = v4 || cidr.Version() == 4
				v6 = v6 || cidr.Version() == 6
			}
		}
		if rule.IPVersion != nil && ((v4 && *rule.IPVersion != 4) || (v6 && *rule.IPVersion != 6)) {
			structLevel.ReportError(reflect.ValueOf(rule.Source.Nets), fieldName,
				"", reason("rule IP version doesn't match CIDR version"))
		}
		if v4 && seenV6 || v6 && seenV4 || v4 && v6 {
			// This field makes the rule inconsistent.
			structLevel.ReportError(reflect.ValueOf(nets), fieldName,
				"", reason("rule contains both IPv4 and IPv6 CIDRs"))
		}
		seenV4 = seenV4 || v4
		seenV6 = seenV6 || v6
	}

	scanNets(rule.Source.Nets, "Source.Nets")
	scanNets(rule.Source.NotNets, "Source.NotNets")
	scanNets(rule.Destination.Nets, "Destination.Nets")
	scanNets(rule.Destination.NotNets, "Destination.NotNets")
}

func validateBackendRule(v *validator.Validate, structLevel *validator.StructLevel) {
	rule := structLevel.CurrentStruct.Interface().(model.Rule)

	// If the protocol is neither tcp (6) nor udp (17) check that the port values have not
	// been specified.
	if rule.Protocol == nil || !rule.Protocol.SupportsPorts() {
		if len(rule.SrcPorts) > 0 {
			structLevel.ReportError(reflect.ValueOf(rule.SrcPorts),
				"SrcPorts", "", reason(protocolPortsMsg))
		}
		if len(rule.NotSrcPorts) > 0 {
			structLevel.ReportError(reflect.ValueOf(rule.NotSrcPorts),
				"NotSrcPorts", "", reason(protocolPortsMsg))
		}

		if len(rule.DstPorts) > 0 {
			structLevel.ReportError(reflect.ValueOf(rule.DstPorts),
				"DstPorts", "", reason(protocolPortsMsg))
		}
		if len(rule.NotDstPorts) > 0 {
			structLevel.ReportError(reflect.ValueOf(rule.NotDstPorts),
				"NotDstPorts", "", reason(protocolPortsMsg))
		}
	}
}

func validateNodeSpec(v *validator.Validate, structLevel *validator.StructLevel) {
	ns := structLevel.CurrentStruct.Interface().(api.NodeSpec)

	if ns.BGP != nil {
		if ns.BGP.IPv4Address == "" && ns.BGP.IPv6Address == "" {
			structLevel.ReportError(reflect.ValueOf(ns.BGP.IPv4Address),
				"BGP.IPv4Address", "", reason("no BGP IP address and subnet specified"))
		}
	}
}

func validateBackendEndpointPort(v *validator.Validate, structLevel *validator.StructLevel) {
	port := structLevel.CurrentStruct.Interface().(model.EndpointPort)

	if port.Protocol.String() != "TCP" && port.Protocol.String() != "UDP" {
		structLevel.ReportError(
			reflect.ValueOf(port.Protocol),
			"EndpointPort.Protocol",
			"",
			reason("EndpointPort protocol must be 'TCP' or 'UDP'."),
		)
	}
}

func validateEndpointPort(v *validator.Validate, structLevel *validator.StructLevel) {
	port := structLevel.CurrentStruct.Interface().(api.EndpointPort)

	if port.Protocol.String() != "TCP" && port.Protocol.String() != "UDP" {
		structLevel.ReportError(
			reflect.ValueOf(port.Protocol),
			"EndpointPort.Protocol",
			"",
			reason("EndpointPort protocol must be 'TCP' or 'UDP'."),
		)
	}
}

func validateGlobalNetworkPolicySpec(v *validator.Validate, structLevel *validator.StructLevel) {
	m := structLevel.CurrentStruct.Interface().(api.GlobalNetworkPolicySpec)

	if m.DoNotTrack && m.PreDNAT {
		structLevel.ReportError(reflect.ValueOf(m.PreDNAT),
			"PolicySpec.PreDNAT", "", reason("PreDNAT and DoNotTrack cannot both be true, for a given PolicySpec"))
	}

	if m.PreDNAT && len(m.Egress) > 0 {
		structLevel.ReportError(reflect.ValueOf(m.Egress),
			"PolicySpec.Egress", "", reason("PreDNAT PolicySpec cannot have any Egress"))
	}

	if m.PreDNAT && len(m.Types) > 0 {
		for _, t := range m.Types {
			if t == api.PolicyTypeEgress {
				structLevel.ReportError(reflect.ValueOf(m.Types),
					"PolicySpec.Types", "", reason("PreDNAT PolicySpec cannot have 'egress' Type"))
			}
		}
	}

	if !m.ApplyOnForward && (m.DoNotTrack || m.PreDNAT) {
		structLevel.ReportError(reflect.ValueOf(m.ApplyOnForward),
			"PolicySpec.ApplyOnForward", "", reason("ApplyOnForward must be true if either PreDNAT or DoNotTrack is true, for a given PolicySpec"))
	}

	// Check (and disallow) any repeats in Types field.
	mp := map[api.PolicyType]bool{}
	for _, t := range m.Types {
		if _, exists := mp[t]; exists {
			structLevel.ReportError(reflect.ValueOf(m.Types),
				"GlobalNetworkPolicySpec.Types", "", reason("'"+string(t)+"' type specified more than once"))
		} else {
			mp[t] = true
		}
	}
}

func validateNetworkPolicySpec(v *validator.Validate, structLevel *validator.StructLevel) {
	m := structLevel.CurrentStruct.Interface().(api.NetworkPolicySpec)

	// Check (and disallow) any repeats in Types field.
	mp := map[api.PolicyType]bool{}
	for _, t := range m.Types {
		if _, exists := mp[t]; exists {
			structLevel.ReportError(reflect.ValueOf(m.Types),
				"NetworkPolicySpec.Types", "", reason("'"+string(t)+"' type specified more than once"))
		} else {
			mp[t] = true
		}
	}
}

func validateProtoPort(v *validator.Validate, structLevel *validator.StructLevel) {
	m := structLevel.CurrentStruct.Interface().(api.ProtoPort)

	if m.Protocol != "TCP" && m.Protocol != "UDP" {
		structLevel.ReportError(
			reflect.ValueOf(m.Protocol),
			"ProtoPort.Protocol",
			"",
			reason("ProtoPort protocol must be 'TCP' or 'UDP'."),
		)
	}
}

func validateBGPPeerSpec(v *validator.Validate, structLevel *validator.StructLevel) {
	m := structLevel.CurrentStruct.Interface().(api.BGPPeerSpec)

	ipAddr := cnet.ParseIP(m.PeerIP)

	if ipAddr == nil {
		structLevel.ReportError(
			reflect.ValueOf(m.PeerIP),
			"BGPPeerSpec.PeerIP",
			"",
			reason("Invalid PeerIP: '" + m.PeerIP + "'."),
		)
	}
}
