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

package conversion

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	apiv2 "github.com/projectcalico/libcalico-go/lib/apis/v2"
	"github.com/projectcalico/libcalico-go/lib/backend/model"
	"github.com/projectcalico/libcalico-go/lib/numorstring"

	kapiv1 "k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var _ = Describe("Test parsing strings", func() {

	// Use a single instance of the Converter for these tests.
	c := Converter{}

	It("should parse workloadIDs", func() {
		workloadName := "Namespace.podName"
		ns, podName := c.ParseWorkloadID(workloadName)
		Expect(ns).To(Equal("Namespace"))
		Expect(podName).To(Equal("podName"))
	})

	It("should parse valid policy names", func() {
		// Parse a NetworkPolicy backed Policy.
		name := "knp.default.Namespace.policyName"
		ns, polName, err := c.ParsePolicyNameNetworkPolicy(name)
		Expect(err).NotTo(HaveOccurred())
		Expect(ns).To(Equal("Namespace"))
		Expect(polName).To(Equal("policyName"))
	})

	It("should not parse invalid policy names", func() {
		name := "something.projectcalico.org/Namespace.Name"

		// As a NetworkPolicy.
		ns, polName, err := c.ParsePolicyNameNetworkPolicy(name)
		Expect(err).To(HaveOccurred())
		Expect(ns).To(Equal(""))
		Expect(polName).To(Equal(""))
	})

	It("should parse valid profile names", func() {
		name := "k8s_ns.default"
		ns, err := c.ProfileNameToNamespace(name)
		Expect(ns).To(Equal("default"))
		Expect(err).NotTo(HaveOccurred())
	})

	It("should not parse invalid profile names", func() {
		name := "ns.projectcalico.org/default"
		ns, err := c.ProfileNameToNamespace(name)
		Expect(err).To(HaveOccurred())
		Expect(ns).To(Equal(""))
	})
})

var _ = Describe("Test Pod conversion", func() {

	// Use a single instance of the Converter for these tests.
	c := Converter{}

	It("should parse a Pod with an IP to a WorkloadEndpoint", func() {
		pod := kapiv1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "podA",
				Namespace: "default",
				Annotations: map[string]string{
					"arbitrary": "annotation",
				},
				Labels: map[string]string{
					"labelA": "valueA",
					"labelB": "valueB",
				},
				ResourceVersion: "1234",
			},
			Spec: kapiv1.PodSpec{
				NodeName: "nodeA",
				Containers: []kapiv1.Container{
					{
						Ports: []kapiv1.ContainerPort{
							{
								ContainerPort: 5678,
							},
							{
								Name:          "no-proto",
								ContainerPort: 1234,
							},
						},
					},
					{
						Ports: []kapiv1.ContainerPort{
							{
								Name:          "tcp-proto",
								Protocol:      kapiv1.ProtocolTCP,
								ContainerPort: 1024,
							},
							{
								Name:          "tcp-proto-with-host-port",
								Protocol:      kapiv1.ProtocolTCP,
								ContainerPort: 8080,
								HostPort:      5678,
							},
							{
								Name:          "udp-proto",
								Protocol:      kapiv1.ProtocolUDP,
								ContainerPort: 432,
							},
							{
								Name:          "unkn-proto",
								Protocol:      kapiv1.Protocol("unknown"),
								ContainerPort: 567,
							},
						},
					},
				},
			},
			Status: kapiv1.PodStatus{
				PodIP: "192.168.0.1",
			},
		}

		wep, err := c.PodToWorkloadEndpoint(&pod)
		Expect(err).NotTo(HaveOccurred())

		// Assert key fields.
		Expect(wep.Key.(model.ResourceKey).Name).To(Equal("podA"))
		Expect(wep.Key.(model.ResourceKey).Kind).To(Equal(apiv2.KindWorkloadEndpoint))

		Expect(wep.Value.(*apiv2.WorkloadEndpoint).Spec.Workload).To(Equal("default.podA"))
		Expect(wep.Value.(*apiv2.WorkloadEndpoint).Spec.Node).To(Equal("nodeA"))
		Expect(wep.Value.(*apiv2.WorkloadEndpoint).Spec.Endpoint).To(Equal("eth0"))
		Expect(wep.Value.(*apiv2.WorkloadEndpoint).Spec.Orchestrator).To(Equal("k8s"))
		Expect(len(wep.Value.(*apiv2.WorkloadEndpoint).Spec.IPNetworks)).To(Equal(1))
		Expect(wep.Value.(*apiv2.WorkloadEndpoint).Spec.IPNetworks[0]).To(Equal("192.168.0.1/32"))
		//Expect(wep.Value.(*model.WorkloadEndpoint).State).To(Equal("active"))
		expectedLabels := map[string]string{"labelA": "valueA", "labelB": "valueB", "calico/k8s_ns": "default"}
		Expect(wep.Value.(*apiv2.WorkloadEndpoint).ObjectMeta.Labels).To(Equal(expectedLabels))

		nsProtoTCP := numorstring.ProtocolFromString("tcp")
		nsProtoUDP := numorstring.ProtocolFromString("udp")
		Expect(wep.Value.(*apiv2.WorkloadEndpoint).Spec.Ports).To(ConsistOf(
			// No proto defaults to TCP (as defined in k8s API spec)
			apiv2.EndpointPort{Name: "no-proto", Port: 1234, Protocol: nsProtoTCP},
			// Explicit TCP proto is OK too.
			apiv2.EndpointPort{Name: "tcp-proto", Port: 1024, Protocol: nsProtoTCP},
			// Host port should be ignored.
			apiv2.EndpointPort{Name: "tcp-proto-with-host-port", Port: 8080, Protocol: nsProtoTCP},
			// UDP is also an option.
			apiv2.EndpointPort{Name: "udp-proto", Port: 432, Protocol: nsProtoUDP},
			// Unknown protocol port is ignored.
		))

		// Assert ResourceVersion is present.
		Expect(wep.Revision).To(Equal("1234"))
	})

	It("should not parse a Pod without an IP to a WorkloadEndpoint", func() {
		pod := kapiv1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "podA",
				Namespace: "default",
				Annotations: map[string]string{
					"arbitrary": "annotation",
				},
				Labels: map[string]string{
					"labelA": "valueA",
					"labelB": "valueB",
				},
			},
			Spec: kapiv1.PodSpec{
				NodeName: "nodeA",
			},
			Status: kapiv1.PodStatus{},
		}

		_, err := c.PodToWorkloadEndpoint(&pod)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should parse a Pod with no labels", func() {
		pod := kapiv1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "podA",
				Namespace: "default",
			},
			Spec: kapiv1.PodSpec{
				NodeName: "nodeA",
			},
			Status: kapiv1.PodStatus{
				PodIP: "192.168.0.1",
			},
		}

		wep, err := c.PodToWorkloadEndpoint(&pod)
		Expect(err).NotTo(HaveOccurred())

		// Assert key fields.
		Expect(wep.Key.(model.ResourceKey).Name).To(Equal("podA"))
		Expect(wep.Key.(model.ResourceKey).Kind).To(Equal(apiv2.KindWorkloadEndpoint))
		// Assert value fields.
		Expect(wep.Value.(*apiv2.WorkloadEndpoint).Spec.Workload).To(Equal("default.podA"))
		Expect(wep.Value.(*apiv2.WorkloadEndpoint).Spec.Node).To(Equal("nodeA"))
		Expect(wep.Value.(*apiv2.WorkloadEndpoint).Spec.Endpoint).To(Equal("eth0"))
		Expect(wep.Value.(*apiv2.WorkloadEndpoint).Spec.Orchestrator).To(Equal("k8s"))
		Expect(len(wep.Value.(*apiv2.WorkloadEndpoint).Spec.IPNetworks)).To(Equal(1))
		Expect(wep.Value.(*apiv2.WorkloadEndpoint).ObjectMeta.Labels).To(Equal(map[string]string{"calico/k8s_ns": "default"}))
		//Expect(wep.Value.(*apiv2.WorkloadEndpoint).State).To(Equal("active"))
	})

	It("should Parse a Pod with no NodeName", func() {
		pod := kapiv1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "podA",
				Namespace: "default",
			},
			Spec: kapiv1.PodSpec{},
			Status: kapiv1.PodStatus{
				PodIP: "192.168.0.1",
			},
		}

		_, err := c.PodToWorkloadEndpoint(&pod)
		Expect(err).NotTo(HaveOccurred())
	})

})

