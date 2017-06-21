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

// Test cases (NetworkSet object e2e):
// Test 1: Pass two fully populated NetworkSetSpecs and expect the series of operations to succeed.
// Test 2: Pass one partially populated NetworkSetSpec and another fully populated NetworkSetSpec and expect the series of operations to succeed.
// Test 3: Pass one fully populated NetworkSetSpec and another empty NetworkSetSpec and expect the series of operations to succeed.
// Test 4: Pass two fully populated NetworkSetSpecs with two NetworkSetMetadata (one IPv4 and another IPv6) and expect the series of operations to succeed.

// Series of operations each test goes through:
// Update meta1 - check for failure (because it doesn't exist).
// Create meta1 with spec1.
// Apply meta2 with spec2.
// Get meta1 and meta2, compare spec1 and spec2.
// Update meta1 with spec2.
// Get meta1 compare spec2.
// List (empty Meta) ... Get meta1 and meta2.
// List (using Meta1) ... Get meta1.
// Delete meta1.
// Get meta1 ... fail.
// Delete meta2.
// List (empty Meta) ... Get no entries (should not error).

package client_test

import (
	"errors"
	"log"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"github.com/projectcalico/libcalico-go/lib/api"
	cnet "github.com/projectcalico/libcalico-go/lib/net"
	"github.com/projectcalico/libcalico-go/lib/testutils"
)

