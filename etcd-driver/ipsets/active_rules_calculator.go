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
	"github.com/tigera/libcalico-go/datastructures/multidict"
	"github.com/tigera/libcalico-go/datastructures/tags"
	"github.com/tigera/libcalico-go/etcd-driver/store"
	"github.com/tigera/libcalico-go/lib/backend/model"
	"github.com/tigera/libcalico-go/lib/selector"
	"reflect"
)

type activeRuleListener interface {
	UpdateRules(key interface{}, inbound, outbound []model.Rule)
}

type FelixSender interface {
	SendUpdateToFelix(update store.Update)
}

type MatchListener interface {
	OnPolicyMatch(policyKey model.PolicyKey, endpointKey interface{})
}

type ActiveRulesCalculator struct {
	// Caches of all known policies/profiles.
	allPolicies     map[model.PolicyKey]model.Policy
	allProfileRules map[string]model.ProfileRules

	// Policy/profile ID to matching endpoint sets.
	policyIDToEndpointKeys  multidict.IfaceToIface
	profileIDToEndpointKeys multidict.IfaceToIface

	// Label index, matching policy selectors against local endpoints.
	labelIndex labels.LabelInheritanceIndex

	// Cache of profile IDs by local endpoint.
	endpointKeyToProfileIDs *tags.EndpointKeyToProfileIDMap

	// Callback objects.
	listener      activeRuleListener
	matchListener MatchListener
	felixSender   FelixSender
}

func NewActiveRulesCalculator(ruleListener activeRuleListener,
	felixSender FelixSender,
	matchListener MatchListener) *ActiveRulesCalculator {
	arc := &ActiveRulesCalculator{
		// Caches of all known policies/profiles.
		allPolicies:     make(map[model.PolicyKey]model.Policy),
		allProfileRules: make(map[string]model.ProfileRules),

		// Policy/profile ID to matching endpoint sets.
		policyIDToEndpointKeys:  multidict.NewIfaceToIface(),
		profileIDToEndpointKeys: multidict.NewIfaceToIface(),

		// Cache of profile IDs by local endpoint.
		endpointKeyToProfileIDs: tags.NewEndpointKeyToProfileIDMap(),

		// Callback object.
		listener:      ruleListener,
		felixSender:   felixSender,
		matchListener: matchListener,
	}
	arc.labelIndex = labels.NewInheritanceIndex(arc.onMatchStarted, arc.onMatchStopped)
	return arc
}

func (arc *ActiveRulesCalculator) OnUpdate(update *store.ParsedUpdate) {
	switch key := update.Key.(type) {
	case model.WorkloadEndpointKey:
		if update.Value != nil {
			glog.V(4).Infof("Updating ARC with endpoint %v", key)
			endpoint := update.Value.(*model.WorkloadEndpoint)
			profileIDs := endpoint.ProfileIDs
			arc.updateEndpointProfileIDs(key, profileIDs)
			arc.labelIndex.UpdateLabels(key, endpoint.Labels, profileIDs)
		} else {
			glog.V(4).Infof("Deleting endpoint %v from ARC", key)
			arc.updateEndpointProfileIDs(key, []string{})
			arc.labelIndex.DeleteLabels(key)
		}
	case model.HostEndpointKey:
		if update.Value != nil {
			// Figure out what's changed and update the cache.
			glog.V(4).Infof("Updating ARC for host endpoint %v", key)
			endpoint := update.Value.(*model.HostEndpoint)
			profileIDs := endpoint.ProfileIDs
			arc.updateEndpointProfileIDs(key, profileIDs)
			arc.labelIndex.UpdateLabels(key, endpoint.Labels, profileIDs)
		} else {
			glog.V(4).Infof("Deleting host endpoint %v from ARC", key)
			arc.updateEndpointProfileIDs(key, []string{})
			arc.labelIndex.DeleteLabels(key)
		}
	case model.ProfileLabelsKey:
		if update.Value != nil {
			glog.V(4).Infof("Updating ARC for profile %v", key)
			labels := update.Value.(map[string]string)
			arc.labelIndex.UpdateParentLabels(key.Name, labels)
		} else {
			glog.V(4).Infof("Removing profile %v from ARC", key)
			arc.labelIndex.DeleteParentLabels(key.Name)
		}
	case model.ProfileRulesKey:
		if update.Value != nil {
			rules := update.Value.(*model.ProfileRules)
			arc.allProfileRules[key.Name] = *rules
			if arc.profileIDToEndpointKeys.ContainsKey(key.Name) {
				glog.V(4).Info("Profile rules updated while active, telling listener/felix")
				arc.sendProfileUpdate(key.Name)
			}
		} else {
			delete(arc.allProfileRules, key.Name)
			if arc.profileIDToEndpointKeys.ContainsKey(key.Name) {
				glog.V(4).Info("Profile rules deleted while active, telling listener/felix")
				arc.sendProfileUpdate(key.Name)
			}
		}
	case model.PolicyKey:
		if update.Value != nil {
			glog.V(4).Infof("Updating ARC for policy %v", key)
			policy := update.Value.(*model.Policy)
			arc.allPolicies[key] = *policy
			// Update the index, which will call us back if the selector no
			// longer matches.
			sel, err := selector.Parse(policy.Selector)
			if err != nil {
				glog.Fatal(err)
			}
			arc.labelIndex.UpdateSelector(key, sel)

			if arc.policyIDToEndpointKeys.ContainsKey(key) {
				// If we get here, the selector still matches something,
				// update the rules.
				// TODO: squash duplicate update if labelIndex.UpdateSelector already made this active
				glog.V(4).Info("Policy updated while active, telling listener")
				arc.sendPolicyUpdate(key)
			}
		} else {
			glog.V(4).Infof("Removing policy %v from ARC", key)
			delete(arc.allPolicies, key)
			arc.labelIndex.DeleteSelector(key)
			// No need to call updatePolicy() because we'll have got a matchStopped
			// callback.
		}
	default:
		glog.V(0).Infof("Ignoring unexpected update: %v %#v",
			reflect.TypeOf(update.Key), update)
	}
}

