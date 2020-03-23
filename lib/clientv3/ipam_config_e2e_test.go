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
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/projectcalico/libcalico-go/lib/apiconfig"
	apiv3 "github.com/projectcalico/libcalico-go/lib/apis/v3"
	"github.com/projectcalico/libcalico-go/lib/backend"
	bapi "github.com/projectcalico/libcalico-go/lib/backend/api"
	"github.com/projectcalico/libcalico-go/lib/clientv3"
	"github.com/projectcalico/libcalico-go/lib/ipam"
	"github.com/projectcalico/libcalico-go/lib/options"
	"github.com/projectcalico/libcalico-go/lib/testutils"
)

var _ = testutils.E2eDatastoreDescribe("IPAMConfig tests", testutils.DatastoreAll, func(config apiconfig.CalicoAPIConfig) {

	ctx := context.Background()
	name1 := "default"
	name2 := "not-default"
	spec1 := apiv3.IPAMConfigSpec{
		StrictAffinity: true,
	}
	spec1mod := apiv3.IPAMConfigSpec{
		StrictAffinity: false,
	}

	It("IPAMConfig e2e CRUD tests", func() {

		c, err := clientv3.New(config)
		Expect(err).NotTo(HaveOccurred())
		llIPAM := c.(clientv3.LowLevelIPAMClient).LowLevelIPAM()

		be, err := backend.NewClient(config)
		Expect(err).NotTo(HaveOccurred())
		err = be.Clean()
		Expect(err).NotTo(HaveOccurred())

		By("Attempting to creating a new IPAMConfig with name1/spec1 and a non-empty ResourceVersion")
		_, outError := llIPAM.IPAMConfig().Create(ctx, &apiv3.IPAMConfig{
			ObjectMeta: metav1.ObjectMeta{Name: name1, ResourceVersion: "12345"},
			Spec:       spec1,
		}, options.SetOptions{})
		Expect(outError).To(HaveOccurred())
		Expect(outError.Error()).To(Equal("error with field Metadata.ResourceVersion = '12345' (field must not be set for a Create request)"))

		By("Creating a new IPAMConfig with name1/spec1")
		res1, outError := llIPAM.IPAMConfig().Create(ctx, &apiv3.IPAMConfig{
			ObjectMeta: metav1.ObjectMeta{Name: name1},
			Spec:       spec1,
		}, options.SetOptions{})
		Expect(outError).NotTo(HaveOccurred())
		Expect(res1).To(MatchResource(apiv3.KindIPAMConfig, testutils.ExpectNoNamespace, name1, spec1))

		By("Attempting to create the same IPAMConfig with name1 but with spec1mod")
		_, outError = llIPAM.IPAMConfig().Create(ctx, &apiv3.IPAMConfig{
			ObjectMeta: metav1.ObjectMeta{Name: name1},
			Spec:       spec1mod,
		}, options.SetOptions{})
		Expect(outError).To(HaveOccurred())
		// The KDD and etcd backends give slightly different errors because of the V1 vs V3 conversion, so just look for "resource does not exist"
		Expect(outError.Error()).To(ContainSubstring("resource already exists:"))

		By("Getting the IPAMConfig, expecting result with spec1")
		out, outError := llIPAM.IPAMConfig().Get(ctx, name1, options.GetOptions{})
		Expect(outError).NotTo(HaveOccurred())
		Expect(out).To(MatchResource(apiv3.KindIPAMConfig, testutils.ExpectNoNamespace, name1, spec1))

		By("Creating a new IPAMConfig with name2/spec1")
		_, outError = llIPAM.IPAMConfig().Create(ctx, &apiv3.IPAMConfig{
			ObjectMeta: metav1.ObjectMeta{Name: name2},
			Spec:       spec1,
		}, options.SetOptions{})
		Expect(outError).To(HaveOccurred())
		Expect(outError.Error()).To(Equal("Cannot create an IPAM Config resource with a name other than \"default\""))

		err = be.Clean()
		Expect(err).ToNot(HaveOccurred())

		By("Getting IPAMConfig and expecting no items")
		_, outError = llIPAM.IPAMConfig().Get(ctx, name1, options.GetOptions{})
		Expect(outError).To(HaveOccurred())
		Expect(outError.Error()).To(ContainSubstring("resource does not exist"))
	})

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

		AfterEach(func() {
			err := be.Clean()
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should list IPAMConfig set in IPAM()", func() {
			_, outError := llIPAM.IPAMConfig().Get(ctx, name1, options.GetOptions{})
			Expect(outError).To(HaveOccurred())
			Expect(outError.Error()).To(ContainSubstring("resource does not exist"))

			cfg := ipam.IPAMConfig{StrictAffinity: true, AutoAllocateBlocks: true}
			err := c.IPAM().SetIPAMConfig(ctx, cfg)
			Expect(err).ToNot(HaveOccurred())

			// Should create IPAM Config
			out, outError := llIPAM.IPAMConfig().Get(ctx, name1, options.GetOptions{})
			Expect(outError).NotTo(HaveOccurred())
			Expect(out).To(MatchResource(
				apiv3.KindIPAMConfig, testutils.ExpectNoNamespace, "default", apiv3.IPAMConfigSpec{
					StrictAffinity:     true,
					AutoAllocateBlocks: true,
				}))

			cfg = ipam.IPAMConfig{StrictAffinity: false, AutoAllocateBlocks: true}
			err = c.IPAM().SetIPAMConfig(ctx, cfg)
			Expect(err).ToNot(HaveOccurred())

			out, outError = llIPAM.IPAMConfig().Get(ctx, name1, options.GetOptions{})
			Expect(outError).NotTo(HaveOccurred())
			Expect(out).To(MatchResource(
				apiv3.KindIPAMConfig, testutils.ExpectNoNamespace, "default", apiv3.IPAMConfigSpec{
					StrictAffinity:     false,
					AutoAllocateBlocks: true,
				}))
		})
	})
})
