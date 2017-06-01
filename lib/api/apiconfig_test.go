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

package api_test

import (
	"reflect"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/projectcalico/libcalico-go/lib/api"
	"github.com/projectcalico/libcalico-go/lib/backend/etcd"
	"github.com/projectcalico/libcalico-go/lib/backend/k8s"
)

var _ = Describe("API Datastore Config tests", func() {
	BeforeEach(func() {
	})

	Describe("Empty API config Spec", func() {
		It("should have no default values", func() {
			c := api.NewCalicoAPIConfig()
			v := reflect.ValueOf(&c.Spec).Elem()
			for i := 0; i < v.NumField(); i++ {
				f := v.Field(i)
				Expect(f.Interface()).To(Equal(reflect.Zero(reflect.TypeOf(f.Interface())).Interface()))
			}
		})
	})
	Describe("PriorityMerge", func() {
		Context("checks versions", func() {
			It("should error if they are different", func() {
				h := api.NewCalicoAPIConfig()
				l := api.NewCalicoAPIConfig()
				l.TypeMetadata.APIVersion = "vnot"
				_, err := api.PriorityMerge(*h, *l)
				Expect(err).NotTo(BeNil())
			})
		})

		Context("merge datastore type", func() {
			It("should take the higher priority's type", func() {
				h := api.NewCalicoAPIConfig()
				l := api.NewCalicoAPIConfig()
				h.Spec.DatastoreType = api.Kubernetes
				l.Spec.DatastoreType = api.EtcdV2
				cfg, err := api.PriorityMerge(*h, *l)
				Expect(err).To(BeNil())
				Expect(cfg.Spec.DatastoreType).To(Equal(api.Kubernetes))
			})
		})

		Context("merge datastore", func() {
			It("should merge only the higher priority datastore type", func() {
				h := api.NewCalicoAPIConfig()
				l := api.NewCalicoAPIConfig()
				h.Spec.DatastoreType = api.Kubernetes
				h.Spec.KubeConfig = k8s.KubeConfig{K8sCertFile: "hcert"}
				l.Spec.DatastoreType = api.EtcdV2
				l.Spec.EtcdConfig = etcd.EtcdConfig{EtcdUsername: "lname"}
				l.Spec.KubeConfig = k8s.KubeConfig{K8sKeyFile: "lkey"}

				cfg, err := api.PriorityMerge(*h, *l)
				Expect(err).To(BeNil())
				Expect(cfg.Spec.KubeConfig.K8sCertFile).To(Equal("hcert"))
				Expect(cfg.Spec.KubeConfig.K8sKeyFile).To(Equal("lkey"))
				Expect(cfg.Spec.EtcdConfig.EtcdUsername).To(Equal(""))
			})
		})
	})
	Describe("PriorityMerge", func() {
		Context("checks versions", func() {
			It("should error if they are different", func() {
				h := api.NewCalicoAPIConfig()
				l := api.NewCalicoAPIConfig()
				l.TypeMetadata.APIVersion = "vnot"
				_, err := api.PriorityMerge(*h, *l)
				Expect(err).NotTo(BeNil())
			})
		})

		Context("merge datastore type", func() {
			It("should take the higher priority's type", func() {
				h := api.NewCalicoAPIConfig()
				l := api.NewCalicoAPIConfig()
				h.Spec.DatastoreType = api.Kubernetes
				l.Spec.DatastoreType = api.EtcdV2
				cfg, err := api.PriorityMerge(*h, *l)
				Expect(err).To(BeNil())
				Expect(cfg.Spec.DatastoreType).To(Equal(api.Kubernetes))
			})
		})

		Context("merge datastore", func() {
			It("should merge only the higher priority datastore type", func() {
				h := api.NewCalicoAPIConfig()
				l := api.NewCalicoAPIConfig()
				h.Spec.DatastoreType = api.Kubernetes
				h.Spec.KubeConfig = k8s.KubeConfig{K8sCertFile: "hcert"}
				l.Spec.DatastoreType = api.EtcdV2
				l.Spec.EtcdConfig = etcd.EtcdConfig{EtcdUsername: "lname"}
				l.Spec.KubeConfig = k8s.KubeConfig{K8sKeyFile: "lkey"}

				cfg, err := api.PriorityMerge(*h, *l)
				Expect(err).To(BeNil())
				Expect(cfg.Spec.KubeConfig.K8sCertFile).To(Equal("hcert"))
				Expect(cfg.Spec.KubeConfig.K8sKeyFile).To(Equal("lkey"))
				Expect(cfg.Spec.EtcdConfig.EtcdUsername).To(Equal(""))
			})
		})
	})
	Describe("UpdateWithDefaults", func() {
		It("should default datastore type and datastore config", func() {
			c := api.NewCalicoAPIConfig()
			c.UpdateWithDefaults()
			Expect(c.Spec.DatastoreType).To(Equal(api.EtcdV2))
			expectedCfg := etcd.EtcdConfig{
				EtcdScheme:    "http",
				EtcdAuthority: "127.0.0.1:2379",
			}
			Expect(c.Spec.EtcdConfig).To(Equal(expectedCfg))
		})
	})
})
