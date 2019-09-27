package updateprocessors_test

import (
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

var TestIngressRule = apiv3.Rule{
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

var TestEgressRule = apiv3.Rule{
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

var Order = float64(101)
var DefaultPolicyOrder = float64(1000)

// v3 model.KVPair revision
var TestRev string = "1234"

func MustParseCIDR(cidr string) *cnet.IPNet {
	ipn := cnet.MustParseCIDR(cidr)
	return &ipn
}

func NewCompleteGNP() (p model.Policy) {
	v1IR := updateprocessors.RuleAPIV2ToBackend(TestIngressRule, "")
	v1ER := updateprocessors.RuleAPIV2ToBackend(TestEgressRule, "")

	return model.Policy{
		Order:          &Order,
		DoNotTrack:     true,
		InboundRules:   []model.Rule{v1IR},
		OutboundRules:  []model.Rule{v1ER},
		PreDNAT:        false,
		ApplyOnForward: true,
		Types:          []string{"ingress"},
	}
}

func NewCompleteNP(namespace string) (p model.Policy) {
	v1IR := updateprocessors.RuleAPIV2ToBackend(TestIngressRule, namespace)
	v1ER := updateprocessors.RuleAPIV2ToBackend(TestEgressRule, namespace)

	return model.Policy{
		Namespace:      namespace,
		Order:          &Order,
		InboundRules:   []model.Rule{v1IR},
		OutboundRules:  []model.Rule{v1ER},
		ApplyOnForward: true,
		Types:          []string{"ingress"},
	}
}