var _ = Describe("Test NetworkPolicy conversion", func() {

	// Use a single instance of the Converter for these tests.
	c := Converter{}

	It("should parse a basic NetworkPolicy to a Policy", func() {
		port80 := intstr.FromInt(80)
		portFoo := intstr.FromString("foo")
		np := extensions.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testPolicy",
				Namespace: "default",
			},
			Spec: extensions.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"label":  "value",
						"label2": "value2",
					},
				},
				Ingress: []extensions.NetworkPolicyIngressRule{
					{
						Ports: []extensions.NetworkPolicyPort{
							{Port: &port80},
							{Port: &portFoo},
						},
						From: []extensions.NetworkPolicyPeer{
							{
								PodSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"k":  "v",
										"k2": "v2",
									},
								},
							},
						},
					},
				},
				PolicyTypes: []extensions.PolicyType{extensions.PolicyTypeIngress},
			},
		}

		// Parse the policy.
		pol, err := c.NetworkPolicyToPolicy(&np)
		Expect(err).NotTo(HaveOccurred())

		// Assert key fields are correct.
		Expect(pol.Key.(model.ResourceKey).Name).To(Equal("knp.default.default.testPolicy"))

		// Assert value fields are correct.
		Expect(int(*pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Order)).To(Equal(1000))
		// Check the selector is correct, and that the matches are sorted.
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Selector).To(Equal(
			"calico/k8s_ns == 'default' && label == 'value' && label2 == 'value2'"))
		protoTCP := numorstring.ProtocolFromString("tcp")
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules).To(ConsistOf(
			apiv2.Rule{
				Action:   "allow",
				Protocol: &protoTCP, // Defaulted to TCP.
				Source: apiv2.EntityRule{
					Selector: "calico/k8s_ns == 'default' && k == 'v' && k2 == 'v2'",
				},
				Destination: apiv2.EntityRule{
					Ports: []numorstring.Port{numorstring.SinglePort(80)},
				},
			},
			apiv2.Rule{
				Action:   "allow",
				Protocol: &protoTCP, // Defaulted to TCP.
				Source: apiv2.EntityRule{
					Selector: "calico/k8s_ns == 'default' && k == 'v' && k2 == 'v2'",
				},
				Destination: apiv2.EntityRule{
					Ports: []numorstring.Port{numorstring.NamedPort("foo")},
				},
			},
		))

		// There should be no EgressRules
		Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.EgressRules)).To(Equal(0))

		// Check that Types field exists and has only 'ingress'
		Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Types)).To(Equal(1))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Types[0]).To(Equal(apiv2.PolicyTypeIngress))
	})

	It("should parse a NetworkPolicy with no rules", func() {
		np := extensions.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testPolicy",
				Namespace: "default",
			},
			Spec: extensions.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{"label": "value"},
				},
				PolicyTypes: []extensions.PolicyType{extensions.PolicyTypeIngress},
			},
		}

		// Parse the policy.
		pol, err := c.NetworkPolicyToPolicy(&np)
		Expect(err).NotTo(HaveOccurred())

		// Assert key fields are correct.
		Expect(pol.Key.(model.ResourceKey).Name).To(Equal("knp.default.default.testPolicy"))

		// Assert value fields are correct.
		Expect(int(*pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Order)).To(Equal(1000))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Selector).To(Equal("calico/k8s_ns == 'default' && label == 'value'"))
		Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules)).To(Equal(0))

		// There should be no EgressRules
		Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.EgressRules)).To(Equal(0))

		// Check that Types field exists and has only 'ingress'
		Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Types)).To(Equal(1))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Types[0]).To(Equal(apiv2.PolicyTypeIngress))
	})

	It("should parse a NetworkPolicy with multiple peers", func() {
		np := extensions.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testPolicy",
				Namespace: "default",
			},
			Spec: extensions.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{"label": "value"},
				},
				Ingress: []extensions.NetworkPolicyIngressRule{
					extensions.NetworkPolicyIngressRule{
						Ports: []extensions.NetworkPolicyPort{
							extensions.NetworkPolicyPort{},
						},
						From: []extensions.NetworkPolicyPeer{
							extensions.NetworkPolicyPeer{
								PodSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"k": "v",
									},
								},
							},
							extensions.NetworkPolicyPeer{
								PodSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"k2": "v2",
									},
								},
							},
						},
					},
				},
				PolicyTypes: []extensions.PolicyType{extensions.PolicyTypeIngress},
			},
		}

		var pol *model.KVPair
		var err error
		By("parsing the policy", func() {
			pol, err = c.NetworkPolicyToPolicy(&np)
			Expect(err).NotTo(HaveOccurred())
			Expect(pol.Key.(model.ResourceKey).Name).To(Equal("knp.default.default.testPolicy"))
			Expect(int(*pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Order)).To(Equal(1000))
		})

		By("having the correct endpoint selector", func() {
			Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Selector).To(Equal("calico/k8s_ns == 'default' && label == 'value'"))
		})

		By("having the correct peer selectors", func() {
			Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules)).To(Equal(2))

			// There should be no EgressRules
			Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.EgressRules)).To(Equal(0))

			// Check that Types field exists and has only 'ingress'
			Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Types)).To(Equal(1))
			Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Types[0]).To(Equal(apiv2.PolicyTypeIngress))

			Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules[0].Source.Selector).To(Equal("calico/k8s_ns == 'default' && k == 'v'"))
			Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules[1].Source.Selector).To(Equal("calico/k8s_ns == 'default' && k2 == 'v2'"))
		})
	})

	It("should parse a NetworkPolicy with multiple peers and ports", func() {
		tcp := kapiv1.ProtocolTCP
		udp := kapiv1.ProtocolUDP
		eighty := intstr.FromInt(80)
		ninety := intstr.FromInt(90)

		np := extensions.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testPolicy",
				Namespace: "default",
			},
			Spec: extensions.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{"label": "value"},
				},
				Ingress: []extensions.NetworkPolicyIngressRule{
					extensions.NetworkPolicyIngressRule{
						Ports: []extensions.NetworkPolicyPort{
							extensions.NetworkPolicyPort{
								Port:     &ninety,
								Protocol: &udp,
							},
							extensions.NetworkPolicyPort{
								Port:     &eighty,
								Protocol: &tcp,
							},
						},
						From: []extensions.NetworkPolicyPeer{
							extensions.NetworkPolicyPeer{
								PodSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"k": "v",
									},
								},
							},
							extensions.NetworkPolicyPeer{
								PodSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"k2": "v2",
									},
								},
							},
						},
					},
				},
				PolicyTypes: []extensions.PolicyType{extensions.PolicyTypeIngress},
			},
		}

		var pol *model.KVPair
		var err error
		By("parsing the policy", func() {
			pol, err = c.NetworkPolicyToPolicy(&np)
			Expect(err).NotTo(HaveOccurred())
			Expect(pol.Key.(model.ResourceKey).Name).To(Equal("knp.default.default.testPolicy"))
			Expect(int(*pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Order)).To(Equal(1000))
		})

		By("having the correct endpoint selector", func() {
			Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Selector).To(Equal("calico/k8s_ns == 'default' && label == 'value'"))
		})

		By("having the correct peer selectors", func() {
			eighty, _ := numorstring.PortFromString("80")
			ninety, _ := numorstring.PortFromString("90")
			Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules)).To(Equal(4))

			// There should be no EgressRules
			Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.EgressRules)).To(Equal(0))

			// Check that Types field exists and has only 'ingress'
			Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Types)).To(Equal(1))
			Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Types[0]).To(Equal(apiv2.PolicyTypeIngress))

			Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules[0].Source.Selector).To(Equal("calico/k8s_ns == 'default' && k == 'v'"))
			Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules[0].Destination.Ports).To(Equal([]numorstring.Port{ninety}))

			Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules[1].Source.Selector).To(Equal("calico/k8s_ns == 'default' && k2 == 'v2'"))
			Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules[1].Destination.Ports).To(Equal([]numorstring.Port{ninety}))

			Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules[2].Source.Selector).To(Equal("calico/k8s_ns == 'default' && k == 'v'"))
			Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules[2].Destination.Ports).To(Equal([]numorstring.Port{eighty}))

			Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules[3].Source.Selector).To(Equal("calico/k8s_ns == 'default' && k2 == 'v2'"))
			Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules[3].Destination.Ports).To(Equal([]numorstring.Port{eighty}))
		})
	})

	It("should parse a NetworkPolicy with empty podSelector", func() {
		np := extensions.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testPolicy",
				Namespace: "default",
			},
			Spec: extensions.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{},
				PolicyTypes: []extensions.PolicyType{extensions.PolicyTypeIngress},
			},
		}

		// Parse the policy.
		pol, err := c.NetworkPolicyToPolicy(&np)
		Expect(err).NotTo(HaveOccurred())

		// Assert key fields are correct.
		Expect(pol.Key.(model.ResourceKey).Name).To(Equal("knp.default.default.testPolicy"))

		// Assert value fields are correct.
		Expect(int(*pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Order)).To(Equal(1000))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Selector).To(Equal("calico/k8s_ns == 'default'"))
		Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules)).To(Equal(0))

		// There should be no EgressRules
		Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.EgressRules)).To(Equal(0))

		// Check that Types field exists and has only 'ingress'
		Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Types)).To(Equal(1))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Types[0]).To(Equal(apiv2.PolicyTypeIngress))
	})

	It("should parse a NetworkPolicy with an empty namespaceSelector", func() {
		np := extensions.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testPolicy",
				Namespace: "default",
			},
			Spec: extensions.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{"label": "value"},
				},
				Ingress: []extensions.NetworkPolicyIngressRule{
					extensions.NetworkPolicyIngressRule{
						From: []extensions.NetworkPolicyPeer{
							extensions.NetworkPolicyPeer{
								NamespaceSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{},
								},
							},
						},
					},
				},
				PolicyTypes: []extensions.PolicyType{extensions.PolicyTypeIngress},
			},
		}

		// Parse the policy.
		pol, err := c.NetworkPolicyToPolicy(&np)
		Expect(err).NotTo(HaveOccurred())

		// Assert key fields are correct.
		Expect(pol.Key.(model.ResourceKey).Name).To(Equal("knp.default.default.testPolicy"))

		// Assert value fields are correct.
		Expect(int(*pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Order)).To(Equal(1000))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Selector).To(Equal("calico/k8s_ns == 'default' && label == 'value'"))
		Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules)).To(Equal(1))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules[0].Source.Selector).To(Equal("has(calico/k8s_ns)"))

		// There should be no EgressRules
		Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.EgressRules)).To(Equal(0))

		// Check that Types field exists and has only 'ingress'
		Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Types)).To(Equal(1))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Types[0]).To(Equal(apiv2.PolicyTypeIngress))
	})

	It("should parse a NetworkPolicy with podSelector.MatchExpressions", func() {
		np := extensions.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testPolicy",
				Namespace: "default",
			},
			Spec: extensions.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						metav1.LabelSelectorRequirement{
							Key:      "k",
							Operator: metav1.LabelSelectorOpIn,
							Values:   []string{"v1", "v2"},
						},
					},
				},
				PolicyTypes: []extensions.PolicyType{extensions.PolicyTypeIngress},
			},
		}

		// Parse the policy.
		pol, err := c.NetworkPolicyToPolicy(&np)
		Expect(err).NotTo(HaveOccurred())

		// Assert key fields are correct.
		Expect(pol.Key.(model.ResourceKey).Name).To(Equal("knp.default.default.testPolicy"))

		// Assert value fields are correct.
		Expect(int(*pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Order)).To(Equal(1000))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Selector).To(Equal("calico/k8s_ns == 'default' && k in { 'v1', 'v2' }"))
		Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules)).To(Equal(0))

		// There should be no EgressRules
		Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.EgressRules)).To(Equal(0))

		// Check that Types field exists and has only 'ingress'
		Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Types)).To(Equal(1))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Types[0]).To(Equal(apiv2.PolicyTypeIngress))
	})

	It("should parse a NetworkPolicy with Ports only", func() {
		protocol := kapiv1.ProtocolTCP
		port := intstr.FromInt(80)
		np := extensions.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testPolicy",
				Namespace: "default",
			},
			Spec: extensions.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{},
				Ingress: []extensions.NetworkPolicyIngressRule{
					extensions.NetworkPolicyIngressRule{
						Ports: []extensions.NetworkPolicyPort{
							extensions.NetworkPolicyPort{
								Protocol: &protocol,
								Port:     &port,
							},
						},
					},
				},
				PolicyTypes: []extensions.PolicyType{extensions.PolicyTypeIngress},
			},
		}

		// Parse the policy.
		pol, err := c.NetworkPolicyToPolicy(&np)
		Expect(err).NotTo(HaveOccurred())

		// Assert key fields are correct.
		Expect(pol.Key.(model.ResourceKey).Name).To(Equal("knp.default.default.testPolicy"))

		// Assert value fields are correct.
		Expect(int(*pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Order)).To(Equal(1000))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Selector).To(Equal("calico/k8s_ns == 'default'"))
		Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules)).To(Equal(1))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules[0].Protocol.String()).To(Equal("tcp"))
		Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules[0].Destination.Ports)).To(Equal(1))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules[0].Destination.Ports[0].String()).To(Equal("80"))

		// There should be no EgressRules
		Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.EgressRules)).To(Equal(0))

		// Check that Types field exists and has only 'ingress'
		Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Types)).To(Equal(1))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Types[0]).To(Equal(apiv2.PolicyTypeIngress))
	})

	It("should parse a NetworkPolicy with an Ingress rule with an IPBlock Peer", func() {
		np := extensions.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testPolicy",
				Namespace: "default",
			},
			Spec: extensions.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{},
				Ingress: []extensions.NetworkPolicyIngressRule{
					{
						From: []extensions.NetworkPolicyPeer{
							{
								IPBlock: &extensions.IPBlock{
									CIDR:   "192.168.0.0/16",
									Except: []string{"192.168.3.0/24", "192.168.4.0/24"},
								},
							},
						},
					},
				},
				PolicyTypes: []extensions.PolicyType{extensions.PolicyTypeIngress},
			},
		}

		// Parse the policy.
		pol, err := c.NetworkPolicyToPolicy(&np)
		Expect(err).NotTo(HaveOccurred())

		// Assert key fields are correct.
		Expect(pol.Key.(model.ResourceKey).Name).To(Equal("knp.default.default.testPolicy"))

		// Assert value fields are correct.
		Expect(int(*pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Order)).To(Equal(1000))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Selector).To(Equal("calico/k8s_ns == 'default'"))
		Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules)).To(Equal(1))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules[0].Source.Nets[0]).To(Equal("192.168.0.0/16"))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules[0].Source.NotNets[0]).To(Equal("192.168.3.0/24"))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules[0].Source.NotNets[1]).To(Equal("192.168.4.0/24"))

		// There should be no EgressRules
		Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.EgressRules)).To(Equal(0))

		// Check that Types field exists and has only 'ingress'
		Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Types)).To(Equal(1))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Types[0]).To(Equal(apiv2.PolicyTypeIngress))
	})

	It("should parse a NetworkPolicy with an Egress rule with an IPBlock Peer", func() {
		np := extensions.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testPolicy",
				Namespace: "default",
			},
			Spec: extensions.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{},
				Egress: []extensions.NetworkPolicyEgressRule{
					{
						To: []extensions.NetworkPolicyPeer{
							{
								IPBlock: &extensions.IPBlock{
									CIDR:   "192.168.0.0/16",
									Except: []string{"192.168.3.0/24", "192.168.4.0/24", "192.168.5.0/24"},
								},
							},
						},
					},
				},
				PolicyTypes: []extensions.PolicyType{extensions.PolicyTypeEgress},
			},
		}

		// Parse the policy.
		pol, err := c.NetworkPolicyToPolicy(&np)
		Expect(err).NotTo(HaveOccurred())

		// Assert key fields are correct.
		Expect(pol.Key.(model.ResourceKey).Name).To(Equal("knp.default.default.testPolicy"))

		// Assert value fields are correct.
		Expect(int(*pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Order)).To(Equal(1000))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Selector).To(Equal("calico/k8s_ns == 'default'"))
		Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.EgressRules)).To(Equal(1))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.EgressRules[0].Destination.Nets[0]).To(Equal("192.168.0.0/16"))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.EgressRules[0].Destination.NotNets[0]).To(Equal("192.168.3.0/24"))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.EgressRules[0].Destination.NotNets[1]).To(Equal("192.168.4.0/24"))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.EgressRules[0].Destination.NotNets[2]).To(Equal("192.168.5.0/24"))

		// There should be no IngressRules
		Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules)).To(Equal(0))

		// Check that Types field exists and has only 'egress'
		Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Types)).To(Equal(1))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Types[0]).To(Equal(apiv2.PolicyTypeEgress))
	})

	It("should parse a NetworkPolicy with an Egress rule with IPBlock and Ports", func() {
		tcp := kapiv1.ProtocolTCP
		udp := kapiv1.ProtocolUDP
		eighty := intstr.FromInt(80)
		ninety := intstr.FromInt(90)

		np := extensions.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testPolicy",
				Namespace: "default",
			},
			Spec: extensions.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{},
				Egress: []extensions.NetworkPolicyEgressRule{
					extensions.NetworkPolicyEgressRule{
						Ports: []extensions.NetworkPolicyPort{
							extensions.NetworkPolicyPort{
								Port:     &ninety,
								Protocol: &udp,
							},
							extensions.NetworkPolicyPort{
								Port:     &eighty,
								Protocol: &tcp,
							},
						},
						To: []extensions.NetworkPolicyPeer{
							{
								IPBlock: &extensions.IPBlock{
									CIDR:   "192.168.0.0/16",
									Except: []string{"192.168.3.0/24", "192.168.4.0/24", "192.168.5.0/24"},
								},
							},
						},
					},
				},
				PolicyTypes: []extensions.PolicyType{extensions.PolicyTypeEgress},
			},
		}

		// Parse the policy.
		pol, err := c.NetworkPolicyToPolicy(&np)
		Expect(err).NotTo(HaveOccurred())

		// Assert key fields are correct.
		Expect(pol.Key.(model.ResourceKey).Name).To(Equal("knp.default.default.testPolicy"))

		eightyName, _ := numorstring.PortFromString("80")
		ninetyName, _ := numorstring.PortFromString("90")

		// Assert value fields are correct.
		Expect(int(*pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Order)).To(Equal(1000))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Selector).To(Equal("calico/k8s_ns == 'default'"))
		Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.EgressRules)).To(Equal(2))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.EgressRules[0].Destination.Ports).To(Equal([]numorstring.Port{ninetyName}))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.EgressRules[0].Destination.Nets[0]).To(Equal("192.168.0.0/16"))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.EgressRules[0].Destination.NotNets[0]).To(Equal("192.168.3.0/24"))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.EgressRules[0].Destination.NotNets[1]).To(Equal("192.168.4.0/24"))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.EgressRules[0].Destination.NotNets[2]).To(Equal("192.168.5.0/24"))

		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.EgressRules[1].Destination.Ports).To(Equal([]numorstring.Port{eightyName}))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.EgressRules[1].Destination.Nets[0]).To(Equal("192.168.0.0/16"))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.EgressRules[1].Destination.NotNets[0]).To(Equal("192.168.3.0/24"))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.EgressRules[1].Destination.NotNets[1]).To(Equal("192.168.4.0/24"))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.EgressRules[1].Destination.NotNets[2]).To(Equal("192.168.5.0/24"))

		// There should be no IngressRules
		Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules)).To(Equal(0))

		// Check that Types field exists and has only 'egress'
		Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Types)).To(Equal(1))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Types[0]).To(Equal(apiv2.PolicyTypeEgress))
	})

	It("should parse a NetworkPolicy with both an Egress and an Ingress rule", func() {
		np := extensions.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testPolicy",
				Namespace: "default",
			},
			Spec: extensions.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{},
				Ingress: []extensions.NetworkPolicyIngressRule{
					{
						From: []extensions.NetworkPolicyPeer{
							{
								IPBlock: &extensions.IPBlock{
									CIDR:   "192.168.0.0/16",
									Except: []string{"192.168.3.0/24", "192.168.4.0/24"},
								},
							},
						},
					},
				},
				Egress: []extensions.NetworkPolicyEgressRule{
					{
						To: []extensions.NetworkPolicyPeer{
							{
								IPBlock: &extensions.IPBlock{
									CIDR:   "10.10.0.0/16",
									Except: []string{"192.168.13.0/24", "192.168.14.0/24", "192.168.15.0/24"},
								},
							},
						},
					},
				},
				PolicyTypes: []extensions.PolicyType{extensions.PolicyTypeEgress, extensions.PolicyTypeIngress},
			},
		}

		pol, err := c.NetworkPolicyToPolicy(&np)
		Expect(err).NotTo(HaveOccurred())

		// Assert key fields are correct.
		Expect(pol.Key.(model.ResourceKey).Name).To(Equal("knp.default.default.testPolicy"))

		// Assert value fields are correct.
		Expect(int(*pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Order)).To(Equal(1000))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Selector).To(Equal("calico/k8s_ns == 'default'"))
		Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.EgressRules)).To(Equal(1))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.EgressRules[0].Destination.Nets[0]).To(Equal("10.10.0.0/16"))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.EgressRules[0].Destination.NotNets[0]).To(Equal("192.168.13.0/24"))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.EgressRules[0].Destination.NotNets[1]).To(Equal("192.168.14.0/24"))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.EgressRules[0].Destination.NotNets[2]).To(Equal("192.168.15.0/24"))

		// There should be one InboundRule
		Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules)).To(Equal(1))

		// Assert InboundRule fields are correct.
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules[0].Source.Nets[0]).To(Equal("192.168.0.0/16"))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules[0].Source.NotNets[0]).To(Equal("192.168.3.0/24"))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules[0].Source.NotNets[1]).To(Equal("192.168.4.0/24"))

		// Check that Types field exists and has both 'egress' and 'ingress'
		Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Types)).To(Equal(2))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Types[0]).To(Equal(apiv2.PolicyTypeIngress))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Types[1]).To(Equal(apiv2.PolicyTypeEgress))
	})
})

