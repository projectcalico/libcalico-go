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
	cnet "github.com/projectcalico/libcalico-go/lib/net"
	"github.com/projectcalico/libcalico-go/lib/options"
	"github.com/projectcalico/libcalico-go/lib/testutils"
)

var _ = testutils.E2eDatastoreDescribe("BlockAffinity tests", testutils.DatastoreAll, func(config apiconfig.CalicoAPIConfig) {

	ctx := context.Background()
	name1 := "node-1-192-168-142-64-26"
	name2 := "node-2-192-168-142-0-26"
	spec1 := apiv3.BlockAffinitySpec{
		State:   "confirmed",
		Node:    "node-1",
		CIDR:    "192.168.142.64/26",
		Deleted: "",
	}
	spec1mod := apiv3.BlockAffinitySpec{
		State:   "confirmed",
		Node:    "node-1",
		CIDR:    "192.168.142.64/26",
		Deleted: "true",
	}
	spec2 := apiv3.BlockAffinitySpec{
		State:   "confirmed",
		Node:    "node-2",
		CIDR:    "192.168.142.0/26",
		Deleted: "",
	}

	DescribeTable("BlockAffinity e2e CRUD tests",
		func(name1, name2 string, spec1, spec2 apiv3.BlockAffinitySpec) {
			c, err := clientv3.New(config)
			Expect(err).NotTo(HaveOccurred())
			llIPAM := c.(clientv3.LowLevelIPAMClient).LowLevelIPAM()

			be, err := backend.NewClient(config)
			Expect(err).NotTo(HaveOccurred())
			err = be.Clean()
			Expect(err).NotTo(HaveOccurred())

			By("Attempting to creating a new BlockAffinity with name1/spec1 and a non-empty ResourceVersion")
			_, outError := llIPAM.BlockAffinities().Create(ctx, &apiv3.BlockAffinity{
				ObjectMeta: metav1.ObjectMeta{Name: name1, ResourceVersion: "12345"},
				Spec:       spec1,
			}, options.SetOptions{})
			Expect(outError).To(HaveOccurred())
			Expect(outError.Error()).To(Equal("error with field Metadata.ResourceVersion = '12345' (field must not be set for a Create request)"))

			By("Creating a new BlockAffinity with name1/spec1")
			res1, outError := llIPAM.BlockAffinities().Create(ctx, &apiv3.BlockAffinity{
				ObjectMeta: metav1.ObjectMeta{Name: name1},
				Spec:       spec1,
			}, options.SetOptions{})
			Expect(outError).NotTo(HaveOccurred())
			Expect(res1).To(MatchResource(apiv3.KindBlockAffinity, testutils.ExpectNoNamespace, name1, spec1))

			// Track the version of the original data for name1.
			rv1_1 := res1.ResourceVersion

			By("Attempting to create the same BlockAffinity with name1 but with spec1mod")
			_, outError = llIPAM.BlockAffinities().Create(ctx, &apiv3.BlockAffinity{
				ObjectMeta: metav1.ObjectMeta{Name: name1},
				Spec:       spec1mod,
			}, options.SetOptions{})
			Expect(outError).To(HaveOccurred())
			// The KDD and etcd backends give slightly different errors because of the V1 vs V3 conversion, so just look for "resource does not exist"
			Expect(outError.Error()).To(ContainSubstring("resource already exists:"))

			By("Listing all the BlockAffinities, expecting a single result with name1/spec1")
			outList, outError := llIPAM.BlockAffinities().List(ctx, options.ListOptions{})
			Expect(outError).NotTo(HaveOccurred())
			Expect(outList.Items).To(ConsistOf(
				testutils.Resource(apiv3.KindBlockAffinity, testutils.ExpectNoNamespace, name1, spec1),
			))

			By("Creating a new BlockAffinity with name2/spec2")
			res2, outError := llIPAM.BlockAffinities().Create(ctx, &apiv3.BlockAffinity{
				ObjectMeta: metav1.ObjectMeta{Name: name2},
				Spec:       spec2,
			}, options.SetOptions{})
			Expect(outError).NotTo(HaveOccurred())
			Expect(res2).To(MatchResource(apiv3.KindBlockAffinity, testutils.ExpectNoNamespace, name2, spec2))

			By("Listing all the LowLevelIPAM().BlockAffinities, expecting a two results with name1/spec1 and name2/spec2")
			outList, outError = llIPAM.BlockAffinities().List(ctx, options.ListOptions{})
			Expect(outError).NotTo(HaveOccurred())
			Expect(outList.Items).To(ConsistOf(
				testutils.Resource(apiv3.KindBlockAffinity, testutils.ExpectNoNamespace, name1, spec1),
				testutils.Resource(apiv3.KindBlockAffinity, testutils.ExpectNoNamespace, name2, spec2),
			))

			if config.Spec.DatastoreType != apiconfig.Kubernetes {
				By("Listing LowLevelIPAM().BlockAffinities with the original resource version and checking for a single result with name1/spec1")
				outList, outError = llIPAM.BlockAffinities().List(ctx, options.ListOptions{ResourceVersion: rv1_1})
				Expect(outError).NotTo(HaveOccurred())
				Expect(outList.Items).To(ConsistOf(
					testutils.Resource(apiv3.KindBlockAffinity, testutils.ExpectNoNamespace, name1, spec1),
				))
			}

			By("Listing LowLevelIPAM().BlockAffinities with the latest resource version and checking for two results with name1/spec2 and name2/spec2")
			outList, outError = llIPAM.BlockAffinities().List(ctx, options.ListOptions{})
			Expect(outError).NotTo(HaveOccurred())
			Expect(outList.Items).To(ConsistOf(
				testutils.Resource(apiv3.KindBlockAffinity, testutils.ExpectNoNamespace, name1, spec1),
				testutils.Resource(apiv3.KindBlockAffinity, testutils.ExpectNoNamespace, name2, spec2),
			))

			err = be.Clean()
			Expect(err).ToNot(HaveOccurred())

			By("Listing all LowLevelIPAM().BlockAffinities and expecting no items")
			outList, outError = llIPAM.BlockAffinities().List(ctx, options.ListOptions{})
			Expect(outError).NotTo(HaveOccurred())
			Expect(outList.Items).To(HaveLen(0))
		},

		// Test 1: Pass two fully populated LowLevelIPAM().BlockAffinitiespecs and expect the series of operations to succeed.
		Entry("Two fully populated BlockAffinitySpecs", name1, name2, spec1, spec2),
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

			It("Should list host affinity claimed by IPAM()", func() {
				ba, err := llIPAM.BlockAffinities().List(ctx, options.ListOptions{})
				Expect(err).ToNot(HaveOccurred())
				Expect(ba.Items).To(HaveLen(0))

				cidr := cnet.MustParseNetwork("10.13.14.0/26")
				claimed, failed, err := c.IPAM().ClaimAffinity(ctx, cidr, "node-1")
				Expect(err).ToNot(HaveOccurred())
				Expect(failed).To(HaveLen(0))
				Expect(claimed).To(HaveLen(1))

				ba, err = llIPAM.BlockAffinities().List(ctx, options.ListOptions{})
				Expect(err).ToNot(HaveOccurred())
				Expect(ba.Items).To(HaveLen(1))

				err = c.IPAM().ReleaseAffinity(ctx, cidr, "node-1", true)
				Expect(err).ToNot(HaveOccurred())

				ba, err = llIPAM.BlockAffinities().List(ctx, options.ListOptions{})
				Expect(err).ToNot(HaveOccurred())
				Expect(ba.Items).To(HaveLen(0))
			})
		})
	})
})
