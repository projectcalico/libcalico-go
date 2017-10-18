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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	apiv2 "github.com/projectcalico/libcalico-go/lib/apis/v2"
	"github.com/projectcalico/libcalico-go/lib/backend/model"
	"github.com/projectcalico/libcalico-go/lib/backend/syncersv1/updateprocessors"
	"github.com/projectcalico/libcalico-go/lib/numorstring"
)

var _ = Describe("Test the (BGP) Node update processor", func() {
	v2NodeKey1 := model.ResourceKey{
		Kind: apiv2.KindNode,
		Name: "bgpnode1",
	}
	numBgpConfigs := 5
	up := updateprocessors.NewBGPNodeUpdateProcessor()

	BeforeEach(func() {
		up.OnSyncerStarting()
	})

	// The Node contains a bunch of v1 per-node BGP configuration - so we can simply use the
	// checkExpectedConfigs() function defined in the configurationprocessor_test to perform
	// our validation.  Note that it expects a node name of bgpnode1.
	It("should handle conversion of valid Nodes", func() {
		By("converting a zero-ed Node")
		res := apiv2.NewNode()
		res.Name = "bgpnode1"
		expected := map[string]interface{}{
			"ip_addr_v4": "",
			"ip_addr_v6": "",
			"network_v4": nil,
			"network_v6": nil,
			"as_num":     nil,
		}
		kvps, err := up.Process(&model.KVPair{
			Key:   v2NodeKey1,
			Value: res,
		})
		Expect(err).NotTo(HaveOccurred())
		checkExpectedConfigs(
			kvps,
			isNodeBgpConfig,
			numBgpConfigs,
			expected,
		)

		By("converting a zero-ed but non-nil BGPNodeSpec")
		res = apiv2.NewNode()
		res.Name = "bgpnode1"
		res.Spec.BGP = &apiv2.NodeBGPSpec{}
		kvps, err = up.Process(&model.KVPair{
			Key:   v2NodeKey1,
			Value: res,
		})
		Expect(err).NotTo(HaveOccurred())
		// same expected results as the fully zeroed struct.
		checkExpectedConfigs(
			kvps,
			isNodeBgpConfig,
			numBgpConfigs,
			expected,
		)

		By("converting a Node with an IPv4 (specified without the network) only - expect /32 net")
		res = apiv2.NewNode()
		res.Name = "bgpnode1"
		res.Spec.BGP = &apiv2.NodeBGPSpec{
			IPv4Address: "1.2.3.4",
		}
		expected = map[string]interface{}{
			"ip_addr_v4": "1.2.3.4",
			"ip_addr_v6": "",
			"network_v4": "1.2.3.4/32",
			"network_v6": nil,
			"as_num":     nil,
		}
		kvps, err = up.Process(&model.KVPair{
			Key:   v2NodeKey1,
			Value: res,
		})
		Expect(err).NotTo(HaveOccurred())
		checkExpectedConfigs(
			kvps,
			isNodeBgpConfig,
			numBgpConfigs,
			expected,
		)

		By("converting a Node with an IPv6 (specified without the network) only - expect /128 net")
		res = apiv2.NewNode()
		res.Name = "bgpnode1"
		res.Spec.BGP = &apiv2.NodeBGPSpec{
			IPv6Address: "aa:bb:cc::",
		}
		expected = map[string]interface{}{
			"ip_addr_v4": "",
			"ip_addr_v6": "aa:bb:cc::",
			"network_v4": nil,
			"network_v6": "aa:bb:cc::/128",
			"as_num":     nil,
		}
		kvps, err = up.Process(&model.KVPair{
			Key:   v2NodeKey1,
			Value: res,
		})
		Expect(err).NotTo(HaveOccurred())
		checkExpectedConfigs(
			kvps,
			isNodeBgpConfig,
			numBgpConfigs,
			expected,
		)

		By("converting a Node with IPv4 and IPv6 network and AS number")
		res = apiv2.NewNode()
		res.Name = "bgpnode1"
		asn := numorstring.ASNumber(12345)
		res.Spec.BGP = &apiv2.NodeBGPSpec{
			IPv4Address: "1.2.3.4/24",
			IPv6Address: "aa:bb:cc::ffff/120",
			ASNumber:    &asn,
		}
		expected = map[string]interface{}{
			"ip_addr_v4": "1.2.3.4",
			"ip_addr_v6": "aa:bb:cc::ffff",
			"network_v4": "1.2.3.0/24",
			"network_v6": "aa:bb:cc::ff00/120",
			"as_num":     "12345",
		}
		kvps, err = up.Process(&model.KVPair{
			Key:   v2NodeKey1,
			Value: res,
		})
		Expect(err).NotTo(HaveOccurred())
		checkExpectedConfigs(
			kvps,
			isNodeBgpConfig,
			numBgpConfigs,
			expected,
		)
	})

	It("should fail to convert an invalid resource", func() {
		By("trying to convert with the wrong key type")
		res := apiv2.NewNode()

		_, err := up.Process(&model.KVPair{
			Key: model.GlobalConfigKey{
				Name: "foobar",
			},
			Value:    res,
			Revision: "abcde",
		})
		Expect(err).To(HaveOccurred())

		By("trying to convert with the wrong value type")
		wres := apiv2.NewBGPPeer()
		_, err = up.Process(&model.KVPair{
			Key:      v2NodeKey1,
			Value:    wres,
			Revision: "abcdef",
		})
		Expect(err).To(HaveOccurred())

		By("trying to convert with an invalid IPv4 address - treat as unassigned")
		res = apiv2.NewNode()
		res.Name = "bgpnode1"
		asn := numorstring.ASNumber(12345)
		res.Spec.BGP = &apiv2.NodeBGPSpec{
			IPv4Address: "1.2.3.4/240",
			IPv6Address: "aa:bb:cc::ffff/120",
			ASNumber:    &asn,
		}
		kvps, err := up.Process(&model.KVPair{
			Key:   v2NodeKey1,
			Value: res,
		})
		// IPv4 address should be blank, network should be nil (deleted)
		expected := map[string]interface{}{
			"ip_addr_v4": "",
			"ip_addr_v6": "aa:bb:cc::ffff",
			"network_v4": nil,
			"network_v6": "aa:bb:cc::ff00/120",
			"as_num":     "12345",
		}
		Expect(err).To(HaveOccurred())
		checkExpectedConfigs(
			kvps,
			isNodeBgpConfig,
			numBgpConfigs,
			expected,
		)

		By("trying to convert with an invalid IPv6 address - treat as unassigned")
		res = apiv2.NewNode()
		res.Name = "bgpnode1"
		res.Spec.BGP = &apiv2.NodeBGPSpec{
			IPv4Address: "1.2.3.4/24",
			IPv6Address: "aazz::qq/100",
		}
		kvps, err = up.Process(&model.KVPair{
			Key:   v2NodeKey1,
			Value: res,
		})
		// IPv6 address should be blank, network should be nil (deleted)
		expected = map[string]interface{}{
			"ip_addr_v4": "1.2.3.4",
			"ip_addr_v6": "",
			"network_v4": "1.2.3.0/24",
			"network_v6": nil,
			"as_num":     nil,
		}
		Expect(err).To(HaveOccurred())
		checkExpectedConfigs(
			kvps,
			isNodeBgpConfig,
			numBgpConfigs,
			expected,
		)
	})
})
