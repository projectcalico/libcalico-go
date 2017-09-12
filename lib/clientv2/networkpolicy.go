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

// NetworkPolicyInterface has methods to work with NetworkPolicy resources.
type NetworkPolicyInterface interface {
	Create(peer *apiv2.NetworkPolicy, opts options.SetOptions) (*apiv2.NetworkPolicy, error)
	Update(peer *apiv2.NetworkPolicy, opts options.SetOptions) (*apiv2.NetworkPolicy, error)
	Delete(name string, revision string) error
	Get(name string, opts options.GetOptions) (*apiv2.NetworkPolicy, error)
	List(opts options.ListOptions) (*apiv2.NetworkPolicyList, error)
	Watch(opts options.ListOptions) (watch.Interface, error)
}

// networkpolicies implements NetworkPolicyInterface
type networkpolicies struct {
	client client
}

// Create takes the representation of a NetworkPolicy and creates it.  Returns the stored
// representation of the NetworkPolicy, and an error, if there is any.
func (r networkpolicies) Create(peer *apiv2.NetworkPolicy, opts options.SetOptions) (*apiv2.NetworkPolicy, error) {
	panic("Create not implemented for NetworkPolicyInterface")
	return nil, nil
}

// Update takes the representation of a NetworkPolicy and updates it. Returns the stored
// representation of the NetworkPolicy, and an error, if there is any.
func (r networkpolicies) Update(peer *apiv2.NetworkPolicy, opts options.SetOptions) (*apiv2.NetworkPolicy, error) {
	panic("Update not implemented for NetworkPolicyInterface")
	return nil, nil
}

// Delete takes name of the NetworkPolicy and deletes it. Returns an error if one occurs.
func (r networkpolicies) Delete(name string, revision string) error {
	panic("Delete not implemented for NetworkPolicyInterface")
	return nil
}

// Get takes name of the NetworkPolicy, and returns the corresponding NetworkPolicy object,
// and an error if there is any.
func (r networkpolicies) Get(name string, opts options.GetOptions) (*apiv2.NetworkPolicy, error) {
	panic("Get not implemented for NetworkPolicyInterface")
	return nil, nil
}

// List returns the list of NetworkPolicy objects that match the supplied options.
func (r networkpolicies) List(opts options.ListOptions) (*apiv2.NetworkPolicyList, error) {
	panic("List not implemented for NetworkPolicyInterface")
	return nil, nil
}

// Watch returns a watch.Interface that watches the NetworkPolicys that match the
// supplied options.
func (r networkpolicies) Watch(opts options.ListOptions) (watch.Interface, error) {
	panic("Watch not implemented for NetworkPolicyInterface")
	return nil, nil
}
