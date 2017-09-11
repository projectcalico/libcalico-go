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

package apiv2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	KindProfile = "Profile"
	KindProfileList = "ProfileList"
)

// Profile contains the details a security profile resource.  A profile is set of security rules
// to apply on an endpoint.  An endpoint (either a host endpoint or an endpoint on a workload) can
// reference zero or more profiles.  The profile rules are applied directly to the endpoint *after*
// the selector-based security policy has been applied, and in the order the profiles are declared on the
// endpoint.
type Profile struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	Metadata metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the Profile.
	Spec ProfileSpec `json:"spec,omitempty"`
}

// ProfileSpec contains the specification for a security Profile resource.
type ProfileSpec struct {
	// The ordered set of ingress rules.  Each rule contains a set of packet match criteria and
	// a corresponding action to apply.
	IngressRules []Rule `json:"ingress,omitempty" validate:"omitempty,dive"`
	// The ordered set of egress rules.  Each rule contains a set of packet match criteria and
	// a corresponding action to apply.
	EgressRules []Rule `json:"egress,omitempty" validate:"omitempty,dive"`
}

// ProfileList contains a list of Profile resources.
type ProfileList struct {
	metav1.TypeMeta `json:",inline"`
	Metadata        metav1.ListMeta `json:"metadata"`
	Items           []Profile       `json:"items"`
}

// NewProfile creates a new (zeroed) Profile struct with the TypeMetadata initialised to the current
// version.
func NewProfile() *Profile {
	return &Profile{
		TypeMeta: metav1.TypeMeta{
			Kind:       KindProfile,
			APIVersion: VersionCurrent,
		},
	}
}

// NewProfileList creates a new (zeroed) ProfileList struct with the TypeMetadata initialised to the current
// version.
func NewProfileList() *ProfileList {
	return &ProfileList{
		TypeMeta: metav1.TypeMeta{
			Kind:       KindProfileList,
			APIVersion: VersionCurrent,
		},
	}
}

// GetObjectKind returns the kind of this object.  Required to satisfy Object interface
func (e *Profile) GetObjectKind() schema.ObjectKind {
	return &e.TypeMeta
}

// GetObjectMeta returns the object metadata of this object. Required to satisfy ObjectMetaAccessor interface
func (e *Profile) GetObjectMeta() metav1.Object {
	return &e.Metadata
}

// GetObjectKind returns the kind of this object. Required to satisfy Object interface
func (el *ProfileList) GetObjectKind() schema.ObjectKind {
	return &el.TypeMeta
}

// GetListMeta returns the list metadata of this object. Required to satisfy ListMetaAccessor interface
func (el *ProfileList) GetListMeta() metav1.List {
	return &el.Metadata
}