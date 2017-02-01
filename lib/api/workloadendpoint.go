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
	"fmt"

	"github.com/projectcalico/libcalico-go/lib/api/unversioned"
	"github.com/projectcalico/libcalico-go/lib/net"
)

type WorkloadEndpoint struct {
	unversioned.TypeMetadata
	Metadata WorkloadEndpointMetadata `json:"metadata,omitempty"`
	Spec     WorkloadEndpointSpec     `json:"spec,omitempty"`
}

// WorkloadEndpointMetadata contains the Metadata for a WorkloadEndpoint resource.
type WorkloadEndpointMetadata struct {
	unversioned.ObjectMetadata

	// The name of the endpoint.  This may be omitted on a create, in which case an endpoint
	// ID will be automatically created, and the endpoint ID will be included in the response.
	Name string `json:"name,omitempty" validate:"omitempty,name"`

	// The name of the workload.
	Workload string `json:"workload,omitempty" valid:"omitempty,name"`

	// The name of the orchestrator.
	Orchestrator string `json:"orchestrator,omitempty" valid:"omitempty,name"`

	// The node name identifying the Calico node instance.
	Node string `json:"node,omitempty" valid:"omitempty,name"`

	// The labels applied to the workload endpoint.  It is expected that many endpoints share
	// the same labels. For example, they could be used to label all “production” workloads
	// with “deployment=prod” so that security policy can be applied to production workloads.
	Labels map[string]string `json:"labels,omitempty" validate:"omitempty,labels"`
}

// WorkloadEndpointMetadata contains the specification for a WorkloadEndpoint resource.
type WorkloadEndpointSpec struct {
	// IPNetworks is a list of subnets allocated to this endpoint. IP packets will only be
	// allowed to leave this interface if they come from an address in one of these subnets.
	//
	// Currently only /32 for IPv4 and /128 for IPv6 networks are supported.
	IPNetworks []net.IPNet `json:"ipNetworks,omitempty" validate:"omitempty"`

	// IPNATs is a list of 1:1 NAT mappings to apply to the endpoint. Inbound connections
	// to the external IP will be forwarded to the internal IP. Connections initiated from the
	// internal IP will not have their source address changed, except when an endpoint attempts
	// to connect one of its own external IPs. Each internal IP must be associated with the same
	// endpoint via the configured IPNetworks.
	IPNATs []IPNAT `json:"ipNATs,omitempty" validate:"omitempty,dive"`

	// IPv4Gateway is the gateway IPv4 address for traffic from the workload.
	IPv4Gateway *net.IP `json:"ipv4Gateway,omitempty" validate:"omitempty"`

	// IPv6Gateway is the gateway IPv6 address for traffic from the workload.
	IPv6Gateway *net.IP `json:"ipv6Gateway,omitempty" validate:"omitempty"`

	// A list of security Profile resources that apply to this endpoint. Each profile is
	// applied in the order that they appear in this list.  Profile rules are applied
	// after the selector-based security policy.
	Profiles []string `json:"profiles,omitempty" validate:"omitempty,dive,name"`

	// InterfaceName the name of the Linux interface on the host: for example, tap80.
	InterfaceName string `json:"interfaceName,omitempty" validate:"interface"`

	// MAC is the MAC address of the endpoint interface.
	MAC *net.MAC `json:"mac,omitempty" validate:"omitempty,mac"`
}

// IPNat contains a single NAT mapping for a WorkloadEndpoint resource.
type IPNAT struct {
	// The internal IP address which must be associated with the owning endpoint via the
	// configured IPNetworks for the endpoint.
	InternalIP net.IP `json:"internalIP"`

	// The external IP address.
	ExternalIP net.IP `json:"externalIP"`
}

// String returns a friendly form of an IPNAT.
func (i IPNAT) String() string {
	return fmt.Sprintf("%s<>%s", i.InternalIP, i.ExternalIP)
}

// NewWorkloadEndpoint creates a new (zeroed) WorkloadEndpoint struct with the TypeMetadata
// initialised to the current version.
func NewWorkloadEndpoint() *WorkloadEndpoint {
	return &WorkloadEndpoint{
		TypeMetadata: unversioned.TypeMetadata{
			Kind:       "workloadEndpoint",
			APIVersion: unversioned.VersionCurrent,
		},
	}
}

// WorkloadEndpointList contains a list of Host Endpoint resources.  List types are returned
// from List() enumerations in the client interface.
type WorkloadEndpointList struct {
	unversioned.TypeMetadata
	Metadata unversioned.ListMetadata `json:"metadata,omitempty"`
	Items    []WorkloadEndpoint       `json:"items" validate:"dive"`
}

// NewWorkloadEndpointList creates a new (zeroed) NodeList struct with the TypeMetadata
// initialised to the current version.
func NewWorkloadEndpointList() *WorkloadEndpointList {
	return &WorkloadEndpointList{
		TypeMetadata: unversioned.TypeMetadata{
			Kind:       "workloadEndpointList",
			APIVersion: unversioned.VersionCurrent,
		},
	}
}
