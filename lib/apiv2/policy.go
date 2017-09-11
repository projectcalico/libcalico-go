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
	KindNetworkPolicy           = "NetworkPolicy"
	KindNetworkPolicyList       = "NetworkPolicyList"
	KindGlobalNetworkPolicy     = "GlobalNetworkPolicy"
	KindGlobalNetworkPolicyList = "GlobalNetworkPolicyList"
)

// GlobalNetworkPolicy contains information about a security Policy resource.  This contains a set of
// security rules to apply.  Security policies allow a selector-based security model which can override
// the security profiles directly referenced by an endpoint.
//
// Each policy must do one of the following:
//
//  	- Match the packet and apply an “allow” action; this immediately accepts the packet, skipping
//        all further policies and profiles. This is not recommended in general, because it prevents
//        further policy from being executed.
// 	- Match the packet and apply a “deny” action; this drops the packet immediately, skipping all
//        further policy and profiles.
// 	- Fail to match the packet; in which case the packet proceeds to the next policy. If there
// 	  are no more policies then the packet is dropped.
//
// Calico implements the security policy for each endpoint individually and only the policies that
// have matching selectors are implemented. This ensures that the number of rules that actually need
// to be inserted into the kernel is proportional to the number of local endpoints rather than the
// total amount of policy.
//
// GlobalNetworkPolicy is globally-scoped (i.e. not Namespaced).
type GlobalNetworkPolicy struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	Metadata metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the Policy.
	Spec PolicySpec `json:"spec,omitempty"`
}

// NetworkPolicy is the Namespaced-equivalent of the GlobalNetworkPolicy.
type NetworkPolicy struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	Metadata metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the Policy.
	Spec PolicySpec `json:"spec,omitempty"`
}

// PolicySpec contains the specification for a selector-based security Policy resource.
type PolicySpec struct {
	// Order is an optional field that specifies the order in which the policy is applied.
	// Policies with higher "order" are applied after those with lower
	// order.  If the order is omitted, it may be considered to be "infinite" - i.e. the
	// policy will be applied last.  Policies with identical order will be applied in
	// alphanumerical order based on the Policy "Name".
	Order *float64 `json:"order,omitempty"`
	// The ordered set of ingress rules.  Each rule contains a set of packet match criteria and
	// a corresponding action to apply.
	IngressRules []Rule `json:"ingress,omitempty" validate:"omitempty,dive"`
	// The ordered set of egress rules.  Each rule contains a set of packet match criteria and
	// a corresponding action to apply.
	EgressRules []Rule `json:"egress,omitempty" validate:"omitempty,dive"`
	// The selector is an expression used to pick pick out the endpoints that the policy should
	// be applied to.
	//
	// Selector expressions follow this syntax:
	//
	// 	label == "string_literal"  ->  comparison, e.g. my_label == "foo bar"
	// 	label != "string_literal"   ->  not equal; also matches if label is not present
	// 	label in { "a", "b", "c", ... }  ->  true if the value of label X is one of "a", "b", "c"
	// 	label not in { "a", "b", "c", ... }  ->  true if the value of label X is not one of "a", "b", "c"
	// 	has(label_name)  -> True if that label is present
	// 	! expr -> negation of expr
	// 	expr && expr  -> Short-circuit and
	// 	expr || expr  -> Short-circuit or
	// 	( expr ) -> parens for grouping
	// 	all() or the empty selector -> matches all endpoints.
	//
	// Label names are allowed to contain alphanumerics, -, _ and /. String literals are more permissive
	// but they do not support escape characters.
	//
	// Examples (with made-up labels):
	//
	// 	type == "webserver" && deployment == "prod"
	// 	type in {"frontend", "backend"}
	// 	deployment != "dev"
	// 	! has(label_name)
	Selector string `json:"selector" validate:"selector"`
	// DoNotTrack indicates whether packets matched by the rules in this policy should go through
	// the data plane's connection tracking, such as Linux conntrack.  If True, the rules in
	// this policy are applied before any data plane connection tracking, and packets allowed by
	// this policy are marked as not to be tracked.
	DoNotTrack bool `json:"doNotTrack,omitempty"`
	// PreDNAT indicates to apply the rules in this policy before any DNAT.
	PreDNAT bool `json:"preDNAT,omitempty"`
	// Types indicates whether this policy applies to ingress, or to egress, or to both.  When
	// not explicitly specified (and so the value on creation is empty or nil), Calico defaults
	// Types according to what IngressRules and EgressRules are present in the policy.  The
	// default is:
	//
	// - [ PolicyTypeIngress ], if there are no EgressRules (including the case where there are
	//   also no IngressRules)
	//
	// - [ PolicyTypeEgress ], if there are EgressRules but no IngressRules
	//
	// - [ PolicyTypeIngress, PolicyTypeEgress ], if there are both IngressRules and EgressRules.
	//
	// When the policy is read back again, Types will always be one of these values, never empty
	// or nil.
	Types []PolicyType `json:"types,omitempty" validate:"omitempty,dive,policytype"`
}

