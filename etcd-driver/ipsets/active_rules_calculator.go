// Copyright (c) 2016 Tigera, Inc. All rights reserved.
//
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

package ipsets

import (
	"github.com/golang/glog"
	"github.com/tigera/libcalico-go/datastructures/labels"
	"github.com/tigera/libcalico-go/datastructures/tags"
	"github.com/tigera/libcalico-go/lib/backend"
	"github.com/tigera/libcalico-go/lib/selector"
)

type ruleListener interface {
	UpdateRules(key interface{}, inbound, outbound []backend.Rule)
}

type ActiveRulesCalculator struct {
	// Caches of all known policies/profiles.
	allPolicies     map[backend.PolicyKey]backend.Policy
	allProfileRules map[string]backend.ProfileRules

	// Policy/profile ID to matching endpoint sets.
	policyIDToEndpointKeys  map[backend.PolicyKey]map[endpointKey]bool
	profileIDToEndpointKeys map[string]map[endpointKey]bool

	// Label index, matching policy selectors against local endpoints.
	labelIndex labels.LabelInheritanceIndex

	// Cache of profile IDs by local endpoint.
	endpointKeyToProfileIDs *tags.EndpointKeyToProfileIDMap

	// Callback object.
	listener ruleListener
}

func NewActiveRulesCalculator(ruleListener ruleListener) *ActiveRulesCalculator {
	arc := &ActiveRulesCalculator{
		// Caches of all known policies/profiles.
		allPolicies:     make(map[backend.PolicyKey]backend.Policy),
		allProfileRules: make(map[string]backend.ProfileRules),

		// Policy/profile ID to matching endpoint sets.
		policyIDToEndpointKeys:  make(map[backend.PolicyKey]map[endpointKey]bool),
		profileIDToEndpointKeys: make(map[string]map[endpointKey]bool),

		// Cache of profile IDs by local endpoint.
		endpointKeyToProfileIDs: tags.NewEndpointKeyToProfileIDMap(),

		// Callback object.
		listener: ruleListener,
	}
	arc.labelIndex = labels.NewInheritanceIndex(arc.onMatchStarted, arc.onMatchStopped)
	return arc
}

func (arc *ActiveRulesCalculator) UpdateWorkloadEndpoint(key backend.WorkloadEndpointKey, endpoint *backend.WorkloadEndpoint) {
	// Figure out what's changed and update the cache.
	profileIDs := endpoint.ProfileIDs
	arc.updateEndpoint(key, profileIDs)
	arc.labelIndex.UpdateLabels(key, endpoint.Labels, profileIDs)
}

func (arc *ActiveRulesCalculator) DeleteWorkloadEndpoint(key backend.WorkloadEndpointKey) {
	arc.updateEndpoint(key, []string{})
	arc.labelIndex.DeleteLabels(key)
}

func (arc *ActiveRulesCalculator) UpdateHostEndpoint(key backend.HostEndpointKey, endpoint *backend.HostEndpoint) {
	// Figure out what's changed and update the cache.
	profileIDs := endpoint.ProfileIDs
	arc.updateEndpoint(key, profileIDs)
	arc.labelIndex.UpdateLabels(key, endpoint.Labels, profileIDs)
}

func (arc *ActiveRulesCalculator) DeleteHostEndpoint(key backend.WorkloadEndpointKey) {
	arc.updateEndpoint(key, []string{})
	arc.labelIndex.DeleteLabels(key)
}

func (arc *ActiveRulesCalculator) UpdateProfileLabels(key backend.ProfileLabelsKey, labels map[string]string) {
	arc.labelIndex.UpdateParentLabels(key.Name, labels)
}

func (arc *ActiveRulesCalculator) DeleteProfileLabels(key backend.ProfileLabelsKey) {
	arc.labelIndex.DeleteParentLabels(key.Name)
}

