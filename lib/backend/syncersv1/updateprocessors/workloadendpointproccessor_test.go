// Copyright (c) 2017 Tigera, Inc. All rights reserved.

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

package updateprocessors_test

import (
	"net"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/projectcalico/libcalico-go/lib/apiv2"
	"github.com/projectcalico/libcalico-go/lib/backend/model"
	"github.com/projectcalico/libcalico-go/lib/backend/syncersv1/updateprocessors"
	cnet "github.com/projectcalico/libcalico-go/lib/net"
	"github.com/projectcalico/libcalico-go/lib/numorstring"
)

var _ = Describe("Test the WorkloadEndpoint update processor", func() {
	hn1 := "host1"
	hn2 := "host2"
	oid1 := "orchestrator1"
	oid2 := "orchestrator2"
	wid1 := "workload1"
	wid2 := "workload2"
	eid1 := "endpoint1"
	eid2 := "endpoint2"
	ns1 := "namespace1"
	ns2 := "namespace2"
	name1 := "host1-orchestrator1-workload1-endpoint1"
	name2 := "host2-orchestrator2-workload2-endpoint2"
	iface1 := "iface1"
	iface2 := "iface2"

	v2WorkloadEndpointKey1 := model.ResourceKey{
		Kind: apiv2.KindWorkloadEndpoint,
		Name: name1,
	}
	v2WorkloadEndpointKey2 := model.ResourceKey{
		Kind: apiv2.KindWorkloadEndpoint,
		Name: name2,
	}
	v1WorkloadEndpointKey1 := model.WorkloadEndpointKey{
		Hostname:       hn1,
		OrchestratorID: oid1,
		WorkloadID:     ns1 + "/" + wid1,
		EndpointID:     eid1,
	}
	v1WorkloadEndpointKey2 := model.WorkloadEndpointKey{
		Hostname:       hn2,
		OrchestratorID: oid2,
		WorkloadID:     ns2 + "/" + wid2,
		EndpointID:     eid2,
	}

	It("should handle conversion of valid WorkloadEndpoints", func() {
		netmac2, err := net.ParseMAC("01:23:45:67:89:ab")
		Expect(err).NotTo(HaveOccurred())
		mac2 := cnet.MAC{netmac2}

		up := updateprocessors.NewWorkloadEndpointUpdateProcessor()

		By("converting a WorkloadEndpoint with minimum configuration")
		res := apiv2.NewWorkloadEndpoint()
		res.Name = name1
		res.Namespace = ns1
		res.Spec.InterfaceName = iface1

		kvps, err := up.Process(&model.KVPair{
			Key:      v2WorkloadEndpointKey1,
			Value:    res,
			Revision: "abcde",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(kvps).To(HaveLen(1))
		Expect(kvps[0]).To(Equal(&model.KVPair{
			Key: v1WorkloadEndpointKey1,
			Value: &model.WorkloadEndpoint{
				State: "active",
				Name:  iface1,
				Ports: []model.EndpointPort{},
			},
			Revision: "abcde",
		}))

		By("adding another WorkloadEndpoint with a full configuration")
		res = apiv2.NewWorkloadEndpoint()
		res.Name = name2
		res.Namespace = ns2
		res.Labels = map[string]string{"testLabel": "label"}
		res.Spec.InterfaceName = iface2
		res.Spec.ContainerID = "container2"
		res.Spec.MAC = "01:23:45:67:89:ab"
		res.Spec.Profiles = []string{"testProfile"}
		res.Spec.IPNetworks = []string{"10.100.10.1"}
		_, ipn, err := cnet.ParseCIDROrIP("10.100.10.1")
		Expect(err).NotTo(HaveOccurred())
		expectedIPv4Net := *(ipn.Network())
		res.Spec.IPNATs = []apiv2.IPNAT{
			apiv2.IPNAT{
				InternalIP: "10.100.1.1",
				ExternalIP: "10.1.10.1",
			},
		}
		expectedIPv4NAT := *updateprocessors.ConvertV2ToV1IPNAT(res.Spec.IPNATs[0])
		res.Spec.IPv4Gateway = "10.10.10.1"
		expectedIPv4Gateway, _, err := cnet.ParseCIDROrIP("10.10.10.1")
		res.Spec.IPv6Gateway = "2001:0db8:85a3:0000:0000:8a2e:0370:7334"
		expectedIPv6Gateway, _, err := cnet.ParseCIDROrIP("2001:0db8:85a3:0000:0000:8a2e:0370:7334")
		res.Spec.Ports = []apiv2.EndpointPort{
			apiv2.EndpointPort{
				Name:     "portname",
				Protocol: numorstring.ProtocolFromInt(uint8(30)),
				Port:     uint16(8080),
			},
		}

		kvps, err = up.Process(&model.KVPair{
			Key:      v2WorkloadEndpointKey2,
			Value:    res,
			Revision: "1234",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(kvps).To(Equal([]*model.KVPair{
			{
				Key: v1WorkloadEndpointKey2,
				Value: &model.WorkloadEndpoint{
					State:            "active",
					Name:             iface2,
					ActiveInstanceID: "container2",
					Mac:              &mac2,
					ProfileIDs:       []string{"testProfile"},
					IPv4Nets:         []cnet.IPNet{expectedIPv4Net},
					IPv4NAT:          []model.IPNAT{expectedIPv4NAT},
					Labels:           map[string]string{"testLabel": "label"},
					IPv4Gateway:      expectedIPv4Gateway,
					IPv6Gateway:      expectedIPv6Gateway,
					Ports: []model.EndpointPort{
						model.EndpointPort{
							Name:     "portname",
							Protocol: numorstring.ProtocolFromInt(uint8(30)),
							Port:     uint16(8080),
						},
					},
				},
				Revision: "1234",
			},
		}))

		By("attempting to delete the first workload endpoint")
		kvps, err = up.Process(&model.KVPair{
			Key: v2WorkloadEndpointKey1,
		})
		Expect(err).To(HaveOccurred())

		By("deleting the first workload endpoint")
		res = apiv2.NewWorkloadEndpoint()
		res.Name = name1
		res.Namespace = ns1
		res.Spec.Node = hn1
		res.Spec.Orchestrator = oid1
		res.Spec.Workload = wid1
		res.Spec.Endpoint = eid1

		kvps, err = up.Process(&model.KVPair{
			Key:   v2WorkloadEndpointKey2,
			Value: res,
		})
		Expect(err).NotTo(HaveOccurred())
		var nilVal *model.WorkloadEndpoint
		Expect(kvps).To(Equal([]*model.KVPair{
			{
				Key:   v1WorkloadEndpointKey1,
				Value: nilVal,
			},
		}))
	})

	It("should fail to convert an invalid resource", func() {
		up := updateprocessors.NewWorkloadEndpointUpdateProcessor()

		By("trying to convert with the wrong key type")
		res := apiv2.NewWorkloadEndpoint()
		res.Name = name1
		res.Namespace = ns1

		_, err := up.Process(&model.KVPair{
			Key: model.GlobalBGPPeerKey{
				PeerIP: cnet.MustParseIP("1.2.3.4"),
			},
			Value:    res,
			Revision: "abcde",
		})
		Expect(err).To(HaveOccurred())

		By("trying to convert with the wrong value type")
		wres := apiv2.NewHostEndpoint()

		_, err = up.Process(&model.KVPair{
			Key:      v2WorkloadEndpointKey1,
			Value:    wres,
			Revision: "abcde",
		})
		Expect(err).To(HaveOccurred())

		By("trying to convert without enough information to create a v1 key")
		eres := apiv2.NewWorkloadEndpoint()

		_, err = up.Process(&model.KVPair{
			Key:      v2WorkloadEndpointKey1,
			Value:    eres,
			Revision: "abcde",
		})
		Expect(err).To(HaveOccurred())
	})
})
