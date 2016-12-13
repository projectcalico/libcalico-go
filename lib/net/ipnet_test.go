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

package net_test

import (
	"encoding/json"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/projectcalico/libcalico-go/lib/net"
	"github.com/projectcalico/libcalico-go/lib/testutils"
)

func init() {

	// Perform tests of JSON unmarshaling of the various field types.
	DescribeTable("IPNet test",
		func(jtext string, storedIP, maskedIP net.IP, maskSize int) {
			ipn := net.IPNet{}
			err := json.Unmarshal([]byte("\""+jtext+"\""), &ipn)
			Expect(err).To(Not(HaveOccurred()))

			ipn2 := &net.IPNet{}
			err = ipn2.UnmarshalText([]byte(jtext))
			Expect(err).To(Not(HaveOccurred()))
			Expect(ipn).To(Equal(*ipn2))

			// Check the masked IP.
			ip := *ipn.IPAddress()
			mip := *ipn.MaskedIPAddress()
			Expect(ip).To(Equal(storedIP))
			Expect(mip).To(Equal(maskedIP))

			// Check the mask size.
			ones, _ := ipn.Mask.Size()
			Expect(ones).To(Equal(maskSize))

			// Check the string format is as expected.  Since this
			// could be v4 or v6 we use the IP.Equal() method to
			// compare since the golang library uses IPv6 notation
			// for IPv4 addresses in some cases.
			s := ipn.String()
			parts := strings.Split(s, "/")
			Expect(testutils.MustParseIP(parts[0]).Equal(storedIP.IP)).To(BeTrue())
			Expect(strconv.Atoi(parts[1])).To(Equal(ones))

			// Check the network.
			ipn2 = ipn.MaskedIPNet()
			ip = *ipn2.IPAddress()
			ones, _ = ipn.Mask.Size()
			Expect(ip).To(Equal(maskedIP))
			Expect(ones).To(Equal(maskSize))
		},
		// IPNet tests.
		Entry("IPv4 parsed IP is the masked IP",
			"1.2.3.4/30",
			testutils.MustParseIPv4("1.2.3.4"),
			testutils.MustParseIPv4("1.2.3.4"),
			30),
		Entry("IPv4 parsed IP is not the masked IP",
			"11.22.33.44/24",
			testutils.MustParseIPv4("11.22.33.44"),
			testutils.MustParseIPv4("11.22.33.0"),
			24),
		Entry("IPv4 parsed IP has no subnet",
			"192.168.10.10",
			testutils.MustParseIPv4("192.168.10.10"),
			testutils.MustParseIPv4("192.168.10.10"),
			32),
		Entry("IPv6 parsed IP is the masked IP",
			"aabb:ccdd:eeff:0011::1204/126",
			testutils.MustParseIP("aabb:ccdd:eeff:0011::1204"),
			testutils.MustParseIP("aabb:ccdd:eeff:0011::1204"),
			126),
		Entry("IPv6 parsed IP is not the masked IP",
			"aabb::1234:1313/112",
			testutils.MustParseIP("aabb::1234:1313"),
			testutils.MustParseIP("aabb::1234:0000"),
			112),
		Entry("IPv6 parsed IP has no subnet",
			"1234::abcd:1313",
			testutils.MustParseIP("1234::abcd:1313"),
			testutils.MustParseIP("1234::abcd:1313"),
			128),
	)
}
