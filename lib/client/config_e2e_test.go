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

// Test operations involving the config API.  This tests unset values,
// default value setting and getting, node specific value settings and getting
// and per-node inherited values.  See code for more details.

package client_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/projectcalico/libcalico-go/lib/client"
	"github.com/projectcalico/libcalico-go/lib/net"
	"github.com/projectcalico/libcalico-go/lib/testutils"
)

var _ = Describe("with config option API tests", func() {

	var config client.ConfigInterface

	BeforeEach(func() {
		// Erase etcd clean
		testutils.CleanEtcd()

		c, err := testutils.NewClient("")
		Expect(err).NotTo(HaveOccurred())
		config = c.Config()
	})

	It("should handle default and inherited per node configuration", func() {
		var err error
		var l string
		var cs client.ConfigLocation

		By("checking default unset value")
		l, err = config.GetGlobalLogLevel()
		Expect(err).NotTo(HaveOccurred())
		Expect(l).To(Equal("info"))

		By("checking node unset value")
		l, cs, err = config.GetNodeLogLevel("testnode")
		Expect(err).NotTo(HaveOccurred())
		Expect(l).To(Equal("info"))
		Expect(cs).To(Equal(client.ConfigLocationGlobal))

		By("checking default set value to None")
		err = config.SetGlobalLogLevel("none")
		Expect(err).NotTo(HaveOccurred())

		l, err = config.GetGlobalLogLevel()
		Expect(err).NotTo(HaveOccurred())
		Expect(l).To(Equal("none"))

		By("checking default set value to Warning")
		err = config.SetGlobalLogLevel("warning")
		Expect(err).NotTo(HaveOccurred())

		l, err = config.GetGlobalLogLevel()
		Expect(err).NotTo(HaveOccurred())
		Expect(l).To(Equal("warning"))

		By("checking node set value")
		err = config.SetNodeLogLevel("testnode", "critical")
		Expect(err).NotTo(HaveOccurred())

		l, cs, err = config.GetNodeLogLevel("testnode")
		Expect(err).NotTo(HaveOccurred())
		Expect(l).To(Equal("critical"))
		Expect(cs).To(Equal(client.ConfigLocationNode))

		By("checking node use default value")
		err = config.SetNodeLogLevelUseGlobal("testnode")
		Expect(err).NotTo(HaveOccurred())

		l, cs, err = config.GetNodeLogLevel("testnode")
		Expect(err).NotTo(HaveOccurred())
		Expect(l).To(Equal("warning"))
		Expect(cs).To(Equal(client.ConfigLocationGlobal))
	})

	It("should handle node to node mesh configuration", func() {
		var err error
		var n bool

		By("checking default unset value")
		n, err = config.GetNodeToNodeMesh()
		Expect(err).NotTo(HaveOccurred())
		Expect(n).To(Equal(true))

		By("checking default set value to true")
		err = config.SetNodeToNodeMesh(true)
		Expect(err).NotTo(HaveOccurred())

		n, err = config.GetNodeToNodeMesh()
		Expect(err).NotTo(HaveOccurred())
		Expect(n).To(Equal(true))

		By("checking default set value to false")
		err = config.SetNodeToNodeMesh(false)
		Expect(err).NotTo(HaveOccurred())

		n, err = config.GetNodeToNodeMesh()
		Expect(err).NotTo(HaveOccurred())
		Expect(n).To(Equal(false))
	})

	It("should handle default AS number configuration", func() {
		var err error
		var asn uint64

		By("checking default unset value")
		asn, err = config.GetGlobalASNumber()
		Expect(err).NotTo(HaveOccurred())
		Expect(asn).To(Equal(uint64(64511)))

		By("checking default set value to true")
		err = config.SetGlobalASNumber(11111)
		Expect(err).NotTo(HaveOccurred())

		asn, err = config.GetGlobalASNumber()
		Expect(err).NotTo(HaveOccurred())
		Expect(asn).To(Equal(uint64(11111)))
	})

	It("should handle default IP in IP configuration", func() {
		var err error
		var n bool

		By("checking default unset value")
		n, err = config.GetGlobalIPIP()
		Expect(err).NotTo(HaveOccurred())
		Expect(n).To(Equal(false))

		By("checking default set value to true")
		err = config.SetGlobalIPIP(true)
		Expect(err).NotTo(HaveOccurred())

		n, err = config.GetGlobalIPIP()
		Expect(err).NotTo(HaveOccurred())
		Expect(n).To(Equal(true))

		By("checking default set value to false")
		err = config.SetGlobalIPIP(false)
		Expect(err).NotTo(HaveOccurred())

		n, err = config.GetGlobalIPIP()
		Expect(err).NotTo(HaveOccurred())
		Expect(n).To(Equal(false))
	})

	It("should handle node IP in IP tunnel address", func() {
		var err error
		var ip *net.IP

		By("checking unset value")
		ip, err = config.GetNodeIPIPTunnelAddress("node1")
		Expect(err).NotTo(HaveOccurred())
		Expect(ip).To(BeNil())

		By("checking address set to 1.2.3.4")
		ipv4 := testutils.MustParseIP("1.2.3.4")
		err = config.SetNodeIPIPTunnelAddress("node1", &ipv4)
		Expect(err).NotTo(HaveOccurred())

		ip, err = config.GetNodeIPIPTunnelAddress("node1")
		Expect(err).NotTo(HaveOccurred())
		Expect(*ip).To(Equal(ipv4))

		By("checking address set to aa::ff")
		ipv6 := testutils.MustParseIP("aa::ff")
		err = config.SetNodeIPIPTunnelAddress("node1", &ipv6)
		Expect(err).NotTo(HaveOccurred())

		ip, err = config.GetNodeIPIPTunnelAddress("node1")
		Expect(err).NotTo(HaveOccurred())
		Expect(*ip).To(Equal(ipv6))

		By("checking address set to nil")
		err = config.SetNodeIPIPTunnelAddress("node1", nil)
		Expect(err).NotTo(HaveOccurred())

		ip, err = config.GetNodeIPIPTunnelAddress("node1")
		Expect(err).NotTo(HaveOccurred())
		Expect(ip).To(BeNil())
	})
})
