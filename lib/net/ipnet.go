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
	"github.com/ugorji/go/codec"
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
	if _, ipnet, err := net.ParseCIDR(s); err != nil {
		return err
	} else {
		i.IP = ipnet.IP
		i.Mask = ipnet.Mask
		return nil
	}
}

// Version returns the IP version for an IPNet
func (i *IPNet) Version() int {
	if i.IP.To4() == nil {
		return 6
	}
	return 4
}

func ParseCIDR(c string) (*IP, *IPNet, error) {
	netIP, netIPNet, e := net.ParseCIDR(c)
	if netIPNet == nil {
		return nil, nil, e
	}
	return &IP{netIP}, &IPNet{*netIPNet}, e
}

func (i IPNet) CodecEncodeSelf(enc *codec.Encoder) {
	enc.Encode(i.String())
}

func (i *IPNet) CodecDecodeSelf(dec *codec.Decoder) {
	var s *string
	dec.MustDecode(s)
	if _, ipnet, err := net.ParseCIDR(*s); err != nil {
		return
	} else {
		i.IP = ipnet.IP
		i.Mask = ipnet.Mask
		return
	}
}