var _ = testutils.E2eDatastoreDescribe("NetworkSet tests", testutils.DatastoreEtcdV2, func(config api.CalicoAPIConfig) {

	DescribeTable("NetworkSet e2e tests",
		func(meta1, meta2 api.NetworkSetMetadata, spec1, spec2 api.NetworkSetSpec) {
			// Create a new client.
			c := testutils.CreateCleanClient(config)
			By("Updating the NetworkSet before it is created")
			_, outError := c.NetworkSets().Update(&api.NetworkSet{Metadata: meta1, Spec: spec1})

			// Should return an error.
			Expect(outError.Error()).To(Equal(errors.New("resource does not exist: NetworkSet(name=netset1)").Error()))

			By("Create, Apply, Get and compare")

			// Create a NetworkSet with meta1 and spec1.
			_, outError = c.NetworkSets().Create(&api.NetworkSet{Metadata: meta1, Spec: spec1})
			Expect(outError).NotTo(HaveOccurred())

			// Apply a NetworkSet with meta2 and spec2.
			_, outError = c.NetworkSets().Apply(&api.NetworkSet{Metadata: meta2, Spec: spec2})
			Expect(outError).NotTo(HaveOccurred())

			// Get NetworkSet with meta1.
			outNetworkSet1, outError1 := c.NetworkSets().Get(meta1)
			log.Println("Out NetworkSet object: ", outNetworkSet1)

			// Get NetworkSet with meta2.
			outNetworkSet2, outError2 := c.NetworkSets().Get(meta2)
			log.Println("Out NetworkSet object: ", outNetworkSet2)

			// Should match spec1 & outNetworkSet1 and outNetworkSet2 & spec2 and errors to be nil.
			Expect(outError1).NotTo(HaveOccurred())
			Expect(outError2).NotTo(HaveOccurred())
			Expect(outNetworkSet1.Spec).To(Equal(spec1))
			Expect(outNetworkSet2.Spec).To(Equal(spec2))

			By("Update, Get and compare")

			// Update meta1 NetworkSet with spec2.
			_, outError = c.NetworkSets().Update(&api.NetworkSet{Metadata: meta1, Spec: spec2})
			Expect(outError).NotTo(HaveOccurred())

			// Get NetworkSet with meta1.
			outNetworkSet1, outError1 = c.NetworkSets().Get(meta1)

			// Assert the Spec for NetworkSet with meta1 matches spec2 and no error.
			Expect(outError1).NotTo(HaveOccurred())
			Expect(outNetworkSet1.Spec).To(Equal(spec2))

			By("List all the NetworkSets and compare")

			// Get a list of NetworkSets.
			NetworkSetList, outError := c.NetworkSets().List(api.NetworkSetMetadata{})
			Expect(outError).NotTo(HaveOccurred())

			log.Println("Get NetworkSet list returns: ", NetworkSetList.Items)
			metas := []api.NetworkSetMetadata{meta1, meta2}
			expectedNetworkSets := []api.NetworkSet{}
			// Go through meta list and append them to expectedNetworkSets.
			for _, v := range metas {
				p, outError := c.NetworkSets().Get(v)
				Expect(outError).NotTo(HaveOccurred())
				expectedNetworkSets = append(expectedNetworkSets, *p)
			}

			// Assert the returned NetworkSetList is has the meta1 and meta2 NetworkSets.
			Expect(NetworkSetList.Items).To(Equal(expectedNetworkSets))

			By("List a specific NetworkSet and compare")

			// Get a NetworkSet list with meta1.
			NetworkSetList, outError = c.NetworkSets().List(meta1)
			Expect(outError).NotTo(HaveOccurred())
			log.Println("Get NetworkSet list returns: ", NetworkSetList.Items)

			// Get a NetworkSet with meta1.
			outNetworkSet1, outError1 = c.NetworkSets().Get(meta1)

			// Assert they are equal and no errors.
			Expect(outError1).NotTo(HaveOccurred())
			Expect(NetworkSetList.Items[0].Spec).To(Equal(outNetworkSet1.Spec))

			By("Delete, Get and assert error")

			// Delete a NetworkSet with meta1.
			outError1 = c.NetworkSets().Delete(meta1)
			Expect(outError1).NotTo(HaveOccurred())

			// Get a NetworkSet with meta1.
			_, outError = c.NetworkSets().Get(meta1)

			// Expect an error since the NetworkSet was deleted.
			Expect(outError.Error()).To(Equal(errors.New("resource does not exist: NetworkSet(name=netset1)").Error()))

			// Delete the second NetworkSet with meta2.
			outError1 = c.NetworkSets().Delete(meta2)
			Expect(outError1).NotTo(HaveOccurred())

			By("Delete all the NetworkSets, Get NetworkSet list and expect empty NetworkSet list")

			// Both NetworkSets are deleted in the calls above.
			// Get the list of all the NetworkSets.
			NetworkSetList, outError = c.NetworkSets().List(api.NetworkSetMetadata{})
			Expect(outError).NotTo(HaveOccurred())
			log.Println("Get NetworkSet list returns: ", NetworkSetList.Items)

			// Create an empty NetworkSet list.
			// Note: you can't use make([]api.NetworkSet, 0) because it creates an empty underlying struct,
			// whereas new([]api.NetworkSet) just returns a pointer without creating an empty struct.
			emptyNetworkSetList := new([]api.NetworkSet)

			// Expect returned NetworkSetList to contain empty NetworkSetList.
			Expect(NetworkSetList.Items).To(Equal(*emptyNetworkSetList))

		},

		// Test 1: Pass two fully populated NetworkSetSpecs and expect the series of operations to succeed.
		Entry("Two fully populated NetworkSetSpecs",
			api.NetworkSetMetadata{
				Name: "netset1",
				Labels: map[string]string{
					"app":  "app-abc",
					"prod": "no",
				}},
			api.NetworkSetMetadata{
				Name: "netset1/with_foo",
				Labels: map[string]string{
					"app":  "app-xyz",
					"prod": "yes",
				}},
			api.NetworkSetSpec{
				Nets: []cnet.IPNet{testutils.MustParseNetwork("10.0.0.0/16"), testutils.MustParseNetwork("20.0.0.0/16")},
			},
			api.NetworkSetSpec{
				Nets: []cnet.IPNet{testutils.MustParseNetwork("192.168.0.0/16"), testutils.MustParseNetwork("192.168.1.1/32")},
			}),

		// Test 2: Pass one partially populated NetworkSetSpec and another fully populated NetworkSetSpec and expect the series of operations to succeed.
		Entry("One partially populated NetworkSetSpec and another fully populated NetworkSetSpec",
			api.NetworkSetMetadata{
				Name: "netset1",
				Labels: map[string]string{
					"app":  "app-abc",
					"prod": "no",
				}},
			api.NetworkSetMetadata{
				Name: "netset1/with.foo",
				Labels: map[string]string{
					"app":  "app-xyz",
					"prod": "yes",
				}},
			api.NetworkSetSpec{},
			api.NetworkSetSpec{
				Nets: []cnet.IPNet{testutils.MustParseNetwork("192.168.0.0/16"), testutils.MustParseNetwork("192.168.1.1/32")},
			}),

		// Test 3: Pass one fully populated NetworkSetSpec and another empty NetworkSetSpec and expect the series of operations to succeed.
		Entry("One fully populated NetworkSetSpec and another (almost) empty NetworkSetSpec",
			api.NetworkSetMetadata{
				Name: "netset1",
				Labels: map[string]string{
					"app":  "app-abc",
					"prod": "no",
				}},
			api.NetworkSetMetadata{
				Name: "netset1/with.foo/and.bar",
				Labels: map[string]string{
					"app":  "app-xyz",
					"prod": "yes",
				}},
			api.NetworkSetSpec{
				Nets: []cnet.IPNet{testutils.MustParseNetwork("10.0.0.0/16"), testutils.MustParseNetwork("20.0.0.0/16")},
			},
			api.NetworkSetSpec{}),

		// Test 4: Pass two fully populated NetworkSetSpecs with two NetworkSetMetadata (one IPv4 and another IPv6) and expect the series of operations to succeed.
		Entry("Two fully populated NetworkSetSpecs with two NetworkSetMetadata (one IPv4 and another IPv6)",
			api.NetworkSetMetadata{
				Name: "netset1",
				Labels: map[string]string{
					"app":  "app-abc",
					"prod": "no",
				}},
			api.NetworkSetMetadata{
				Name: "netset2",
				Labels: map[string]string{
					"app":  "app-xyz",
					"prod": "yes",
				}},
			api.NetworkSetSpec{
				Nets: []cnet.IPNet{testutils.MustParseNetwork("10.0.0.0/16"), testutils.MustParseNetwork("192.168.1.1/32")},
			},
			api.NetworkSetSpec{
				Nets: []cnet.IPNet{testutils.MustParseNetwork("fe80::00/96"), testutils.MustParseNetwork("fe80::33/128")},
			}),
	)

})
