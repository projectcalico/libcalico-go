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

package client

import (
	"github.com/projectcalico/libcalico-go/lib/api"
	"github.com/projectcalico/libcalico-go/lib/api/unversioned"
	"github.com/projectcalico/libcalico-go/lib/backend/model"
	"github.com/projectcalico/libcalico-go/lib/net"
)

// NetworkSetInterface has methods to work with host endpoint resources.
type NetworkSetInterface interface {

	// List enumerates host endpoint resources matching the supplied metadata and
	// wildcarding missing identifiers.
	List(api.NetworkSetMetadata) (*api.NetworkSetList, error)

	// Get returns the host endpoint resource matching the supplied metadata.  The metadata
	// should contain all identifiers to uniquely identify a single resource.  If the
	// resource does not exist, a errors.ErrorResourceNotFound error is returned.
	Get(api.NetworkSetMetadata) (*api.NetworkSet, error)

	// Create will create a new host endpoint resource.  If the resource already exists,
	// a errors.ErrorResourceAlreadyExists error is returned.
	Create(*api.NetworkSet) (*api.NetworkSet, error)

	// Update will update an existing host endpoint resource.  If the resource does not exist,
	// a errors.ErrorResourceDoesNotExist error is returned.
	Update(*api.NetworkSet) (*api.NetworkSet, error)

	// Apply with update an existing host endpoint resource, or create a new one if it does
	// not exist.
	Apply(*api.NetworkSet) (*api.NetworkSet, error)

	// Delete will delete a host endpoint resource.  The metadata should contain all identifiers
	// to uniquely identify a single resource.  If the resource does not exist, a
	// errors.ErrorResourceDoesNotExist error is returned.
	Delete(api.NetworkSetMetadata) error
}

// NetworkSets implements NetworkSetInterface
type NetworkSets struct {
	c *Client
}

// newNetworkSets returns a NetworkSetInterface bound to the supplied client.
func newNetworkSets(c *Client) NetworkSetInterface {
	return &NetworkSets{c}
}

// Create creates a new host endpoint.
func (h *NetworkSets) Create(a *api.NetworkSet) (*api.NetworkSet, error) {
	return a, h.c.create(*a, h)
}

// Update updates an existing host endpoint.
func (h *NetworkSets) Update(a *api.NetworkSet) (*api.NetworkSet, error) {
	return a, h.c.update(*a, h)
}

// Apply updates a host endpoint if it exists, or creates a new host endpoint if it does not exist.
func (h *NetworkSets) Apply(a *api.NetworkSet) (*api.NetworkSet, error) {
	return a, h.c.apply(*a, h)
}

// Delete deletes an existing host endpoint.
func (h *NetworkSets) Delete(metadata api.NetworkSetMetadata) error {
	return h.c.delete(metadata, h)
}

// Get returns information about a particular host endpoint.
func (h *NetworkSets) Get(metadata api.NetworkSetMetadata) (*api.NetworkSet, error) {
	if a, err := h.c.get(metadata, h); err != nil {
		return nil, err
	} else {
		return a.(*api.NetworkSet), nil
	}
}

// List takes a Metadata, and returns a NetworkSetList that contains the list of host endpoints
// that match the Metadata (wildcarding missing fields).
func (h *NetworkSets) List(metadata api.NetworkSetMetadata) (*api.NetworkSetList, error) {
	l := api.NewNetworkSetList()
	err := h.c.list(metadata, h, l)
	return l, err
}

// convertMetadataToListInterface converts a NetworkSetMetadata to a NetworkSetListOptions.
// This is part of the conversionHelper interface.
func (h *NetworkSets) convertMetadataToListInterface(m unversioned.ResourceMetadata) (model.ListInterface, error) {
	hm := m.(api.NetworkSetMetadata)
	l := model.NetworkSetListOptions{
		Name: hm.Name,
	}
	return l, nil
}

// convertMetadataToKey converts a NetworkSetMetadata to a NetworkSetKey
// This is part of the conversionHelper interface.
func (h *NetworkSets) convertMetadataToKey(m unversioned.ResourceMetadata) (model.Key, error) {
	hm := m.(api.NetworkSetMetadata)
	k := model.NetworkSetKey{
		Name: hm.Name,
	}
	return k, nil
}

// convertAPIToKVPair converts an API NetworkSet structure to a KVPair containing a
// backend NetworkSet and NetworkSetKey.
// This is part of the conversionHelper interface.
func (h *NetworkSets) convertAPIToKVPair(a unversioned.Resource) (*model.KVPair, error) {
	ah := a.(api.NetworkSet)
	k, err := h.convertMetadataToKey(ah.Metadata)
	if err != nil {
		return nil, err
	}

	var ipv4Nets []net.IPNet
	var ipv6Nets []net.IPNet
	for _, n := range ah.Spec.Nets {
		if n.IP.To4() == nil {
			ipv6Nets = append(ipv6Nets, n)
		} else {
			ipv4Nets = append(ipv4Nets, n)
		}
	}

	d := model.KVPair{
		Key: k,
		Value: &model.NetworkSet{
			Labels:   ah.Metadata.Labels,
			IPv4Nets: ipv4Nets,
			IPv6Nets: ipv6Nets,
		},
	}

	return &d, nil
}

// convertKVPairToAPI converts a KVPair containing a backend NetworkSet and NetworkSetKey
// to an API NetworkSet structure.
// This is part of the conversionHelper interface.
func (h *NetworkSets) convertKVPairToAPI(d *model.KVPair) (unversioned.Resource, error) {
	bh := d.Value.(*model.NetworkSet)
	bk := d.Key.(model.NetworkSetKey)

	nets := make([]net.IPNet, 0, len(bh.IPv4Nets)+len(bh.IPv6Nets))
	nets = append(nets, bh.IPv4Nets...)
	nets = append(nets, bh.IPv6Nets...)

	ah := api.NewNetworkSet()
	ah.Metadata.Name = bk.Name
	ah.Metadata.Labels = bh.Labels
	ah.Spec.Nets = nets

	return ah, nil
}
