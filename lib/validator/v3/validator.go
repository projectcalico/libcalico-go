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
	"fmt"
	"net"
	"reflect"
	"regexp"
	"strings"

	validator "gopkg.in/go-playground/validator.v8"

	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8svalidation "k8s.io/apimachinery/pkg/util/validation"

	api "github.com/projectcalico/libcalico-go/lib/apis/v3"
	"github.com/projectcalico/libcalico-go/lib/errors"
	cnet "github.com/projectcalico/libcalico-go/lib/net"
	"github.com/projectcalico/libcalico-go/lib/numorstring"
	"github.com/projectcalico/libcalico-go/lib/selector"
)

// Split the validation into three groups.  Since a struct validator will trump
// any field validation or sub-structure validation we need to split these out.
var validatorField *validator.Validate
var validatorPrimary *validator.Validate
var validatorSecondary *validator.Validate
var validatorTertiary *validator.Validate

// Maximum size of annotations.
const totalAnnotationSizeLimitB int64 = 256 * (1 << 10) // 256 kB

var (
	nameLabelFmt     = "[a-z0-9]([-a-z0-9]*[a-z0-9])?"
	nameSubdomainFmt = nameLabelFmt + "(\\." + nameLabelFmt + ")*"

	// All resource names must follow the subdomain name format.  Some resources we impose
	// more restrictive naming requirements.
	nameRegex = regexp.MustCompile("^" + nameSubdomainFmt + "$")

	containerIDFmt   = "[a-zA-Z0-9]([-a-zA-Z0-9]*[a-zA-Z0-9])?"
	containerIDRegex = regexp.MustCompile("^" + containerIDFmt + "$")

	// NetworkPolicy names must either be a simple DNS1123 label format (nameLabelFmt), or
	// must be the standard name format (nameRegex) prefixed with "knp.default".
	networkPolicyNameRegex = regexp.MustCompile("^((" + nameLabelFmt + ")|(knp\\.default\\.(" + nameSubdomainFmt + ")))$")

	// GlobalNetworkPolicy names must be a simple DNS1123 label format (nameLabelFmt).
	globalNetworkPolicyNameRegex = regexp.MustCompile("^(" + nameLabelFmt + ")$")

	interfaceRegex        = regexp.MustCompile("^[a-zA-Z0-9_-]{1,15}$")
	actionRegex           = regexp.MustCompile("^(Allow|Deny|Log|Pass)$")
	protocolRegex         = regexp.MustCompile("^(TCP|UDP|ICMP|ICMPv6|SCTP|UDPLite)$")
	ipipModeRegex         = regexp.MustCompile("^(Always|CrossSubnet|Never)$")
	logLevelRegex         = regexp.MustCompile("^(Debug|Info|Warning|Error|Fatal)$")
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
	protocolIcmpMsg       = "rules that specify ICMP fields must set protocol to ICMP"

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
	// Perform field-only validation first, that way the struct validators can assume
	// individual fields are valid format.
	if err := validatorField.Struct(current); err != nil {
		return convertError(err)
	}
	// Then perform primary, secondary and tertiary struct-based validation.
	if err := validatorPrimary.Struct(current); err != nil {
		return convertError(err)
	}
	if err := validatorSecondary.Struct(current); err != nil {
		return convertError(err)
	}
	if err := validatorTertiary.Struct(current); err != nil {
		return convertError(err)
	}
	return nil
}

