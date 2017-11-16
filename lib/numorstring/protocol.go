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

package numorstring

import "strings"

const (
	ProtocolUDP     = "UDP"
	ProtocolTCP     = "TCP"
	ProtocolICMP    = "ICMP"
	ProtocolICMPv6  = "ICMPv6"
	ProtocolSCTP    = "SCTP"
	ProtocolUDPLite = "UDPLite"

	ProtocolV1UDP     = "udp"
	ProtocolV1TCP     = "tcp"
	ProtocolV1ICMP    = "icmp"
	ProtocolV1ICMPv6  = "icmpv6"
	ProtocolV1SCTP    = "sctp"
	ProtocolV1UDPLite = "udplite"
)

var (
	allProtocolNames = []string{
		ProtocolUDP,
		ProtocolTCP,
		ProtocolICMP,
		ProtocolICMPv6,
		ProtocolSCTP,
		ProtocolUDPLite,
	}
)

type Protocol Uint8OrString

// ProtocolFromInt creates a Protocol struct from an integer value.
func ProtocolFromInt(p uint8) Protocol {
	return Protocol(
		Uint8OrString{Type: NumOrStringNum, NumVal: p},
	)
}

// ProtocolFromString creates a Protocol struct from a string value.
func ProtocolFromString(p string) Protocol {
	return Protocol(
		Uint8OrString{Type: NumOrStringString, StrVal: p},
	)
}

// UnmarshalJSON implements the json.Unmarshaller interface.
func (p *Protocol) UnmarshalJSON(b []byte) error {
	return (*Uint8OrString)(p).UnmarshalJSON(b)
}

// MarshalJSON implements the json.Marshaller interface.
func (p Protocol) MarshalJSON() ([]byte, error) {
	return Uint8OrString(p).MarshalJSON()
}

// String returns the string value, or the Itoa of the int value.
func (p Protocol) String() string {
	return (Uint8OrString)(p).String()
}

// ToV1 returns the V1 equivalent Protocol (i.e. lowercase protocol string).
func (p Protocol) ToV1() Protocol {
	if p.Type == NumOrStringNum {
		return p
	}
	return ProtocolFromString(strings.ToLower(p.StrVal))
}

// NumValue returns the NumVal if type Int, or if
// it is a String, will attempt a conversion to int.
func (p Protocol) NumValue() (uint8, error) {
	return (Uint8OrString)(p).NumValue()
}

// SupportsProtocols returns whether this protocol supports ports.  This returns true if
// the numerical or string verion of the protocol indicates TCP (6) or UDP (17).
func (p Protocol) SupportsPorts() bool {
	num, err := p.NumValue()
	if err == nil {
		return num == 6 || num == 17
	} else {
		switch p.StrVal {
		case ProtocolTCP, ProtocolUDP, ProtocolV1TCP, ProtocolV1UDP:
			return true
		}
		return false
	}
}
