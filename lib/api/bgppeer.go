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
	. "github.com/tigera/libcalico-go/lib/api/unversioned"
	. "github.com/tigera/libcalico-go/lib/net"
)

type BGPPeerMetadata struct {
	ObjectMetadata

	// The hostname of the node that is peering with this peer.  If left blank,
	// Calico assumes this is a "global" BGP peer - i.e. it peers with every Calico
	// node in the deployment.
	Hostname string `json:"hostname,omitempty" validate:"omitempty,name"`

	// The IP address of the peer.
	PeerIP IP `json:"peerIP" validate:"required,ip"`
}

type BGPPeerSpec struct {
	// The AS Number of the peer.
	ASNumber int `json:"asNumber" validate:"required,asn"`
}

type BGPPeer struct {
	TypeMetadata

	// Metadata for a BGPPeer.
	Metadata BGPPeerMetadata `json:"metadata,omitempty"`

	// Specification for a BGPPeer.
	Spec BGPPeerSpec `json:"spec,omitempty"`
}

func NewBGPPeer() *BGPPeer {
	return &BGPPeer{TypeMetadata: TypeMetadata{Kind: "bgpPeer", APIVersion: "v1"}}
}

type BGPPeerList struct {
	TypeMetadata
	Metadata ListMetadata `json:"metadata,omitempty"`
	Items    []BGPPeer    `json:"items" validate:"dive"`
}

func NewBGPPeerList() *BGPPeerList {
	return &BGPPeerList{TypeMetadata: TypeMetadata{Kind: "bgpPeerList", APIVersion: "v1"}}
}
