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

package api

import (
	"github.com/projectcalico/libcalico-go/lib/net"
	"github.com/projectcalico/libcalico-go/lib/numorstring"
)

// A Rule encapsulates a set of match criteria and an action.  Both selector-based security Policy
// and security Profiles reference rules - separated out as a list of rules for both
// ingress and egress packet matching.
//
// Each positive match criteria has a negated version, prefixed with ”Not”. All the match
// criteria within a rule must be satisfied for a packet to match. A single rule can contain
// the positive and negative version of a match and both must be satisfied for the rule to match.
type Rule struct {
	Action string `json:"action" validate:"action"`

	// IPVersion is an optional field that restricts the rule to only match a specific IP
	// version.
	IPVersion *int `json:"ipVersion,omitempty" validate:"omitempty,ipversion"`

	// Protocol is an optional field that restricts the rule to only apply to traffic of
	// a specific IP protocol. Required if any of the EntityRules contain Ports
	// (because ports only apply to certain protocols).
	//
	// Must be one of these string values: "tcp", "udp", "icmp", "icmpv6", "sctp", "udplite"
	// or an integer in the range 1-255.
	Protocol *numorstring.Protocol `json:"protocol,omitempty" validate:"omitempty"`

	// ICMP is an optional field that restricts the rule to apply to a specific type and
	// code of ICMP traffic.  This should only be specified if the Protocol field is set to
	// "icmp" or "icmpv6".
	ICMP *ICMPFields `json:"icmp,omitempty" validate:"omitempty"`

	// NotProtocol is the negated version of the Protocol field.
	NotProtocol *numorstring.Protocol `json:"notProtocol,omitempty" validate:"omitempty"`

	// NotICMP is the negated version of the ICMP field.
	NotICMP *ICMPFields `json:"notICMP,omitempty" validate:"omitempty"`

	// Source contains the match criteria that apply to source entity.
	Source EntityRule `json:"source,omitempty" validate:"omitempty"`

	// Destination contains the match criteria that apply to destination entity.
	Destination EntityRule `json:"destination,omitempty" validate:"omitempty"`
}

// ICMPFields defines structure for ICMP and NotICMP sub-struct for ICMP code and type
type ICMPFields struct {
	// Match on a specific ICMP type.  For example a value of 8 refers to ICMP Echo Request
	// (i.e. pings).
	Type *int `json:"type,omitempty" validate:"omitempty,gte=0,lte=254"`

	// Match on a specific ICMP code.  If specified, the Type value must also be specified.
	// This is a technical limitation imposed by the kernel’s iptables firewall, which
	// Calico uses to enforce the rule.
	Code *int `json:"code,omitempty" validate:"omitempty,gte=0,lte=255"`
}

// An EntityRule is a sub-component of a Rule comprising the match criteria specific
// to a particular entity (that is either the source or destination).
//
// A source EntityRule matches the source endpoint and originating traffic.
// A desination EntityRule matches the destination endpoint and terminating traffic.
type EntityRule struct {
	// Tag is an optional field that restricts the rule to only apply to traffic that
	// originates from (or terminates at) endpoints that have profiles with the given tag
	// in them.
	Tag string `json:"tag,omitempty" validate:"omitempty,tag"`

	// Net is an optional field that restricts the rule to only apply to traffic that
	// originates from (or terminates at) IP addresses in the given subnet.
	Net *net.IPNet `json:"net,omitempty" validate:"omitempty"`

	// Selector is an optional field that contains a selector expression (see Policy for
	// sample syntax).  Only traffic that originates from (terminates at) endpoints matching
	// the selector will be matched.
	//
	// Note that: in addition to the negated version of the Selector (see NotSelector below), the
	// selector expression syntax itself supports negation.  The two types of negation are subtly
	// different. One negates the set of matched endpoints, the other negates the whole match:
	//
	//	Selector = "!has(my_label)" matches packets that are from other Calico-controlled
	// 	endpoints that do not have the label “my_label”.
	//
	// 	NotSelector = "has(my_label)" matches packets that are not from Calico-controlled
	// 	endpoints that do have the label “my_label”.
	//
	// The effect is that the latter will accept packets from non-Calico sources whereas the
	// former is limited to packets from Calico-controlled endpoints.
	Selector string `json:"selector,omitempty" validate:"omitempty,selector"`

	// Ports is an optional field that restricts the rule to only apply to traffic that has a
	// source (destination) port that matches one of these ranges/values. This value is a
	// list of integers or strings that represent ranges of ports.
	//
	// Since only some protocols have ports, if any ports are specified it requires the
	// Protocol match in the Rule to be set to "tcp" or "udp".
	Ports []numorstring.Port `json:"ports,omitempty" validate:"omitempty,dive"`

	// NotTag is the negated version of the Tag field.
	NotTag string `json:"notTag,omitempty" validate:"omitempty,tag"`

	// NotNet is the negated version of the Net field.
	NotNet *net.IPNet `json:"notNet,omitempty" validate:"omitempty"`

	// NotSelector is the negated version of the Selector field.  See Selector field for
	// subtleties with negated selectors.
	NotSelector string `json:"notSelector,omitempty" validate:"omitempty,selector"`

	// NotPorts is the negated version of the Ports field.
	//
	// Since only some protocols have ports, if any ports are specified it requires the
	// Protocol match in the Rule to be set to "tcp" or "udp".
	NotPorts []numorstring.Port `json:"notPorts,omitempty" validate:"omitempty,dive"`
}
