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

	"github.com/projectcalico/libcalico-go/lib/apiv2"
	"github.com/projectcalico/libcalico-go/lib/options"
)

// PolicyInterface has methods to work with BgpPeer resources.
type PolicyInterface interface {
	Create(peer *apiv2.Policy, opts options.SetOptions) (*apiv2.Policy, error)
	Update(peer *apiv2.Policy, opts options.SetOptions) (*apiv2.Policy, error)
	Delete(name string, revision string) error
	Get(name string, opts options.GetOptions) (*apiv2.Policy, error)
	List(opts options.ListOptions) (*apiv2.PolicyList, error)
	Watch(opts options.ListOptions) (watch.Interface, error)
}

// policies implements PolicyInterface
type policies struct {
	client *client
}

// Create takes the representation of a Policy and creates it.  Returns the stored
// representation of the Policy, and an error, if there is any.
func (r policies) Create(peer *apiv2.Policy, opts options.SetOptions) (*apiv2.Policy, error) {
	panic("Create not implemented for PolicyInterface")
	return nil, nil
}

// Update takes the representation of a Policy and updates it. Returns the stored
// representation of the Policy, and an error, if there is any.
func (r policies) Update(peer *apiv2.Policy, opts options.SetOptions) (*apiv2.Policy, error) {
	panic("Update not implemented for PolicyInterface")
	return nil, nil
}

// Delete takes name of the Policy and deletes it. Returns an error if one occurs.
func (r policies) Delete(name string, revision string) error {
	panic("Delete not implemented for PolicyInterface")
	return nil
}

// Get takes name of the Policy, and returns the corresponding Policy object,
// and an error if there is any.
func (r policies) Get(name string, opts options.GetOptions) (*apiv2.Policy, error) {
	panic("Get not implemented for PolicyInterface")
	return nil, nil
}

// List returns the list of Policy objects that match the supplied options.
func (r policies) List(opts options.ListOptions) (*apiv2.PolicyList, error) {
	panic("List not implemented for PolicyInterface")
	return nil, nil
}

// Watch returns a watch.Interface that watches the Policys that match the
// supplied options.
func (r policies) Watch(opts options.ListOptions) (watch.Interface, error) {
	panic("Watch not implemented for PolicyInterface")
	return nil, nil
}