func (arc *ActiveRulesCalculator) updateEndpointProfileIDs(key endpointKey, profileIDs []string) {
	// Figure out which profiles have been added/removed.
	glog.V(4).Infof("Endpoint %#v now has profile IDs: %v", key, profileIDs)
	removedIDs, addedIDs := arc.endpointKeyToProfileIDs.Update(key, profileIDs)

	// Update the index of required profile IDs for added profiles,
	// triggering events for profiles that just became active.
	for id, _ := range addedIDs {
		if !arc.profileIDToEndpointKeys.ContainsKey(id) {
			// This profile is now active.
			arc.sendProfileUpdate(id)
		}
		arc.profileIDToEndpointKeys.Put(id, key)
	}

	// Update the index for no-longer required profile IDs, triggering
	// events for profiles that just became inactive.
	for id, _ := range removedIDs {
		arc.profileIDToEndpointKeys.Discard(id, key)
		if !arc.profileIDToEndpointKeys.ContainsKey(id) {
			// No endpoint refers to this ID any more.  Clean it
			// up.
			arc.sendProfileUpdate(id)
		}
	}
}

func (arc *ActiveRulesCalculator) onMatchStarted(selId, labelId interface{}) {
	polKey := selId.(model.PolicyKey)
	policyWasActive := arc.policyIDToEndpointKeys.ContainsKey(polKey)
	arc.policyIDToEndpointKeys.Put(selId, labelId)
	if !policyWasActive {
		// Policy wasn't active before, tell the listener.  The policy
		// must be in allPolicies because we can only match on a policy
		// that we've seen.
		glog.V(3).Infof("Policy %v now matches a local endpoint", polKey)
		arc.sendPolicyUpdate(polKey)
	}
	if arc.matchListener != nil {
		arc.matchListener.OnPolicyMatch(polKey, labelId)
	}
}

func (arc *ActiveRulesCalculator) onMatchStopped(selID, labelId interface{}) {
	arc.policyIDToEndpointKeys.Discard(selID, labelId)
	if !arc.policyIDToEndpointKeys.ContainsKey(selID) {
		// Policy no longer active.
		polKey := selID.(model.PolicyKey)
		glog.V(3).Infof("Policy %v no longer matches a local endpoint", polKey)
		arc.sendPolicyUpdate(polKey)
	}
}

func (arc *ActiveRulesCalculator) sendProfileUpdate(profileID string) {
	glog.V(3).Infof("Sending profile update for profile %v", profileID)
	rules, known := arc.allProfileRules[profileID]
	active := arc.profileIDToEndpointKeys.ContainsKey(profileID)
	profileKey := model.ProfileKey{Name: profileID}
	asEtcdKey, err := profileKey.DefaultPath()
	if err != nil {
		glog.Fatalf("Failed to marshal key %#v", profileKey)
	}
	update := store.Update{Key: asEtcdKey}
	var inRules, outRules []model.Rule
	if known && active {
		jsonBytes, err := json.Marshal(rules)
		if err != nil {
			glog.Fatalf("Failed to marshal rules as json: %#v",
				rules)
		}
		jsonStr := string(jsonBytes)
		update.ValueOrNil = &jsonStr
		inRules = rules.InboundRules
		outRules = rules.OutboundRules
	}
	if arc.listener != nil {
		arc.listener.UpdateRules(profileID, inRules, outRules)
	}
	if arc.felixSender != nil {
		arc.felixSender.SendUpdateToFelix(update)
	}
}

func (arc *ActiveRulesCalculator) sendPolicyUpdate(policyKey model.PolicyKey) {
	policy, known := arc.allPolicies[policyKey]
	active := arc.policyIDToEndpointKeys.ContainsKey(policyKey)
	glog.V(3).Infof("Sending policy update for policy %v (known: %v, active: %v)",
		policyKey, known, active)
	asEtcdKey, err := policyKey.DefaultPath()
	if err != nil {
		glog.Fatalf("Failed to marshal key %#v", policyKey)
	}
	update := store.Update{Key: asEtcdKey}
	if known && active {
		var policyCopy model.Policy
		jsonCopy, err := json.Marshal(policy)
		if err != nil {
			glog.Fatal("Failed to marshal policy")
		}
		err = json.Unmarshal(jsonCopy, &policyCopy)
		if err != nil {
			glog.Fatal("Failed to unmarshal policy")
		}

		// FIXME UpdateRules modifies the rules!
		if arc.listener != nil {
			arc.listener.UpdateRules(policyKey, policyCopy.InboundRules, policyCopy.OutboundRules)
		}
		jsonBytes, err := json.Marshal(policyCopy)
		if err != nil {
			glog.Fatalf("Failed to marshal policy as json: %#v",
				policyKey)
		}
		jsonStr := string(jsonBytes)
		update.ValueOrNil = &jsonStr
	} else {
		if arc.listener != nil {
			arc.listener.UpdateRules(policyKey, []model.Rule{}, []model.Rule{})
		}
	}
	if arc.felixSender != nil {
		arc.felixSender.SendUpdateToFelix(update)
	}
}
