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

package v2

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

const (
	KindIPPool     = "IPPool"
	KindIPPoolList = "IPPoolList"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// IPPool contains information about a IPPool resource.
type IPPool struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the IPPool.
	Spec IPPoolSpec `json:"spec,omitempty"`
}

// IPPoolSpec contains the specification for an IPPool resource.
type IPPoolSpec struct {
	// The pool CIDR.
	CIDR string `json:"cidr" validate:"omitempty,cidr"`
	// Contains configuration for ipip tunneling for this pool. If not specified,
	// then ipip tunneling is disabled for this pool.
	IPIP *IPIPConfiguration `json:"ipip,omitempty"`
	// When nat-outgoing is true, packets sent from Calico networked containers in
	// this pool to destinations outside of this pool will be masqueraded.
	NATOutgoing bool `json:"natOutgoing,omitempty"`
	// When disabled is true, Calico IPAM will not assign addresses from this pool.
	Disabled bool `json:"disabled,omitempty"`
}

type IPIPConfiguration struct {
	// The IPIP mode.  This can be one of "Never", "Always" or "CrossSubnet".  A mode
	// of "Always" will also use IPIP tunneling for routing to destination IP
	// addresses within this pool.  A mode of "CrossSubnet" will only use IPIP
	// tunneling when the destination node is on a different subnet to the
	// originating node.  The default value (if not specified) is "Always".
	Mode IPIPMode `json:"mode,omitempty" validate:"omitempty,ipipmode"`
}

type IPIPMode string

const (
	IPIPModeNever       IPIPMode = "Never"
	IPIPModeAlways               = "Always"
	IPIPModeCrossSubnet          = "CrossSubnet"
)
const DefaultMode = IPIPModeAlways

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// IPPoolList contains a list of IPPool resources.
type IPPoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []IPPool `json:"items"`
}

// NewIPPool creates a new (zeroed) IPPool struct with the TypeMetadata initialised to the current
// version.
func NewIPPool() *IPPool {
	return &IPPool{
		TypeMeta: metav1.TypeMeta{
			Kind:       KindIPPool,
			APIVersion: GroupVersionCurrent,
		},
	}
}

// NewIPPoolList creates a new (zeroed) IPPoolList struct with the TypeMetadata initialised to the current
// version.
func NewIPPoolList() *IPPoolList {
	return &IPPoolList{
		TypeMeta: metav1.TypeMeta{
			Kind:       KindIPPoolList,
			APIVersion: GroupVersionCurrent,
		},
	}
}
