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
	"github.com/projectcalico/libcalico-go/lib/api/unversioned"
	"github.com/projectcalico/libcalico-go/lib/net"
	"github.com/projectcalico/libcalico-go/lib/numorstring"
)

// Node contains the details of a node resource.  A node represents a Calico compute
// host and provides the configuration for a Calico node instance running on host.
// In addition to creating a Node resource through calicoctl or the Calico API,
// the Calico node instance must also be running on the specific host and should be
// provided the same Name as that configured on the Node resource.  Note that, by
// default, the Calico node instance uses the hostname of the compute host when it
// is not explicitly specified - in this case, the equivalent Node resource should
// be created using the same hostname as the Name of the Node resource.
//
// Operations on the Node resources is expected to be required when adding a new
// host into a Calico network, and when removing a host from a Calico network, and
// occasionally to modify certain configuration.  Care should be taken when operating
// on Node resources: deleting a Node resource will remove all Node specific data.
type Node struct {
	unversioned.TypeMetadata
	Metadata NodeMetadata `json:"metadata,omitempty"`
	Spec     NodeSpec     `json:"spec,omitempty"`
}

// NodeMetadata contains the metadata for a Calico Node resource.
type NodeMetadata struct {
	unversioned.ObjectMetadata

	// The name of the node.
	Name string `json:"name,omitempty" validate:"omitempty,name"`

	// The labels applied to the Calico node instance.  Theses labels are
	// not yet used by Calico.
	Labels map[string]string `json:"labels,omitempty" validate:"omitempty,labels"`
}

// NodeSpec contains the specification for a Calico Node resource.
type NodeSpec struct {
	// BGP configuration for this node.  If this omitted, the Calico node
	// will be run in policy-only mode.
	BGP *NodeBGPSpec `json:"bgp,omitempty" validate:"omitempty"`
}

// NodeSpec contains the specification for a Calico Node resource.
type NodeBGPSpec struct {
	// The AS Number of the node.  If this is not specified, the global
	// default value will be used.
	ASNumber *numorstring.ASNumber `json:"asNumber,omitempty"`

	// IPv4Address is the IPv4 address of this node.  At least one of the
	// IPv4 and IPv6 addresses should be specified.
	IPv4Address *net.IP `json:"ipv4Address,omitempty" validate:"omitempty,ipv4"`

	// IPv6Address is the IPv6 address of this node.  At least one of the
	// IPv4 and IPv6 addresses should be specified.
	IPv6Address *net.IP `json:"ipv6Address,omitempty" validate:"omitempty,ipv6"`
}

// NewNode creates a new (zeroed) Node struct with the TypeMetadata initialised to the current
// version.
func NewNode() *Node {
	return &Node{
		TypeMetadata: unversioned.TypeMetadata{
			Kind:       "node",
			APIVersion: unversioned.VersionCurrent,
		},
	}
}

// A NodeList contains a list of security Node resources.  List types are returned from List()
// enumerations on the client interface.
type NodeList struct {
	unversioned.TypeMetadata
	Metadata unversioned.ListMetadata `json:"metadata,omitempty"`
	Items    []Node                   `json:"items" validate:"dive,omitempty"`
}

// NewNode creates a new (zeroed) Node struct with the TypeMetadata initialised to the current
// version.
func NewNodeList() *NodeList {
	return &NodeList{
		TypeMetadata: unversioned.TypeMetadata{
			Kind:       "nodeList",
			APIVersion: unversioned.VersionCurrent,
		},
	}
}
