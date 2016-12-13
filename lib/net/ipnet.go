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

package net

import (
	"encoding/json"
	"net"
)

// Sub class net.IPNet so that we can add JSON marshalling and unmarshalling.
type IPNet struct {
	net.IPNet
}

// MarshalJSON interface for an IPNet
func (i IPNet) MarshalJSON() ([]byte, error) {
	return json.Marshal(i.String())
}

// UnmarshalJSON interface for an IPNet
func (i *IPNet) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	return i.UnmarshalText([]byte(s))
}

// UnmarshalText interface for an IPNet
func (i *IPNet) UnmarshalText(b []byte) error {
	// First try to parse as a CIDR, if that does not work try to parse as
	// an IP address (with /32 or /128 mask).  If neither parse, return the
	// original error.
	//
	// the golang net library seems to be inconsistent in it's choice of
	// internal slice size of IPv4 addresses - so for consistency let's
	// always uses 4-byte slice for IPv4 addresses in an IPNet.
	s := string(b)
	if ip, ipnet, err := net.ParseCIDR(s); err == nil {
		i.FromIPAndMask(ip, ipnet.Mask)
		return nil
	} else if ip = net.ParseIP(s); ip != nil {
		i.FromIP(ip)
		return nil
	} else {
		return err
	}
}

// Version returns the IP version for an IPNet.  Returns 0 if the IP address is
// not valid.
func (i *IPNet) Version() int {
	if i.IP.To4() != nil {
		return 4
	}
	if len(i.IP) == net.IPv6len {
		return 6
	}
	return 0
}

// MaskedIP returns the masked IP address stored in this IPNet object.
func (i IPNet) IPAddress() *IP {
	return &IP{i.IP}
}

// MaskedIP returns the masked IP address stored in this IPNet object.
func (i IPNet) MaskedIPAddress() *IP {
	return &IP{i.IP.Mask(i.IPNet.Mask)}
}

// Network returns the normalized network of this IPNet object.
func (i IPNet) MaskedIPNet() *IPNet {
	return &IPNet{net.IPNet{IP: i.IP.Mask(i.IPNet.Mask), Mask: i.Mask}}
}

// Fill in the IPNet from the supplied IP and Mask (normalising the length
// of the byte slices).
func (i *IPNet) FromIPAndMask(ip net.IP, mask net.IPMask) {
	i.IP = ip.To4()
	if i.IP == nil {
		i.IP = ip
	}
	i.Mask = mask
}

// Fill in the IPNet from the supplied IP (assuming a /32 or /128).
func (i *IPNet) FromIP(ip net.IP) {
	i.IP = ip.To4()
	if i.IP == nil {
		i.IP = ip
	}
	ipbits := 8 * len(i.IP)
	i.Mask = net.CIDRMask(ipbits, ipbits)
}

// ParseCIDR parses a CIDR string returning the IP address and the normalized
// network (i.e. the IP address with mask applied).
func ParseCIDR(c string) (*IP, *IPNet, error) {
	netIP, netIPNet, e := net.ParseCIDR(c)
	if netIPNet == nil {
		return nil, nil, e
	}
	return &IP{netIP}, &IPNet{*netIPNet}, e
}

// String returns a friendly name for the network.  The standard net package
// implements String() on the pointer, which means it will not be invoked on a
// struct type, so we re-implement on the struct type.
func (i IPNet) String() string {
	ipn := &i.IPNet
	return ipn.String()
}
