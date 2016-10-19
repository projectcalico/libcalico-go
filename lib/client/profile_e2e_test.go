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

// Test cases (Profile object e2e):
// Test 1: Pass two fully populated ProfileSpecs and expect the series of operations to succeed.
// Test 2: Pass one fully populated ProfileSpec and another empty ProfileSpec and expect the series of operations to succeed.
// Test 3: Pass one partially populated ProfileSpec and another fully populated ProfileSpec and expect the series of operations to succeed.

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
	"net"
	"strconv"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/projectcalico/libcalico-go/lib/client"
	"github.com/projectcalico/libcalico-go/lib/numorstring"

	"github.com/projectcalico/libcalico-go/lib/api"
	"github.com/projectcalico/libcalico-go/lib/testutils"

	cnet "github.com/projectcalico/libcalico-go/lib/net"
)

var _ = Describe("Profile tests", func() {

	DescribeTable("Profile e2e tests",
		func(meta1, meta2 api.ProfileMetadata, spec1, spec2 api.ProfileSpec) {

			// Erase etcd clean.
			testutils.CleanEtcd()

			// Create a new client.
			c, err := testutils.NewClient("")
			if err != nil {
				log.Println("Error creating client:", err)
			}
			By("Updating the profile before it is created")
			_, outError := profileUpdate(c, &api.Profile{Metadata: meta1, Spec: spec1})

			// Should return an error.
			Expect(outError.Error()).To(Equal(errors.New("resource does not exist: ProfileTags(name=profile1)").Error()))

			By("Create, Apply, Get and compare")

			// Create a profile with meta1 and spec1.
			_, outError = profileCreate(c, &api.Profile{Metadata: meta1, Spec: spec1})

			// Apply a profile with meta2 and spec2.
			_, outError = profileApply(c, &api.Profile{Metadata: meta2, Spec: spec2})

			// Get profile with meta1.
			outProfile1, outError1 := c.Profiles().Get(meta1)
			log.Println("Out Profile object: ", outProfile1)

			// Get profile with meta2.
			outProfile2, outError2 := c.Profiles().Get(meta2)
			log.Println("Out Profile object: ", outProfile2)

			// Should match spec1 & outProfile1 and outProfile2 & spec2 and errors to be nil.
			Expect(outProfile1.Spec).To(Equal(spec1))
			Expect(outProfile2.Spec).To(Equal(spec2))
			Expect(outError1).To(BeNil())
			Expect(outError2).To(BeNil())

			By("Update, Get and compare")

			// Update meta1 profile with spec2.
			profileUpdate(c, &api.Profile{Metadata: meta1, Spec: spec2})

			// Get profile with meta1.
			outProfile1, outError1 = c.Profiles().Get(meta1)

			// Assert the Spec for profile with meta1 matches spec2 and no error.
			Expect(outProfile1.Spec).To(Equal(spec2))
			Expect(outError1).To(BeNil())

			By("List all the profiles and compare")

			// Get a list of profiless.
			profileList, outError := c.Profiles().List(api.ProfileMetadata{})
			log.Println("Get profile list returns: ", profileList.Items)
			metas := []api.ProfileMetadata{meta1, meta2}
			expectedProfiles := []api.Profile{}
			// Go through meta list and append them to expectedProfiles.
			for _, v := range metas {
				p, _ := c.Profiles().Get(v)
				expectedProfiles = append(expectedProfiles, *p)
			}

			// Assert the returned profileList is has the meta1 and meta2 profiles.
			Expect(profileList.Items).To(Equal(expectedProfiles))

			By("List a specific profile and compare")

			// Get a profile list with meta1.
			profileList, outError = c.Profiles().List(meta1)
			log.Println("Get profile list returns: ", profileList.Items)

			// Get a profile with meta1.
			outProfile1, outError1 = c.Profiles().Get(meta1)

			// Assert they are equal and no errors.
			Expect(profileList.Items[0].Spec).To(Equal(outProfile1.Spec))
			Expect(outError1).To(BeNil())

			By("Delete, Get and assert error")

			// Delete a profile with meta1.
			outError1 = c.Profiles().Delete(meta1)

			// Get a profile with meta1.
			_, outError = c.Profiles().Get(meta1)

			// Expect an error since the profile was deleted.
			Expect(outError.Error()).To(Equal(errors.New("resource does not exist: Profile(name=profile1)").Error()))

			// Delete the second profile with meta2.
			outError1 = c.Profiles().Delete(meta2)

			By("Delete all the profiles, Get profile list and expect empty profile list")

			// Both profiles are deleted in the calls above.
			// Get the list of all the profiles.
			profileList, outError = c.Profiles().List(api.ProfileMetadata{})
			log.Println("Get profile list returns: ", profileList.Items)

			// Create an empty profile list.
			// Note: you can't use make([]api.Profile, 0) because it creates an empty underlying struct,
			// whereas new([]api.Profile) just returns a pointer without creating an empty struct.
			emptyProfileList := new([]api.Profile)

			// Expect returned profileList to contain empty profileList.
			Expect(profileList.Items).To(Equal(*emptyProfileList))

		},

		// Test 1: Pass two fully populated ProfileSpecs and expect the series of operations to succeed.
		Entry("Two fully populated ProfileSpecs",
			api.ProfileMetadata{Name: "profile1"},
			api.ProfileMetadata{Name: "profile2"},
			*createAPIProfileSpecObject("profile1", []string{"profile1-tag1", "profile1-tag2"}),
			*createAPIProfileSpecObject("profile2", []string{"profile2-tag1", "profile2-tag2"}),
		),

		// // Test 2: Pass one fully populated ProfileSpec and another empty ProfileSpec and expect the series of operations to succeed.
		// Entry("One fully populated ProfileSpec and another empty ProfileSpec",
		// 	api.ProfileMetadata{Name: "profile1"},
		// 	api.ProfileMetadata{Name: "profile2"},
		// 	*createAPIProfileSpecObject("profile1", 99.999, "profile1-selector"),
		// 	api.ProfileSpec{},
		// ),

		// // Test 3: Pass one partially populated ProfileSpec and another fully populated ProfileSpec and expect the series of operations to succeed.
		// Entry("One partially populated ProfileSpec and another fully populated ProfileSpec",
		// 	api.ProfileMetadata{Name: "profile1"},
		// 	api.ProfileMetadata{Name: "profile2"},
		// 	api.ProfileSpec{
		// 		Selector: "profile1-selector",
		// 	},
		// 	*createAPIProfileSpecObject("profile2", 22.222, "profile2-selector"),
		// ),
	)
})