func convertError(err error) errors.ErrorValidation {
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
	validatorField = validator.New(&validator.Config{TagName: "validate", FieldNameTag: "json"})
	validatorPrimary = validator.New(&validator.Config{TagName: "validate", FieldNameTag: "json"})
	validatorSecondary = validator.New(&validator.Config{TagName: "validate", FieldNameTag: "json"})
	validatorTertiary = validator.New(&validator.Config{TagName: "validate", FieldNameTag: "json"})

	// Register field validators.
	registerFieldValidator("action", validateAction)
	registerFieldValidator("interface", validateInterface)
	registerFieldValidator("datastoreType", validateDatastoreType)
	registerFieldValidator("name", validateName)
	registerFieldValidator("containerID", validateContainerID)
	registerFieldValidator("selector", validateSelector)
	registerFieldValidator("labels", validateLabels)
	registerFieldValidator("ipVersion", validateIPVersion)
	registerFieldValidator("ipIpMode", validateIPIPMode)
	registerFieldValidator("policyType", validatePolicyType)
	registerFieldValidator("logLevel", validateLogLevel)
	registerFieldValidator("dropAcceptReturn", validateFelixEtoHAction)
	registerFieldValidator("acceptReturn", validateAcceptReturn)
	registerFieldValidator("portName", validatePortName)
	registerFieldValidator("mustBeNil", validateMustBeNil)
	registerFieldValidator("mustBeFalse", validateMustBeFalse)

	// Register network validators (i.e. validating a correctly masked CIDR).  Also
	// accepts an IP address without a mask (assumes a full mask).
	registerFieldValidator("netv4", validateIPv4Network)
	registerFieldValidator("netv6", validateIPv6Network)
	registerFieldValidator("net", validateIPNetwork)

	// Override the default CIDR validator.  Validates an arbitrary CIDR (does not
	// need to be correctly masked).  Also accepts an IP address without a mask.
	registerFieldValidator("cidrv4", validateCIDRv4)
	registerFieldValidator("cidrv6", validateCIDRv6)
	registerFieldValidator("cidr", validateCIDR)

	// Register structs that have no additional sub structs for first round of validation.
	registerStructValidator(validatorPrimary, validateProtocol, numorstring.Protocol{})
	registerStructValidator(validatorPrimary, validateProtoPort, api.ProtoPort{})
	registerStructValidator(validatorPrimary, validatePort, numorstring.Port{})
	registerStructValidator(validatorPrimary, validateEndpointPort, api.EndpointPort{})
	registerStructValidator(validatorPrimary, validateIPNAT, api.IPNAT{})
	registerStructValidator(validatorPrimary, validateICMPFields, api.ICMPFields{})
	registerStructValidator(validatorPrimary, validateIPPoolSpec, api.IPPoolSpec{})
	registerStructValidator(validatorPrimary, validateNodeSpec, api.NodeSpec{})
	registerStructValidator(validatorPrimary, validateObjectMeta, metav1.ObjectMeta{})

	// Register structs that have one level of additional structs to validate.
	registerStructValidator(validatorSecondary, validateWorkloadEndpointSpec, api.WorkloadEndpointSpec{})
	registerStructValidator(validatorSecondary, validateHostEndpointSpec, api.HostEndpointSpec{})
	registerStructValidator(validatorSecondary, validateRule, api.Rule{})

	// Register structs that have two level of additional structs to validate.
	registerStructValidator(validatorTertiary, validateNetworkPolicy, api.NetworkPolicy{})
	registerStructValidator(validatorTertiary, validateGlobalNetworkPolicy, api.GlobalNetworkPolicy{})
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
	// We need to register the field validation funcs for all validators otherwise
	// the validator panics on an unknown validation type.
	validatorField.RegisterValidation(key, fn)
	validatorPrimary.RegisterValidation(key, fn)
	validatorSecondary.RegisterValidation(key, fn)
	validatorTertiary.RegisterValidation(key, fn)
}