// PolicyType enumerates the possible values of the PolicySpec Types field.
type PolicyType string

const (
	PolicyTypeIngress PolicyType = "Ingress"
	PolicyTypeEgress  PolicyType = "Egress"
)

// GlobalNetworkPolicyList contains a list of GlobalNetworkPolicy resources.
type GlobalNetworkPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	Metadata        metav1.ListMeta       `json:"metadata"`
	Items           []GlobalNetworkPolicy `json:"items"`
}

// NewGlobalNetworkPolicy creates a new (zeroed) GlobalNetworkPolicy struct with the TypeMetadata initialised to the current
// version.
func NewGlobalNetworkPolicy() *GlobalNetworkPolicy {
	return &GlobalNetworkPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       KindGlobalNetworkPolicy,
			APIVersion: GroupVersionCurrent,
		},
	}
}

// NewGlobalNetworkPolicyList creates a new (zeroed) GlobalNetworkPolicyList struct with the TypeMetadata initialised to the current
// version.
func NewGlobalNetworkPolicyList() *GlobalNetworkPolicyList {
	return &GlobalNetworkPolicyList{
		TypeMeta: metav1.TypeMeta{
			Kind:       KindGlobalNetworkPolicyList,
			APIVersion: GroupVersionCurrent,
		},
	}
}

// NetworkPolicyList contains a list of NetworkPolicy resources.
type NetworkPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	Metadata        metav1.ListMeta `json:"metadata"`
	Items           []NetworkPolicy `json:"items"`
}

// NewNetworkPolicy creates a new (zeroed) NetworkPolicy struct with the TypeMetadata initialised to the current
// version.
func NewNetworkPolicy() *NetworkPolicy {
	return &NetworkPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       KindNetworkPolicy,
			APIVersion: GroupVersionCurrent,
		},
	}
}

// NewNetworkPolicyList creates a new (zeroed) NetworkPolicyList struct with the TypeMetadata initialised to the current
// version.
func NewNetworkPolicyList() *NetworkPolicyList {
	return &NetworkPolicyList{
		TypeMeta: metav1.TypeMeta{
			Kind:       KindNetworkPolicyList,
			APIVersion: GroupVersionCurrent,
		},
	}
}

// GetObjectKind returns the kind of this object.  Required to satisfy Object interface
func (e *GlobalNetworkPolicy) GetObjectKind() schema.ObjectKind {
	return &e.TypeMeta
}

// GetObjectMeta returns the object metadata of this object. Required to satisfy ObjectMetaAccessor interface
func (e *GlobalNetworkPolicy) GetObjectMeta() metav1.Object {
	return &e.Metadata
}

// GetObjectKind returns the kind of this object. Required to satisfy Object interface
func (el *GlobalNetworkPolicyList) GetObjectKind() schema.ObjectKind {
	return &el.TypeMeta
}

// GetListMeta returns the list metadata of this object. Required to satisfy ListMetaAccessor interface
func (el *GlobalNetworkPolicyList) GetListMeta() metav1.List {
	return &el.Metadata
}

// GetObjectKind returns the kind of this object.  Required to satisfy Object interface
func (e *NetworkPolicy) GetObjectKind() schema.ObjectKind {
	return &e.TypeMeta
}

// GetObjectMeta returns the object metadata of this object. Required to satisfy ObjectMetaAccessor interface
func (e *NetworkPolicy) GetObjectMeta() metav1.Object {
	return &e.Metadata
}

// GetObjectKind returns the kind of this object. Required to satisfy Object interface
func (el *NetworkPolicyList) GetObjectKind() schema.ObjectKind {
	return &el.TypeMeta
}

// GetListMeta returns the list metadata of this object. Required to satisfy ListMetaAccessor interface
func (el *NetworkPolicyList) GetListMeta() metav1.List {
	return &el.Metadata
}
