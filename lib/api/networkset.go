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

package api

import (
	"github.com/projectcalico/libcalico-go/lib/api/unversioned"
	"github.com/projectcalico/libcalico-go/lib/net"
)

// NetworkSet contains a set of arbitrary IP sub-networks/CIDRs that share labels to
// allow rules to refer to them via selectors.  The labels of NetworkSets are in the same
// "namespace" as those for endpoints.  NetworkSets are useful to refer to groups
// of "external" non-Calico workloads/hosts.
type NetworkSet struct {
	unversioned.TypeMetadata
	Metadata NetworkSetMetadata `json:"metadata,omitempty"`
	Spec     NetworkSetSpec     `json:"spec,omitempty"`
}

// NetworkSetMetadata contains the Metadata for a NetworkSet resource.
type NetworkSetMetadata struct {
	unversioned.ObjectMetadata

	// The name of the endpoint.
	Name string `json:"name,omitempty" validate:"omitempty,namespacedname"`

	// The labels applied to the group.  It is expected that many endpoints and CIDR groups
	// share the same labels. For example, they could be used to label all “production”
	// workloads with “deployment=prod” so that security policy can be applied to production
	// workloads.
	Labels map[string]string `json:"labels,omitempty" validate:"omitempty,labels"`
}

// NetworkSetSpec contains the specification for a NetworkSet resource.
type NetworkSetSpec struct {
	// The list of IP networks that belong to this set.
	Nets []net.IPNet `json:"nets,omitempty" validate:"omitempty"`
}

// NewNetworkSet creates a new (zeroed) NetworkSet struct with the TypeMetadata initialised to the current
// version.
func NewNetworkSet() *NetworkSet {
	return &NetworkSet{
		TypeMetadata: unversioned.TypeMetadata{
			Kind:       "networkSet",
			APIVersion: unversioned.VersionCurrent,
		},
	}
}

// NetworkSetList contains a list of NetworkSet resources.  List types are returned from List()
// enumerations in the client interface.
type NetworkSetList struct {
	unversioned.TypeMetadata
	Metadata unversioned.ListMetadata `json:"metadata,omitempty"`
	Items    []NetworkSet             `json:"items" validate:"dive"`
}

// NewNetworkSet creates a new (zeroed) NetworkSetList struct with the TypeMetadata initialised to the current
// version.
func NewNetworkSetList() *NetworkSetList {
	return &NetworkSetList{
		TypeMetadata: unversioned.TypeMetadata{
			Kind:       "NetworkSetList",
			APIVersion: unversioned.VersionCurrent,
		},
	}
}