func (arc *ActiveRulesCalculator) UpdateProfileRules(key backend.ProfileRulesKey, rules *backend.ProfileRules) {
	arc.allProfileRules[key.Name] = *rules
	if _, ok := arc.profileIDToEndpointKeys[key.Name]; ok {
		glog.V(4).Info("Profile rules updated while active, telling listener")
		arc.listener.UpdateRules(key.Name, rules.InboundRules, rules.OutboundRules)
	}
}

func (arc *ActiveRulesCalculator) DeleteProfileRules(key backend.ProfileRulesKey) {
	delete(arc.allProfileRules, key.Name)
	if _, ok := arc.profileIDToEndpointKeys[key.Name]; ok {
		glog.V(4).Info("Profile rules deleted while active, telling listener")
		arc.listener.UpdateRules(key.Name, []backend.Rule{}, []backend.Rule{})
	}
}

func (arc *ActiveRulesCalculator) UpdatePolicy(key backend.PolicyKey, policy *backend.Policy) {
	arc.allPolicies[key] = *policy
	// Update the index, which will call us back if the selector no
	// longer matches.
	sel, err := selector.Parse(policy.Selector)
	if err != nil {
		glog.Fatal(err)
	}
	arc.labelIndex.UpdateSelector(key, sel)
	if _, ok := arc.policyIDToEndpointKeys[key]; ok {
		// Still matches, update the rules.
		glog.V(4).Info("Policy updated while active, telling listener")
		arc.listener.UpdateRules(key.Name, policy.InboundRules, policy.OutboundRules)
	}
}

func (arc *ActiveRulesCalculator) DeletePolicy(key backend.PolicyKey) {
	delete(arc.allPolicies, key)
	arc.labelIndex.DeleteSelector(key)
	// No need to check policyIDToEndpointKeys since DeleteSelector will
	// have called us back to remove the policy.
}

func (arc *ActiveRulesCalculator) updateEndpoint(key endpointKey, profileIDs []string) {
	removedIDs, addedIDs := arc.endpointKeyToProfileIDs.Update(key, profileIDs)

	for id, _ := range addedIDs {
		keys, ok := arc.profileIDToEndpointKeys[id]
		if !ok {
			keys = make(map[endpointKey]bool)
			arc.profileIDToEndpointKeys[id] = keys
			rules, ok := arc.allProfileRules[id]
			if ok {
				arc.listener.UpdateRules(id,
					rules.InboundRules,
					rules.OutboundRules)
			}
		}
		keys[key] = true
	}

	for id, _ := range removedIDs {
		keys := arc.profileIDToEndpointKeys[id]
		delete(keys, key)
		if len(keys) == 0 {
			delete(arc.profileIDToEndpointKeys, id)
			arc.listener.UpdateRules(id,
				[]backend.Rule{},
				[]backend.Rule{})
		}
	}
}

func (arc *ActiveRulesCalculator) onMatchStarted(selId, labelId interface{}) {
	polKey := selId.(backend.PolicyKey)
	keys, ok := arc.policyIDToEndpointKeys[polKey]
	if !ok {
		keys = make(map[endpointKey]bool)
		arc.policyIDToEndpointKeys[polKey] = keys
		// Policy wasn't active before, tell the listener.  The policy
		// must be in allPolicies because we can only match on a policy
		// that we've seen.
		policy := arc.allPolicies[polKey]
		arc.listener.UpdateRules(polKey, policy.InboundRules, policy.OutboundRules)
	}
	keys[labelId] = true
}

func (arc *ActiveRulesCalculator) onMatchStopped(selId, labelId interface{}) {
	polKey := selId.(backend.PolicyKey)
	keys := arc.policyIDToEndpointKeys[polKey]
	delete(keys, labelId)
	if len(keys) == 0 {
		delete(arc.policyIDToEndpointKeys, polKey)
		// Policy no longer active.
		arc.listener.UpdateRules(polKey, []backend.Rule{}, []backend.Rule{})
	}
}
