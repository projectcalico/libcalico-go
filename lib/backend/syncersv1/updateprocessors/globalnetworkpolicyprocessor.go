// Copyright (c) 2017-2018 Tigera, Inc. All rights reserved.

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

package updateprocessors

import (
	"errors"
	"fmt"

	apiv3 "github.com/projectcalico/libcalico-go/lib/apis/v3"
	"github.com/projectcalico/libcalico-go/lib/backend/model"
	"github.com/projectcalico/libcalico-go/lib/backend/watchersyncer"
)

// Create a new SyncerUpdateProcessor to sync GlobalNetworkPolicy data in v1 format for
// consumption by Felix.
func NewGlobalNetworkPolicyUpdateProcessor() watchersyncer.SyncerUpdateProcessor {
	return NewSimpleUpdateProcessor(apiv3.KindGlobalNetworkPolicy, convertGlobalNetworkPolicyV2ToV1Key, convertGlobalNetworkPolicyV2ToV1Value)
}

func convertGlobalNetworkPolicyV2ToV1Key(v3key model.ResourceKey) (model.Key, error) {
	if v3key.Name == "" {
		return model.PolicyKey{}, errors.New("Missing Name field to create a v1 NetworkPolicy Key")
	}
	return model.PolicyKey{
		Name: v3key.Name,
	}, nil

}

func convertGlobalNetworkPolicyV2ToV1Value(val interface{}) (interface{}, error) {
	v3res, ok := val.(*apiv3.GlobalNetworkPolicy)
	if !ok {
		return nil, errors.New("Value is not a valid GlobalNetworkPolicy resource value")
	}

	spec := v3res.Spec
	selector := spec.Selector

	// If this policy has a Namespace and/or a NamespaceSelector add any of those values as a logical AND
	// to any pre-existing selectors.
	appendSelectorClause := func(selector, field, label string) string {
		if field != "" {
			s := fmt.Sprintf("%s == '%s'", label, field)
			if selector != "" {
				selector = fmt.Sprintf("(%s) && %s", selector, s)
			} else {
				selector = s
			}
		}
		return selector
	}

	// WIP: this needs to add the right prefix to these fields instead of using 'apiv3.LabelNamespace'
	selector = appendSelectorClause(selector, spec.NamespaceSelector, apiv3.LabelNamespace)

	v1value := &model.Policy{
		Namespace:      "", // Empty string used to signal a GlobalNetworkPolicy.
		Order:          spec.Order,
		InboundRules:   RulesAPIV2ToBackend(spec.Ingress, ""),
		OutboundRules:  RulesAPIV2ToBackend(spec.Egress, ""),
		Selector:       selector,
		Types:          policyTypesAPIV2ToBackend(spec.Types),
		DoNotTrack:     spec.DoNotTrack,
		PreDNAT:        spec.PreDNAT,
		ApplyOnForward: spec.ApplyOnForward,
	}

	return v1value, nil
}
