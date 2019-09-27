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

	apiv3 "github.com/projectcalico/libcalico-go/lib/apis/v3"
	"github.com/projectcalico/libcalico-go/lib/backend/model"
	"github.com/projectcalico/libcalico-go/lib/backend/syncersv1/updateprocessors"
	cnet "github.com/projectcalico/libcalico-go/lib/net"
)

var _ = Describe("Test the GlobalNetworkPolicy update processor", func() {
	emptyGNPKey := model.ResourceKey{Kind: apiv3.KindGlobalNetworkPolicy, Name: "empty"}
	emptyGNP := apiv3.NewGlobalNetworkPolicy()

	minimalGNPKey := model.ResourceKey{Kind: apiv3.KindGlobalNetworkPolicy, Name: "minimal"}
	minimalGNP := apiv3.NewGlobalNetworkPolicy()
	minimalGNP.Spec.PreDNAT = true
	minimalGNP.Spec.ApplyOnForward = true

	simpleGNPKey := model.ResourceKey{Kind: apiv3.KindGlobalNetworkPolicy, Name: "simple"}
	simpleGNP := apiv3.NewGlobalNetworkPolicy()
	simpleGNP.Namespace = "default"
	simpleGNP.Spec.Order = &order
	simpleGNP.Spec.Ingress = []apiv3.Rule{irule}
	simpleGNP.Spec.Egress = []apiv3.Rule{erule}
	simpleGNP.Spec.Selector = "calico/k8s_ns == selectme"
	simpleGNP.Spec.DoNotTrack = true
	simpleGNP.Spec.PreDNAT = false
	simpleGNP.Spec.ApplyOnForward = true
	simpleGNP.Spec.Types = []apiv3.PolicyType{apiv3.PolicyTypeIngress}

	// GlobalNetworkPolicies with valid and invalid ServiceAccountSelectors
	validSASelectorKey := model.ResourceKey{Kind: apiv3.KindGlobalNetworkPolicy, Name: "validSASelector"}
	validSASelector := simpleGNP.DeepCopy()
	validSASelector.Spec.ServiceAccountSelector = "role == 'development'"

	invalidSASelectorKey := model.ResourceKey{Kind: apiv3.KindGlobalNetworkPolicy, Name: "invalidSASelector"}
	invalidSASelector := simpleGNP.DeepCopy()
	invalidSASelector.Spec.ServiceAccountSelector = "role 'development'"

	// GlobalNetworkPolicies with valid and invalid NamespaceSelectors
	validNSSelectorKey := model.ResourceKey{Kind: apiv3.KindGlobalNetworkPolicy, Name: "validNSSelector"}
	validNSSelector := simpleGNP.DeepCopy()
	validNSSelector.Spec.NamespaceSelector = "name == 'testing'"

	invalidNSSelectorKey := model.ResourceKey{Kind: apiv3.KindGlobalNetworkPolicy, Name: "invalidNSSelector"}
	invalidNSSelector := simpleGNP.DeepCopy()
	invalidNSSelector.Spec.NamespaceSelector = "name 'testing'"

	// V3 model.KVPair Revision
	rev := "1234"

	Context("test processing of a valid GlobalNetworkPolicy from V3 to V1", func() {
		up := updateprocessors.NewGlobalNetworkPolicyUpdateProcessor()

		It("should accept a GlobalNetworkPolicy with minimum configuration", func() {
			kvps, err := up.Process(&model.KVPair{Key: minimalGNPKey, Value: minimalGNP, Revision: rev})
			Expect(err).NotTo(HaveOccurred())
			Expect(kvps).To(HaveLen(1))

			v1Key := model.PolicyKey{Name: "minimal"}
			Expect(kvps[0]).To(Equal(&model.KVPair{
				Key: v1Key,
				Value: &model.Policy{
					PreDNAT:        true,
					ApplyOnForward: true,
				},
				Revision: rev,
			}))
		})

		It("should accept a GlobalNetworkPolicy with a simple configuration", func() {
			kvps, err := up.Process(&model.KVPair{Key: simpleGNPKey, Value: simpleGNP, Revision: rev})
			Expect(err).NotTo(HaveOccurred())

			policy := NewSimplePolicy()
			v1Key := model.PolicyKey{Name: "simple"}
			Expect(kvps).To(Equal([]*model.KVPair{{Key: v1Key, Value: &policy, Revision: rev}}))

			By("should be able to delete the simple network policy")
			kvps, err = up.Process(&model.KVPair{Key: simpleGNPKey, Value: nil})
			Expect(err).NotTo(HaveOccurred())
			Expect(kvps).To(Equal([]*model.KVPair{{Key: v1Key, Value: nil}}))
		})

		It("should NOT accept a GlobalNetworkPolicy with the wrong Key type", func() {
			_, err := up.Process(&model.KVPair{
				Key:      model.GlobalBGPPeerKey{PeerIP: cnet.MustParseIP("1.2.3.4")},
				Value:    emptyGNP,
				Revision: "abcde",
			})
			Expect(err).To(HaveOccurred())
		})

		It("should NOT accept a GlobalNetworkPolicy with the wrong Value type", func() {
			kvps, err := up.Process(&model.KVPair{Key: emptyGNPKey, Value: apiv3.NewHostEndpoint(), Revision: rev})
			Expect(err).NotTo(HaveOccurred())

			v1Key := model.PolicyKey{Name: "empty"}
			Expect(kvps).To(Equal([]*model.KVPair{{Key: v1Key, Value: nil}}))
		})

		It("should accept a GlobalNetworkPolicy with a ServiceAccountSelector", func() {
			kvps, err := up.Process(&model.KVPair{Key: validSASelectorKey, Value: validSASelector, Revision: rev})
			Expect(err).NotTo(HaveOccurred())

			policy := NewSimplePolicy()
			policy.Selector = `(calico/k8s_ns == selectme) && pcsa.role == "development"`
			v1Key := model.PolicyKey{Name: "validSASelector"}
			Expect(kvps).To(Equal([]*model.KVPair{{Key: v1Key, Value: &policy, Revision: rev}}))
		})

		It("should NOT add an invalid ServiceAccountSelector to the GNP's Selector field", func() {
			kvps, err := up.Process(&model.KVPair{Key: invalidSASelectorKey, Value: invalidSASelector, Revision: rev})
			Expect(err).NotTo(HaveOccurred())

			policy := NewSimplePolicy()
			v1Key := model.PolicyKey{Name: "invalidSASelector"}
			Expect(kvps).To(Equal([]*model.KVPair{{Key: v1Key, Value: &policy, Revision: rev}}))
		})

		It("should accept a GlobalNetworkPolicy with a NamespaceSelector", func() {
			kvps, err := up.Process(&model.KVPair{Key: validNSSelectorKey, Value: validNSSelector, Revision: rev})
			Expect(err).NotTo(HaveOccurred())

			policy := NewSimplePolicy()
			policy.Selector = `(calico/k8s_ns == selectme) && pcns.name == "testing"`
			v1Key := model.PolicyKey{Name: "validNSSelector"}
			Expect(kvps).To(Equal([]*model.KVPair{{Key: v1Key, Value: &policy, Revision: rev}}))
		})

		It("should NOT add an invalid NamespaceSelector to the GNP's Selector field", func() {
			kvps, err := up.Process(&model.KVPair{Key: invalidNSSelectorKey, Value: invalidNSSelector, Revision: rev})
			Expect(err).NotTo(HaveOccurred())

			policy := NewSimplePolicy()
			v1Key := model.PolicyKey{Name: "invalidNSSelector"}
			Expect(kvps).To(Equal([]*model.KVPair{{Key: v1Key, Value: &policy, Revision: rev}}))
		})
	})
})
