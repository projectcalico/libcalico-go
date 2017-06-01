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

package client

import (
	"io/ioutil"
	"reflect"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"os"

	"github.com/projectcalico/libcalico-go/lib/api"
)

var _ = Describe("Client internal config tests", func() {

	Describe("loadClientConfigFromBytesWithoutDefaults has no default data", func() {
		It("should default no data (necessary for proper merging)", func() {
			c, err := loadClientConfigFromBytesWithoutDefaults([]byte(`
apiVersion: v1
kind: calicoApiConfig
`))
			Expect(err).To(BeNil())
			v := reflect.ValueOf(&c.Spec).Elem()
			for i := 0; i < v.NumField(); i++ {
				f := v.Field(i)
				Expect(f.Interface()).To(Equal(reflect.Zero(reflect.TypeOf(f.Interface())).Interface()))
			}
		})
	})

	Describe("LoadClientConfig", func() {
		It("unnamed file should read from default file", func() {
			fileRead := ""
			ioutil_ReadFile = func(filename string) (name []byte, err error) {
				fileRead = filename

				return []byte(`{
				    "kind": "calicoApiConfig",
				    "apiVersion": "v1",
				    "spec": {
					    "datastoreType": "etcdv2",
				        "etcdEndpoints": "endpoint_in_file"
				    }
				}`), nil
			}
			defer func() { ioutil_ReadFile = ioutil.ReadFile }()

			_, err := LoadClientConfig("")
			Expect(err).To(BeNil())

			Expect(fileRead).To(Equal(DefaultDatastoreConfigFile))
		})

		It("reads from specified file", func() {
			fileRead := ""
			ioutil_ReadFile = func(filename string) (name []byte, err error) {
				fileRead = filename

				return []byte(`{
				    "kind": "calicoApiConfig",
				    "apiVersion": "v1",
				    "spec": {
					    "datastoreType": "kubernetes"
				    }
				}`), nil
			}
			defer func() { ioutil_ReadFile = ioutil.ReadFile }()

			_, err := LoadClientConfig("specified_file")
			Expect(err).To(BeNil())

			Expect(fileRead).To(Equal("specified_file"))
		})

		It("file values is not overwritten with defaults", func() {
			ioutil_ReadFile = func(filename string) (name []byte, err error) {
				return []byte(`{
				    "kind": "calicoApiConfig",
				    "apiVersion": "v1",
				    "spec": {
					    "datastoreType": "kubernetes"
				    }
				}`), nil
			}
			defer func() { ioutil_ReadFile = ioutil.ReadFile }()

			os.Unsetenv("DATASTORE_TYPE")
			c, err := LoadClientConfig("specified_file")
			Expect(err).To(BeNil())
			Expect(c.Spec.DatastoreType).To(Equal(api.Kubernetes))
		})

		It("file values are overwritten from Env", func() {
			ioutil_ReadFile = func(filename string) (name []byte, err error) {
				return []byte(`{
				    "kind": "calicoApiConfig",
				    "apiVersion": "v1",
				    "spec": {
					    "datastoreType": "kubernetes"
				    }
				}`), nil
			}
			defer func() { ioutil_ReadFile = ioutil.ReadFile }()

			os.Setenv("DATASTORE_TYPE", "etcdv2")
			c, err := LoadClientConfig("specified_file")
			os.Unsetenv("DATASTORE_TYPE")
			Expect(err).To(BeNil())
			Expect(c.Spec.DatastoreType).To(Equal(api.EtcdV2))
		})
	})
})
