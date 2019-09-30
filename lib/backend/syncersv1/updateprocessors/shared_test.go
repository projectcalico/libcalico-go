package updateprocessors_test

import (
	"fmt"

	apiv3 "github.com/projectcalico/libcalico-go/lib/apis/v3"
	"github.com/projectcalico/libcalico-go/lib/backend/model"
	"github.com/projectcalico/libcalico-go/lib/backend/syncersv1/updateprocessors"
	cnet "github.com/projectcalico/libcalico-go/lib/net"
	"github.com/projectcalico/libcalico-go/lib/numorstring"
)

// Ingress rule for a simple GNP.
var v4 = 4
var itype = 1
var intype = 3
var icode = 4
var incode = 6
var ProtocolTCP = numorstring.ProtocolFromString("TCP")
var ProtocolUDP = numorstring.ProtocolFromString("UDP")
var Port80 = numorstring.SinglePort(uint16(80))
var Port443 = numorstring.SinglePort(uint16(443))

var testIngressRule = apiv3.Rule{
	Action:    apiv3.Allow,
	IPVersion: &v4,
	Protocol:  &ProtocolTCP,
	ICMP: &apiv3.ICMPFields{
		Type: &itype,
		Code: &icode,
	},
	NotProtocol: &ProtocolUDP,
	NotICMP: &apiv3.ICMPFields{
		Type: &intype,
		Code: &incode,
	},
	Source: apiv3.EntityRule{
		Nets:        []string{"10.100.10.1"},
		Selector:    "mylabel == selector1",
		Ports:       []numorstring.Port{Port80},
		NotNets:     []string{"192.168.40.1"},
		NotSelector: "has(label1)",
		NotPorts:    []numorstring.Port{Port443},
	},
	Destination: apiv3.EntityRule{
		Nets:        []string{"10.100.1.1"},
		Selector:    "mylabel == selector2",
		Ports:       []numorstring.Port{Port443},
		NotNets:     []string{"192.168.80.1"},
		NotSelector: "has(label2)",
		NotPorts:    []numorstring.Port{Port80},
	},
}

// Egress rule for a simple GNP.
var etype = 2
var entype = 7
var ecode = 5
var encode = 8
var eproto = numorstring.ProtocolFromInt(uint8(30))
var enproto = numorstring.ProtocolFromInt(uint8(62))

var testEgressRule = apiv3.Rule{
	Action:    apiv3.Allow,
	IPVersion: &v4,
	Protocol:  &eproto,
	ICMP: &apiv3.ICMPFields{
		Type: &etype,
		Code: &ecode,
	},
	NotProtocol: &enproto,
	NotICMP: &apiv3.ICMPFields{
		Type: &entype,
		Code: &encode,
	},
	Source: apiv3.EntityRule{
		Nets:        []string{"10.100.1.1"},
		Selector:    "mylabel == selector2",
		Ports:       []numorstring.Port{Port443},
		NotNets:     []string{"192.168.80.1"},
		NotSelector: "has(label2)",
		NotPorts:    []numorstring.Port{Port80},
	},
	Destination: apiv3.EntityRule{
		Nets:        []string{"10.100.10.1"},
		Selector:    "mylabel == selector1",
		Ports:       []numorstring.Port{Port80},
		NotNets:     []string{"192.168.40.1"},
		NotSelector: "has(label1)",
		NotPorts:    []numorstring.Port{Port443},
	},
}

var testPolicyOrder101 = float64(101)
var testDefaultPolicyOrder = float64(1000)

// v3 model.KVPair revision
var testRev string = "1234"

func mustParseCIDR(cidr string) *cnet.IPNet {
	ipn := cnet.MustParseCIDR(cidr)
	return &ipn
}

// fullGNPv1 returns a v1 GNP with all fields filled out.
func fullGNPv1() (p model.Policy) {
	v1IR := updateprocessors.RuleAPIV2ToBackend(testIngressRule, "")
	v1ER := updateprocessors.RuleAPIV2ToBackend(testEgressRule, "")

	return model.Policy{
		Order:          &testPolicyOrder101,
		DoNotTrack:     true,
		InboundRules:   []model.Rule{v1IR},
		OutboundRules:  []model.Rule{v1ER},
		PreDNAT:        false,
		ApplyOnForward: true,
		Types:          []string{"ingress", "egress"},
	}
}

// fullGNPv3 returns a v3 GNP with all fields filled out.
func fullGNPv3(namespace, selector string) *apiv3.GlobalNetworkPolicy {
	fullGNP := apiv3.NewGlobalNetworkPolicy()
	fullGNP.Namespace = namespace
	fullGNP.Spec.Order = &testPolicyOrder101
	fullGNP.Spec.Ingress = []apiv3.Rule{testIngressRule}
	fullGNP.Spec.Egress = []apiv3.Rule{testEgressRule}
	fullGNP.Spec.Selector = selector
	fullGNP.Spec.DoNotTrack = true
	fullGNP.Spec.PreDNAT = false
	fullGNP.Spec.ApplyOnForward = true
	fullGNP.Spec.Types = []apiv3.PolicyType{apiv3.PolicyTypeIngress, apiv3.PolicyTypeEgress}
	return fullGNP
}

// fullNPv1 returns a v1 NP with all fields filled out.
func fullNPv1(namespace string) (p model.Policy) {
	v1IR := updateprocessors.RuleAPIV2ToBackend(testIngressRule, namespace)
	v1ER := updateprocessors.RuleAPIV2ToBackend(testEgressRule, namespace)

	fmt.Printf("\n\n\n\n%+v\n\n\n", v1IR)
	return model.Policy{
		Namespace:      namespace,
		Order:          &testPolicyOrder101,
		InboundRules:   []model.Rule{v1IR},
		OutboundRules:  []model.Rule{v1ER},
		ApplyOnForward: true,
		Types:          []string{"ingress", "egress"},
	}
}

// fullNPv3 returns a v3 NP with all fields filled out.
func fullNPv3(name, namespace, selector string) *apiv3.NetworkPolicy {
	fullNP := apiv3.NewNetworkPolicy()
	fullNP.Name = name
	fullNP.Namespace = namespace
	fullNP.Spec.Order = &testPolicyOrder101
	fullNP.Spec.Ingress = []apiv3.Rule{testIngressRule}
	fullNP.Spec.Egress = []apiv3.Rule{testEgressRule}
	fullNP.Spec.Selector = selector
	fullNP.Spec.Types = []apiv3.PolicyType{apiv3.PolicyTypeIngress, apiv3.PolicyTypeEgress}

	return fullNP
}
