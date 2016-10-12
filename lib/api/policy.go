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
)

// Policy contains information about a tiered security Policy resource.  This contains a set of
// security rules to apply.  Security policies allow a tiered security model which can override the
// security profiles directly referenced by an endpoint.
//
// Each policy must do one of the following:
//
//  	- Match the packet and apply a “next-tier” action; this skips the rest of the tier, deferring
//        to the next tier (or the explicit profiles if this is the last tier.
//  	- Match the packet and apply an “allow” action; this immediately accepts the packet, skipping
//        all further tiers and profiles. This is not recommended in general, because it prevents
//        further policy from being executed.
// 	- Match the packet and apply a “deny” action; this drops the packet immediately, skipping all
//        further tiers and profiles.
// 	- Fail to match the packet; in which case the packet proceeds to the next policy in the tier.
//        If there are no more policies in the tier then the packet is dropped.
//
// Note:
// 	If no policies in a tier match an endpoint then the packet skips the tier completely. The
// 	“default deny” behavior described above only applies if some of the policies in a tier match
// 	the endpoint.
//
// Calico implements the security policy for each endpoint individually and only the policies that
// have matching selectors are implemented. This ensures that the number of rules that actually need
// to be inserted into the kernel is proportional to the number of local endpoints rather than the
// total amount of policy. If no policies in a tier match a given endpoint then that tier is skipped.
type Policy struct {
	unversioned.TypeMetadata
	Metadata PolicyMetadata `json:"metadata,omitempty"`
	Spec     PolicySpec     `json:"spec,omitempty"`
}

// PolicyMetadata contains the metadata for a tiered security Policy resource.
type PolicyMetadata struct {
	unversioned.ObjectMetadata

	// The name of the tiered security policy.
	Name string `json:"name,omitempty" validate:"omitempty,name"`

	// The name of the tier that this policy belongs to.  If this is omitted, the default
	// tier (name is "default") is assumed.  The specified tier must exist in order to create
	// security policies within the tier, the "default" tier is created automatically if it
	// does not exist, this means for deployments requiring only a single Tier, the tier name
	// may be omitted on all policy management requests.
	Tier string `json:"tier,omitempty" validate:"omitempty,name"`
}

// PolicySpec contains the specification for a tiered security Policy resource.
type PolicySpec struct {
	// Order is an optional field that specifies the order in which the policy is applied
	// within a given tier.  Policies with higher "order" are applied after those with lower
	// order.  If the order is omitted, it may be considered to be "infinite" - i.e. the
	// policy will be applied last.  Policies with identical order and within the same Tier
	// will be applied in alphanumerical order based on the Policy "Name".
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
}

// NewPolicy creates a new (zeroed) Policy struct with the TypeMetadata initialised to the current
// version.
func NewPolicy() *Policy {
	return &Policy{
		TypeMetadata: unversioned.TypeMetadata{
			Kind:       "policy",
			APIVersion: unversioned.VersionCurrent,
		},
	}
}

// PolicyList contains a list of tier security Policy resources.  List types are returned from List()
// enumerations on the client interface.
type PolicyList struct {
	unversioned.TypeMetadata
	Metadata unversioned.ListMetadata `json:"metadata,omitempty"`
	Items    []Policy                 `json:"items" validate:"dive"`
}

// NewPolicyList creates a new (zeroed) PolicyList struct with the TypeMetadata initialised to the current
// version.
func NewPolicyList() *PolicyList {
	return &PolicyList{
		TypeMetadata: unversioned.TypeMetadata{
			Kind:       "policyList",
			APIVersion: unversioned.VersionCurrent,
		},
	}
}
