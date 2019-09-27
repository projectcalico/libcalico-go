// Copyright (c) 2017-2018 Tigera, Inc. All rights reserved.

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
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	kapiv1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	apiv3 "github.com/projectcalico/libcalico-go/lib/apis/v3"
	"github.com/projectcalico/libcalico-go/lib/backend/k8s/conversion"
	"github.com/projectcalico/libcalico-go/lib/backend/model"
	"github.com/projectcalico/libcalico-go/lib/backend/syncersv1/updateprocessors"
	cnet "github.com/projectcalico/libcalico-go/lib/net"
	"github.com/projectcalico/libcalico-go/lib/numorstring"
)

var _ = Describe("Test the NetworkPolicy update processor", func() {
	ns1 := "namespace1"
	ns2 := "namespace2"

	emptyNPKey := model.ResourceKey{Kind: apiv3.KindNetworkPolicy, Name: "empty", Namespace: ns1}
	emptyNP := apiv3.NewNetworkPolicy()

	minimalNPKey := model.ResourceKey{Kind: apiv3.KindNetworkPolicy, Name: "minimal", Namespace: ns1}
	minimalNP := apiv3.NewNetworkPolicy()
	minimalNP.Name = "minimal"
	minimalNP.Namespace = ns1

	fullNPKey := model.ResourceKey{Kind: apiv3.KindNetworkPolicy, Name: "simple", Namespace: ns2}
	fullNP := apiv3.NewNetworkPolicy()
	fullNP.Name = "full"
	fullNP.Namespace = ns2
	fullNP.Spec.Order = &Order
	fullNP.Spec.Ingress = []apiv3.Rule{TestIngressRule}
	fullNP.Spec.Egress = []apiv3.Rule{TestEgressRule}
	fullNP.Spec.Selector = "mylabel == 'selectme'"
	fullNP.Spec.Types = []apiv3.PolicyType{apiv3.PolicyTypeIngress}

	// NetworkPolicies with valid and invalid ServiceAccountSelectors
	validSASelectorKey := model.ResourceKey{Kind: apiv3.KindNetworkPolicy, Name: "validSASelector", Namespace: ns2}
	validSASelector := fullNP.DeepCopy()
	validSASelector.Spec.ServiceAccountSelector = "role == 'development'"

	invalidSASelectorKey := model.ResourceKey{Kind: apiv3.KindNetworkPolicy, Name: "invalidSASelector", Namespace: ns2}
	invalidSASelector := fullNP.DeepCopy()
	invalidSASelector.Spec.ServiceAccountSelector = "role 'development'"

	Context("test processing of a valid NetworkPolicy from V3 to V1", func() {
		up := updateprocessors.NewNetworkPolicyUpdateProcessor()

		It("should accept a NetworkPolicy with a minimal configuration", func() {
			kvps, err := up.Process(&model.KVPair{Key: minimalNPKey, Value: minimalNP, Revision: TestRev})
			Expect(err).NotTo(HaveOccurred())
			Expect(kvps).To(HaveLen(1))

			v1Key := model.PolicyKey{Name: ns1 + "/minimal"}
			Expect(kvps[0]).To(Equal(&model.KVPair{
				Key: v1Key,
				Value: &model.Policy{
					Namespace:      ns1,
					Selector:       "projectcalico.org/namespace == 'namespace1'",
					ApplyOnForward: true,
				},
				Revision: TestRev,
			}))
		})

		It("should accept a NetworkPolicy with a full configuration", func() {
			kvps, err := up.Process(&model.KVPair{Key: fullNPKey, Value: fullNP, Revision: TestRev})
			Expect(err).NotTo(HaveOccurred())

			policy := NewCompleteNP(ns2)
			policy.Selector = fmt.Sprintf("(mylabel == 'selectme') && projectcalico.org/namespace == '%s'", ns2)

			v1Key := model.PolicyKey{Name: ns2 + "/simple"}
			Expect(kvps).To(Equal([]*model.KVPair{{Key: v1Key, Value: &policy, Revision: TestRev}}))

			By("should be able to delete the simple network policy")
			kvps, err = up.Process(&model.KVPair{Key: fullNPKey, Value: nil})
			Expect(err).NotTo(HaveOccurred())
			Expect(kvps).To(Equal([]*model.KVPair{{Key: v1Key, Value: nil}}))
		})

		It("should NOT accept a NetworkPolicy with the wrong Key type", func() {
			_, err := up.Process(&model.KVPair{
				Key:      model.GlobalBGPPeerKey{PeerIP: cnet.MustParseIP("1.2.3.4")},
				Value:    emptyNP,
				Revision: "abcde",
			})
			Expect(err).To(HaveOccurred())
		})

		It("should NOT accept a NetworkPolicy with the wrong Value type", func() {
			kvps, err := up.Process(&model.KVPair{Key: emptyNPKey, Value: apiv3.NewHostEndpoint(), Revision: TestRev})
			Expect(err).NotTo(HaveOccurred())

			v1Key := model.PolicyKey{Name: ns1 + "/empty"}
			Expect(kvps).To(Equal([]*model.KVPair{{Key: v1Key, Value: nil}}))
		})

		It("should accept a NetworkPolicy with a ServiceAccountSelector", func() {
			kvps, err := up.Process(&model.KVPair{Key: validSASelectorKey, Value: validSASelector, Revision: TestRev})
			Expect(err).NotTo(HaveOccurred())

			policy := NewCompleteNP(ns2)
			policy.Selector = `((mylabel == 'selectme') && projectcalico.org/namespace == 'namespace2') && pcsa.role == "development"`
			v1Key := model.PolicyKey{Name: ns2 + "/validSASelector"}
			Expect(kvps).To(Equal([]*model.KVPair{{Key: v1Key, Value: &policy, Revision: TestRev}}))
		})

		It("should NOT add an invalid ServiceAccountSelector to the NP's Selector field", func() {
			kvps, err := up.Process(&model.KVPair{Key: invalidSASelectorKey, Value: invalidSASelector, Revision: TestRev})
			Expect(err).NotTo(HaveOccurred())

			policy := NewCompleteNP(ns2)
			policy.Selector = `(mylabel == 'selectme') && projectcalico.org/namespace == 'namespace2'`
			v1Key := model.PolicyKey{Name: ns2 + "/invalidSASelector"}
			Expect(kvps).To(Equal([]*model.KVPair{{Key: v1Key, Value: &policy, Revision: TestRev}}))
		})
	})
})

