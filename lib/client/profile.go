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
)

// ProfileInterface has methods to work with Profile resources.
type ProfileInterface interface {
	List(api.ProfileMetadata) (*api.ProfileList, error)
	Get(api.ProfileMetadata) (*api.Profile, error)
	Create(*api.Profile) (*api.Profile, error)
	Update(*api.Profile) (*api.Profile, error)
	Apply(*api.Profile) (*api.Profile, error)
	Delete(api.ProfileMetadata) error
}

// profiles implements ProfileInterface
type profiles struct {
	c *Client
}

// newProfiles returns a profiles
func newProfiles(c *Client) *profiles {
	return &profiles{c}
}

// Create creates a new profile.
func (h *profiles) Create(a *api.Profile) (*api.Profile, error) {
	return a, h.c.create(*a, h)
}

// Update updates an existing profile.
func (h *profiles) Update(a *api.Profile) (*api.Profile, error) {
	return a, h.c.update(*a, h)
}

// Apply creates a new or replaces an existing profile.
func (h *profiles) Apply(a *api.Profile) (*api.Profile, error) {
	return a, h.c.apply(*a, h)
}

// Delete deletes an existing profile.
func (h *profiles) Delete(metadata api.ProfileMetadata) error {
	return h.c.delete(metadata, h)
}

// Get returns information about a particular profile.
func (h *profiles) Get(metadata api.ProfileMetadata) (*api.Profile, error) {
	if a, err := h.c.get(metadata, h); err != nil {
		return nil, err
	} else {
		return a.(*api.Profile), nil
	}
}

// List takes a Metadata, and returns the list of profiles that match that Metadata
// (wildcarding missing fields)
func (h *profiles) List(metadata api.ProfileMetadata) (*api.ProfileList, error) {
	l := api.NewProfileList()
	err := h.c.list(metadata, h, l)
	return l, err
}

// Convert a ProfileMetadata to a ProfileListInterface
func (h *profiles) convertMetadataToListInterface(m interface{}) (backend.ListInterface, error) {
	hm := m.(api.ProfileMetadata)
	l := backend.ProfileListOptions{
		Name: hm.Name,
	}
	return l, nil
}

// Convert a ProfileMetadata to a ProfileKeyInterface
func (h *profiles) convertMetadataToKeyInterface(m interface{}) (backend.KeyInterface, error) {
	hm := m.(api.ProfileMetadata)
	k := backend.ProfileKey{
		Name: hm.Name,
	}
	return k, nil
}

// Convert an API Profile structure to a Backend Profile structure
func (h *profiles) convertAPIToDatastoreObject(a interface{}) (*backend.DatastoreObject, error) {
	ap := a.(api.Profile)
	k, err := h.convertMetadataToKeyInterface(ap.Metadata)
	if err != nil {
		return nil, err
	}

	d := backend.DatastoreObject{
		Key: k,
		Object: backend.Profile{
			Rules: backend.ProfileRules{
				InboundRules:  rulesAPIToBackend(ap.Spec.IngressRules),
				OutboundRules: rulesAPIToBackend(ap.Spec.EgressRules),
			},
			Tags:   ap.Spec.Tags,
			Labels: ap.Metadata.Labels,
		},
	}

	return &d, nil
}

// Convert a Backend Profile structure to an API Profile structure
func (h *profiles) convertDatastoreObjectToAPI(d *backend.DatastoreObject) (interface{}, error) {
	bp := d.Object.(backend.Profile)
	bk := d.Key.(backend.ProfileKey)

	ap := api.NewProfile()
	ap.Metadata.Name = bk.Name
	ap.Metadata.Labels = bp.Labels
	ap.Spec.IngressRules = rulesBackendToAPI(bp.Rules.InboundRules)
	ap.Spec.EgressRules = rulesBackendToAPI(bp.Rules.OutboundRules)
	ap.Spec.Tags = bp.Tags

	return ap, nil
}
