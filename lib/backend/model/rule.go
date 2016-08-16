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

package model

import (
	"fmt"
	"github.com/tigera/libcalico-go/lib/net"
	"github.com/tigera/libcalico-go/lib/numorstring"
	"strconv"
	"strings"
)

type Rule struct {
	Action string `json:"action,omitempty" validate:"backendaction"`

	Protocol    *numorstring.Protocol `json:"protocol,omitempty" validate:"omitempty"`
	SrcTag      string                `json:"src_tag,omitempty" validate:"omitempty,tag"`
	SrcNet      *net.IPNet            `json:"src_net,omitempty" validate:"omitempty"`
	SrcSelector string                `json:"src_selector,omitempty" validate:"omitempty,selector"`
	SrcPorts    []numorstring.Port    `json:"src_ports,omitempty" validate:"omitempty"`
	DstTag      string                `json:"dst_tag,omitempty" validate:"omitempty,tag"`
	DstSelector string                `json:"dst_selector,omitempty" validate:"omitempty,selector"`
	DstNet      *net.IPNet            `json:"dst_net,omitempty" validate:"omitempty"`
	DstPorts    []numorstring.Port    `json:"dst_ports,omitempty" validate:"omitempty"`
	ICMPType    *int                  `json:"icmp_type,omitempty" validate:"omitempty,gte=1,lte=255"`
	ICMPCode    *int                  `json:"icmp_code,omitempty" validate:"omitempty,gte=1,lte=255"`

	NotProtocol    *numorstring.Protocol `json:"!protocol,omitempty" validate:"omitempty"`
	NotSrcTag      string                `json:"!src_tag,omitempty" validate:"omitempty,tag"`
	NotSrcNet      *net.IPNet            `json:"!src_net,omitempty" validate:"omitempty"`
	NotSrcSelector string                `json:"!src_selector,omitempty" validate:"omitempty,selector"`
	NotSrcPorts    []numorstring.Port    `json:"!src_ports,omitempty" validate:"omitempty"`
	NotDstTag      string                `json:"!dst_tag,omitempty" validate:"omitempty"`
	NotDstSelector string                `json:"!dst_selector,omitempty" validate:"omitempty,selector"`
	NotDstNet      *net.IPNet            `json:"!dst_net,omitempty" validate:"omitempty"`
	NotDstPorts    []numorstring.Port    `json:"!dst_ports,omitempty" validate:"omitempty"`
	NotICMPType    *int                  `json:"!icmp_type,omitempty" validate:"omitempty,gte=1,lte=255"`
	NotICMPCode    *int                  `json:"!icmp_code,omitempty" validate:"omitempty,gte=1,lte=255"`
}

func (r Rule) String() string {
	parts := make([]string, 0)
	if r.Action != "" {
		parts = append(parts, r.Action)
	} else {
		parts = append(parts, "allow")
	}
	if r.Protocol != nil {
		parts = append(parts, r.Protocol.String())
	}
	if r.ICMPType != nil {
		parts = append(parts, "type", strconv.Itoa(*r.ICMPType))
	}
	if r.ICMPCode != nil {
		parts = append(parts, "code", strconv.Itoa(*r.ICMPCode))
	}
	if r.NotICMPType != nil {
		parts = append(parts, "!type", strconv.Itoa(*r.NotICMPType))
	}
	if r.NotICMPCode != nil {
		parts = append(parts, "!code", strconv.Itoa(*r.NotICMPCode))
	}

	if r.SrcTag != "" || len(r.SrcPorts) > 0 || r.SrcNet != nil || r.SrcSelector != "" ||
		r.NotSrcTag != "" || len(r.NotSrcPorts) > 0 || r.NotSrcNet != nil || r.NotSrcSelector != "" {
		parts = append(parts, "from")
	}
	if len(r.SrcPorts) > 0 {
		srcPorts := make([]string, len(r.SrcPorts))
		for ii, port := range r.SrcPorts {
			srcPorts[ii] = port.String()
		}
		parts = append(parts, "ports", strings.Join(srcPorts, ","))
	}
	if r.SrcTag != "" {
		parts = append(parts, "tag", r.SrcTag)
	}
	if r.SrcSelector != "" {
		parts = append(parts, "selector", fmt.Sprintf("%#v", r.SrcSelector))
	}
	if r.SrcNet != nil {
		parts = append(parts, "cidr", r.SrcNet.String())
	}
	if len(r.NotSrcPorts) > 0 {
		notSrcPorts := make([]string, len(r.NotSrcPorts))
		for ii, port := range r.NotSrcPorts {
			notSrcPorts[ii] = port.String()
		}
		parts = append(parts, "!ports", strings.Join(notSrcPorts, ","))
	}
	if r.NotSrcTag != "" {
		parts = append(parts, "!tag", r.NotSrcTag)
	}
	if r.NotSrcSelector != "" {
		parts = append(parts, "!selector", fmt.Sprintf("%#v", r.NotSrcSelector))
	}
	if r.NotSrcNet != nil {
		parts = append(parts, "!cidr", r.NotSrcNet.String())
	}

	if r.DstTag != "" || len(r.DstPorts) > 0 || r.DstNet != nil || r.DstSelector != "" ||
		r.NotDstTag != "" || len(r.NotDstPorts) > 0 || r.NotDstNet != nil || r.NotDstSelector != "" {
		parts = append(parts, "to")
	}
	if len(r.DstPorts) > 0 {
		DstPorts := make([]string, len(r.DstPorts))
		for ii, port := range r.DstPorts {
			DstPorts[ii] = port.String()
		}
		parts = append(parts, "ports", strings.Join(DstPorts, ","))
	}
	if r.DstTag != "" {
		parts = append(parts, "tag", r.DstTag)
	}
	if r.DstSelector != "" {
		parts = append(parts, "selector", fmt.Sprintf("%#v", r.DstSelector))
	}
	if r.DstNet != nil {
		parts = append(parts, "cidr", r.DstNet.String())
	}
	if len(r.NotDstPorts) > 0 {
		NotDstPorts := make([]string, len(r.NotDstPorts))
		for ii, port := range r.NotDstPorts {
			NotDstPorts[ii] = port.String()
		}
		parts = append(parts, "!ports", strings.Join(NotDstPorts, ","))
	}
	if r.NotDstTag != "" {
		parts = append(parts, "!tag", r.NotDstTag)
	}
	if r.NotDstSelector != "" {
		parts = append(parts, "!selector", fmt.Sprintf("%#v", r.NotDstSelector))
	}
	if r.NotDstNet != nil {
		parts = append(parts, "!cidr", r.NotDstNet.String())
	}

	return strings.Join(parts, " ")
}
