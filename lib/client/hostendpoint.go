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
	"github.com/tigera/libcalico-go/lib/backend"
	. "github.com/tigera/libcalico-go/lib/common"
)

// HostEndpointInterface has methods to work with host endpoint resources.
type HostEndpointInterface interface {

	// List enumerates host endpoint resources matching the supplied metadata and
	// wildcarding missing identifiers.
	List(api.HostEndpointMetadata) (*api.HostEndpointList, error)

	// Get returns the host endpoint resource matching the supplied metadata.  The metadata
	// should contain all identifiers to uniquely identify a single resource.  If the
	// resource does not exist, a common.ErrorResourceNotFound error is returned.
	Get(api.HostEndpointMetadata) (*api.HostEndpoint, error)

	// Create will create a new host endpoint resource.  If the resource already exists,
	// a common.ErrorResourceAlreadyExists error is returned.
	Create(*api.HostEndpoint) (*api.HostEndpoint, error)

	// Update will update an existing host endpoint resource.  If the resource does not exist,
	// a common.ErrorResourceDoesNotExist error is returned.
	Update(*api.HostEndpoint) (*api.HostEndpoint, error)

	// Apply with update an existing host endpoint resource, or create a new one if it does
	// not exist.
	Apply(*api.HostEndpoint) (*api.HostEndpoint, error)

	// Delete will delete a host endpoint resource.  The metadata should contain all identifiers
	// to uniquely identify a single resource.  If the resource does not exist, a
	// common.ErrorResourceDoesNotExist error is returned.
	Delete(api.HostEndpointMetadata) error
}

// hostEndpoints implements HostEndpointInterface
type hostEndpoints struct {
	c *Client
}

// newHostEndpoints returns a hostEndpoints
func newHostEndpoints(c *Client) *hostEndpoints {
	return &hostEndpoints{c}
}

// Create creates a new host endpoint.
func (h *hostEndpoints) Create(a *api.HostEndpoint) (*api.HostEndpoint, error) {
	return a, h.c.create(*a, h)
}

// Create creates a new host endpoint.
func (h *hostEndpoints) Update(a *api.HostEndpoint) (*api.HostEndpoint, error) {
	return a, h.c.update(*a, h)
}

// Create creates a new host endpoint.
func (h *hostEndpoints) Apply(a *api.HostEndpoint) (*api.HostEndpoint, error) {
	return a, h.c.apply(*a, h)
}

// Delete deletes an existing host endpoint.
func (h *hostEndpoints) Delete(metadata api.HostEndpointMetadata) error {
	return h.c.delete(metadata, h)
}

// Get returns information about a particular host endpoint.
func (h *hostEndpoints) Get(metadata api.HostEndpointMetadata) (*api.HostEndpoint, error) {
	if a, err := h.c.get(metadata, h); err != nil {
		return nil, err
	} else {
		return a.(*api.HostEndpoint), nil
	}
}

// List takes a Metadata, and returns the list of host endpoints that match that Metadata
// (wildcarding missing fields)
func (h *hostEndpoints) List(metadata api.HostEndpointMetadata) (*api.HostEndpointList, error) {
	l := api.NewHostEndpointList()
	err := h.c.list(metadata, h, l)
	return l, err
}

// Convert a HostEndpointMetadata to a HostEndpointListInterface
func (h *hostEndpoints) convertMetadataToListInterface(m interface{}) (backend.ListInterface, error) {
	hm := m.(api.HostEndpointMetadata)
	l := backend.HostEndpointListOptions{
		Hostname:   hm.Hostname,
		EndpointID: hm.Name,
	}
	return l, nil
}

// Convert a HostEndpointMetadata to a HostEndpointKeyInterface
func (h *hostEndpoints) convertMetadataToKeyInterface(m interface{}) (backend.KeyInterface, error) {
	hm := m.(api.HostEndpointMetadata)
	k := backend.HostEndpointKey{
		Hostname:   hm.Hostname,
		EndpointID: hm.Name,
	}
	return k, nil
}

// Convert an API HostEndpoint structure to a Backend HostEndpoint structure
func (h *hostEndpoints) convertAPIToDatastoreObject(a interface{}) (*backend.DatastoreObject, error) {
	ah := a.(api.HostEndpoint)
	k, err := h.convertMetadataToKeyInterface(ah.Metadata)
	if err != nil {
		return nil, err
	}

	var ipv4Addrs []IP
	var ipv6Addrs []IP
	for _, ip := range ah.Spec.ExpectedIPs {
		if ip.Version() == 4 {
			ipv4Addrs = append(ipv4Addrs, ip)
		} else {
			ipv6Addrs = append(ipv6Addrs, ip)
		}
	}

	d := backend.DatastoreObject{
		Key: k,
		Object: backend.HostEndpoint{
			Labels: ah.Metadata.Labels,

			Name:              ah.Spec.InterfaceName,
			ProfileIDs:        ah.Spec.Profiles,
			ExpectedIPv4Addrs: ipv4Addrs,
			ExpectedIPv6Addrs: ipv6Addrs,
		},
	}

	return &d, nil
}

// Convert a Backend HostEndpoint structure to an API HostEndpoint structure
func (h *hostEndpoints) convertDatastoreObjectToAPI(d *backend.DatastoreObject) (interface{}, error) {
	bh := d.Object.(backend.HostEndpoint)
	bk := d.Key.(backend.HostEndpointKey)

	ips := bh.ExpectedIPv4Addrs
	ips = append(ips, bh.ExpectedIPv6Addrs...)

	ah := api.NewHostEndpoint()
	ah.Metadata.Hostname = bk.Hostname
	ah.Metadata.Name = bk.EndpointID
	ah.Metadata.Labels = bh.Labels
	ah.Spec.InterfaceName = bh.Name
	ah.Spec.Profiles = bh.ProfileIDs
	ah.Spec.ExpectedIPs = ips

	return ah, nil
}
