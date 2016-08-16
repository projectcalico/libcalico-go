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

package client

import (
	"github.com/tigera/libcalico-go/lib/api"
	"github.com/tigera/libcalico-go/lib/api/unversioned"
	"github.com/tigera/libcalico-go/lib/backend/model"
)

// BGPPeerInterface has methods to work with BGPPeer resources.
type BGPPeerInterface interface {
	List(api.BGPPeerMetadata) (*api.BGPPeerList, error)
	Get(api.BGPPeerMetadata) (*api.BGPPeer, error)
	Create(*api.BGPPeer) (*api.BGPPeer, error)
	Update(*api.BGPPeer) (*api.BGPPeer, error)
	Apply(*api.BGPPeer) (*api.BGPPeer, error)
	Delete(api.BGPPeerMetadata) error
}

// peers implements BGPPeerInterface
type peers struct {
	c *Client
}

// newBGPPeers returns a peers
func newBGPPeers(c *Client) *peers {
	return &peers{c}
}

// Create creates a new pool.
func (h *peers) Create(a *api.BGPPeer) (*api.BGPPeer, error) {
	return a, h.c.create(*a, h)
}

// Create creates a new pool.
func (h *peers) Update(a *api.BGPPeer) (*api.BGPPeer, error) {
	return a, h.c.update(*a, h)
}

// Create creates a new pool.
func (h *peers) Apply(a *api.BGPPeer) (*api.BGPPeer, error) {
	return a, h.c.apply(*a, h)
}

// Delete deletes an existing pool.
func (h *peers) Delete(metadata api.BGPPeerMetadata) error {
	return h.c.delete(metadata, h)
}

// Get returns information about a particular pool.
func (h *peers) Get(metadata api.BGPPeerMetadata) (*api.BGPPeer, error) {
	if a, err := h.c.get(metadata, h); err != nil {
		return nil, err
	} else {
		return a.(*api.BGPPeer), nil
	}
}

// List takes a Metadata, and returns the list of peers that match that Metadata
// (wildcarding missing fields)
func (h *peers) List(metadata api.BGPPeerMetadata) (*api.BGPPeerList, error) {
	l := api.NewBGPPeerList()
	err := h.c.list(metadata, h, l)
	return l, err
}

// Convert a BGPPeerMetadata to a BGPPeerListInterface
func (h *peers) convertMetadataToListInterface(m unversioned.ResourceMetadata) (model.ListInterface, error) {
	pm := m.(api.BGPPeerMetadata)
	l := model.BGPPeerListOptions{
		PeerIP:   pm.PeerIP,
		Hostname: pm.Hostname,
	}
	return l, nil
}

// Convert a BGPPeerMetadata to a BGPPeerKeyInterface
func (h *peers) convertMetadataToKey(m unversioned.ResourceMetadata) (model.Key, error) {
	pm := m.(api.BGPPeerMetadata)
	k := model.BGPPeerKey{
		PeerIP:   pm.PeerIP,
		Hostname: pm.Hostname,
	}
	return k, nil
}

// Convert an API BGPPeer structure to a Backend BGPPeer structure
func (h *peers) convertAPIToKVPair(a unversioned.Resource) (*model.KVPair, error) {
	ap := a.(api.BGPPeer)
	k, err := h.convertMetadataToKey(ap.Metadata)
	if err != nil {
		return nil, err
	}

	d := model.KVPair{
		Key: k,
		Value: model.BGPPeer{
			PeerIP: ap.Metadata.PeerIP,
			ASNum:  ap.Spec.ASNum,
		},
	}

	return &d, nil
}

// Convert a Backend BGPPeer structure to an API BGPPeer structure
func (h *peers) convertKVPairToAPI(d *model.KVPair) (unversioned.Resource, error) {
	backendBGPPeer := d.Value.(model.BGPPeer)
	backendBGPPeerKey := d.Key.(model.BGPPeerKey)

	apiBGPPeer := api.NewBGPPeer()
	apiBGPPeer.Metadata.PeerIP = backendBGPPeerKey.PeerIP
	apiBGPPeer.Metadata.Hostname = backendBGPPeerKey.Hostname
	apiBGPPeer.Spec.ASNum = backendBGPPeer.ASNum

	return apiBGPPeer, nil
}