func registerStructValidator(validator *validator.Validate, fn validator.StructLevelFunc, t ...interface{}) {
	validator.RegisterStructValidation(fn, t...)
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

func validateContainerID(v *validator.Validate, topStruct reflect.Value, currentStructOrField reflect.Value, field reflect.Value, fieldType reflect.Type, fieldKind reflect.Kind, param string) bool {
	s := field.String()
	log.Debugf("Validate containerID: %s", s)
	return containerIDRegex.MatchString(s)
}

func validatePortName(v *validator.Validate, topStruct reflect.Value, currentStructOrField reflect.Value, field reflect.Value, fieldType reflect.Type, fieldKind reflect.Kind, param string) bool {
	s := field.String()
	log.Debugf("Validate port name: %s", s)
	return len(s) != 0 && len(k8svalidation.IsValidPortName(s)) == 0
}

func validateMustBeNil(v *validator.Validate, topStruct reflect.Value, currentStructOrField reflect.Value, field reflect.Value, fieldType reflect.Type, fieldKind reflect.Kind, param string) bool {
	log.WithField("field", field.String()).Debugf("Validate field must be nil")
	return field.IsNil()
}

func validateMustBeFalse(v *validator.Validate, topStruct reflect.Value, currentStructOrField reflect.Value, field reflect.Value, fieldType reflect.Type, fieldKind reflect.Kind, param string) bool {
	log.WithField("field", field.String()).Debugf("Validate field must be false")
	return !field.Bool()
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

func validateLogLevel(v *validator.Validate, topStruct reflect.Value, currentStructOrField reflect.Value, field reflect.Value, fieldType reflect.Type, fieldKind reflect.Kind, param string) bool {
	s := field.String()
	log.Debugf("Validate Felix log level: %s", s)
	return logLevelRegex.MatchString(s)
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
	labels := field.Interface().(map[string]string)
	for k, v := range labels {
		if len(k8svalidation.IsQualifiedName(k)) != 0 {
			return false
		}
		if len(k8svalidation.IsValidLabelValue(v)) != 0 {
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

// validateIPv4Network validates the field is a valid (strictly masked) IPv4 network.
// An IP address is valid, and assumed to be fully masked (i.e /32)
func validateIPv4Network(v *validator.Validate, topStruct reflect.Value, currentStructOrField reflect.Value, field reflect.Value, fieldType reflect.Type, fieldKind reflect.Kind, param string) bool {
	n := field.String()
	log.Debugf("Validate IPv4 network: %s", n)
	ipa, ipn, err := cnet.ParseCIDROrIP(n)
	if err != nil {
		return false
	}

	// Check for the correct version and that the CIDR is correctly masked (by comparing the
	// parsed IP against the IP in the parsed network).
	return ipa.Version() == 4 && ipn.IP.String() == ipa.String()
}

// validateIPv4Network validates the field is a valid (strictly masked) IPv6 network.
// An IP address is valid, and assumed to be fully masked (i.e /128)
func validateIPv6Network(v *validator.Validate, topStruct reflect.Value, currentStructOrField reflect.Value, field reflect.Value, fieldType reflect.Type, fieldKind reflect.Kind, param string) bool {
	n := field.String()
	log.Debugf("Validate IPv6 network: %s", n)
	ipa, ipn, err := cnet.ParseCIDROrIP(n)
	if err != nil {
		return false
	}

	// Check for the correct version and that the CIDR is correctly masked (by comparing the
	// parsed IP against the IP in the parsed network).
	return ipa.Version() == 6 && ipn.IP.String() == ipa.String()
}

// validateIPv4Network validates the field is a valid (strictly masked) IP network.
// An IP address is valid, and assumed to be fully masked (i.e /32 or /128)
func validateIPNetwork(v *validator.Validate, topStruct reflect.Value, currentStructOrField reflect.Value, field reflect.Value, fieldType reflect.Type, fieldKind reflect.Kind, param string) bool {
	n := field.String()
	log.Debugf("Validate IP network: %s", n)
	ipa, ipn, err := cnet.ParseCIDROrIP(n)
	if err != nil {
		return false
	}

	// Check  that the CIDR is correctly masked (by comparing the parsed IP against
	// the IP in the parsed network).
	return ipn.IP.String() == ipa.String()
}

// validateIPv4Network validates the field is a valid (not strictly masked) IPv4 network.
// An IP address is valid, and assumed to be fully masked (i.e /32)
func validateCIDRv4(v *validator.Validate, topStruct reflect.Value, currentStructOrField reflect.Value, field reflect.Value, fieldType reflect.Type, fieldKind reflect.Kind, param string) bool {
	n := field.String()
	log.Debugf("Validate IPv4 network: %s", n)
	ipa, _, err := cnet.ParseCIDROrIP(n)
	if err != nil {
		return false
	}

	return ipa.Version() == 4
}

// validateIPv4Network validates the field is a valid (not strictly masked) IPv6 network.
// An IP address is valid, and assumed to be fully masked (i.e /128)
func validateCIDRv6(v *validator.Validate, topStruct reflect.Value, currentStructOrField reflect.Value, field reflect.Value, fieldType reflect.Type, fieldKind reflect.Kind, param string) bool {
	n := field.String()
	log.Debugf("Validate IPv6 network: %s", n)
	ipa, _, err := cnet.ParseCIDROrIP(n)
	if err != nil {
		return false
	}

	return ipa.Version() == 6
}

// validateIPv4Network validates the field is a valid (not strictly masked) IP network.
// An IP address is valid, and assumed to be fully masked (i.e /32 or /128)
func validateCIDR(v *validator.Validate, topStruct reflect.Value, currentStructOrField reflect.Value, field reflect.Value, fieldType reflect.Type, fieldKind reflect.Kind, param string) bool {
	n := field.String()
	log.Debugf("Validate IP network: %s", n)
	_, _, err := cnet.ParseCIDROrIP(n)
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
	} else if p.MinPort < 1 {
		structLevel.ReportError(reflect.ValueOf(p.MinPort),
			"Port", "", reason("port range invalid, port number must be between 1 and 65535"))
	} else if p.MaxPort < 1 {
		structLevel.ReportError(reflect.ValueOf(p.MaxPort),
			"Port", "", reason("port range invalid, port number must be between 1 and 65535"))
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
	// A host endpoint must have a nodename specified.
	if h.Node == "" {
		structLevel.ReportError(reflect.ValueOf(h.Node),
			"InterfaceName", "", reason("no node has been specified"))
	}
}

func validateIPPoolSpec(v *validator.Validate, structLevel *validator.StructLevel) {
	pool := structLevel.CurrentStruct.Interface().(api.IPPoolSpec)

	// Spec.CIDR field must not be empty.
	if pool.CIDR == "" {
		structLevel.ReportError(reflect.ValueOf(pool.CIDR),
			"IPpool.CIDR", "", reason("IPPool CIDR must be specified"))
	}

	// Make sure the CIDR is parsable.
	ipAddr, cidr, err := cnet.ParseCIDROrIP(pool.CIDR)
	if err != nil {
		structLevel.ReportError(reflect.ValueOf(pool.CIDR),
			"IPpool.CIDR", "", reason("IPPool CIDR must be a valid subnet"))
	}

	// Normalize the CIDR before persisting.
	pool.CIDR = cidr.String()

	// IPIP cannot be enabled for IPv6.
	if cidr.Version() == 6 && pool.IPIPMode != api.IPIPModeNever {
		structLevel.ReportError(reflect.ValueOf(pool.IPIPMode),
			"IPpool.IPIPMode", "", reason("IPIPMode other than 'Never' is not supported on an IPv6 IP pool"))
	}

	// The Calico IPAM places restrictions on the minimum IP pool size.  If
	// the ippool is enabled, check that the pool is at least the minimum size.
	if !pool.Disabled {
		ones, bits := cidr.Mask.Size()
		log.Debugf("Pool CIDR: %s, num bits: %d", cidr.String(), bits-ones)
		if bits-ones < 6 {
			if cidr.Version() == 4 {
				structLevel.ReportError(reflect.ValueOf(pool.CIDR),
					"IPpool.CIDR", "", reason(poolSmallIPv4))
			} else {
				structLevel.ReportError(reflect.ValueOf(pool.CIDR),
					"IPpool.CIDR", "", reason(poolSmallIPv6))
			}
		}
	}

	// The Calico CIDR should be strictly masked
	log.Debugf("IPPool CIDR: %s, Masked IP: %d", pool.CIDR, cidr.IP)
	if cidr.IP.String() != ipAddr.String() {
		structLevel.ReportError(reflect.ValueOf(pool.CIDR),
			"IPpool.CIDR", "", reason(poolUnstictCIDR))
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
		structLevel.ReportError(reflect.ValueOf(pool.CIDR),
			"IPpool.CIDR", "", reason(overlapsV4LinkLocal))
	}

	if cidr.Version() == 6 && cidr.IsNetOverlap(ipv6LinkLocalNet) {
		structLevel.ReportError(reflect.ValueOf(pool.CIDR),
			"IPpool.CIDR", "", reason(overlapsV6LinkLocal))
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

	icmp := numorstring.ProtocolFromString("ICMP")
	icmpv6 := numorstring.ProtocolFromString("ICMPv6")
	if rule.ICMP != nil && (rule.Protocol == nil || (*rule.Protocol != icmp && *rule.Protocol != icmpv6)) {
		structLevel.ReportError(reflect.ValueOf(rule.ICMP), "ICMP", "", reason(protocolIcmpMsg))
	}

	// Check that the IPVersion of the protocol matches the IPVersion of the ICMP protocol.
	if (rule.Protocol != nil && *rule.Protocol == icmp) || (rule.NotProtocol != nil && *rule.NotProtocol == icmp) {
		if rule.IPVersion != nil && *rule.IPVersion != 4 {
			structLevel.ReportError(reflect.ValueOf(rule.ICMP), "IPVersion", "", reason("must set ipversion to '4' with protocol icmp"))
		}
	}
	if (rule.Protocol != nil && *rule.Protocol == icmpv6) || (rule.NotProtocol != nil && *rule.NotProtocol == icmpv6) {
		if rule.IPVersion != nil && *rule.IPVersion != 6 {
			structLevel.ReportError(reflect.ValueOf(rule.ICMP), "IPVersion", "", reason("must set ipversion to '6' with protocol icmpv6"))
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

func validateNodeSpec(v *validator.Validate, structLevel *validator.StructLevel) {
	ns := structLevel.CurrentStruct.Interface().(api.NodeSpec)

	if ns.BGP != nil {
		if ns.BGP.IPv4Address == "" && ns.BGP.IPv6Address == "" && ns.BGP.IPv4IPIPTunnelAddr == "" {
			structLevel.ReportError(reflect.ValueOf(ns.BGP.IPv4Address),
				"BGP", "", reason("BGP IP or IPv4IPIPTunnelAddr should be set"))
		}
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

func validateProtoPort(v *validator.Validate, structLevel *validator.StructLevel) {
	m := structLevel.CurrentStruct.Interface().(api.ProtoPort)

	if m.Protocol != "TCP" && m.Protocol != "UDP" {
		structLevel.ReportError(
			reflect.ValueOf(m.Protocol),
			"ProtoPort.Protocol",
			"",
			reason("protocol must be 'TCP' or 'UDP'."),
		)
	}
}

func validateObjectMeta(v *validator.Validate, structLevel *validator.StructLevel) {
	om := structLevel.CurrentStruct.Interface().(metav1.ObjectMeta)

	// Check the name is within the max length.
	if len(om.Name) > k8svalidation.DNS1123SubdomainMaxLength {
		structLevel.ReportError(
			reflect.ValueOf(om.Name),
			"Metadata.Name",
			"",
			reason(fmt.Sprintf("name is too long by %d bytes", len(om.Name)-k8svalidation.DNS1123SubdomainMaxLength)),
		)
	}

	// Uses the k8s DN1123 subdomain format for most resource names.
	matched := nameRegex.MatchString(om.Name)
	if !matched {
		structLevel.ReportError(
			reflect.ValueOf(om.Name),
			"Metadata.Name",
			"",
			reason("name must consist of lower case alphanumeric characters, '-' or '.' (regex: "+nameSubdomainFmt+")"),
		)
	}

	validateObjectMetaAnnotations(v, structLevel, om.Annotations)
	validateObjectMetaLabels(v, structLevel, om.Labels)
}

func validateNetworkPolicy(v *validator.Validate, structLevel *validator.StructLevel) {
	np := structLevel.CurrentStruct.Interface().(api.NetworkPolicy)
	spec := np.Spec

	// Check (and disallow) any repeats in Types field.
	mp := map[api.PolicyType]bool{}
	for _, t := range spec.Types {
		if _, exists := mp[t]; exists {
			structLevel.ReportError(reflect.ValueOf(spec.Types),
				"NetworkPolicySpec.Types", "", reason("'"+string(t)+"' type specified more than once"))
		} else {
			mp[t] = true
		}
	}

	// Check the name is within the max length.
	if len(np.Name) > k8svalidation.DNS1123SubdomainMaxLength {
		structLevel.ReportError(
			reflect.ValueOf(np.Name),
			"Metadata.Name",
			"",
			reason(fmt.Sprintf("name is too long by %d bytes", len(np.Name)-k8svalidation.DNS1123SubdomainMaxLength)),
		)
	}

	// Uses the k8s DN1123 label format for policy names (plus knp.default prefixed k8s policies).
	matched := networkPolicyNameRegex.MatchString(np.Name)
	if !matched {
		structLevel.ReportError(
			reflect.ValueOf(np.Name),
			"Metadata.Name",
			"",
			reason("name must consist of lower case alphanumeric characters or '-' (regex: "+nameLabelFmt+")"),
		)
	}

	validateObjectMetaAnnotations(v, structLevel, np.Annotations)
	validateObjectMetaLabels(v, structLevel, np.Labels)
}

func validateGlobalNetworkPolicy(v *validator.Validate, structLevel *validator.StructLevel) {
	gnp := structLevel.CurrentStruct.Interface().(api.GlobalNetworkPolicy)
	spec := gnp.Spec

	// Check the name is within the max length.
	if len(gnp.Name) > k8svalidation.DNS1123SubdomainMaxLength {
		structLevel.ReportError(
			reflect.ValueOf(gnp.Name),
			"Metadata.Name",
			"",
			reason(fmt.Sprintf("name is too long by %d bytes", len(gnp.Name)-k8svalidation.DNS1123SubdomainMaxLength)),
		)
	}

	// Uses the k8s DN1123 label format for policy names.
	matched := globalNetworkPolicyNameRegex.MatchString(gnp.Name)
	if !matched {
		structLevel.ReportError(
			reflect.ValueOf(gnp.Name),
			"Metadata.Name",
			"",
			reason("name must consist of lower case alphanumeric characters or '-' (regex: "+nameLabelFmt+")"),
		)
	}

	validateObjectMetaAnnotations(v, structLevel, gnp.Annotations)
	validateObjectMetaLabels(v, structLevel, gnp.Labels)

	if spec.DoNotTrack && spec.PreDNAT {
		structLevel.ReportError(reflect.ValueOf(spec.PreDNAT),
			"PolicySpec.PreDNAT", "", reason("PreDNAT and DoNotTrack cannot both be true, for a given PolicySpec"))
	}

	if spec.PreDNAT && len(spec.Egress) > 0 {
		structLevel.ReportError(reflect.ValueOf(spec.Egress),
			"PolicySpec.Egress", "", reason("PreDNAT PolicySpec cannot have any Egress rules"))
	}

	if spec.PreDNAT && len(spec.Types) > 0 {
		for _, t := range spec.Types {
			if t == api.PolicyTypeEgress {
				structLevel.ReportError(reflect.ValueOf(spec.Types),
					"PolicySpec.Types", "", reason("PreDNAT PolicySpec cannot have 'egress' Type"))
			}
		}
	}

	if !spec.ApplyOnForward && (spec.DoNotTrack || spec.PreDNAT) {
		structLevel.ReportError(reflect.ValueOf(spec.ApplyOnForward),
			"PolicySpec.ApplyOnForward", "", reason("ApplyOnForward must be true if either PreDNAT or DoNotTrack is true, for a given PolicySpec"))
	}

	// Check (and disallow) any repeats in Types field.
	mp := map[api.PolicyType]bool{}
	for _, t := range spec.Types {
		if _, exists := mp[t]; exists {
			structLevel.ReportError(reflect.ValueOf(spec.Types),
				"GlobalNetworkPolicySpec.Types", "", reason("'"+string(t)+"' type specified more than once"))
		} else {
			mp[t] = true
		}
	}
}

func validateObjectMetaAnnotations(v *validator.Validate, structLevel *validator.StructLevel, annotations map[string]string) {
	var totalSize int64
	for k, v := range annotations {
		for _, errStr := range k8svalidation.IsQualifiedName(strings.ToLower(k)) {
			structLevel.ReportError(
				reflect.ValueOf(k),
				"Metadata.Annotations (key)",
				"",
				reason(errStr),
			)
		}
		totalSize += (int64)(len(k)) + (int64)(len(v))
	}

	if totalSize > (int64)(totalAnnotationSizeLimitB) {
		structLevel.ReportError(
			reflect.ValueOf(annotations),
			"Metadata.Annotations (key)",
			"",
			reason(fmt.Sprintf("total size of annotations is too large by %d bytes", totalSize-totalAnnotationSizeLimitB)),
		)
	}
}

func validateObjectMetaLabels(v *validator.Validate, structLevel *validator.StructLevel, labels map[string]string) {
	for k, v := range labels {
		for _, errStr := range k8svalidation.IsQualifiedName(k) {
			structLevel.ReportError(
				reflect.ValueOf(k),
				"Metadata.Labels (label)",
				"",
				reason(errStr),
			)
		}
		for _, errStr := range k8svalidation.IsValidLabelValue(v) {
			structLevel.ReportError(
				reflect.ValueOf(v),
				"Metadata.Labels (value)",
				"",
				reason(errStr),
			)
		}
	}
}
