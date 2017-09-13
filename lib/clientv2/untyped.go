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

package clientv2

import (
	"k8s.io/apimachinery/pkg/watch"

	"github.com/projectcalico/libcalico-go/lib/options"
	"k8s.io/apimachinery/pkg/runtime"
	"github.com/projectcalico/libcalico-go/lib/backend/model"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"github.com/projectcalico/libcalico-go/lib/errors"
	"k8s.io/apimachinery/pkg/api/meta"
)

type resource interface {
	runtime.Object
	v1.ObjectMetaAccessor
}

type resourceList interface {
	runtime.Object
	v1.ListMetaAccessor
}

// untypedInterface has methods to work with BgpPeer resources.
type untypedInterface interface {
	Create(opts options.SetOptions, in resource)  (resource, error)
	Update(opts options.SetOptions, in resource)  (resource, error)
	Delete(opts options.DeleteOptions, kind, namespace, name string)  error
	Get(opts options.GetOptions, kind, namespace, name string) (resource, error)
	List(opts options.ListOptions, kind, namespace, name string, inout resourceList) error
	Watch(opts options.ListOptions, kind, namespace, name string) (watch.Interface, error)
}

// untyped implements UntypedInterface
type untyped struct {
	client client
}

// Create creates a resource in the backend datastore.
func (c *untyped) Create(opts options.SetOptions, in resource) (resource, error) {
	if len(in.GetObjectMeta().GetResourceVersion()) != 0 {
		return nil, errors.ErrorValidation{
			ErroredFields: []errors.ErroredField{{
				Name: "Metadata.ResourceVersion",
				Reason: "ResourceVersion should not be set for a Create request",
				Value: in.GetObjectMeta().GetResourceVersion(),
			}},
		}
	}
	kvp := c.resourceToKVPair(opts, in)
	kvp, err := c.client.Backend.Create(kvp)
	if err != nil {
		return nil, err
	}
	out := c.kvPairToResource(kvp)
	return out, nil
}

// Update updates a resource in the backend datastore.
func (c *untyped) Update(opts options.SetOptions, in resource) (resource, error) {
	if len(in.GetObjectMeta().GetResourceVersion()) == 0 {
		return nil, errors.ErrorValidation{
			ErroredFields: []errors.ErroredField{{
				Name: "Metadata.ResourceVersion",
				Reason: "ResourceVersion must be set for an Update request",
				Value: in.GetObjectMeta().GetResourceVersion(),
			}},
		}
	}
	kvp, err := c.client.Backend.Update(c.resourceToKVPair(opts, in))
	if err != nil {
		return nil, err
	}
	out := c.kvPairToResource(kvp)
	return out, nil
}

// Delete deletes a resource from the backend datastore.
func (c *untyped) Delete(opts options.DeleteOptions, kind, namespace, name string) error {
	key := model.ResourceKey{
		Kind: kind,
		Name: name,
		Namespace: namespace,
	}
	return c.client.Backend.Delete(key, opts.ResourceVersion)
}

// Get gets a resource from the backend datastore.
func (c *untyped) Get(opts options.GetOptions, kind, namespace, name string) (resource, error) {
	key := model.ResourceKey{
			Kind: kind,
			Name: name,
			Namespace: namespace,
		}
	kvp, err := c.client.Backend.Get(key, opts.ResourceVersion)
	if err != nil {
		return nil, err
	}
	out := c.kvPairToResource(kvp)
	return out, nil
}

// List lists a resource from the backend datastore.
func (c *untyped) List(opts options.ListOptions, kind, namespace, name string, listObj resourceList) error {
	key := model.ResourceListOptions{
		Kind: kind,
		Name: name,
		Namespace: namespace,
	}

	// Query the backend.
	kvps, err := c.client.Backend.List(key, opts.ResourceVersion)
	if err != nil {
		return err
	}

	// Convert the slice of KVPairs to a slice of Objects.
	resources := []runtime.Object{}
	for _, kvp := range kvps.KVPairs {
		resources = append(resources, c.kvPairToResource(kvp))
	}
	err = meta.SetList(listObj, resources)
	if err != nil {
		return err
	}

	// Finally, set the resource version of the list object.
	listObj.GetListMeta().SetResourceVersion(kvps.Revision)

	return nil
}

// Watch watches a specific resource or resource type.
func (c *untyped) Watch(opts options.ListOptions, kind, namespace, name string) (watch.Interface, error) {
	panic("Not implemented")
	return nil, nil
}

func (c *untyped) resourceToKVPair(opts options.SetOptions, in resource) *model.KVPair {
	// Prepare the resource to remove non-persisted fields.
	in.GetObjectMeta().SetResourceVersion("")
	in.GetObjectMeta().SetSelfLink("")

	// Create a KVPair using the "generic" resource Key, and the actual object as
	// the value.
	return &model.KVPair{
		TTL: opts.TTL,
		Value: in,
		Key: model.ResourceKey{
			Kind: in.GetObjectKind().GroupVersionKind().Kind,
			Name: in.GetObjectMeta().GetName(),
			Namespace: in.GetObjectMeta().GetNamespace(),
		},
		Revision: in.GetObjectMeta().GetResourceVersion(),
	}
}

func (c *untyped) kvPairToResource(kvp *model.KVPair) resource {
	// Extract the resource from the returned value - the backend will already have
	// decoded it.
	out := kvp.Value.(resource)

	// Remove fields that should not be persisted and set the resource version.
	out.GetObjectMeta().SetResourceVersion("")
	out.GetObjectMeta().SetSelfLink("")
	out.GetObjectMeta().SetResourceVersion(kvp.Revision)

	return out
}

