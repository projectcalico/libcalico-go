// Copyright (c) 2016 Tigera, Inc. All rights reserved.

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

package validator_test

import (
	"github.com/projectcalico/libcalico-go/lib/validator"

	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/projectcalico/libcalico-go/lib/api"
	"github.com/projectcalico/libcalico-go/lib/backend/model"
	"github.com/projectcalico/libcalico-go/lib/net"
	"github.com/projectcalico/libcalico-go/lib/numorstring"
	"github.com/projectcalico/libcalico-go/lib/scope"
	"github.com/projectcalico/libcalico-go/lib/testutils"
)

func init() {
	// We need some pointers to ints, so just define as values here.
	var V0 = 0
	var V4 = 4
	var V6 = 6
	var V128 = 128
	var V254 = 254
	var V255 = 255
	var V256 = 256

	// Set up some values we use in various tests.
	ipv4_1 := testutils.MustParseIP("1.2.3.4")
	ipv4_2 := testutils.MustParseIP("100.200.0.0")
	ipv6_1 := testutils.MustParseIP("aabb:aabb::ffff")
	ipv6_2 := testutils.MustParseIP("aabb::abcd")
	netv4_1 := testutils.MustParseNetwork("1.2.3.4/32")
	netv4_2 := testutils.MustParseNetwork("1.2.0.0/32")
	netv4_3 := testutils.MustParseNetwork("1.2.3.0/26")
	netv4_4 := testutils.MustParseNetwork("1.2.3.4/10")
	netv4_5 := testutils.MustParseNetwork("1.2.3.4/27")
	netv6_1 := testutils.MustParseNetwork("aabb:aabb::ffff/128")
	netv6_2 := testutils.MustParseNetwork("aabb:aabb::/128")
	netv6_3 := testutils.MustParseNetwork("aabb:aabb::ffff/122")
	netv6_4 := testutils.MustParseNetwork("aabb:aabb::ffff/10")

	// Perform basic validation of different fields and structures to test simple valid/invalid
	// scenarios.  This does not test precise error strings - but does cover a lot of the validation
	// code paths.
	DescribeTable("Validator",
		func(input interface{}, valid bool) {
			if valid {
				Expect(validator.Validate(input)).To(BeNil(),
					"expected value to be valid")
			} else {
				Expect(validator.Validate(input)).ToNot(BeNil(),
					"expected value to be invalid")
			}
		},
		// Empty rule is valid, it means "allow all".
		Entry("empty rule (m)", model.Rule{}, true),

		// (Backend model) Actions.
		Entry("should accept allow action (m)", model.Rule{Action: "allow"}, true),
		Entry("should accept deny action (m)", model.Rule{Action: "deny"}, true),
		Entry("should accept log action (m)", model.Rule{Action: "log"}, true),
		Entry("should reject unknown action (m)", model.Rule{Action: "unknown"}, false),
		Entry("should reject unknown action (m)", model.Rule{Action: "allowfoo"}, false),

		// (API) Actions.
		Entry("should accept allow action", api.Rule{Action: "allow"}, true),
		Entry("should accept deny action", api.Rule{Action: "deny"}, true),
		Entry("should accept log action", api.Rule{Action: "log"}, true),
		Entry("should reject unknown action", api.Rule{Action: "unknown"}, false),
		Entry("should reject unknown action", api.Rule{Action: "allowfoo"}, false),
		Entry("should reject rule with no action", api.Rule{}, false),

		// (Backend model) IP version.
		Entry("should accept IP version 4 (m)", model.Rule{IPVersion: &V4}, true),
		Entry("should accept IP version 6 (m)", model.Rule{IPVersion: &V6}, true),
		Entry("should reject IP version 0 (m)", model.Rule{IPVersion: &V0}, false),

		// (API) IP version.
		Entry("should accept IP version 4", api.Rule{Action: "allow", IPVersion: &V4}, true),
		Entry("should accept IP version 6", api.Rule{Action: "allow", IPVersion: &V6}, true),
		Entry("should reject IP version 0", api.Rule{Action: "allow", IPVersion: &V0}, false),

		// (API) Names.
		Entry("should accept a valid name", api.ProfileMetadata{Name: ".My-valid-Profile_190"}, true),
		Entry("should reject ! in a name", api.ProfileMetadata{Name: "my!nvalid-Profile"}, false),
		Entry("should reject $ in a name", api.ProfileMetadata{Name: "my-invalid-profile$"}, false),

		// (API) Selectors.  Selectors themselves are thorougly UT'd so only need to test simple
		// accept and reject cases here.
		Entry("should accept valid selector", api.EntityRule{Selector: "foo == \"bar\""}, true),
		Entry("should accept valid selector with 'has' and a '/'", api.EntityRule{Selector: "has(calico/k8s_ns)"}, true),
		Entry("should accept valid selector with 'has'and two '/'", api.EntityRule{Selector: "has(calico/k8s_ns/role)"}, true),
		Entry("should reject invalid selector", api.EntityRule{Selector: "thing=hello &"}, false),

		// (API) Tags.
		Entry("should accept a valid tag", api.ProfileMetadata{Tags: []string{".My-valid-tag_190"}}, true),
		Entry("should reject ! in a tag", api.ProfileMetadata{Tags: []string{"my!nvalid-tag"}}, false),
		Entry("should reject $ in a tag", api.ProfileMetadata{Tags: []string{"my-invalid-tag$"}}, false),

		// (API) Labels.
		Entry("should accept a valid label", api.HostEndpointMetadata{Labels: map[string]string{"rank_.0-9": "gold._0-9"}}, true),
		Entry("should accept label key starting with 0-9", api.HostEndpointMetadata{Labels: map[string]string{"2rank": "gold"}}, true),
		Entry("should accept label value starting with 0-9", api.HostEndpointMetadata{Labels: map[string]string{"rank": "2gold"}}, true),
		Entry("should accept label key with dns prefix", api.HostEndpointMetadata{Labels: map[string]string{"calico/k8s_ns": "kube-system"}}, true),
		Entry("should accept label key where prefix is 253 characters", api.HostEndpointMetadata{Labels: map[string]string{"projectcalico.org.projectcalico.org.projectcalico.org.projectcalico.org.projectcalico.org.projectcalico.org.projectcalico.org.projectcalico.org.projectcalico.org.projectcalico.org.projectcalico.org.projectcalico.org.projectcalico.org.calico12345/k8s_ns": "gold"}}, true),
		Entry("should accept label key where prefix begins with an uppercase character", api.HostEndpointMetadata{Labels: map[string]string{"Projectcalico.org12345/k8s_ns": "gold"}}, true),
		Entry("should accept label key with multiple /", api.HostEndpointMetadata{Labels: map[string]string{"k8s_ns/label/role": "gold"}}, true),
		Entry("should reject label key with !", api.HostEndpointMetadata{Labels: map[string]string{"rank!": "gold"}}, false),
		Entry("should reject label key starting with ~", api.HostEndpointMetadata{Labels: map[string]string{"~rank_.0-9": "gold"}}, false),
		Entry("should reject label key ending with ~", api.HostEndpointMetadata{Labels: map[string]string{"rank_.0-9~": "gold"}}, false),
		Entry("should reject label key where prefix > 253 characters", api.HostEndpointMetadata{Labels: map[string]string{"Projectcalico.org.projectcalico.org.projectcalico.org.projectcalico.org.projectcalico.org.projectcalico.org.projectcalico.org.projectcalico.org.projectcalico.org.projectcalico.org.projectcalico.org.projectcalico.org.projectcalico.org.projectcalico.org123/k8s_ns": "gold"}}, false),
		Entry("should reject label value starting with ~", api.HostEndpointMetadata{Labels: map[string]string{"rank_.0-9": "~gold"}}, false),
		Entry("should reject label value ending with ~", api.HostEndpointMetadata{Labels: map[string]string{"rank_.0-9": "gold~"}}, false),

		// (API) Interface.
		Entry("should accept a valid interface", api.WorkloadEndpointSpec{InterfaceName: "ValidIntface0-9"}, true),
		Entry("should reject an interface that is too long", api.WorkloadEndpointSpec{InterfaceName: "interfaceTooLong"}, false),
		Entry("should reject & in an interface", api.WorkloadEndpointSpec{InterfaceName: "Invalid&Intface"}, false),
		Entry("should reject # in an interface", api.WorkloadEndpointSpec{InterfaceName: "Invalid#Intface"}, false),
		Entry("should reject . in an interface", api.WorkloadEndpointSpec{InterfaceName: "Invalid.Intface"}, false),
		Entry("should reject : in an interface", api.WorkloadEndpointSpec{InterfaceName: "Invalid:Intface"}, false),

		// (API) Scope
		Entry("should accept no scope", api.BGPPeerMetadata{}, true),
		Entry("should accept scope global", api.BGPPeerMetadata{Scope: scope.Global}, true),
		Entry("should accept scope node", api.BGPPeerMetadata{Scope: scope.Node}, true),
		Entry("should reject scope foo", api.BGPPeerMetadata{Scope: scope.Scope("foo")}, false),

		// (API) Protocol
		Entry("should accept protocol tcp", protocolFromString("tcp"), true),
		Entry("should accept protocol udp", protocolFromString("udp"), true),
		Entry("should accept protocol icmp", protocolFromString("icmp"), true),
		Entry("should accept protocol icmpv6", protocolFromString("icmpv6"), true),
		Entry("should accept protocol sctp", protocolFromString("sctp"), true),
		Entry("should accept protocol udplite", protocolFromString("udplite"), true),
		Entry("should accept protocol 1 as int", protocolFromInt(1), true),
		Entry("should accept protocol 255 as int", protocolFromInt(255), true),
		Entry("should accept protocol 255 as string", protocolFromString("255"), true),
		Entry("should accept protocol 1 as string", protocolFromString("1"), true),
		Entry("should reject protocol 0 as int", protocolFromInt(0), false),
		Entry("should reject protocol 256 as string", protocolFromString("256"), false),
		Entry("should reject protocol 0 as string", protocolFromString("0"), false),
		Entry("should reject protocol tcpfoo", protocolFromString("tcpfoo"), false),
		Entry("should reject protocol footcp", protocolFromString("footcp"), false),
		Entry("should reject protocol TCP", protocolFromString("TCP"), false),

		// (API) IPNAT
		Entry("should accept valid IPNAT IPv4",
			api.IPNAT{
				InternalIP: ipv4_1,
				ExternalIP: ipv4_2,
			}, true),
		Entry("should accept valid IPNAT IPv6",
			api.IPNAT{
				InternalIP: ipv6_1,
				ExternalIP: ipv6_2,
			}, true),
		Entry("should reject IPNAT mixed IPv4 (int) and IPv6 (ext)",
			api.IPNAT{
				InternalIP: ipv4_1,
				ExternalIP: ipv6_1,
			}, false),
		Entry("should reject IPNAT mixed IPv6 (int) and IPv4 (ext)",
			api.IPNAT{
				InternalIP: ipv6_1,
				ExternalIP: ipv4_1,
			}, false),

		// (API) WorkloadEndpointSpec
		Entry("should accept workload endpoint with interface only",
			api.WorkloadEndpointSpec{
				InterfaceName: "cali012371237",
			}, true),
		Entry("should accept workload endpoint with networks and no nats",
			api.WorkloadEndpointSpec{
				InterfaceName: "cali012371237",
				IPNetworks:    []net.IPNet{netv4_1, netv4_2, netv6_1, netv6_2},
			}, true),
		Entry("should accept workload endpoint with IPv4 NAT covered by network",
			api.WorkloadEndpointSpec{
				InterfaceName: "cali012371237",
				IPNetworks:    []net.IPNet{netv4_1},
				IPNATs:        []api.IPNAT{{InternalIP: ipv4_1, ExternalIP: ipv4_2}},
			}, true),
		Entry("should accept workload endpoint with IPv6 NAT covered by network",
			api.WorkloadEndpointSpec{
				InterfaceName: "cali012371237",
				IPNetworks:    []net.IPNet{netv6_1},
				IPNATs:        []api.IPNAT{{InternalIP: ipv6_1, ExternalIP: ipv6_2}},
			}, true),
		Entry("should accept workload endpoint with IPv4 and IPv6 NAT covered by network",
			api.WorkloadEndpointSpec{
				InterfaceName: "cali012371237",
				IPNetworks:    []net.IPNet{netv4_1, netv6_1},
				IPNATs: []api.IPNAT{
					{InternalIP: ipv4_1, ExternalIP: ipv4_2},
					{InternalIP: ipv6_1, ExternalIP: ipv6_2},
				},
			}, true),
		Entry("should reject workload endpoint with no config", api.WorkloadEndpointSpec{}, false),
		Entry("should reject workload endpoint with IPv4 networks that contain >1 address",
			api.WorkloadEndpointSpec{
				InterfaceName: "cali012371237",
				IPNetworks:    []net.IPNet{netv4_3},
			}, false),
		Entry("should reject workload endpoint with IPv6 networks that contain >1 address",
			api.WorkloadEndpointSpec{
				InterfaceName: "cali012371237",
				IPNetworks:    []net.IPNet{netv6_3},
			}, false),
		Entry("should reject workload endpoint with nats and no networks",
			api.WorkloadEndpointSpec{
				InterfaceName: "cali012371237",
				IPNATs:        []api.IPNAT{{InternalIP: ipv4_2, ExternalIP: ipv4_1}},
			}, false),
		Entry("should reject workload endpoint with IPv4 NAT not covered by network",
			api.WorkloadEndpointSpec{
				InterfaceName: "cali012371237",
				IPNetworks:    []net.IPNet{netv4_1},
				IPNATs:        []api.IPNAT{{InternalIP: ipv4_2, ExternalIP: ipv4_1}},
			}, false),
		Entry("should reject workload endpoint with IPv6 NAT not covered by network",
			api.WorkloadEndpointSpec{
				InterfaceName: "cali012371237",
				IPNetworks:    []net.IPNet{netv6_1},
				IPNATs:        []api.IPNAT{{InternalIP: ipv6_2, ExternalIP: ipv6_1}},
			}, false),

		// (API) HostEndpointSpec
		Entry("should accept host endpoint with interface",
			api.HostEndpointSpec{
				InterfaceName: "eth0",
			}, true),
		Entry("should accept host endpoint with expected IPs",
			api.HostEndpointSpec{
				ExpectedIPs: []net.IP{ipv4_1, ipv6_1},
			}, true),
		Entry("should accept host endpoint with interface and expected IPs",
			api.HostEndpointSpec{
				InterfaceName: "eth0",
				ExpectedIPs:   []net.IP{ipv4_1, ipv6_1},
			}, true),
		Entry("should reject host endpoint with no config", api.HostEndpointSpec{}, false),
		Entry("should reject host endpoint with blank interface an no IPs",
			api.HostEndpointSpec{
				InterfaceName: "",
				ExpectedIPs:   []net.IP{},
			}, false),

		// (API) PoolMetadata
		Entry("should accept IP pool with IPv4 CIDR /26", api.IPPoolMetadata{CIDR: netv4_3}, true),
		Entry("should accept IP pool with IPv4 CIDR /10", api.IPPoolMetadata{CIDR: netv4_4}, true),
		Entry("should accept IP pool with IPv6 CIDR /122", api.IPPoolMetadata{CIDR: netv6_3}, true),
		Entry("should accept IP pool with IPv6 CIDR /10", api.IPPoolMetadata{CIDR: netv6_4}, true),
		Entry("should reject IP pool with IPv4 CIDR /27", api.IPPoolMetadata{CIDR: netv4_5}, false),
		Entry("should reject IP pool with IPv6 CIDR /128", api.IPPoolMetadata{CIDR: netv6_1}, false),

		// (API) ICMPFields
		Entry("should accept ICMP with no config", api.ICMPFields{}, true),
		Entry("should accept ICMP with type with min value", api.ICMPFields{Type: &V0}, true),
		Entry("should accept ICMP with type with max value", api.ICMPFields{Type: &V254}, true),
		Entry("should accept ICMP with type and code with min value", api.ICMPFields{Type: &V128, Code: &V0}, true),
		Entry("should accept ICMP with type and code with min value", api.ICMPFields{Type: &V128, Code: &V255}, true),
		Entry("should reject ICMP with code and no type", api.ICMPFields{Code: &V0}, false),
		Entry("should reject ICMP with type too high", api.ICMPFields{Type: &V255}, false),
		Entry("should reject ICMP with code too high", api.ICMPFields{Type: &V128, Code: &V256}, false),

		// (API) Rule
		Entry("should accept Rule with protocol sctp and no other config",
			api.Rule{
				Action:   "allow",
				Protocol: protocolFromString("sctp"),
			}, true),
		Entry("should accept Rule with source ports and protocol type 6",
			api.Rule{
				Action:   "allow",
				Protocol: protocolFromInt(6),
				Source: api.EntityRule{
					Ports: []numorstring.Port{numorstring.SinglePort(1)},
				},
			}, true),
		Entry("should accept Rule with empty source ports and protocol type 7",
			api.Rule{
				Action:   "allow",
				Protocol: protocolFromInt(7),
				Source: api.EntityRule{
					Ports: []numorstring.Port{},
				},
			}, true),
		Entry("should accept Rule with source !ports and protocol type 17",
			api.Rule{
				Action:   "allow",
				Protocol: protocolFromInt(17),
				Source: api.EntityRule{
					NotPorts: []numorstring.Port{numorstring.SinglePort(1)},
				},
			}, true),
		Entry("should accept Rule with empty source !ports and protocol type 100",
			api.Rule{
				Action:   "allow",
				Protocol: protocolFromInt(100),
				Source: api.EntityRule{
					NotPorts: []numorstring.Port{},
				},
			}, true),
		Entry("should accept Rule with dest ports and protocol type tcp",
			api.Rule{
				Action:   "allow",
				Protocol: protocolFromString("tcp"),
				Destination: api.EntityRule{
					Ports: []numorstring.Port{numorstring.SinglePort(1)},
				},
			}, true),
		Entry("should reject Rule with invalid port (port 0)",
			api.Rule{
				Action:   "allow",
				Protocol: protocolFromString("tcp"),
				Destination: api.EntityRule{
					NotPorts: []numorstring.Port{numorstring.SinglePort(0)},
				},
			}, false),
		Entry("should accept Rule with empty dest ports and protocol type sctp",
			api.Rule{
				Action:   "allow",
				Protocol: protocolFromString("sctp"),
				Destination: api.EntityRule{
					Ports: []numorstring.Port{},
				},
			}, true),
		Entry("should accept Rule with empty dest !ports and protocol type icmpv6",
			api.Rule{
				Action:   "allow",
				Protocol: protocolFromString("icmpv6"),
				Destination: api.EntityRule{
					NotPorts: []numorstring.Port{},
				},
			}, true),
		Entry("should reject Rule with source ports and protocol type 7",
			api.Rule{
				Action:   "allow",
				Protocol: protocolFromInt(7),
				Source: api.EntityRule{
					Ports: []numorstring.Port{numorstring.SinglePort(1)},
				},
			}, false),
		Entry("should reject Rule with source !ports and protocol type 100",
			api.Rule{
				Action:   "allow",
				Protocol: protocolFromInt(100),
				Source: api.EntityRule{
					NotPorts: []numorstring.Port{numorstring.SinglePort(1)},
				},
			}, false),
		Entry("should reject Rule with dest ports and protocol type tcp",
			api.Rule{
				Action:   "allow",
				Protocol: protocolFromString("sctp"),
				Destination: api.EntityRule{
					Ports: []numorstring.Port{numorstring.SinglePort(1)},
				},
			}, false),
		Entry("should reject Rule with dest !ports and protocol type udp",
			api.Rule{
				Action:   "allow",
				Protocol: protocolFromString("icmp"),
				Destination: api.EntityRule{
					NotPorts: []numorstring.Port{numorstring.SinglePort(1)},
				},
			}, false),
		Entry("should reject Rule with invalid source ports and protocol type tcp",
			api.Rule{
				Action:   "allow",
				Protocol: protocolFromString("tcp"),
				Source: api.EntityRule{
					Ports: []numorstring.Port{numorstring.Port{MinPort: 200, MaxPort: 100}},
				},
			}, false),
		Entry("should reject Rule with invalid source !ports and protocol type tcp",
			api.Rule{
				Action:   "allow",
				Protocol: protocolFromString("tcp"),
				Source: api.EntityRule{
					NotPorts: []numorstring.Port{numorstring.Port{MinPort: 200, MaxPort: 100}},
				},
			}, false),
		Entry("should reject Rule with invalid dest ports and protocol type tcp",
			api.Rule{
				Action:   "allow",
				Protocol: protocolFromString("tcp"),
				Destination: api.EntityRule{
					Ports: []numorstring.Port{numorstring.Port{MinPort: 200, MaxPort: 100}},
				},
			}, false),
		Entry("should reject Rule with invalid dest !ports and protocol type tcp",
			api.Rule{
				Action:   "allow",
				Protocol: protocolFromString("tcp"),
				Destination: api.EntityRule{
					NotPorts: []numorstring.Port{numorstring.Port{MinPort: 200, MaxPort: 100}},
				},
			}, false),
		Entry("should reject Rule with one invalid port in the port range (MinPort 0)",
			api.Rule{
				Action:   "allow",
				Protocol: protocolFromString("tcp"),
				Destination: api.EntityRule{
					NotPorts: []numorstring.Port{numorstring.Port{MinPort: 0, MaxPort: 100}},
				},
			}, false),

		// (API) NodeSpec
		Entry("should accept node with IPv4 BGP", api.NodeSpec{BGP: &api.NodeBGPSpec{IPv4Address: &netv4_1}}, true),
		Entry("should accept node with IPv6 BGP", api.NodeSpec{BGP: &api.NodeBGPSpec{IPv6Address: &netv6_1}}, true),
		Entry("should accept node with no BGP", api.NodeSpec{}, true),
		Entry("should reject node with BGP but no IPs", api.NodeSpec{BGP: &api.NodeBGPSpec{}}, false),
		Entry("should reject node with IPv6 address in IPv4 field", api.NodeSpec{BGP: &api.NodeBGPSpec{IPv4Address: &netv6_1}}, false),
		Entry("should reject node with IPv4 address in IPv6 field", api.NodeSpec{BGP: &api.NodeBGPSpec{IPv6Address: &netv4_1}}, false),
	)
}

func protocolFromString(s string) *numorstring.Protocol {
	p := numorstring.ProtocolFromString(s)
	return &p
}

func protocolFromInt(i uint8) *numorstring.Protocol {
	p := numorstring.ProtocolFromInt(i)
	return &p
}
