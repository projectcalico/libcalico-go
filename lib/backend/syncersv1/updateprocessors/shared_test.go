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
var iproto = numorstring.ProtocolFromString("TCP")
var inproto = numorstring.ProtocolFromString("UDP")
var port80 = numorstring.SinglePort(uint16(80))
var port443 = numorstring.SinglePort(uint16(443))

var irule = apiv3.Rule{
	Action:    apiv3.Allow,
	IPVersion: &v4,
	Protocol:  &iproto,
	ICMP: &apiv3.ICMPFields{
		Type: &itype,
		Code: &icode,
	},
	NotProtocol: &inproto,
	NotICMP: &apiv3.ICMPFields{
		Type: &intype,
		Code: &incode,
	},
	Source: apiv3.EntityRule{
		Nets:        []string{"10.100.10.1"},
		Selector:    "calico/k8s_ns == selector1",
		Ports:       []numorstring.Port{port80},
		NotNets:     []string{"192.168.40.1"},
		NotSelector: "has(label1)",
		NotPorts:    []numorstring.Port{port443},
	},
	Destination: apiv3.EntityRule{
		Nets:        []string{"10.100.1.1"},
		Selector:    "calico/k8s_ns == selector2",
		Ports:       []numorstring.Port{port443},
		NotNets:     []string{"192.168.80.1"},
		NotSelector: "has(label2)",
		NotPorts:    []numorstring.Port{port80},
	},
}

// Egress rule for a simple GNP.
var etype = 2
var entype = 7
var ecode = 5
var encode = 8
var eproto = numorstring.ProtocolFromInt(uint8(30))
var enproto = numorstring.ProtocolFromInt(uint8(62))

var erule = apiv3.Rule{
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
		Selector:    "calico/k8s_ns == selector2",
		Ports:       []numorstring.Port{port443},
		NotNets:     []string{"192.168.80.1"},
		NotSelector: "has(label2)",
		NotPorts:    []numorstring.Port{port80},
	},
	Destination: apiv3.EntityRule{
		Nets:        []string{"10.100.10.1"},
		Selector:    "calico/k8s_ns == selector1",
		Ports:       []numorstring.Port{port80},
		NotNets:     []string{"192.168.40.1"},
		NotSelector: "has(label1)",
		NotPorts:    []numorstring.Port{port443},
	},
}

var order = float64(101)
var defaultOrder = float64(1000)

func MustParseCIDR(cidr string) *cnet.IPNet {
	ipn := cnet.MustParseCIDR(cidr)
	return &ipn
}

// NewSimplePolicy() returns a statically define model.Policy.
func NewSimplePolicy() (p model.Policy) {
	v1irule := updateprocessors.RuleAPIV2ToBackend(irule, "")
	v1erule := updateprocessors.RuleAPIV2ToBackend(erule, "")

	return model.Policy{
		Order:          &order,
		DoNotTrack:     true,
		InboundRules:   []model.Rule{v1irule},
		OutboundRules:  []model.Rule{v1erule},
		PreDNAT:        false,
		ApplyOnForward: true,
		Types:          []string{"ingress"},
		Selector:       "calico/k8s_ns == selectme",
	}
}
