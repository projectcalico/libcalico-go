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
// Test 1: IPv4 with non-default values for IPIP, NATOut, IPAM - assert the returned Metadata and Spec values.
// Test 2: IPv4 with default values for IPIP, NATOut, IPAM - assert the returned Metadata and Spec values.
// Test 3: IPv6 with non-default bools for IPIP, NATOut, IPAM - assert the returned Metadata and Spec values.

package client

import (
	"fmt"
	"net"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/projectcalico/libcalico-go/lib/api"
	"github.com/projectcalico/libcalico-go/lib/api/unversioned"
	"github.com/projectcalico/libcalico-go/lib/backend/model"
	cnet "github.com/projectcalico/libcalico-go/lib/net"
)

var _ = FDescribe("Pool tests", func() {

	// Describe and call convertAPItoKVPairTest.
	Describe("Pool API to KVPair tests", func() {
		convertAPItoKVPairTest()
	})

	// Describe and call convertKVPairtoAPITest.
	Describe("Pool KVPair to API tests", func() {
		convertKVPairtoAPITest()
	})
})

// convertKVPairtoAPITest passes different values of CIDR, IPIPInterface, NATOut (NATOutgoing)
// and Disabled to convertKVPairToAPI function (function-under-test) and asserts the output values.
func convertKVPairtoAPITest() {

	// table describes and initializes the table for TDD (Table Driven Test) input and expected output.
	table := []struct {
		description   string
		CIDR          net.IPNet
		IPIPInterface string
		NATOut        bool
		Disabled      bool
		expectedIPIP  bool
	}{
		{
			description: "For IPv4 with non-default values for NATOut, Disabled and IPIPInterface",
			CIDR: net.IPNet{
				IP:   net.IPv4(10, 0, 0, 0),
				Mask: net.CIDRMask(24, 32),
			},
			IPIPInterface: "tunl0",
			NATOut:        true,
			Disabled:      true,
			expectedIPIP:  true,
		},
		{
			description: "For IPv4 with default values for NATOut, Disabled and IPIPInterface",
			CIDR: net.IPNet{
				IP:   net.IPv4(10, 0, 0, 0),
				Mask: net.CIDRMask(24, 32),
			},
			IPIPInterface: "",
			NATOut:        false,
			Disabled:      false,
			expectedIPIP:  false,
		},
		{
			description: "For IPv6 with non-default values for NATOut, Disabled and IPIPInterface",
			CIDR: net.IPNet{
				IP:   net.ParseIP("fe80::00"),
				Mask: net.CIDRMask(120, 128),
			},
			IPIPInterface: "tunl0",
			NATOut:        true,
			Disabled:      true,
			expectedIPIP:  true,
		},
	}

	// Go through all the values in table and test them one at a time.
	for _, v := range table {
		fmt.Fprintf(GinkgoWriter, "Testing KVPairtoAPI: %s\n", v.description)

		It("", func() {

			// Initialize KVPair struct with test arguments.
			kvp := model.KVPair{
				Value: &model.Pool{
					CIDR:          cnet.IPNet{v.CIDR},
					IPIPInterface: v.IPIPInterface,
					Masquerade:    v.NATOut,
					Disabled:      v.Disabled,
				},
			}

			// Create a new client.
			client, _ := newClient("")

			p := pools{
				c: client,
			}

			// Call function under test with test arguments and get the result.
			out, err := p.convertKVPairToAPI(&kvp)

			// Assert output to the expected values.
			Expect(out.(*api.Pool).Metadata.CIDR.IPNet).To(Equal(v.CIDR))
			Expect(out.(*api.Pool).Spec.NATOutgoing).To(Equal(v.NATOut))
			Expect(out.(*api.Pool).Spec.Disabled).To(Equal(v.Disabled))
			Expect(out.(*api.Pool).Spec.IPIP.Enabled).To(Equal(v.expectedIPIP))
			Expect(err).ToNot(HaveOccurred())
		})
	}
}

// convertAPItoKVPairTest passes different values of CIDR, IPIPInterface, NATOut (NATOutgoing)
// and Disabled to convertAPIToKVPair function (function-under-test) and asserts the output values.
func convertAPItoKVPairTest() {

	// table describes and initializes the table for TDD (Table Driven Test) input and expected output.
	table := []struct {
		description           string
		CIDR                  net.IPNet
		IPIP                  bool
		NATOut                bool
		Disabled              bool
		expectedIPIPInterface string
	}{
		{
			description: "For IPv4 with non-default values for IPIP, NATOut, IPAM",
			CIDR: net.IPNet{
				IP:   net.IPv4(10, 0, 0, 0),
				Mask: net.CIDRMask(24, 32),
			},
			IPIP:                  true,
			NATOut:                true,
			Disabled:              true,
			expectedIPIPInterface: "tunl0",
		},
		{
			description: "For IPv4 with default values for IPIP, NATOut, IPAM",
			CIDR: net.IPNet{
				IP:   net.IPv4(10, 0, 0, 0),
				Mask: net.CIDRMask(24, 32),
			},
			IPIP:                  false,
			NATOut:                false,
			Disabled:              false,
			expectedIPIPInterface: "",
		},
		{
			description: "For IPv6 with non-default bools for IPIP, NATOut, IPAM",
			CIDR: net.IPNet{
				IP:   net.ParseIP("fe80::00"),
				Mask: net.CIDRMask(120, 128),
			},
			IPIP:                  true,
			NATOut:                true,
			Disabled:              true,
			expectedIPIPInterface: "tunl0",
		},
	}

	// Go through all the values in table and test them one at a time.
	for _, v := range table {
		fmt.Fprintf(GinkgoWriter, "Testing APItoKVPair: %s\n", v.description)

		It("", func() {

			// Initialize api.Pool struct with test arguments.
			pool := api.Pool{
				TypeMetadata: unversioned.TypeMetadata{
					Kind:       "pool",
					APIVersion: "v1",
				},
				Metadata: api.PoolMetadata{
					ObjectMetadata: unversioned.ObjectMetadata{},
					CIDR:           cnet.IPNet{v.CIDR},
				},
				Spec: api.PoolSpec{
					IPIP: &api.IPIPConfiguration{
						Enabled: v.IPIP,
					},
					NATOutgoing: v.NATOut,
					Disabled:    v.Disabled,
				},
			}

			// Create a new client.
			client, _ := newClient("")

			p := pools{
				c: client,
			}

			// Call function under test with test arguments and get the result.
			out, err := p.convertAPIToKVPair(pool)

			// Assert output to the expected values.
			Expect(out.Value.(*model.Pool).CIDR.IPNet).To(Equal(v.CIDR))
			Expect(out.Value.(*model.Pool).Masquerade).To(Equal(v.NATOut))
			Expect(out.Value.(*model.Pool).Disabled).To(Equal(v.Disabled))
			Expect(out.Value.(*model.Pool).IPIPInterface).To(Equal(v.expectedIPIPInterface))
			Expect(err).ToNot(HaveOccurred())
		})
	}
}

// newClient is a util function to create a new default client.
// When passed empty string, it loads the default config instead from a config file.
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