// This suite of tests is useful for ensuring we continue to support kubernetes apiserver
// versions <= 1.7.x, and can be removed when that is no longer required.
var _ = Describe("Test NetworkPolicy conversion (k8s <= 1.7, no policyTypes)", func() {

	// Use a single instance of the Converter for these tests.
	c := Converter{}

	It("should parse a basic NetworkPolicy to a Policy", func() {
		port80 := intstr.FromInt(80)
		np := extensions.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testPolicy",
				Namespace: "default",
			},
			Spec: extensions.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"label":  "value",
						"label2": "value2",
					},
				},
				Ingress: []extensions.NetworkPolicyIngressRule{
					{
						Ports: []extensions.NetworkPolicyPort{
							{Port: &port80},
						},
						From: []extensions.NetworkPolicyPeer{
							{
								PodSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"k":  "v",
										"k2": "v2",
									},
								},
							},
						},
					},
				},
			},
		}

		// Parse the policy.
		pol, err := c.NetworkPolicyToPolicy(&np)
		Expect(err).NotTo(HaveOccurred())

		// Assert key fields are correct.
		Expect(pol.Key.(model.ResourceKey).Name).To(Equal("knp.default.default.testPolicy"))

		// Assert value fields are correct.
		Expect(int(*pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Order)).To(Equal(1000))
		// Check the selector is correct, and that the matches are sorted.
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Selector).To(Equal(
			"calico/k8s_ns == 'default' && label == 'value' && label2 == 'value2'"))
		protoTCP := numorstring.ProtocolFromString("tcp")
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules).To(ConsistOf(apiv2.Rule{
			Action:   "allow",
			Protocol: &protoTCP, // Defaulted to TCP.
			Source: apiv2.EntityRule{
				Selector: "calico/k8s_ns == 'default' && k == 'v' && k2 == 'v2'",
			},
			Destination: apiv2.EntityRule{
				Ports: []numorstring.Port{numorstring.SinglePort(80)},
			},
		}))

		// There should be no EgressRules
		Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.EgressRules)).To(Equal(0))

		// Check that Types field exists and has only 'ingress'
		Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Types)).To(Equal(1))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Types[0]).To(Equal(apiv2.PolicyTypeIngress))
	})

	It("should parse a NetworkPolicy with no rules", func() {
		np := extensions.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testPolicy",
				Namespace: "default",
			},
			Spec: extensions.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{"label": "value"},
				},
			},
		}

		// Parse the policy.
		pol, err := c.NetworkPolicyToPolicy(&np)
		Expect(err).NotTo(HaveOccurred())

		// Assert key fields are correct.
		Expect(pol.Key.(model.ResourceKey).Name).To(Equal("knp.default.default.testPolicy"))

		// Assert value fields are correct.
		Expect(int(*pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Order)).To(Equal(1000))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Selector).To(Equal("calico/k8s_ns == 'default' && label == 'value'"))
		Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules)).To(Equal(0))

		// There should be no EgressRules
		Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.EgressRules)).To(Equal(0))

		// Check that Types field exists and has only 'ingress'
		Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Types)).To(Equal(1))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Types[0]).To(Equal(apiv2.PolicyTypeIngress))
	})

	It("should parse a NetworkPolicy with multiple peers", func() {
		np := extensions.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testPolicy",
				Namespace: "default",
			},
			Spec: extensions.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{"label": "value"},
				},
				Ingress: []extensions.NetworkPolicyIngressRule{
					extensions.NetworkPolicyIngressRule{
						Ports: []extensions.NetworkPolicyPort{
							extensions.NetworkPolicyPort{},
						},
						From: []extensions.NetworkPolicyPeer{
							extensions.NetworkPolicyPeer{
								PodSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"k": "v",
									},
								},
							},
							extensions.NetworkPolicyPeer{
								PodSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"k2": "v2",
									},
								},
							},
						},
					},
				},
			},
		}

		var pol *model.KVPair
		var err error
		By("parsing the policy", func() {
			pol, err = c.NetworkPolicyToPolicy(&np)
			Expect(err).NotTo(HaveOccurred())
			Expect(pol.Key.(model.ResourceKey).Name).To(Equal("knp.default.default.testPolicy"))
			Expect(int(*pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Order)).To(Equal(1000))
		})

		By("having the correct endpoint selector", func() {
			Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Selector).To(Equal("calico/k8s_ns == 'default' && label == 'value'"))
		})

		By("having the correct peer selectors", func() {
			Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules)).To(Equal(2))

			// There should be no EgressRules
			Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.EgressRules)).To(Equal(0))

			// Check that Types field exists and has only 'ingress'
			Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Types)).To(Equal(1))
			Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Types[0]).To(Equal(apiv2.PolicyTypeIngress))

			Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules[0].Source.Selector).To(Equal("calico/k8s_ns == 'default' && k == 'v'"))
			Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules[1].Source.Selector).To(Equal("calico/k8s_ns == 'default' && k2 == 'v2'"))
		})
	})

	It("should parse a NetworkPolicy with multiple peers and ports", func() {
		tcp := kapiv1.ProtocolTCP
		udp := kapiv1.ProtocolUDP
		eighty := intstr.FromInt(80)
		ninety := intstr.FromInt(90)

		np := extensions.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testPolicy",
				Namespace: "default",
			},
			Spec: extensions.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{"label": "value"},
				},
				Ingress: []extensions.NetworkPolicyIngressRule{
					extensions.NetworkPolicyIngressRule{
						Ports: []extensions.NetworkPolicyPort{
							extensions.NetworkPolicyPort{
								Port:     &ninety,
								Protocol: &udp,
							},
							extensions.NetworkPolicyPort{
								Port:     &eighty,
								Protocol: &tcp,
							},
						},
						From: []extensions.NetworkPolicyPeer{
							extensions.NetworkPolicyPeer{
								PodSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"k": "v",
									},
								},
							},
							extensions.NetworkPolicyPeer{
								PodSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"k2": "v2",
									},
								},
							},
						},
					},
				},
			},
		}

		var pol *model.KVPair
		var err error
		By("parsing the policy", func() {
			pol, err = c.NetworkPolicyToPolicy(&np)
			Expect(err).NotTo(HaveOccurred())
			Expect(pol.Key.(model.ResourceKey).Name).To(Equal("knp.default.default.testPolicy"))
			Expect(int(*pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Order)).To(Equal(1000))
		})

		By("having the correct endpoint selector", func() {
			Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Selector).To(Equal("calico/k8s_ns == 'default' && label == 'value'"))
		})

		By("having the correct peer selectors", func() {
			eighty, _ := numorstring.PortFromString("80")
			ninety, _ := numorstring.PortFromString("90")
			Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules)).To(Equal(4))

			// There should be no EgressRules
			Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.EgressRules)).To(Equal(0))

			// Check that Types field exists and has only 'ingress'
			Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Types)).To(Equal(1))
			Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Types[0]).To(Equal(apiv2.PolicyTypeIngress))

			Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules[0].Source.Selector).To(Equal("calico/k8s_ns == 'default' && k == 'v'"))
			Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules[0].Destination.Ports).To(Equal([]numorstring.Port{ninety}))

			Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules[1].Source.Selector).To(Equal("calico/k8s_ns == 'default' && k2 == 'v2'"))
			Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules[1].Destination.Ports).To(Equal([]numorstring.Port{ninety}))

			Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules[2].Source.Selector).To(Equal("calico/k8s_ns == 'default' && k == 'v'"))
			Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules[2].Destination.Ports).To(Equal([]numorstring.Port{eighty}))

			Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules[3].Source.Selector).To(Equal("calico/k8s_ns == 'default' && k2 == 'v2'"))
			Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules[3].Destination.Ports).To(Equal([]numorstring.Port{eighty}))
		})
	})

	It("should parse a NetworkPolicy with empty podSelector", func() {
		np := extensions.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testPolicy",
				Namespace: "default",
			},
			Spec: extensions.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{},
			},
		}

		// Parse the policy.
		pol, err := c.NetworkPolicyToPolicy(&np)
		Expect(err).NotTo(HaveOccurred())

		// Assert key fields are correct.
		Expect(pol.Key.(model.ResourceKey).Name).To(Equal("knp.default.default.testPolicy"))

		// Assert value fields are correct.
		Expect(int(*pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Order)).To(Equal(1000))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Selector).To(Equal("calico/k8s_ns == 'default'"))
		Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules)).To(Equal(0))

		// There should be no EgressRules
		Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.EgressRules)).To(Equal(0))

		// Check that Types field exists and has only 'ingress'
		Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Types)).To(Equal(1))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Types[0]).To(Equal(apiv2.PolicyTypeIngress))
	})

	It("should parse a NetworkPolicy with an empty namespaceSelector", func() {
		np := extensions.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testPolicy",
				Namespace: "default",
			},
			Spec: extensions.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{"label": "value"},
				},
				Ingress: []extensions.NetworkPolicyIngressRule{
					extensions.NetworkPolicyIngressRule{
						From: []extensions.NetworkPolicyPeer{
							extensions.NetworkPolicyPeer{
								NamespaceSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{},
								},
							},
						},
					},
				},
			},
		}

		// Parse the policy.
		pol, err := c.NetworkPolicyToPolicy(&np)
		Expect(err).NotTo(HaveOccurred())

		// Assert key fields are correct.
		Expect(pol.Key.(model.ResourceKey).Name).To(Equal("knp.default.default.testPolicy"))

		// Assert value fields are correct.
		Expect(int(*pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Order)).To(Equal(1000))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Selector).To(Equal("calico/k8s_ns == 'default' && label == 'value'"))
		Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules)).To(Equal(1))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules[0].Source.Selector).To(Equal("has(calico/k8s_ns)"))

		// There should be no EgressRules
		Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.EgressRules)).To(Equal(0))

		// Check that Types field exists and has only 'ingress'
		Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Types)).To(Equal(1))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Types[0]).To(Equal(apiv2.PolicyTypeIngress))
	})

	It("should parse a NetworkPolicy with podSelector.MatchExpressions", func() {
		np := extensions.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testPolicy",
				Namespace: "default",
			},
			Spec: extensions.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						metav1.LabelSelectorRequirement{
							Key:      "k",
							Operator: metav1.LabelSelectorOpIn,
							Values:   []string{"v1", "v2"},
						},
					},
				},
			},
		}

		// Parse the policy.
		pol, err := c.NetworkPolicyToPolicy(&np)
		Expect(err).NotTo(HaveOccurred())

		// Assert key fields are correct.
		Expect(pol.Key.(model.ResourceKey).Name).To(Equal("knp.default.default.testPolicy"))

		// Assert value fields are correct.
		Expect(int(*pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Order)).To(Equal(1000))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Selector).To(Equal("calico/k8s_ns == 'default' && k in { 'v1', 'v2' }"))
		Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules)).To(Equal(0))

		// There should be no EgressRules
		Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.EgressRules)).To(Equal(0))

		// Check that Types field exists and has only 'ingress'
		Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Types)).To(Equal(1))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Types[0]).To(Equal(apiv2.PolicyTypeIngress))
	})

	It("should parse a NetworkPolicy with Ports only", func() {
		protocol := kapiv1.ProtocolTCP
		port := intstr.FromInt(80)
		np := extensions.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testPolicy",
				Namespace: "default",
			},
			Spec: extensions.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{},
				Ingress: []extensions.NetworkPolicyIngressRule{
					extensions.NetworkPolicyIngressRule{
						Ports: []extensions.NetworkPolicyPort{
							extensions.NetworkPolicyPort{
								Protocol: &protocol,
								Port:     &port,
							},
						},
					},
				},
			},
		}

		// Parse the policy.
		pol, err := c.NetworkPolicyToPolicy(&np)
		Expect(err).NotTo(HaveOccurred())

		// Assert key fields are correct.
		Expect(pol.Key.(model.ResourceKey).Name).To(Equal("knp.default.default.testPolicy"))

		// Assert value fields are correct.
		Expect(int(*pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Order)).To(Equal(1000))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Selector).To(Equal("calico/k8s_ns == 'default'"))
		Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules)).To(Equal(1))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules[0].Protocol.String()).To(Equal("tcp"))
		Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules[0].Destination.Ports)).To(Equal(1))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.IngressRules[0].Destination.Ports[0].String()).To(Equal("80"))

		// There should be no EgressRules
		Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.EgressRules)).To(Equal(0))

		// Check that Types field exists and has only 'ingress'
		Expect(len(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Types)).To(Equal(1))
		Expect(pol.Value.(*apiv2.GlobalNetworkPolicy).Spec.Types[0]).To(Equal(apiv2.PolicyTypeIngress))
	})
})

