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

package resources

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	apiv3 "github.com/projectcalico/libcalico-go/lib/apis/v3"

	k8sapi "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/projectcalico/libcalico-go/lib/net"
	"github.com/projectcalico/libcalico-go/lib/numorstring"
)

var _ = Describe("Test Node conversion", func() {

	It("should parse a k8s Node to a Calico Node", func() {
		l := map[string]string{"net.beta.kubernetes.io/role": "master"}
		node := k8sapi.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "TestNode",
				Labels:          l,
				ResourceVersion: "1234",
				Annotations: map[string]string{
					nodeBgpIpv4AddrAnnotation: "172.17.17.10",
					nodeBgpAsnAnnotation:      "2546",
				},
			},
			Status: k8sapi.NodeStatus{
				Addresses: []k8sapi.NodeAddress{
					k8sapi.NodeAddress{
						Type:    k8sapi.NodeInternalIP,
						Address: "172.17.17.10",
					},
					k8sapi.NodeAddress{
						Type:    k8sapi.NodeExternalIP,
						Address: "192.168.1.100",
					},
					k8sapi.NodeAddress{
						Type:    k8sapi.NodeHostName,
						Address: "172-17-17-10",
					},
				},
			},
			Spec: k8sapi.NodeSpec{
				PodCIDR: "10.0.0.1/24",
			},
		}

		n, err := K8sNodeToCalico(&node)
		Expect(err).NotTo(HaveOccurred())

		// Ensure we got the correct values.
		bgpIpv4Address := n.Value.(*apiv3.Node).Spec.BGP.IPv4Address
		ipInIpAddr := n.Value.(*apiv3.Node).Spec.BGP.IPv4IPIPTunnelAddr
		asn := n.Value.(*apiv3.Node).Spec.BGP.ASNumber

		ip := net.ParseIP("172.17.17.10")

		Expect(bgpIpv4Address).To(Equal(ip.String()))
		Expect(ipInIpAddr).To(Equal("10.0.0.2"))
		Expect(asn.String()).To(Equal("2546"))
	})

	It("Should parse and remove BGP info when given Calico Node with empty BGP spec", func() {
		l := map[string]string{"net.beta.kubernetes.io/role": "master"}
		k8sNode := &k8sapi.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "TestNode",
				Labels:          l,
				ResourceVersion: "1234",
				Annotations: map[string]string{
					nodeBgpIpv4AddrAnnotation: "172.17.17.10",
					nodeBgpAsnAnnotation:      "2546",
				},
			},
			Spec: k8sapi.NodeSpec{},
		}

		calicoNode := apiv3.NewNode()

		newK8sNode, err := mergeCalicoNodeIntoK8sNode(calicoNode, k8sNode)
		Expect(err).NotTo(HaveOccurred())
		Expect(newK8sNode.Annotations).NotTo(HaveKey(nodeBgpIpv4AddrAnnotation))
		Expect(newK8sNode.Annotations).NotTo(HaveKey(nodeBgpAsnAnnotation))
	})

	It("Should merge Calico Nodes into K8s Nodes", func() {
		kl := map[string]string{"net.beta.kubernetes.io/role": "master"}
		cl := map[string]string{
			"label1": "foo",
			"label2": "bar",
		}
		ca := map[string]string{
			"anno1": "foo",
			"anno2": "bar",
		}
		k8sNode := &k8sapi.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "TestNode",
				Labels:          kl,
				ResourceVersion: "1234",
				Annotations:     make(map[string]string),
			},
			Spec: k8sapi.NodeSpec{},
		}

		asn, _ := numorstring.ASNumberFromString("2456")

		By("Merging calico node config into the k8s node")
		calicoNode := apiv3.NewNode()
		calicoNode.Name = "TestNode"
		calicoNode.ResourceVersion = "1234"
		calicoNode.Labels = cl
		calicoNode.Annotations = ca
		calicoNode.Spec = apiv3.NodeSpec{
			BGP: &apiv3.NodeBGPSpec{
				IPv4Address: "172.17.17.10/24",
				IPv6Address: "aa:bb:cc::ffff/120",
				ASNumber:    &asn,
			},
		}

		newK8sNode, err := mergeCalicoNodeIntoK8sNode(calicoNode, k8sNode)
		Expect(err).NotTo(HaveOccurred())
		Expect(newK8sNode.Annotations).To(HaveKeyWithValue(nodeBgpIpv4AddrAnnotation, "172.17.17.10/24"))
		Expect(newK8sNode.Annotations).To(HaveKeyWithValue(nodeBgpIpv6AddrAnnotation, "aa:bb:cc::ffff/120"))
		Expect(newK8sNode.Annotations).To(HaveKeyWithValue(nodeBgpAsnAnnotation, "2456"))

		// The calico node annotations and labels should not have escaped directly into the node annotations
		// and labels.
		Expect(newK8sNode.Annotations["anno1"]).To(Equal(""))
		Expect(newK8sNode.Labels["label1"]).To(Equal(""))

		By("Converting the k8s node back into a calico node")
		// Set the PodCIDR so we can also test the IPIP tunnel field
		newK8sNode.Spec.PodCIDR = "172.100.0.0/24"
		calicoNode.Spec.BGP.IPv4IPIPTunnelAddr = "172.100.0.1"
		newCalicoNode, err := K8sNodeToCalico(newK8sNode)
		Expect(err).NotTo(HaveOccurred())
		Expect(newCalicoNode.Value).To(Equal(calicoNode))
	})
})
