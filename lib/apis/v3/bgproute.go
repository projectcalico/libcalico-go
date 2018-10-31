// Copyright (c) 2018 Tigera, Inc. All rights reserved.

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

package v3

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

const (
	KindBGPRoute     = "BGPRoute"
	KindBGPRouteList = "BGPRouteList"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BGPRoute describes an additional route to advertise from a Calico node.
type BGPRoute struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the BGPRoute.
	Spec BGPRouteSpec `json:"spec,omitempty"`
}

// BGPRouteSpec contains the specification for a BGPRoute resource.
type BGPRouteSpec struct {
	// The node name that should advertise the route.
	Node string `json:"node,omitempty" validate:"omitempty,name"`
	// The route to advertise.
	CIDR string `json:"cidr" validate:"net"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BGPRouteList contains a list of BGPRoute resources.
type BGPRouteList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []BGPRoute `json:"items"`
}

// NewBGPRoute creates a new (zeroed) BGPRoute struct with the TypeMetadata initialised to the current
// version.
func NewBGPRoute() *BGPRoute {
	return &BGPRoute{
		TypeMeta: metav1.TypeMeta{
			Kind:       KindBGPRoute,
			APIVersion: GroupVersionCurrent,
		},
	}
}

// NewBGPRouteList creates a new (zeroed) BGPRouteList struct with the TypeMetadata initialised to the current
// version.
func NewBGPRouteList() *BGPRouteList {
	return &BGPRouteList{
		TypeMeta: metav1.TypeMeta{
			Kind:       KindBGPRouteList,
			APIVersion: GroupVersionCurrent,
		},
	}
}