// profileCreate takes client and api.Profile and creates a profile.
func profileCreate(c *client.Client, p *api.Profile) (*api.Profile, error) {

	pOut, errOut := c.Profiles().Create(p)

	if errOut != nil {
		log.Printf("Error creating profile: %s\n", errOut)
	}
	return pOut, errOut

}

// profileApply takes client and api.Profile and applies the profile config to
// an existing profile or creates a new one if it doesn't already exist.
func profileApply(c *client.Client, p *api.Profile) (*api.Profile, error) {

	pOut, errOut := c.Profiles().Apply(p)
	if errOut != nil {
		log.Printf("Error applying profile: %s\n", errOut)
	}

	return pOut, errOut
}

// profileUpdate takes client and api.Profile and updates an existing profile.
func profileUpdate(c *client.Client, p *api.Profile) (*api.Profile, error) {

	pOut, errOut := c.Profiles().Update(p)
	if errOut != nil {
		log.Printf("Error updating profile: %s\n", errOut)
	}

	return pOut, errOut
}

// createAPIProfileSpecObject takes profile configuration options (name, order, selector),
// creates 2 fixed set of ingress and egress rules (one with IPv4 and one with IPv6),
// and composes & returns an api.ProfileSpec object.
func createAPIProfileSpecObject(name string, tags []string) *api.ProfileSpec {
	inRule1, eRule1 := createRule(4, 100, 200, "icmp", "10.0.0.0/24", "abc-tag", "abc-selector", "allow", "deny")
	inRule2, eRule2 := createRule(6, 111, 222, "111", "fe80::00/120", "xyz-tag", "xyz-selector", "deny", "allow")

	inRules := []api.Rule{inRule1, inRule2}
	eRules := []api.Rule{eRule1, eRule2}

	return &api.ProfileSpec{
		IngressRules: inRules,
		EgressRules:  eRules,
		Tags:         tags,
	}
}

// createProfileRule takes all fields necessary to create a api.Rule object and returns ingress and egress api.Rules.
func createProfileRule(ipv, icmpType, icmpCode int, proto, cidrStr, tag, selector, inAction, eAction string) (api.Rule, api.Rule) {

	var protocol numorstring.Protocol

	i, err := strconv.Atoi(proto)
	if err != nil {
		protocol = numorstring.ProtocolFromString(proto)
	} else {
		protocol = numorstring.ProtocolFromInt(uint8(i))
	}

	icmp := api.ICMPFields{
		Type: &icmpType,
		Code: &icmpCode,
	}

	_, cidr, err := net.ParseCIDR(cidrStr)
	if err != nil {
		log.Printf("Error parsing CIDR: %s\n", err)
	}

	src := api.EntityRule{
		Tag: tag,
		Net: &cnet.IPNet{
			*cidr,
		},
		Selector: selector,
	}

	inRule := api.Rule{
		Action:    inAction,
		IPVersion: &ipv,
		Protocol:  &protocol,
		ICMP:      &icmp,
		Source:    src,
	}

	eRule := api.Rule{
		Action:    eAction,
		IPVersion: &ipv,
		Protocol:  &protocol,
		ICMP:      &icmp,
	}

	return inRule, eRule
}
