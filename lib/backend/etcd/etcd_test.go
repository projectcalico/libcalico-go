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

package etcd_test

import (
	"reflect"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/projectcalico/libcalico-go/lib/backend/etcd"
)

var _ = Describe("Etcd Datastore Config tests", func() {
	BeforeEach(func() {
	})

	Describe("Created empty config", func() {
		c := etcd.EtcdConfig{}

		It("should have no default values", func() {
			v := reflect.ValueOf(&c).Elem()
			for i := 0; i < v.NumField(); i++ {
				f := v.Field(i)
				// Check that the value is the zero value of the type
				Expect(f.Interface()).To(Equal(reflect.Zero(reflect.TypeOf(f.Interface())).Interface()))
			}
		})
	})

	Describe("UpdateWithDefaults", func() {
		It("should default scheme and authority", func() {
			c := etcd.EtcdConfig{}
			c.UpdateWithDefaults()
			Expect(c.EtcdScheme).To(Equal("http"))
			Expect(c.EtcdAuthority).To(Equal("127.0.0.1:2379"))
		})

		It("should not overwrite scheme", func() {
			c := etcd.EtcdConfig{}
			expected_scheme := "scheme"
			c.EtcdScheme = expected_scheme
			c.UpdateWithDefaults()
			Expect(c.EtcdScheme).To(Equal(expected_scheme))
		})

		It("should not overwrite authority", func() {
			c := etcd.EtcdConfig{}
			expected_authority := "authority"
			c.EtcdAuthority = expected_authority
			c.UpdateWithDefaults()
			Expect(c.EtcdAuthority).To(Equal(expected_authority))
		})

		It("should not default scheme and authority with endpoints set", func() {
			c := etcd.EtcdConfig{}
			expected_endpoint := "does_not_matter"
			c.EtcdEndpoints = expected_endpoint
			c.UpdateWithDefaults()
			Expect(c.EtcdEndpoints).To(Equal(expected_endpoint))
			Expect(c.EtcdScheme).To(Equal(""))
			Expect(c.EtcdAuthority).To(Equal(""))
		})
	})

	Describe("PriorityMerge", func() {
		It("should take values from the lower when no value is in the higher", func() {
			h := etcd.EtcdConfig{}
			l := etcd.EtcdConfig{EtcdEndpoints: "lower"}
			r, err := etcd.PriorityMerge(h, l)
			Expect(err).To(BeNil())
			Expect(r.EtcdEndpoints).To(Equal(l.EtcdEndpoints))
			Expect(r.EtcdAuthority).To(Equal(""))
		})

		It("should take high priority Scheme and Authority over lower Endpoints", func() {
			expected_scheme := "scheme"
			expected_authority := "authority"
			h := etcd.EtcdConfig{
				EtcdScheme:    expected_scheme,
				EtcdAuthority: expected_authority,
			}
			l := etcd.EtcdConfig{EtcdEndpoints: "lower"}
			r, err := etcd.PriorityMerge(h, l)
			Expect(err).To(BeNil())
			Expect(r.EtcdEndpoints).To(Equal(""))
			Expect(r.EtcdScheme).To(Equal(expected_scheme))
			Expect(r.EtcdAuthority).To(Equal(expected_authority))
		})
	})

})