// Define network policies and the corresponding expected v1 KVPairs.
//
// np1 is a NetworkPolicy with a single Egress rule, which contains ports only,
// and no selectors.
var protocol = kapiv1.ProtocolTCP
var port = intstr.FromInt(80)
var np1 = networkingv1.NetworkPolicy{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "test.policy",
		Namespace: "default",
	},
	Spec: networkingv1.NetworkPolicySpec{
		PodSelector: metav1.LabelSelector{},
		Egress: []networkingv1.NetworkPolicyEgressRule{
			networkingv1.NetworkPolicyEgressRule{
				Ports: []networkingv1.NetworkPolicyPort{
					networkingv1.NetworkPolicyPort{
						Protocol: &protocol,
						Port:     &port,
					},
				},
			},
		},
		PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
	},
}

// expected1 is the expected v1 KVPair representation of np1 from above.
var tcp = numorstring.ProtocolFromStringV1("tcp")
var expected1 = []*model.KVPair{
	&model.KVPair{
		Key: model.PolicyKey{Name: "default/knp.default.test.policy"},
		Value: &model.Policy{
			Namespace:      "default",
			Order:          &DefaultPolicyOrder,
			Selector:       "(projectcalico.org/orchestrator == 'k8s') && projectcalico.org/namespace == 'default'",
			Types:          []string{"egress"},
			ApplyOnForward: true,
			OutboundRules: []model.Rule{
				{
					Action:      "allow",
					Protocol:    &tcp,
					SrcSelector: "",
					DstSelector: "",
					DstPorts:    []numorstring.Port{Port80},
				},
			},
		},
	},
}

// np2 is a NeteworkPolicy with a single Ingress rule which allows from all namespaces.
var np2 = networkingv1.NetworkPolicy{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "test.policy",
		Namespace: "default",
	},
	Spec: networkingv1.NetworkPolicySpec{
		PodSelector: metav1.LabelSelector{},
		Ingress: []networkingv1.NetworkPolicyIngressRule{
			networkingv1.NetworkPolicyIngressRule{
				From: []networkingv1.NetworkPolicyPeer{
					{
						NamespaceSelector: &metav1.LabelSelector{},
					},
				},
			},
		},
		PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
	},
}
var expected2 = []*model.KVPair{
	&model.KVPair{
		Key: model.PolicyKey{Name: "default/knp.default.test.policy"},
		Value: &model.Policy{
			Namespace:      "default",
			Order:          &DefaultPolicyOrder,
			Selector:       "(projectcalico.org/orchestrator == 'k8s') && projectcalico.org/namespace == 'default'",
			Types:          []string{"ingress"},
			ApplyOnForward: true,
			InboundRules: []model.Rule{
				{
					Action:                       "allow",
					SrcSelector:                  "(has(projectcalico.org/namespace)) && (projectcalico.org/orchestrator == 'k8s')",
					DstSelector:                  "",
					OriginalSrcSelector:          "projectcalico.org/orchestrator == 'k8s'",
					OriginalSrcNamespaceSelector: "all()",
				},
			},
		},
	},
}

var _ = Describe("Test the NetworkPolicy update processor + conversion", func() {
	up := updateprocessors.NewNetworkPolicyUpdateProcessor()

	DescribeTable("NetworkPolicy update processor + conversion tests",
		func(np networkingv1.NetworkPolicy, expected []*model.KVPair) {
			// First, convert the NetworkPolicy using the k8s conversion logic.
			c := conversion.Converter{}
			kvp, err := c.K8sNetworkPolicyToCalico(&np)
			Expect(err).NotTo(HaveOccurred())

			// Next, run the policy through the update processor.
			out, err := up.Process(kvp)
			Expect(err).NotTo(HaveOccurred())

			// Finally, assert the expected result.
			Expect(out).To(Equal(expected))
		},

		Entry("should handle a NetworkPolicy with no rule selectors", np1, expected1),
		Entry("should handle a NetworkPolicy with an empty ns selector", np2, expected2),
	)
})
