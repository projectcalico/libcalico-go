// Copyright (c) 2020 Tigera, Inc. All rights reserved.

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

package clientv3_test

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/projectcalico/libcalico-go/lib/apiconfig"
	apiv3 "github.com/projectcalico/libcalico-go/lib/apis/v3"
	"github.com/projectcalico/libcalico-go/lib/backend"
	bapi "github.com/projectcalico/libcalico-go/lib/backend/api"
	"github.com/projectcalico/libcalico-go/lib/clientv3"
	"github.com/projectcalico/libcalico-go/lib/ipam"
	cnet "github.com/projectcalico/libcalico-go/lib/net"
	"github.com/projectcalico/libcalico-go/lib/options"
	"github.com/projectcalico/libcalico-go/lib/testutils"
)

var _ = testutils.E2eDatastoreDescribe("IPAMBlock tests", testutils.DatastoreAll, func(config apiconfig.CalicoAPIConfig) {

	ctx := context.Background()
	name1 := "192-168-142-64-26"
	name2 := "192-168-142-0-26"
	spec1 := apiv3.IPAMBlockSpec{
		CIDR:       "192.168.142.64/26",
		Attributes: []apiv3.AllocationAttribute{},
	}
	spec1mod := apiv3.IPAMBlockSpec{
		CIDR:       "192.168.142.64/26",
		Attributes: []apiv3.AllocationAttribute{},
		Deleted:    true,
	}
	spec2 := apiv3.IPAMBlockSpec{
		CIDR:       "192.168.142.0/26",
		Attributes: []apiv3.AllocationAttribute{},
	}

	DescribeTable("IPAMBlock e2e CRUD tests",
		func(name1, name2 string, spec1, spec2 apiv3.IPAMBlockSpec) {
			c, err := clientv3.New(config)
			Expect(err).NotTo(HaveOccurred())
			llIPAM := c.(clientv3.LowLevelIPAMClient).LowLevelIPAM()

			be, err := backend.NewClient(config)
			Expect(err).NotTo(HaveOccurred())
			err = be.Clean()
			Expect(err).NotTo(HaveOccurred())

			By("Attempting to creating a new IPAMBlock with name1/spec1 and a non-empty ResourceVersion")
			_, outError := llIPAM.IPAMBlocks().Create(ctx, &apiv3.IPAMBlock{
				ObjectMeta: metav1.ObjectMeta{Name: name1, ResourceVersion: "12345"},
				Spec:       spec1,
			}, options.SetOptions{})
			Expect(outError).To(HaveOccurred())
			Expect(outError.Error()).To(Equal("error with field Metadata.ResourceVersion = '12345' (field must not be set for a Create request)"))

			By("Creating a new IPAMBlock with name1/spec1")
			res1, outError := llIPAM.IPAMBlocks().Create(ctx, &apiv3.IPAMBlock{
				ObjectMeta: metav1.ObjectMeta{Name: name1},
				Spec:       spec1,
			}, options.SetOptions{})
			Expect(outError).NotTo(HaveOccurred())
			Expect(res1).To(MatchResource(apiv3.KindIPAMBlock, testutils.ExpectNoNamespace, name1, spec1))

			// Track the version of the original data for name1.
			rv1_1 := res1.ResourceVersion

			By("Attempting to create the same IPAMBlock with name1 but with spec1mod")
			_, outError = llIPAM.IPAMBlocks().Create(ctx, &apiv3.IPAMBlock{
				ObjectMeta: metav1.ObjectMeta{Name: name1},
				Spec:       spec1mod,
			}, options.SetOptions{})
			Expect(outError).To(HaveOccurred())
			// The KDD and etcd backends give slightly different errors because of the V1 vs V3 conversion, so just look for "resource does not exist"
			Expect(outError.Error()).To(ContainSubstring("resource already exists:"))

			By("Listing all the IPAMBlocks, expecting a single result with name1/spec1")
			outList, outError := llIPAM.IPAMBlocks().List(ctx, options.ListOptions{})
			Expect(outError).NotTo(HaveOccurred())
			Expect(outList.Items).To(ConsistOf(
				testutils.Resource(apiv3.KindIPAMBlock, testutils.ExpectNoNamespace, name1, spec1),
			))

			By("Creating a new IPAMBlock with name2/spec2")
			res2, outError := llIPAM.IPAMBlocks().Create(ctx, &apiv3.IPAMBlock{
				ObjectMeta: metav1.ObjectMeta{Name: name2},
				Spec:       spec2,
			}, options.SetOptions{})
			Expect(outError).NotTo(HaveOccurred())
			Expect(res2).To(MatchResource(apiv3.KindIPAMBlock, testutils.ExpectNoNamespace, name2, spec2))

			By("Listing all the LowLevelIPAM().IPAMBlocks, expecting a two results with name1/spec1 and name2/spec2")
			outList, outError = llIPAM.IPAMBlocks().List(ctx, options.ListOptions{})
			Expect(outError).NotTo(HaveOccurred())
			Expect(outList.Items).To(ConsistOf(
				testutils.Resource(apiv3.KindIPAMBlock, testutils.ExpectNoNamespace, name1, spec1),
				testutils.Resource(apiv3.KindIPAMBlock, testutils.ExpectNoNamespace, name2, spec2),
			))

			if config.Spec.DatastoreType != apiconfig.Kubernetes {
				By("Listing LowLevelIPAM().IPAMBlocks with the original resource version and checking for a single result with name1/spec1")
				outList, outError = llIPAM.IPAMBlocks().List(ctx, options.ListOptions{ResourceVersion: rv1_1})
				Expect(outError).NotTo(HaveOccurred())
				Expect(outList.Items).To(ConsistOf(
					testutils.Resource(apiv3.KindIPAMBlock, testutils.ExpectNoNamespace, name1, spec1),
				))
			}

			By("Listing LowLevelIPAM().IPAMBlocks with the latest resource version and checking for two results with name1/spec2 and name2/spec2")
			outList, outError = llIPAM.IPAMBlocks().List(ctx, options.ListOptions{})
			Expect(outError).NotTo(HaveOccurred())
			Expect(outList.Items).To(ConsistOf(
				testutils.Resource(apiv3.KindIPAMBlock, testutils.ExpectNoNamespace, name1, spec1),
				testutils.Resource(apiv3.KindIPAMBlock, testutils.ExpectNoNamespace, name2, spec2),
			))

			err = be.Clean()
			Expect(err).ToNot(HaveOccurred())

			By("Listing all LowLevelIPAM().IPAMBlocks and expecting no items")
			outList, outError = llIPAM.IPAMBlocks().List(ctx, options.ListOptions{})
			Expect(outError).NotTo(HaveOccurred())
			Expect(outList.Items).To(HaveLen(0))
		},

		// Test 1: Pass two fully populated LowLevelIPAM().IPAMBlocks specs and expect the series of operations to succeed.
		Entry("Two fully populated IPAMBlockSpecs", name1, name2, spec1, spec2),
	)

	Describe("correspondence between IPAM and LowLevelIPAM", func() {
		var c clientv3.Interface
		var be bapi.Client
		var llIPAM clientv3.LowLevelIPAMInterface

		BeforeEach(func() {
			var err error
			c, err = clientv3.New(config)
			Expect(err).NotTo(HaveOccurred())
			llIPAM = c.(clientv3.LowLevelIPAMClient).LowLevelIPAM()

			be, err = backend.NewClient(config)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("with IPPool defined", func() {
			BeforeEach(func() {
				err := be.Clean()
				Expect(err).NotTo(HaveOccurred())

				pool := apiv3.NewIPPool()
				pool.Name = "default"
				pool.Spec.CIDR = "10.13.0.0/16"
				pool, err = c.IPPools().Create(ctx, pool, options.SetOptions{})
				Expect(err).NotTo(HaveOccurred())
			})

			AfterEach(func() {
				err := be.Clean()
				Expect(err).NotTo(HaveOccurred())
			})

			It("Should list IPAMBlocks for assigned addresses", func() {
				ba, err := llIPAM.IPAMBlocks().List(ctx, options.ListOptions{})
				Expect(err).ToNot(HaveOccurred())
				Expect(ba.Items).To(HaveLen(0))

				// Assign an IP
				ip := cnet.MustParseIP("10.13.0.2")
				args := ipam.AssignIPArgs{
					IP:       ip,
					HandleID: nil,
					Attrs:    nil,
					Hostname: "testnode",
				}
				err = c.IPAM().AssignIP(ctx, args)
				Expect(err).ToNot(HaveOccurred())

				// Should create 1 block
				ba, err = llIPAM.IPAMBlocks().List(ctx, options.ListOptions{})
				Expect(err).ToNot(HaveOccurred())
				Expect(ba.Items).To(HaveLen(1))
			})
		})
	})
})
