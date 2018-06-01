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

package v3

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/projectcalico/libcalico-go/lib/numorstring"
)

const (
	KindBGPPeer     = "BGPPeer"
	KindBGPPeerList = "BGPPeerList"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BGPPeer contains information about a BGPPeer resource that is a peer of a Calico
// compute node.
type BGPPeer struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the BGPPeer.
	Spec BGPPeerSpec `json:"spec,omitempty"`
}

// BGPPeerSpec contains the specification for a BGPPeer resource.
type BGPPeerSpec struct {
	// The node name identifying the Calico node instance that is peering with this peer.
	// If this is not set, this represents a global peer, i.e. a peer that peers with
	// every node in the deployment.
	Node string `json:"node,omitempty" validate:"omitempty,name"`
	// The IP address of the peer.
	PeerIP string `json:"peerIP" validate:"omitempty,ip"`
	// The AS Number of the peer.
	ASNumber numorstring.ASNumber `json:"asNumber"`
	// Password, if specified, is the TCP MD5 password used to authenticate peerings to this peer.
	Password string `json:"password,omitempty" validate:"omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BGPPeerList contains a list of BGPPeer resources.
type BGPPeerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []BGPPeer `json:"items"`
}

// NewBGPPeer creates a new (zeroed) BGPPeer struct with the TypeMetadata initialised to the current
// version.
func NewBGPPeer() *BGPPeer {
	return &BGPPeer{
		TypeMeta: metav1.TypeMeta{
			Kind:       KindBGPPeer,
			APIVersion: GroupVersionCurrent,
		},
	}
}

// NewBGPPeerList creates a new (zeroed) BGPPeerList struct with the TypeMetadata initialised to the current
// version.
func NewBGPPeerList() *BGPPeerList {
	return &BGPPeerList{
		TypeMeta: metav1.TypeMeta{
			Kind:       KindBGPPeerList,
			APIVersion: GroupVersionCurrent,
		},
	}
}
