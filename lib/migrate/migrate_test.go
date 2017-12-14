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

package migrate

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("UT for checking the version for migration.", func() {

	DescribeTable("Checking canMigrate.",
		func(ver string, result bool) {
			Expect(canMigrate(ver)).To(Equal(result))
		},

		Entry("Expect v2.6.4 to migrate", "v2.6.4", true),
		Entry("Expect v2.6.4-rc1 to migrate", "v2.6.4-rc1", true),
		Entry("Expect v2.6.4-15-g0986c6cd to migrate", "v2.6.4-15-g0986c6cd", true),
		Entry("Expect v2.6.5 to migrate", "v2.6.5", true),
		Entry("Expect v2.6.5-rc1 to migrate", "v2.6.5-rc1", true),
		Entry("Expect v2.7.0 to migrate", "v2.7.0", true),
		Entry("Expect v2.7.0-rc1 to migrate", "v2.7.0-rc1", true),
		Entry("Expect v2.6.3 to not migrate", "v2.6.3", false),
		Entry("Expect v2.6.3-rc1 to not migrate", "v2.6.3-rc1", false),
		Entry("Expect v2.6.3-15-g0986c6cd to not migrate", "v2.6.3-15-g0986c6cd", false),
		Entry("Expect v2.6.x-deadbeef to not migrate", "v2.6.x-deadbeef", false),
		Entry("Expect master to not migrate", "master", false),
		Entry("Expect empty string to not migrate", "", false),
		Entry("Expect garbage to not migrate", "garbage", false),
		Entry("Expect 1.2.3.4.5 to not migrate", "1.2.3.4.5", false),
	)
})
