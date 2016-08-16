// Copyright (c) 2016 Tigera, Inc. All rights reserved.
//
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

package compat

import (
	"github.com/golang/glog"
	"github.com/tigera/libcalico-go/lib/backend/api"
	. "github.com/tigera/libcalico-go/lib/backend/model"
)

type ModelAdaptor struct {
	client api.Client
}

var _ api.Client = (*ModelAdaptor)(nil)

func NewAdaptor(c api.Client) *ModelAdaptor {
	return &ModelAdaptor{client: c}
}

// Create an entry in the datastore.  This errors if the entry already exists.
func (c *ModelAdaptor) Create(d *KVPair) (*KVPair, error) {
	if _, ok := d.Key.(ProfileKey); ok {
		t, l, r := toTagsLabelsRules(d)
		if t, err := c.client.Create(t); err != nil {
			return nil, err
		} else if _, err := c.client.Create(l); err != nil {
			return nil, err
		} else if _, err := c.client.Create(r); err != nil {
			return nil, err
		} else {
			d.Revision = t.Revision
			return d, nil
		}
	}
	return c.client.Create(d)
}

// Update an existing entry in the datastore.  This errors if the entry does
// not exist.
func (c *ModelAdaptor) Update(d *KVPair) (*KVPair, error) {
	if _, ok := d.Key.(ProfileKey); ok {
		t, l, r := toTagsLabelsRules(d)
		if t, err := c.client.Update(t); err != nil {
			return nil, err
		} else if _, err := c.client.Apply(l); err != nil {
			return nil, err
		} else if _, err := c.client.Apply(r); err != nil {
			return nil, err
		} else {
			d.Revision = t.Revision
			return d, nil
		}
	}
	return c.client.Update(d)
}

// Set an existing entry in the datastore.  This ignores whether an entry already
// exists.
func (c *ModelAdaptor) Apply(d *KVPair) (*KVPair, error) {
	if _, ok := d.Key.(ProfileKey); ok {
		t, l, r := toTagsLabelsRules(d)
		if t, err := c.client.Apply(t); err != nil {
			return nil, err
		} else if _, err := c.client.Apply(l); err != nil {
			return nil, err
		} else if _, err := c.client.Apply(r); err != nil {
			return nil, err
		} else {
			d.Revision = t.Revision
			return d, nil
		}
	}
	return c.client.Apply(d)
}

// Delete an entry in the datastore.  This errors if the entry does not exists.
func (c *ModelAdaptor) Delete(d *KVPair) error {
	return c.client.Delete(d)
}

// Get an entry from the datastore.  This errors if the entry does not exist.
func (c *ModelAdaptor) Get(k Key) (*KVPair, error) {
	if _, ok := k.(ProfileKey); ok {
		var t, l, r *KVPair
		var err error
		pk := k.(ProfileKey)

		if t, err = c.client.Get(ProfileTagsKey{pk}); err != nil {
			return nil, err
		}
		d := KVPair{
			Key: k,
			Value: Profile{
				Tags: t.Value.([]string),
			},
			Revision: t.Revision,
		}
		p := d.Value.(Profile)
		if l, err = c.client.Get(ProfileLabelsKey{pk}); err == nil {
			p.Labels = l.Value.(map[string]string)
		}
		if r, err = c.client.Get(ProfileRulesKey{pk}); err == nil {
			p.Rules = r.Value.(ProfileRules)
		}
		return &d, nil
	}
	return c.client.Get(k)
}

// List entries in the datastore.  This may return an empty list of there are
// no entries matching the request in the ListInterface.
func (c *ModelAdaptor) List(l ListInterface) ([]*KVPair, error) {
	return c.client.List(l)
}

func (c *ModelAdaptor) Syncer(callbacks api.SyncerCallbacks) api.Syncer {
	return c.client.Syncer(callbacks)
}

// Convert a Profile KVPair to separate KVPair types for Keys, Labels and Rules.
// These separate KVPairs are used to write three separate objects that make up
// a single profile.
func toTagsLabelsRules(d *KVPair) (t, l, r *KVPair) {
	p := d.Value.(Profile)
	pk := d.Key.(ProfileKey)

	t = &KVPair{
		Key:      ProfileTagsKey{pk},
		Value:    p.Tags,
		Revision: d.Revision,
	}
	l = &KVPair{
		Key:   ProfileLabelsKey{pk},
		Value: p.Labels,
	}
	r = &KVPair{
		Key:   ProfileRulesKey{pk},
		Value: p.Rules,
	}

	// Fix up tags and labels so to be empty values rather than nil.  Felix does not
	// expect a null value in the JSON, so we fix up to make Labels an empty map
	// and tags an empty slice.
	if p.Labels == nil {
		glog.V(1).Info("Labels is nil - convert to empty map for backend")
		l.Value = map[string]string{}
	}
	if p.Tags == nil {
		glog.V(1).Info("Tags is nil - convert to empty map for backend")
		t.Value = []string{}
	}

	return t, l, r
}
