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

// BGPPeerInterface has methods to work with BGPPeer resources.
type BGPPeerInterface interface {
	Create(peer *apiv2.BGPPeer, opts options.SetOptions) (*apiv2.BGPPeer, error)
	Update(peer *apiv2.BGPPeer, opts options.SetOptions) (*apiv2.BGPPeer, error)
	Delete(name string, revision string) error
	Get(name string, opts options.GetOptions) (*apiv2.BGPPeer, error)
	List(opts options.ListOptions) (*apiv2.BGPPeerList, error)
	Watch(opts options.ListOptions) (watch.Interface, error)
}

// bgpPeers implements BGPPeerInterface
type bgpPeers struct {
	client client
}

// Create takes the representation of a BGPPeer and creates it.  Returns the stored
// representation of the BGPPeer, and an error, if there is any.
func (r bgpPeers) Create(peer *apiv2.BGPPeer, opts options.SetOptions) (*apiv2.BGPPeer, error) {
	panic("Create not implemented for BGPPeerInterface")
	return nil, nil
}

// Update takes the representation of a BGPPeer and updates it. Returns the stored
// representation of the BGPPeer, and an error, if there is any.
func (r bgpPeers) Update(peer *apiv2.BGPPeer, opts options.SetOptions) (*apiv2.BGPPeer, error) {
	panic("Update not implemented for BGPPeerInterface")
	return nil, nil
}

// Delete takes name of the BGPPeer and deletes it. Returns an error if one occurs.
func (r bgpPeers) Delete(name string, revision string) error {
	panic("Delete not implemented for BGPPeerInterface")
	return nil
}

// Get takes name of the BGPPeer, and returns the corresponding BGPPeer object,
// and an error if there is any.
func (r bgpPeers) Get(name string, opts options.GetOptions) (*apiv2.BGPPeer, error) {
	panic("Get not implemented for BGPPeerInterface")
	return nil, nil
}

// List returns the list of BGPPeer objects that match the supplied options.
func (r bgpPeers) List(opts options.ListOptions) (*apiv2.BGPPeerList, error) {
	panic("List not implemented for BGPPeerInterface")
	return nil, nil
}

// Watch returns a watch.Interface that watches the BGPPeers that match the
// supplied options.
func (r bgpPeers) Watch(opts options.ListOptions) (watch.Interface, error) {
	panic("Watch not implemented for BGPPeerInterface")
	return nil, nil
}
