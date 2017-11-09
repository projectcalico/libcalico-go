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

package validator_test

import (
	"github.com/projectcalico/libcalico-go/lib/validator"

	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	api "github.com/projectcalico/libcalico-go/lib/apis/v2"
	"github.com/projectcalico/libcalico-go/lib/backend/model"
	"github.com/projectcalico/libcalico-go/lib/numorstring"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
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
	ipv4_1 := "1.2.3.4"
	ipv4_2 := "100.200.0.0"
	ipv6_1 := "aabb:aabb::ffff"
	ipv6_2 := "aabb::abcd"
	netv4_1 := "1.2.3.4/32"
	netv4_2 := "1.2.0.0/32"
	netv4_3 := "1.2.3.0/26"
	netv4_4 := "1.0.0.0/10"
	netv4_5 := "1.2.3.0/27"
	netv6_1 := "aabb:aabb::ffff/128"
	netv6_2 := "aabb:aabb::/128"
	netv6_3 := "aabb:aabb::0000/122"
	netv6_4 := "aa00:0000::0000/10"

	protoTCP := numorstring.ProtocolFromString("tcp")
	protoUDP := numorstring.ProtocolFromString("udp")
	protoNumeric := numorstring.ProtocolFromInt(123)

	// badPorts contains a port that should fail validation because it mixes named and numeric
	// ports.
	badPorts := []numorstring.Port{{
		PortName: "foo",
		MinPort:  1,
		MaxPort:  123,
	}}

	// Perform basic validation of different fields and structures to test simple valid/invalid
	// scenarios.  This does not test precise error strings - but does cover a lot of the validation
	// code paths.
	DescribeTable("Validator",
		func(input interface{}, valid bool) {
			if valid {
				Expect(validator.Validate(input)).NotTo(HaveOccurred(),
					"expected value to be valid")
			} else {
				Expect(validator.Validate(input)).To(HaveOccurred(),
					"expected value to be invalid")
			}
		},
		// Empty rule is valid, it means "allow all".
		Entry("empty rule (m)", model.Rule{}, true),

		// (Backend model) ICMP.
		Entry("should accept ICMP 0/any (m)", model.Rule{ICMPType: &V0}, true),
		Entry("should accept ICMP 0/any (m)", model.Rule{ICMPType: &V0}, true),
		Entry("should accept ICMP 0/0 (m)", model.Rule{ICMPType: &V0, ICMPCode: &V0}, true),
		Entry("should accept ICMP 128/0 (m)", model.Rule{ICMPType: &V128, ICMPCode: &V0}, true),
		Entry("should accept ICMP 254/255 (m)", model.Rule{ICMPType: &V254, ICMPCode: &V255}, true),
		Entry("should reject ICMP 255/255 (m)", model.Rule{ICMPType: &V255, ICMPCode: &V255}, false),
		Entry("should reject ICMP 128/256 (m)", model.Rule{ICMPType: &V128, ICMPCode: &V256}, false),
		Entry("should accept not ICMP 0/any (m)", model.Rule{NotICMPType: &V0}, true),
		Entry("should accept not ICMP 0/0 (m)", model.Rule{NotICMPType: &V0, NotICMPCode: &V0}, true),
		Entry("should accept not ICMP 128/0 (m)", model.Rule{NotICMPType: &V128, NotICMPCode: &V0}, true),
		Entry("should accept not ICMP 254/255 (m)", model.Rule{NotICMPType: &V254, NotICMPCode: &V255}, true),
		Entry("should reject not ICMP 255/255 (m)", model.Rule{NotICMPType: &V255, NotICMPCode: &V255}, false),
		Entry("should reject not ICMP 128/256 (m)", model.Rule{NotICMPType: &V128, NotICMPCode: &V256}, false),

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

		// (Backend model) Ports.
		Entry("should accept ports with tcp protocol (m)", model.Rule{
			Protocol: &protoTCP,
			SrcPorts: []numorstring.Port{numorstring.SinglePort(80)},
		}, true),
		Entry("should reject src ports with no protocol (m)", model.Rule{
			SrcPorts: []numorstring.Port{numorstring.SinglePort(80)},
		}, false),
		Entry("should reject dst ports with no protocol (m)", model.Rule{
			DstPorts: []numorstring.Port{numorstring.SinglePort(80)},
		}, false),
		Entry("should reject !src ports with no protocol (m)", model.Rule{
			NotSrcPorts: []numorstring.Port{numorstring.SinglePort(80)},
		}, false),
		Entry("should reject !dst ports with no protocol (m)", model.Rule{
			NotDstPorts: []numorstring.Port{numorstring.SinglePort(80)},
		}, false),
		Entry("should accept src named ports with tcp protocol (m)", model.Rule{
			Protocol: &protoTCP,
			SrcPorts: []numorstring.Port{numorstring.NamedPort("foo")},
		}, true),
		Entry("should accept dst named ports with tcp protocol (m)", model.Rule{
			Protocol: &protoTCP,
			DstPorts: []numorstring.Port{numorstring.NamedPort("foo")},
		}, true),
		Entry("should accept !src named ports with tcp protocol (m)", model.Rule{
			Protocol:    &protoTCP,
			NotSrcPorts: []numorstring.Port{numorstring.NamedPort("foo")},
		}, true),
		Entry("should accept !dst named ports with tcp protocol (m)", model.Rule{
			Protocol:    &protoTCP,
			NotDstPorts: []numorstring.Port{numorstring.NamedPort("foo")},
		}, true),
		Entry("should reject src named ports with no protocol (m)", model.Rule{
			SrcPorts: []numorstring.Port{numorstring.NamedPort("foo")},
		}, false),
		Entry("should reject dst named ports with no protocol (m)", model.Rule{
			DstPorts: []numorstring.Port{numorstring.NamedPort("foo")},
		}, false),
		Entry("should reject !src named ports with no protocol (m)", model.Rule{
			NotSrcPorts: []numorstring.Port{numorstring.NamedPort("foo")},
		}, false),
		Entry("should reject !dst named ports with no protocol (m)", model.Rule{
			NotDstPorts: []numorstring.Port{numorstring.NamedPort("foo")},
		}, false),
		// Check that we tell the validator to "dive" and validate the port too.
		Entry("should reject src named ports with min and max (m)", model.Rule{
			Protocol: &protoTCP,
			SrcPorts: badPorts,
		}, false),
		Entry("should reject !src named ports with min and max (m)", model.Rule{
			Protocol:    &protoTCP,
			NotSrcPorts: badPorts,
		}, false),
		Entry("should reject dst named ports with min and max (m)", model.Rule{
			Protocol: &protoTCP,
			DstPorts: badPorts,
		}, false),
		Entry("should reject !dst named ports with min and max (m)", model.Rule{
			Protocol:    &protoTCP,
			NotDstPorts: badPorts,
		}, false),

		// (Backend model) EndpointPorts.
		Entry("should accept EndpointPort with tcp protocol (m)", model.EndpointPort{
			Name:     "a_Jolly-port",
			Protocol: protoTCP,
			Port:     1234,
		}, true),
		Entry("should accept EndpointPort with udp protocol (m)", model.EndpointPort{
			Name:     "a_Jolly-port",
			Protocol: protoUDP,
			Port:     1234,
		}, true),
		Entry("should reject EndpointPort with empty name (m)", model.EndpointPort{
			Name:     "",
			Protocol: protoUDP,
			Port:     1234,
		}, false),
		Entry("should reject EndpointPort with no protocol (m)", model.EndpointPort{
			Name: "a_Jolly-port",
			Port: 1234,
		}, false),
		Entry("should reject EndpointPort with numeric protocol (m)", model.EndpointPort{
			Name:     "a_Jolly-port",
			Protocol: protoNumeric,
			Port:     1234,
		}, false),
		Entry("should reject EndpointPort with no port (m)", model.EndpointPort{
			Name:     "a_Jolly-port",
			Protocol: protoTCP,
		}, false),

		// (API model) EndpointPorts.
		Entry("should accept EndpointPort with tcp protocol", api.EndpointPort{
			Name:     "a_Jolly-port",
			Protocol: protoTCP,
			Port:     1234,
		}, true),
		Entry("should accept EndpointPort with udp protocol", api.EndpointPort{
			Name:     "a_Jolly-port",
			Protocol: protoUDP,
			Port:     1234,
		}, true),
		Entry("should reject EndpointPort with empty name", api.EndpointPort{
			Name:     "",
			Protocol: protoUDP,
			Port:     1234,
		}, false),
		Entry("should reject EndpointPort with no protocol", api.EndpointPort{
			Name: "a_Jolly-port",
			Port: 1234,
		}, false),
		Entry("should reject EndpointPort with numeric protocol", api.EndpointPort{
			Name:     "a_Jolly-port",
			Protocol: protoNumeric,
			Port:     1234,
		}, false),
		Entry("should reject EndpointPort with no port", api.EndpointPort{
			Name:     "a_Jolly-port",
			Protocol: protoTCP,
		}, false),

		// (Backend model) WorkloadEndpoint.
		Entry("should accept WorkloadEndpoint with a port (m)",
			model.WorkloadEndpoint{
				Ports: []model.EndpointPort{
					{
						Name:     "a_Jolly-port",
						Protocol: protoTCP,
						Port:     1234,
					},
				},
			},
			true,
		),
		Entry("should reject WorkloadEndpoint with an unnamed port (m)",
			model.WorkloadEndpoint{
				Ports: []model.EndpointPort{
					{
						Protocol: protoTCP,
						Port:     1234,
					},
				},
			},
			false,
		),
		Entry("should accept WorkloadEndpoint with name-clashing ports (m)",
			model.WorkloadEndpoint{
				Ports: []model.EndpointPort{
					{
						Name:     "a_Jolly-port",
						Protocol: protoTCP,
						Port:     1234,
					},
					{
						Name:     "a_Jolly-port",
						Protocol: protoUDP,
						Port:     5456,
					},
				},
			},
			true,
		),

		// (API) WorkloadEndpointSpec.
		Entry("should accept WorkloadEndpointSpec with a port (m)",
			api.WorkloadEndpointSpec{
				InterfaceName: "eth0",
				Ports: []api.EndpointPort{
					{
						Name:     "a_Jolly-port",
						Protocol: protoTCP,
						Port:     1234,
					},
				},
			},
			true,
		),
		Entry("should reject WorkloadEndpointSpec with an unnamed port (m)",
			api.WorkloadEndpointSpec{
				InterfaceName: "eth0",
				Ports: []api.EndpointPort{
					{
						Protocol: protoTCP,
						Port:     1234,
					},
				},
			},
			false,
		),
		Entry("should accept WorkloadEndpointSpec with name-clashing ports (m)",
			api.WorkloadEndpointSpec{
				InterfaceName: "eth0",
				Ports: []api.EndpointPort{
					{
						Name:     "a_Jolly-port",
						Protocol: protoTCP,
						Port:     1234,
					},
					{
						Name:     "a_Jolly-port",
						Protocol: protoUDP,
						Port:     5456,
					},
				},
			},
			true,
		),

		// (Backend model) HostEndpoint.
		Entry("should accept HostEndpoint with a port (m)",
			model.HostEndpoint{
				Ports: []model.EndpointPort{
					{
						Name:     "a_Jolly-port",
						Protocol: protoTCP,
						Port:     1234,
					},
				},
			},
			true,
		),
		Entry("should reject HostEndpoint with an unnamed port (m)",
			model.HostEndpoint{
				Ports: []model.EndpointPort{
					{
						Protocol: protoTCP,
						Port:     1234,
					},
				},
			},
			false,
		),
		Entry("should accept HostEndpoint with name-clashing ports (m)",
			model.HostEndpoint{
				Ports: []model.EndpointPort{
					{
						Name:     "a_Jolly-port",
						Protocol: protoTCP,
						Port:     1234,
					},
					{
						Name:     "a_Jolly-port",
						Protocol: protoUDP,
						Port:     5456,
					},
				},
			},
			true,
		),

		// (API) HostEndpointSpec.
		Entry("should accept HostEndpointSpec with a port (m)",
			api.HostEndpointSpec{
				InterfaceName: "eth0",
				Ports: []api.EndpointPort{
					{
						Name:     "a_Jolly-port",
						Protocol: protoTCP,
						Port:     1234,
					},
				},
			},
			true,
		),
		Entry("should reject HostEndpointSpec with an unnamed port (m)",
			api.HostEndpointSpec{
				InterfaceName: "eth0",
				Ports: []api.EndpointPort{
					{
						Protocol: protoTCP,
						Port:     1234,
					},
				},
			},
			false,
		),
		Entry("should accept HostEndpointSpec with name-clashing ports (m)",
			api.HostEndpointSpec{
				InterfaceName: "eth0",
				Ports: []api.EndpointPort{
					{
						Name:     "a_Jolly-port",
						Protocol: protoTCP,
						Port:     1234,
					},
					{
						Name:     "a_Jolly-port",
						Protocol: protoUDP,
						Port:     5456,
					},
				},
			},
			true,
		),

		Entry("should accept a valid BGP logging level: Info", api.BGPConfigurationSpec{LogSeverityScreen: "Info"}, true),
		Entry("should reject an invalid BGP logging level: info", api.BGPConfigurationSpec{LogSeverityScreen: "info"}, false),
		Entry("should reject an invalid BGP logging level: INFO", api.BGPConfigurationSpec{LogSeverityScreen: "INFO"}, false),
		Entry("should reject an invalid BGP logging level: invalidLvl", api.BGPConfigurationSpec{LogSeverityScreen: "invalidLvl"}, false),

		// (API) IP version.
		Entry("should accept IP version 4", api.Rule{Action: "allow", IPVersion: &V4}, true),
		Entry("should accept IP version 6", api.Rule{Action: "allow", IPVersion: &V6}, true),
		Entry("should reject IP version 0", api.Rule{Action: "allow", IPVersion: &V0}, false),

		// (API) Names.
		//Entry("should accept a valid name", api.Profile{ObjectMeta: v1.ObjectMeta{Name: ".My-valid-Profile_190"}}, true),
		//Entry("should reject ! in a name", api.Profile{ObjectMeta: v1.ObjectMeta{Name: "my!nvalid-Profile"}}, false),
		//Entry("should reject $ in a name", api.Profile{ObjectMeta: v1.ObjectMeta{Name: "my-invalid-profile$"}}, false),

		// (API) Selectors.  Selectors themselves are thoroughly UT'd so only need to test simple
		// accept and reject cases here.
		Entry("should accept valid selector", api.EntityRule{Selector: "foo == \"bar\""}, true),
		Entry("should accept valid selector with 'has' and a '/'", api.EntityRule{Selector: "has(calico/k8s_ns)"}, true),
		Entry("should accept valid selector with 'has' and two '/'", api.EntityRule{Selector: "has(calico/k8s_ns/role)"}, true),
		Entry("should accept valid selector with 'has' and two '/' and '-.'", api.EntityRule{Selector: "has(calico/k8s_NS-.1/role)"}, true),
		Entry("should reject invalid selector", api.EntityRule{Selector: "thing=hello &"}, false),

		// (API) Tags.
		Entry("should accept a valid labelsToApply", api.ProfileSpec{LabelsToApply: map[string]string{".My-valid-label_420": ""}}, true),
		Entry("should reject ! in a labelsToApply", api.ProfileSpec{LabelsToApply: map[string]string{"my!nvalid-label": ""}}, false),
		Entry("should reject $ in a labelsToApply", api.ProfileSpec{LabelsToApply: map[string]string{"my-invalid-label$": ""}}, false),

		// (API) Labels.
		//Entry("should accept a valid label", api.HostEndpointMetadata{Labels: map[string]string{"rank_.0-9": "gold._0-9"}}, true),
		//Entry("should accept label key starting with 0-9", api.HostEndpointMetadata{Labels: map[string]string{"2rank": "gold"}}, true),
		//Entry("should accept label value starting with 0-9", api.HostEndpointMetadata{Labels: map[string]string{"rank": "2gold"}}, true),
		//Entry("should accept label key with dns prefix", api.HostEndpointMetadata{Labels: map[string]string{"calico/k8s_ns": "kube-system"}}, true),
		//Entry("should accept label key where prefix is 253 characters", api.HostEndpointMetadata{Labels: map[string]string{"projectcalico.org.projectcalico.org.projectcalico.org.projectcalico.org.projectcalico.org.projectcalico.org.projectcalico.org.projectcalico.org.projectcalico.org.projectcalico.org.projectcalico.org.projectcalico.org.projectcalico.org.calico12345/k8s_ns": "gold"}}, true),
		//Entry("should accept label key where prefix begins with an uppercase character", api.HostEndpointMetadata{Labels: map[string]string{"Projectcalico.org12345/k8s_ns": "gold"}}, true),
		//Entry("should accept label key with multiple /", api.HostEndpointMetadata{Labels: map[string]string{"k8s_ns/label/role": "gold"}}, true),
		//Entry("should accept label key with - and .", api.HostEndpointMetadata{Labels: map[string]string{"k8s_ns/label-ro.le": "gold"}}, true),
		//Entry("should reject label key with !", api.HostEndpointMetadata{Labels: map[string]string{"rank!": "gold"}}, false),
		//Entry("should reject label key starting with ~", api.HostEndpointMetadata{Labels: map[string]string{"~rank_.0-9": "gold"}}, false),
		//Entry("should reject label key ending with ~", api.HostEndpointMetadata{Labels: map[string]string{"rank_.0-9~": "gold"}}, false),
		//Entry("should reject label value starting with ~", api.HostEndpointMetadata{Labels: map[string]string{"rank_.0-9": "~gold"}}, false),
		//Entry("should reject label value ending with ~", api.HostEndpointMetadata{Labels: map[string]string{"rank_.0-9": "gold~"}}, false),

		// (API) Interface.
		Entry("should accept a valid interface", api.WorkloadEndpointSpec{InterfaceName: "ValidIntface0-9"}, true),
		Entry("should reject an interface that is too long", api.WorkloadEndpointSpec{InterfaceName: "interfaceTooLong"}, false),
		Entry("should reject & in an interface", api.WorkloadEndpointSpec{InterfaceName: "Invalid&Intface"}, false),
		Entry("should reject # in an interface", api.WorkloadEndpointSpec{InterfaceName: "Invalid#Intface"}, false),
		Entry("should reject . in an interface", api.WorkloadEndpointSpec{InterfaceName: "Invalid.Intface"}, false),
		Entry("should reject : in an interface", api.WorkloadEndpointSpec{InterfaceName: "Invalid:Intface"}, false),

		// (API) FelixConfiguration.
		Entry("should accept a valid DefaultEndpointToHostAction value", api.FelixConfigurationSpec{DefaultEndpointToHostAction: "Drop"}, true),
		Entry("should reject an invalid DefaultEndpointToHostAction value 'drop' (lower case)", api.FelixConfigurationSpec{DefaultEndpointToHostAction: "drop"}, false),
		Entry("should accept a valid IptablesFilterAllowAction value 'Accept'", api.FelixConfigurationSpec{IptablesFilterAllowAction: "Accept"}, true),
		Entry("should accept a valid IptablesMangleAllowAction value 'Return'", api.FelixConfigurationSpec{IptablesMangleAllowAction: "Return"}, true),
		Entry("should reject an invalid IptablesMangleAllowAction value 'Drop'", api.FelixConfigurationSpec{IptablesMangleAllowAction: "Drop"}, false),

		Entry("should reject an invalid LogSeverityScreen value 'badVal'", api.FelixConfigurationSpec{LogSeverityScreen: "badVal"}, false),
		Entry("should reject an invalid LogSeverityFile value 'badVal'", api.FelixConfigurationSpec{LogSeverityFile: "badVal"}, false),
		Entry("should reject an invalid LogSeveritySys value 'badVal'", api.FelixConfigurationSpec{LogSeveritySys: "badVal"}, false),
		Entry("should accept a valid LogSeverityScreen value 'Warning'", api.FelixConfigurationSpec{LogSeverityScreen: "Warning"}, true),
		Entry("should accept a valid LogSeverityFile value 'Debug'", api.FelixConfigurationSpec{LogSeverityFile: "Debug"}, true),
		Entry("should accept a valid LogSeveritySys value 'Info'", api.FelixConfigurationSpec{LogSeveritySys: "Info"}, true),

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
				IPNetworks:    []string{netv4_1, netv4_2, netv6_1, netv6_2},
			}, true),
		Entry("should accept workload endpoint with IPv4 NAT covered by network",
			api.WorkloadEndpointSpec{
				InterfaceName: "cali012371237",
				IPNetworks:    []string{netv4_1},
				IPNATs:        []api.IPNAT{{InternalIP: ipv4_1, ExternalIP: ipv4_2}},
			}, true),
		Entry("should accept workload endpoint with IPv6 NAT covered by network",
			api.WorkloadEndpointSpec{
				InterfaceName: "cali012371237",
				IPNetworks:    []string{netv6_1},
				IPNATs:        []api.IPNAT{{InternalIP: ipv6_1, ExternalIP: ipv6_2}},
			}, true),
		Entry("should accept workload endpoint with IPv4 and IPv6 NAT covered by network",
			api.WorkloadEndpointSpec{
				InterfaceName: "cali012371237",
				IPNetworks:    []string{netv4_1, netv6_1},
				IPNATs: []api.IPNAT{
					{InternalIP: ipv4_1, ExternalIP: ipv4_2},
					{InternalIP: ipv6_1, ExternalIP: ipv6_2},
				},
			}, true),
		Entry("should reject workload endpoint with no config", api.WorkloadEndpointSpec{}, false),
		Entry("should reject workload endpoint with IPv4 networks that contain >1 address",
			api.WorkloadEndpointSpec{
				InterfaceName: "cali012371237",
				IPNetworks:    []string{netv4_3},
			}, false),
		Entry("should reject workload endpoint with IPv6 networks that contain >1 address",
			api.WorkloadEndpointSpec{
				InterfaceName: "cali012371237",
				IPNetworks:    []string{netv6_3},
			}, false),
		Entry("should reject workload endpoint with nats and no networks",
			api.WorkloadEndpointSpec{
				InterfaceName: "cali012371237",
				IPNATs:        []api.IPNAT{{InternalIP: ipv4_2, ExternalIP: ipv4_1}},
			}, false),
		Entry("should reject workload endpoint with IPv4 NAT not covered by network",
			api.WorkloadEndpointSpec{
				InterfaceName: "cali012371237",
				IPNetworks:    []string{netv4_1},
				IPNATs:        []api.IPNAT{{InternalIP: ipv4_2, ExternalIP: ipv4_1}},
			}, false),
		Entry("should reject workload endpoint with IPv6 NAT not covered by network",
			api.WorkloadEndpointSpec{
				InterfaceName: "cali012371237",
				IPNetworks:    []string{netv6_1},
				IPNATs:        []api.IPNAT{{InternalIP: ipv6_2, ExternalIP: ipv6_1}},
			}, false),

		// (API) HostEndpointSpec
		Entry("should accept host endpoint with interface",
			api.HostEndpointSpec{
				InterfaceName: "eth0",
			}, true),
		Entry("should accept host endpoint with expected IPs",
			api.HostEndpointSpec{
				ExpectedIPs: []string{ipv4_1, ipv6_1},
			}, true),
		Entry("should accept host endpoint with interface and expected IPs",
			api.HostEndpointSpec{
				InterfaceName: "eth0",
				ExpectedIPs:   []string{ipv4_1, ipv6_1},
			}, true),
		Entry("should reject host endpoint with no config", api.HostEndpointSpec{}, false),
		Entry("should reject host endpoint with blank interface an no IPs",
			api.HostEndpointSpec{
				InterfaceName: "",
				ExpectedIPs:   []string{},
			}, false),

		// (API) IPPool
		Entry("should accept IP pool with IPv4 CIDR /26",
			api.IPPool{ObjectMeta: v1.ObjectMeta{Name: "poolyMcPoolface"},
				Spec: api.IPPoolSpec{CIDR: netv4_3},
			}, true),
		Entry("should accept IP pool with IPv4 CIDR /10",
			api.IPPool{ObjectMeta: v1.ObjectMeta{Name: "poolyMcPoolface"},
				Spec: api.IPPoolSpec{CIDR: netv4_4},
			}, true),
		Entry("should accept IP pool with IPv6 CIDR /122",
			api.IPPool{ObjectMeta: v1.ObjectMeta{Name: "poolyMcPoolface"},
				Spec: api.IPPoolSpec{
					CIDR:     netv6_3,
					IPIPMode: api.IPIPModeNever},
			}, true),
		Entry("should accept IP pool with IPv6 CIDR /10",
			api.IPPool{ObjectMeta: v1.ObjectMeta{Name: "poolyMcPoolface"},
				Spec: api.IPPoolSpec{
					CIDR:     netv6_4,
					IPIPMode: api.IPIPModeNever},
			}, true),
		Entry("should accept a disabled IP pool with IPv4 CIDR /27",
			api.IPPool{
				ObjectMeta: v1.ObjectMeta{Name: "poolyMcPoolface"},
				Spec: api.IPPoolSpec{
					CIDR:     netv4_5,
					Disabled: true},
			}, true),
		Entry("should accept a disabled IP pool with IPv6 CIDR /128",
			api.IPPool{
				ObjectMeta: v1.ObjectMeta{Name: "poolyMcPoolface"},
				Spec: api.IPPoolSpec{
					CIDR:     netv6_1,
					IPIPMode: api.IPIPModeNever,
					Disabled: true},
			}, true),
		Entry("should reject IP pool with IPv4 CIDR /27", api.IPPool{ObjectMeta: v1.ObjectMeta{Name: "poolyMcPoolface"}, Spec: api.IPPoolSpec{CIDR: netv4_5}}, false),
		Entry("should reject IP pool with IPv6 CIDR /128", api.IPPool{ObjectMeta: v1.ObjectMeta{Name: "poolyMcPoolface"}, Spec: api.IPPoolSpec{CIDR: netv6_1}}, false),
		Entry("should reject IPIPMode 'Always' for IPv6 pool",
			api.IPPool{
				ObjectMeta: v1.ObjectMeta{Name: "poolyMcPoolface"},
				Spec: api.IPPoolSpec{
					CIDR:     netv6_1,
					IPIPMode: api.IPIPModeAlways},
			}, false),
		Entry("should reject IPv4 pool with a CIDR range overlapping with Link Local range",
			api.IPPool{ObjectMeta: v1.ObjectMeta{Name: "poolyMcPoolface"}, Spec: api.IPPoolSpec{CIDR: "169.254.5.0/24"}}, false),
		Entry("should reject IPv6 pool with a CIDR range overlapping with Link Local range",
			api.IPPool{ObjectMeta: v1.ObjectMeta{Name: "poolyMcPoolface"}, Spec: api.IPPoolSpec{CIDR: "fe80::/120"}}, false),

		// (API) IPIPConfiguration
		Entry("should accept IPPool with no IPIP mode specified", api.IPPoolSpec{}, true),
		Entry("should accept IPIP mode Never (api)", api.IPPoolSpec{IPIPMode: api.IPIPModeNever}, true),
		Entry("should accept IPIP mode Never", api.IPPoolSpec{IPIPMode: "Never"}, true),
		Entry("should accept IPIP mode Always", api.IPPoolSpec{IPIPMode: "Always"}, true),
		Entry("should accept IPIP mode CrossSubnet", api.IPPoolSpec{IPIPMode: "CrossSubnet"}, true),
		Entry("should reject IPIP mode badVal", api.IPPoolSpec{IPIPMode: "badVal"}, false),
		Entry("should reject IPIP mode never (lower case)", api.IPPoolSpec{IPIPMode: "never"}, false),

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
		Entry("should accept Rule with source named ports and protocol type 6",
			api.Rule{
				Action:   "allow",
				Protocol: protocolFromInt(6),
				Source: api.EntityRule{
					Ports: []numorstring.Port{numorstring.NamedPort("foo")},
				},
			}, true),
		Entry("should accept Rule with source named ports and protocol type tcp",
			api.Rule{
				Action:   "allow",
				Protocol: protocolFromString("tcp"),
				Source: api.EntityRule{
					Ports: []numorstring.Port{numorstring.NamedPort("foo")},
				},
			}, true),
		Entry("should accept Rule with source named ports and protocol type udp",
			api.Rule{
				Action:   "allow",
				Protocol: protocolFromString("udp"),
				Source: api.EntityRule{
					Ports: []numorstring.Port{numorstring.NamedPort("foo")},
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
		Entry("should reject Rule with dest ports and no protocol",
			api.Rule{
				Action: "allow",
				Destination: api.EntityRule{
					Ports: []numorstring.Port{numorstring.SinglePort(1)},
				},
			}, false),
		Entry("should reject Rule with invalid port (port 0)",
			api.Rule{
				Action:   "allow",
				Protocol: protocolFromString("tcp"),
				Destination: api.EntityRule{
					NotPorts: []numorstring.Port{numorstring.SinglePort(0)},
				},
			}, false),
		Entry("should reject Rule with invalid port (name + number)",
			api.Rule{
				Action:   "allow",
				Protocol: protocolFromString("tcp"),
				Destination: api.EntityRule{
					NotPorts: []numorstring.Port{{
						PortName: "foo",
						MinPort:  123,
						MaxPort:  456,
					}},
				},
			}, false),
		Entry("should reject named port Rule with invalid protocol",
			api.Rule{
				Action:   "allow",
				Protocol: protocolFromString("unknown"),
				Destination: api.EntityRule{
					NotPorts: []numorstring.Port{numorstring.NamedPort("foo")},
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
					Ports: []numorstring.Port{{MinPort: 200, MaxPort: 100}},
				},
			}, false),
		Entry("should reject Rule with invalid source !ports and protocol type tcp",
			api.Rule{
				Action:   "allow",
				Protocol: protocolFromString("tcp"),
				Source: api.EntityRule{
					NotPorts: []numorstring.Port{{MinPort: 200, MaxPort: 100}},
				},
			}, false),
		Entry("should reject Rule with invalid dest ports and protocol type tcp",
			api.Rule{
				Action:   "allow",
				Protocol: protocolFromString("tcp"),
				Destination: api.EntityRule{
					Ports: []numorstring.Port{{MinPort: 200, MaxPort: 100}},
				},
			}, false),
		Entry("should reject Rule with invalid dest !ports and protocol type tcp",
			api.Rule{
				Action:   "allow",
				Protocol: protocolFromString("tcp"),
				Destination: api.EntityRule{
					NotPorts: []numorstring.Port{{MinPort: 200, MaxPort: 100}},
				},
			}, false),
		Entry("should reject Rule with one invalid port in the port range (MinPort 0)",
			api.Rule{
				Action:   "allow",
				Protocol: protocolFromString("tcp"),
				Destination: api.EntityRule{
					NotPorts: []numorstring.Port{{MinPort: 0, MaxPort: 100}},
				},
			}, false),
		Entry("should reject rule mixed IPv4 (src) and IPv6 (dest)",
			api.Rule{
				Action:   "allow",
				Protocol: protocolFromString("tcp"),
				Source: api.EntityRule{
					Nets: []string{netv4_3},
				},
				Destination: api.EntityRule{
					Nets: []string{netv6_3},
				},
			}, false),
		Entry("should reject rule mixed IPv6 (src) and IPv4 (dest)",
			api.Rule{
				Action:   "allow",
				Protocol: protocolFromString("tcp"),
				Source: api.EntityRule{
					Nets: []string{netv6_2},
				},
				Destination: api.EntityRule{
					Nets: []string{netv4_2},
				},
			}, false),
		Entry("should reject rule mixed IPv6 version and IPv4 Net",
			api.Rule{
				Action:    "allow",
				Protocol:  protocolFromString("tcp"),
				IPVersion: &V6,
				Source: api.EntityRule{
					Nets: []string{netv4_4},
				},
				Destination: api.EntityRule{
					Nets: []string{netv4_2},
				},
			}, false),
		Entry("should reject rule mixed IPVersion and Source Net IP version",
			api.Rule{
				Action:    "allow",
				Protocol:  protocolFromString("tcp"),
				IPVersion: &V6,
				Source: api.EntityRule{
					Nets: []string{netv4_1},
				},
			}, false),
		Entry("should reject rule mixed IPVersion and Dest Net IP version",
			api.Rule{
				Action:    "allow",
				Protocol:  protocolFromString("tcp"),
				IPVersion: &V4,
				Destination: api.EntityRule{
					Nets: []string{netv6_1},
				},
			}, false),
		Entry("net list: should reject rule mixed IPv4 (src) and IPv6 (dest)",
			api.Rule{
				Action:   "allow",
				Protocol: protocolFromString("tcp"),
				Source: api.EntityRule{
					Nets: []string{netv4_3},
				},
				Destination: api.EntityRule{
					Nets: []string{netv6_3},
				},
			}, false),
		Entry("net list: should reject rule mixed IPv6 (src) and IPv4 (dest)",
			api.Rule{
				Action:   "allow",
				Protocol: protocolFromString("tcp"),
				Source: api.EntityRule{
					Nets: []string{netv6_2},
				},
				Destination: api.EntityRule{
					Nets: []string{netv4_2},
				},
			}, false),
		Entry("net list: should reject rule mixed IPv6 version and IPv4 Net",
			api.Rule{
				Action:    "allow",
				Protocol:  protocolFromString("tcp"),
				IPVersion: &V6,
				Source: api.EntityRule{
					Nets: []string{netv4_4},
				},
				Destination: api.EntityRule{
					Nets: []string{netv4_2},
				},
			}, false),
		Entry("net list: should reject rule mixed IPv6 version and IPv4 Net",
			api.Rule{
				Action:    "allow",
				Protocol:  protocolFromString("tcp"),
				IPVersion: &V6,
				Source: api.EntityRule{
					Nets: []string{netv4_4},
				},
				Destination: api.EntityRule{
					NotNets: []string{netv4_2},
				},
			}, false),
		Entry("net list: should reject rule mixed IPVersion and Source Net IP version",
			api.Rule{
				Action:    "allow",
				Protocol:  protocolFromString("tcp"),
				IPVersion: &V6,
				Source: api.EntityRule{
					Nets: []string{netv4_1},
				},
			}, false),
		Entry("net list: should reject rule mixed IPVersion and Dest Net IP version",
			api.Rule{
				Action:    "allow",
				Protocol:  protocolFromString("tcp"),
				IPVersion: &V4,
				Destination: api.EntityRule{
					Nets: []string{netv6_1},
				},
			}, false),

		// (API) NodeSpec
		Entry("should accept node with IPv4 BGP", api.NodeSpec{BGP: &api.NodeBGPSpec{IPv4Address: netv4_1}}, true),
		Entry("should accept node with IPv6 BGP", api.NodeSpec{BGP: &api.NodeBGPSpec{IPv6Address: netv6_1}}, true),
		Entry("should accept node with no BGP", api.NodeSpec{}, true),
		Entry("should reject node with BGP but no IPs", api.NodeSpec{BGP: &api.NodeBGPSpec{}}, false),
		Entry("should reject node with IPv6 address in IPv4 field", api.NodeSpec{BGP: &api.NodeBGPSpec{IPv4Address: netv6_1}}, false),
		Entry("should reject node with IPv4 address in IPv6 field", api.NodeSpec{BGP: &api.NodeBGPSpec{IPv6Address: netv4_1}}, false),
		Entry("should reject GlobalNetworkPolicy with both PreDNAT and DoNotTrack",
			api.GlobalNetworkPolicySpec{
				PreDNAT:        true,
				DoNotTrack:     true,
				ApplyOnForward: true,
			}, false),
		Entry("should accept GlobalNetworkPolicy with PreDNAT but not DoNotTrack",
			api.GlobalNetworkPolicySpec{
				PreDNAT:        true,
				ApplyOnForward: true,
			}, true),
		Entry("should accept Policy with DoNotTrack but not PreDNAT",
			api.GlobalNetworkPolicySpec{
				PreDNAT:        false,
				DoNotTrack:     true,
				ApplyOnForward: true,
			}, true),
		Entry("should reject pre-DNAT GlobalNetworkPolicy with egress rules",
			api.GlobalNetworkPolicySpec{
				PreDNAT:        true,
				ApplyOnForward: true,
				EgressRules:    []api.Rule{{Action: "allow"}},
			}, false),
		Entry("should accept pre-DNAT GlobalNetworkPolicy with ingress rules",
			api.GlobalNetworkPolicySpec{
				PreDNAT:        true,
				ApplyOnForward: true,
				IngressRules:   []api.Rule{{Action: "allow"}},
			}, true),

		// PolicySpec ApplyOnForward field checks.
		Entry("should accept GlobalNetworkPolicy with ApplyOnForward but not PreDNAT",
			api.GlobalNetworkPolicySpec{
				PreDNAT:        false,
				ApplyOnForward: true,
			}, true),
		Entry("should accept GlobalNetworkPolicy with ApplyOnForward but not DoNotTrack",
			api.GlobalNetworkPolicySpec{
				DoNotTrack:     false,
				ApplyOnForward: true,
			}, true),
		Entry("should accept GlobalNetworkPolicy with ApplyOnForward and PreDNAT",
			api.GlobalNetworkPolicySpec{
				PreDNAT:        true,
				ApplyOnForward: true,
			}, true),
		Entry("should accept GlobalNetworkPolicy with ApplyOnForward and DoNotTrack",
			api.GlobalNetworkPolicySpec{
				DoNotTrack:     true,
				ApplyOnForward: true,
			}, true),
		Entry("should accept GlobalNetworkPolicy with no ApplyOnForward DoNotTrack PreDNAT",
			api.GlobalNetworkPolicySpec{
				PreDNAT:        false,
				DoNotTrack:     false,
				ApplyOnForward: false,
			}, true),
		Entry("should reject GlobalNetworkPolicy with PreDNAT but not ApplyOnForward",
			api.GlobalNetworkPolicySpec{
				PreDNAT:        true,
				ApplyOnForward: false,
			}, false),
		Entry("should reject GlobalNetworkPolicy with DoNotTrack but not ApplyOnForward",
			api.GlobalNetworkPolicySpec{
				DoNotTrack:     true,
				ApplyOnForward: false,
			}, false),

		// GlobalNetworkPolicySpec Types field checks.
		Entry("allow missing Types", api.GlobalNetworkPolicySpec{}, true),
		Entry("allow empty Types", api.GlobalNetworkPolicySpec{Types: []api.PolicyType{}}, true),
		Entry("allow ingress Types", api.GlobalNetworkPolicySpec{Types: []api.PolicyType{api.PolicyTypeIngress}}, true),
		Entry("allow egress Types", api.GlobalNetworkPolicySpec{Types: []api.PolicyType{api.PolicyTypeEgress}}, true),
		Entry("allow ingress+egress Types", api.GlobalNetworkPolicySpec{Types: []api.PolicyType{api.PolicyTypeIngress, api.PolicyTypeEgress}}, true),
		Entry("disallow repeated egress Types", api.GlobalNetworkPolicySpec{Types: []api.PolicyType{api.PolicyTypeEgress, api.PolicyTypeEgress}}, false),
		Entry("disallow unexpected value", api.GlobalNetworkPolicySpec{Types: []api.PolicyType{"unexpected"}}, false),

		// NetworkPolicySpec Types field checks.
		Entry("allow missing Types", api.NetworkPolicySpec{}, true),
		Entry("allow empty Types", api.NetworkPolicySpec{Types: []api.PolicyType{}}, true),
		Entry("allow ingress Types", api.NetworkPolicySpec{Types: []api.PolicyType{api.PolicyTypeIngress}}, true),
		Entry("allow egress Types", api.NetworkPolicySpec{Types: []api.PolicyType{api.PolicyTypeEgress}}, true),
		Entry("allow ingress+egress Types", api.NetworkPolicySpec{Types: []api.PolicyType{api.PolicyTypeIngress, api.PolicyTypeEgress}}, true),
		Entry("disallow repeated egress Types", api.NetworkPolicySpec{Types: []api.PolicyType{api.PolicyTypeEgress, api.PolicyTypeEgress}}, false),
		Entry("disallow unexpected value", api.NetworkPolicySpec{Types: []api.PolicyType{"unexpected"}}, false),

		// In the initial implementation, we validated against the following two cases but we found
		// that prevented us from doing a smooth upgrade from type-less to typed policy since we
		// couldn't write a policy that would work for back-level Felix instances while also
		// specifying the type for up-level Felix instances.
		//
		// For NetworkPolicySpec
		Entry("allow Types without ingress when IngressRules present",
			api.NetworkPolicySpec{
				IngressRules: []api.Rule{{Action: "allow"}},
				Types:        []api.PolicyType{api.PolicyTypeEgress},
			}, true),
		Entry("allow Types without egress when EgressRules present",
			api.NetworkPolicySpec{
				EgressRules: []api.Rule{{Action: "allow"}},
				Types:       []api.PolicyType{api.PolicyTypeIngress},
			}, true),

		Entry("allow Types with ingress when IngressRules present",
			api.NetworkPolicySpec{
				IngressRules: []api.Rule{{Action: "allow"}},
				Types:        []api.PolicyType{api.PolicyTypeIngress},
			}, true),
		Entry("allow Types with ingress+egress when IngressRules present",
			api.NetworkPolicySpec{
				IngressRules: []api.Rule{{Action: "allow"}},
				Types:        []api.PolicyType{api.PolicyTypeIngress, api.PolicyTypeEgress},
			}, true),
		Entry("allow Types with egress when EgressRules present",
			api.NetworkPolicySpec{
				EgressRules: []api.Rule{{Action: "allow"}},
				Types:       []api.PolicyType{api.PolicyTypeEgress},
			}, true),
		Entry("allow Types with ingress+egress when EgressRules present",
			api.NetworkPolicySpec{
				EgressRules: []api.Rule{{Action: "allow"}},
				Types:       []api.PolicyType{api.PolicyTypeIngress, api.PolicyTypeEgress},
			}, true),

		// For GlobalNetworkPolicySpec
		Entry("allow Types without ingress when IngressRules present (gnp)",
			api.GlobalNetworkPolicySpec{
				IngressRules: []api.Rule{{Action: "allow"}},
				Types:        []api.PolicyType{api.PolicyTypeEgress},
			}, true),
		Entry("allow Types without egress when EgressRules present (gnp)",
			api.GlobalNetworkPolicySpec{
				EgressRules: []api.Rule{{Action: "allow"}},
				Types:       []api.PolicyType{api.PolicyTypeIngress},
			}, true),

		Entry("allow Types with ingress when IngressRules present (gnp)",
			api.GlobalNetworkPolicySpec{
				IngressRules: []api.Rule{{Action: "allow"}},
				Types:        []api.PolicyType{api.PolicyTypeIngress},
			}, true),
		Entry("allow Types with ingress+egress when IngressRules present (gnp)",
			api.GlobalNetworkPolicySpec{
				IngressRules: []api.Rule{{Action: "allow"}},
				Types:        []api.PolicyType{api.PolicyTypeIngress, api.PolicyTypeEgress},
			}, true),
		Entry("allow Types with egress when EgressRules present (gnp)",
			api.GlobalNetworkPolicySpec{
				EgressRules: []api.Rule{{Action: "allow"}},
				Types:       []api.PolicyType{api.PolicyTypeEgress},
			}, true),
		Entry("allow Types with ingress+egress when EgressRules present (gnp)",
			api.GlobalNetworkPolicySpec{
				EgressRules: []api.Rule{{Action: "allow"}},
				Types:       []api.PolicyType{api.PolicyTypeIngress, api.PolicyTypeEgress},
			}, true),
		Entry("allow ingress Types with pre-DNAT (gnp)",
			api.GlobalNetworkPolicySpec{
				PreDNAT:        true,
				ApplyOnForward: true,
				Types:          []api.PolicyType{api.PolicyTypeIngress},
			}, true),
		Entry("disallow egress Types with pre-DNAT (gnp)",
			api.GlobalNetworkPolicySpec{
				PreDNAT:        true,
				ApplyOnForward: true,
				Types:          []api.PolicyType{api.PolicyTypeEgress},
			}, false),
		Entry("disallow ingress+egress Types with pre-DNAT (gnp)",
			api.GlobalNetworkPolicySpec{
				PreDNAT:        true,
				ApplyOnForward: true,
				Types:          []api.PolicyType{api.PolicyTypeIngress, api.PolicyTypeEgress},
			}, false),
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
