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

// Test cases:
// API to KVPair:
// Test 1: IPv4 with non-default values for IPIP, NATOut, IPAM - assert the returned values, expect IPIPInterface to be "tunl0", and no error.
// Test 2: IPv4 with default values for IPIP, NATOut, IPAM - assert the returned values, expect IPIPInterface to be "", and no error.
// Test 3: IPv6 with non-default bools for IPIP, NATOut, IPAM - assert the returned values, expect IPIPInterface to be "tunl0", and no error.

// KVPair to API:
// Test 1: IPv4 with non-default values for masquerade, disabled and ipipInterface - assert the returned Metadata and Spec values, expect IPIP.Enabled to be true and no error.
// Test 2: IPv4 with default values for masquerade, disabled and ipipInterface - assert the returned Metadata and Spec values, expect IPIP.Enabled to be false and no error.
// Test 3: IPv6 with non-default bools for masquerade, disabled and ipipInterface - assert the returned Metadata and Spec values, expect IPIP.Enabled to be true and no error.

package client

import (
	"log"
	"net"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/projectcalico/libcalico-go/lib/api/unversioned"

	"github.com/projectcalico/libcalico-go/lib/api"
	"github.com/projectcalico/libcalico-go/lib/backend/model"
	cnet "github.com/projectcalico/libcalico-go/lib/net"
)

var _ = Describe("Pool tests", func() {

	// Describe and call convertAPItoKVPairTest.
	DescribeTable("Pool API to KVPair tests",
		func(poolCIDR string, ipip, natOut, disabled bool, expectedIPIPInterfaceface string, expectedError error) {

			_, cidr, _ := net.ParseCIDR(poolCIDR)
			outKVP, outErr := convertAPItoKVPairTest(*cidr, ipip, natOut, disabled)

			// Assert output to the expected values.
			Expect(outKVP.Value.(*model.Pool).CIDR.IPNet).To(Equal(*cidr))
			Expect(outKVP.Value.(*model.Pool).Masquerade).To(Equal(natOut))
			Expect(outKVP.Value.(*model.Pool).Disabled).To(Equal(disabled))
			Expect(outKVP.Value.(*model.Pool).IPIPInterface).To(Equal(expectedIPIPInterfaceface))

			if expectedError != nil {
				Expect(outErr.Error()).To(Equal(expectedError.Error()))
			} else {
				Expect(outErr).NotTo(HaveOccurred())
			}
		},

		// Test 1: IPv4 with non-default values for IPIP, NATOut, IPAM - assert the returned values, expect IPIPInterface to be "tunl0", and no error.
		Entry("For IPv4 with non-default values for IPIP, NATOut, IPAM", "10.0.0.0/24", true, true, true, "tunl0", nil),

		// Test 2: IPv4 with default values for IPIP, NATOut, IPAM - assert the returned values, expect IPIPInterface to be "", and no error.
		Entry("For IPv4 with default values for IPIP, NATOut, IPAM", "20.0.0.0/24", false, false, false, "", nil),

		// Test 3: IPv6 with non-default bools for IPIP, NATOut, IPAM - assert the returned values, expect IPIPInterface to be "tunl0", and no error.
		Entry("For IPv6 with non-default bools for IPIP, NATOut, IPAM", "fe80::00/120", true, true, true, "tunl0", nil),
	)

	// Describe and call convertKVPairtoAPITest.
	DescribeTable("Pool KVPair to API tests",
		func(poolCIDR string, masquerade, disabled bool, ipipInterface string, expectedIPIP bool, expectedError error) {

			_, cidr, _ := net.ParseCIDR(poolCIDR)
			outResource, outErr := convertKVPairtoAPITest(*cidr, ipipInterface, masquerade, disabled)

			// Assert output to the expected values.
			Expect(outResource.(*api.Pool).Metadata.CIDR.IPNet).To(Equal(*cidr))
			Expect(outResource.(*api.Pool).Spec.NATOutgoing).To(Equal(masquerade))
			Expect(outResource.(*api.Pool).Spec.Disabled).To(Equal(disabled))
			Expect(outResource.(*api.Pool).Spec.IPIP.Enabled).To(Equal(expectedIPIP))

			if expectedError != nil {
				Expect(outErr.Error()).To(Equal(expectedError.Error()))
			} else {
				Expect(outErr).NotTo(HaveOccurred())
			}

		},

		// Test 1: IPv4 with non-default values for masquerade, disabled and ipipInterface - expect IPIP.Enabled to be true and no error.
		Entry("For IPv4 with non-default values for NATOut, Disabled and IPIPInterface", "10.0.0.0/24", true, true, "tunl0", true, nil),

		// Test 2: IPv4 with default values for masquerade, disabled and ipipInterface - expect IPIP.Enabled to be false and no error.
		Entry("For IPv4 with default values for NATOut, Disabled and IPIPInterface", "20.0.0.0/24", false, false, "", false, nil),

		// Test 3: IPv6 with non-default bools for masquerade, disabled and ipipInterface - expect IPIP.Enabled to be true and no error.
		Entry("For IPv6 with non-default values for NATOut, Disabled and IPIPInterface", "fe80::00/120", true, true, "tunl0", true, nil),
	)
})

// convertKVPairtoAPITest takes cidr, ipipInterface, masquerade (NATOutgoing)
// and disabled to convertKVPairToAPI function (function-under-test) and returns the output values.
func convertKVPairtoAPITest(cidr net.IPNet, ipipInterface string, masquerade, disabled bool) (unversioned.Resource, error) {

	c, err := newClient("")
	if err != nil {
		log.Fatalf("Error creating client: %s\n", err)
	}

	// Initialize KVPair struct with test arguments.
	kvp := model.KVPair{
		Value: &model.Pool{
			CIDR:          cnet.IPNet{cidr},
			IPIPInterface: ipipInterface,
			Masquerade:    masquerade,
			Disabled:      disabled,
		},
	}

	p := pools{
		c: c,
	}

	// Call function under test with test arguments and get the result.
	outResource, outErr := p.convertKVPairToAPI(&kvp)

	return outResource, outErr
}

// convertAPItoKVPairTest takes cidr, natOut (NATOutgoing)
// and disabled to convertAPIToKVPair function (function-under-test) and returns the output values.
func convertAPItoKVPairTest(cidr net.IPNet, ipip, natOut, disabled bool) (*model.KVPair, error) {

	c, err := newClient("")
	if err != nil {
		log.Fatalf("Error creating client: %s\n", err)
	}

	// Initialize api.Pool struct with test arguments.
	pool := api.Pool{
		Metadata: api.PoolMetadata{
			CIDR: cnet.IPNet{cidr},
		},
		Spec: api.PoolSpec{
			IPIP: &api.IPIPConfiguration{
				Enabled: ipip,
			},
			NATOutgoing: natOut,
			Disabled:    disabled,
		},
	}

	p := pools{
		c: c,
	}

	// Call function under test with test arguments and get the result.
	outKVP, outErr := p.convertAPIToKVPair(pool)

	return outKVP, outErr
}

// newClient is a util function to create a new default client.
// When passed empty string, it loads the default config instead from a config file.
// Note: the reason for not using `testutils.NewClient()` is to avoid import cycle.
func newClient(cf string) (*Client, error) {
	if _, err := os.Stat(cf); err != nil {
		cf = ""
	}

	cfg, err := LoadClientConfig(cf)
	if err != nil {
		return nil, err
	}

	c, err := New(*cfg)
	if err != nil {
		return nil, err
	}

	return c, err
}