var _ = Describe("Test Namespace conversion", func() {

	// Use a single instance of the Converter for these tests.
	c := Converter{}

	It("should parse a Namespace to a Profile", func() {
		ns := kapiv1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "default",
				Labels: map[string]string{
					"foo":   "bar",
					"roger": "rabbit",
				},
				Annotations: map[string]string{},
			},
			Spec: kapiv1.NamespaceSpec{},
		}

		p, err := c.NamespaceToProfile(&ns)
		Expect(err).NotTo(HaveOccurred())

		Expect(p.Key.(model.ResourceKey).Name).To(Equal("k8s_ns.default"))
		Expect(p.Key.(model.ResourceKey).Kind).To(Equal(apiv2.KindProfile))

		// Ensure rules are correct for profile.
		ingressRules := p.Value.(*apiv2.Profile).Spec.IngressRules
		egressRules := p.Value.(*apiv2.Profile).Spec.EgressRules
		Expect(len(ingressRules)).To(Equal(1))
		Expect(len(egressRules)).To(Equal(1))

		// Ensure both inbound and outbound rules are set to allow.
		Expect(ingressRules[0]).To(Equal(apiv2.Rule{Action: apiv2.Allow}))
		Expect(egressRules[0]).To(Equal(apiv2.Rule{Action: apiv2.Allow}))

		// Check labels.
		labels := p.Value.(*apiv2.Profile).Labels
		Expect(labels["pcns.foo"]).To(Equal("bar"))
		Expect(labels["pcns.roger"]).To(Equal("rabbit"))
	})

	It("should parse a Namespace to a Profile with no labels", func() {
		ns := kapiv1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "default",
				Annotations: map[string]string{},
			},
			Spec: kapiv1.NamespaceSpec{},
		}

		p, err := c.NamespaceToProfile(&ns)
		Expect(err).NotTo(HaveOccurred())

		// Ensure rules are correct.
		ingressRules := p.Value.(*apiv2.Profile).Spec.IngressRules
		egressRules := p.Value.(*apiv2.Profile).Spec.EgressRules
		Expect(len(ingressRules)).To(Equal(1))
		Expect(len(egressRules)).To(Equal(1))

		// Ensure both inbound and outbound rules are set to allow.
		Expect(ingressRules[0]).To(Equal(apiv2.Rule{Action: apiv2.Allow}))
		Expect(egressRules[0]).To(Equal(apiv2.Rule{Action: apiv2.Allow}))

		// Check labels.
		labels := p.Value.(*apiv2.Profile).ObjectMeta.Labels
		Expect(len(labels)).To(Equal(0))
	})

	It("should ignore the network-policy Namespace annotation", func() {
		ns := kapiv1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "default",
				Annotations: map[string]string{
					"net.beta.kubernetes.io/network-policy": "{\"ingress\": {\"isolation\": \"DefaultDeny\"}}",
				},
			},
			Spec: kapiv1.NamespaceSpec{},
		}

		// Ensure it generates the correct Profile.
		p, err := c.NamespaceToProfile(&ns)
		Expect(err).NotTo(HaveOccurred())
		// Ensure rules are correct for profile.
		ingressRules := p.Value.(*apiv2.Profile).Spec.IngressRules
		egressRules := p.Value.(*apiv2.Profile).Spec.EgressRules
		Expect(len(ingressRules)).To(Equal(1))
		Expect(len(egressRules)).To(Equal(1))

		// Ensure both inbound and outbound rules are set to allow.
		Expect(ingressRules[0]).To(Equal(apiv2.Rule{Action: apiv2.Allow}))
		Expect(egressRules[0]).To(Equal(apiv2.Rule{Action: apiv2.Allow}))

	})

	It("should not fail for malformed annotation", func() {
		ns := kapiv1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "default",
				Annotations: map[string]string{
					"net.beta.kubernetes.io/network-policy": "invalidJSON",
				},
			},
			Spec: kapiv1.NamespaceSpec{},
		}

		By("converting to a Profile", func() {
			_, err := c.NamespaceToProfile(&ns)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	It("should handle a valid but not DefaultDeny annotation", func() {
		ns := kapiv1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "default",
				Annotations: map[string]string{
					"net.beta.kubernetes.io/network-policy": "{}",
				},
			},
			Spec: kapiv1.NamespaceSpec{},
		}

		By("converting to a Profile", func() {
			p, err := c.NamespaceToProfile(&ns)
			Expect(err).NotTo(HaveOccurred())

			// Ensure rules are correct.
			ingressRules := p.Value.(*apiv2.Profile).Spec.IngressRules
			egressRules := p.Value.(*apiv2.Profile).Spec.EgressRules
			Expect(len(ingressRules)).To(Equal(1))
			Expect(len(egressRules)).To(Equal(1))

			// Ensure both inbound and outbound rules are set to allow.
			Expect(ingressRules[0]).To(Equal(apiv2.Rule{Action: apiv2.Allow}))
			Expect(egressRules[0]).To(Equal(apiv2.Rule{Action: apiv2.Allow}))
		})
	})
})
