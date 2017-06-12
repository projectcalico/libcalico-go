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

package net

import (
	"encoding/json"
	"net"
)

// Sub class net.IP so that we can add JSON marshalling and unmarshalling.
type IP struct {
	net.IP
}

// MarshalJSON interface for an IP
func (i IP) MarshalJSON() ([]byte, error) {
	s, err := i.MarshalText()
	if err != nil {
		return nil, err
	}
	return json.Marshal(string(s))
}

// UnmarshalJSON interface for an IP
func (i *IP) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	return i.UnmarshalText([]byte(s))
}

// ParseIP returns an IP from a string
func ParseIP(ip string) *IP {
	addr := net.ParseIP(ip)
	if addr == nil {
		return nil
	}
	return &IP{addr}
}

// Version returns the IP version for an IP, or 0 if the IP is not valid.
func (i IP) Version() int {
	if i.To4() != nil {
		return 4
	} else if len(i.IP) == net.IPv6len {
		return 6
	}
	return 0
}

// Network returns the IP address as a fully masked IPNet type.
func (i *IP) Network() *IPNet {
	// Unmarshaling an IPv4 address returns a 16-byte format of the
	// address, so convert to 4-byte format to match the mask.
	n := &IPNet{}
	if i.Version() == 4 {
		n.IP = i.IP.To4()
		n.Mask = net.CIDRMask(net.IPv4len*8, net.IPv4len*8)
	} else {
		n.IP = i.IP
		n.Mask = net.CIDRMask(net.IPv6len*8, net.IPv6len*8)
	}
	return n
}

func (i *IP) String() string {
	if i == nil {
		return "<nil>"
	}
	return i.IP.String()
}
