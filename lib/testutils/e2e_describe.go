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
package testutils

import (
	"fmt"

	"github.com/projectcalico/libcalico-go/lib/api"
	"github.com/projectcalico/libcalico-go/lib/backend/consul"
	"github.com/projectcalico/libcalico-go/lib/backend/etcd"
	"github.com/projectcalico/libcalico-go/lib/backend/k8s"

	. "github.com/onsi/ginkgo"
)

type DatastoreType int

const (
	DatastoreEtcdV2 DatastoreType = 1 << iota
	DatastoreK8s
	DatastoreConsulV1

	DatastoreAll = DatastoreEtcdV2 | DatastoreK8s | DatastoreConsulV1
)

// E2eDatastoreDescribe is a replacement for ginkgo.Describe which invokes Describe
// multiple times for one or more different datastore drivers - passing in the
// Calico API configuration as a parameter to the test function.  This allows
// easy construction of end-to-end tests covering multiple different datastore
// drivers.
//
// The *datastores* parameter is a bit-wise OR of the required datastore drivers
// that will be tested.
func E2eDatastoreDescribe(description string, datastores DatastoreType, body func(config api.CalicoAPIConfig)) bool {
	if datastores&DatastoreEtcdV2 != 0 {
		Describe(fmt.Sprintf("%s (etcdv2 backend)", description),
			func() {
				body(api.CalicoAPIConfig{
					Spec: api.CalicoAPIConfigSpec{
						DatastoreType: api.EtcdV2,
						EtcdConfig: etcd.EtcdConfig{
							EtcdEndpoints: "http://127.0.0.1:2379",
						},
					},
				})
			})
	}

	if datastores&DatastoreK8s != 0 {
		Describe(fmt.Sprintf("%s (kubernetes backend)", description),
			func() {
				body(api.CalicoAPIConfig{
					Spec: api.CalicoAPIConfigSpec{
						DatastoreType: api.Kubernetes,
						KubeConfig: k8s.KubeConfig{
							K8sAPIEndpoint: "http://localhost:8080",
						},
					},
				})
			})
	}

	if datastores&DatastoreConsulV1 != 0 {
		Describe(fmt.Sprintf("%s (consul backend)", description),
			func() {
				body(api.CalicoAPIConfig{
					Spec: api.CalicoAPIConfigSpec{
						DatastoreType: api.ConsulV1,
						ConsulConfig: consul.ConsulConfig{
							ConsulAddress: "127.0.0.1:8500",
							ConsulScheme:  "http",
						},
					},
				})
			})
	}

	return true
}
