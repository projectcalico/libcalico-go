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
package testutils

import (
	gonet "net"

	"github.com/projectcalico/libcalico-go/lib/net"
)

// MustParseCIDR returns the parsed CIDR as a net.IPNet.  This returns the
// unnormalized form of the CIDR (i.e. the mask is not applied to the IP).
func MustParseCIDR(c string) net.IPNet {
	ip, cidr, err := gonet.ParseCIDR(c)
	if err != nil {
		panic(err)
	}
	ipn := net.IPNet{}
	ipn.FromIPAndMask(ip, cidr.Mask)
	return ipn
}

// MustParseIP parses an IP address into a net.IP struct.  The default golang
// net library always converts IPv4 addresses to an IPv6 16-byte format, so use
// MustParseIPv4 if your tests require 4-byte lengths.
func MustParseIP(i string) net.IP {
	var ip net.IP
	err := ip.UnmarshalText([]byte(i))
	if err != nil {
		panic(err)
	}
	return ip
}

// MustParseIPv4 parses an IPv4 address into a net.IP struct and ensures the
// IP address is a 4-byte representation.
func MustParseIPv4(i string) net.IP {
	var ip net.IP
	err := ip.UnmarshalText([]byte(i))
	if err != nil {
		panic(err)
	}
	if ip.Version() != 4 {
		panic("IP version is not v4")
	}

	return net.IP{ip.To4()}
}
