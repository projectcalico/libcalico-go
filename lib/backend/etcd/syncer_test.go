// Copyright (c) 2016-2017 Tigera, Inc. All rights reserved.
//
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
	. "github.com/projectcalico/libcalico-go/lib/backend/etcd"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/projectcalico/libcalico-go/lib/backend/api"
)

var _ = Describe("Syncer", func() {
	var etcd *mockEtcd
	var syncer api.Syncer
	var updates chan interface{}

	BeforeEach(func() {
		etcd = newMockEtcd()
		syncer = NewSyncer(etcd, callbacks)
	})
})

type

