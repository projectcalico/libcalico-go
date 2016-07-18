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
	"encoding/json"
	"github.com/golang/glog"
	"github.com/tigera/libcalico-go/datastructures/labels"
	"github.com/tigera/libcalico-go/datastructures/tags"
	"github.com/tigera/libcalico-go/etcd-driver/store"
	"github.com/tigera/libcalico-go/lib/backend"
	"github.com/tigera/libcalico-go/lib/selector"
)

type ruleListener interface {
	UpdateRules(key interface{}, inbound, outbound []backend.Rule)
}

type FelixSender interface {
	SendUpdateToFelix(update store.Update)
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
	listener    ruleListener
	felixSender FelixSender
}

func NewActiveRulesCalculator(ruleListener ruleListener, felixSender FelixSender) *ActiveRulesCalculator {
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
		listener:    ruleListener,
		felixSender: felixSender,
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
		glog.V(4).Info("Profile rules updated while active, telling listener/felix")
		arc.sendProfileUpdate(key.Name)
	}
}

func (arc *ActiveRulesCalculator) DeleteProfileRules(key backend.ProfileRulesKey) {
	delete(arc.allProfileRules, key.Name)
	if _, ok := arc.profileIDToEndpointKeys[key.Name]; ok {
		glog.V(4).Info("Profile rules deleted while active, telling listener/felix")
		arc.sendProfileUpdate(key.Name)
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
		// If we get here, the selector still matches something,
		// update the rules.
		glog.V(4).Info("Policy updated while active, telling listener")
		arc.sendPolicyUpdate(key)
	}
}

func (arc *ActiveRulesCalculator) DeletePolicy(key backend.PolicyKey) {
	delete(arc.allPolicies, key)
	arc.labelIndex.DeleteSelector(key)
	// No need to call updatePolicy() because we'll have got a matchStopped
	// callback.
}

func (arc *ActiveRulesCalculator) updateEndpoint(key endpointKey, profileIDs []string) {
	// Figure out which profiles have been added/removed.
	removedIDs, addedIDs := arc.endpointKeyToProfileIDs.Update(key, profileIDs)

	// Update the index of required profile IDs for added profiles,
	// triggering events for profiles that just became active.
	for id, _ := range addedIDs {
		keys, ok := arc.profileIDToEndpointKeys[id]
		if !ok {
			// This profile is now active.
			keys = make(map[endpointKey]bool)
			arc.profileIDToEndpointKeys[id] = keys
			arc.sendProfileUpdate(id)
		}
		keys[key] = true
	}

	// Update the index for no-longer required profile IDs, triggering
	// events for profiles that just became inactive.
	for id, _ := range removedIDs {
		keys := arc.profileIDToEndpointKeys[id]
		delete(keys, key)
		if len(keys) == 0 {
			// No endpoint refers to this ID any more.  Clean it
			// up.
			delete(arc.profileIDToEndpointKeys, id)
			arc.sendProfileUpdate(id)
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
		arc.sendPolicyUpdate(polKey)
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
		arc.sendPolicyUpdate(polKey)
	}
}

func (arc *ActiveRulesCalculator) sendProfileUpdate(profileID string) {
	rules, known := arc.allProfileRules[profileID]
	_, active := arc.profileIDToEndpointKeys[profileID]
	profileKey := backend.ProfileKey{Name: profileID}
	asEtcdKey, err := backend.KeyToFelixKey(profileKey)
	if err != nil {
		glog.Fatalf("Failed to marshal key %#v", profileKey)
	}
	update := store.Update{Key: asEtcdKey}
	if known && active {
		jsonBytes, err := json.Marshal(rules)
		if err != nil {
			glog.Fatalf("Failed to marshal rules as json: %#v",
				rules)
		}
		jsonStr := string(jsonBytes)
		update.ValueOrNil = &jsonStr
		arc.listener.UpdateRules(profileID,
			rules.InboundRules,
			rules.OutboundRules)
	} else {
		arc.listener.UpdateRules(profileID,
			[]backend.Rule{},
			[]backend.Rule{})
	}
	arc.felixSender.SendUpdateToFelix(update)
}

func (arc *ActiveRulesCalculator) sendPolicyUpdate(policyKey backend.PolicyKey) {
	policy, known := arc.allPolicies[policyKey]
	_, active := arc.policyIDToEndpointKeys[policyKey]
	asEtcdKey, err := backend.KeyToFelixKey(policyKey)
	if err != nil {
		glog.Fatalf("Failed to marshal key %#v", policyKey)
	}
	update := store.Update{Key: asEtcdKey}
	if known && active {
		var policyCopy backend.Policy
		jsonCopy, err := json.Marshal(policy)
		if err != nil {
			glog.Fatal("Failed to marshal policy")
		}
		err = json.Unmarshal(jsonCopy, &policyCopy)
		if err != nil {
			glog.Fatal("Failed to unmarshal policy")
		}
		// FIXME UpdateRules modifies the rules!
		arc.listener.UpdateRules(policyKey, policyCopy.InboundRules, policyCopy.OutboundRules)
		jsonBytes, err := json.Marshal(policyCopy)
		if err != nil {
			glog.Fatalf("Failed to marshal policy as json: %#v",
				policyKey)
		}
		jsonStr := string(jsonBytes)
		update.ValueOrNil = &jsonStr
	} else {
		arc.listener.UpdateRules(policyKey, []backend.Rule{}, []backend.Rule{})
	}
	arc.felixSender.SendUpdateToFelix(update)
}
