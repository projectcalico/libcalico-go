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
	"github.com/tigera/libcalico-go/lib/backend"
	"github.com/tigera/libcalico-go/lib/hash"
	"github.com/tigera/libcalico-go/lib/selector"
)

// ActiveSelectorCalculator calculates the active set of selectors from the current set of policies/profiles.
// It generates events for selectors becoming active/inactive.
type ActiveSelectorCalculator struct {
	// selectorsByUid maps from a selector's UID to the selector itself.
	selectorsByUid selByUid
	// activeUidsByResource maps from policy or profile ID to "set" of selector UIDs
	activeUidsByResource map[interface{}]map[string]bool
	// activeResourcesByUid maps from selector UID back to the "set" of resources using it.
	activeResourcesByUid map[string]map[interface{}]bool

	OnSelectorActive   func(selector selector.Selector)
	OnSelectorInactive func(selector selector.Selector)
}

func NewActiveSelectorCalculator() *ActiveSelectorCalculator {
	calc := &ActiveSelectorCalculator{
		selectorsByUid:       make(selByUid),
		activeUidsByResource: make(map[interface{}]map[string]bool),
		activeResourcesByUid: make(map[string]map[interface{}]bool),
	}
	return calc
}

func (calc *ActiveSelectorCalculator) UpdateRules(key interface{}, inbound, outbound []backend.Rule) {
	// Extract all the new selectors.
	currentSelsByUid := make(selByUid)
	currentSelsByUid.addSelectorsFromRules(inbound)
	currentSelsByUid.addSelectorsFromRules(outbound)

	// Find the set of old selectors.
	knownUids, knownUidsPresent := calc.activeUidsByResource[key]
	glog.V(4).Infof("Known UIDs for %v: %v", key, knownUids)

	// Figure out which selectors are new.
	addedUids := make(map[string]bool)
	for uid, _ := range currentSelsByUid {
		if !knownUids[uid] {
			glog.V(4).Infof("Added UID: %v", uid)
			addedUids[uid] = true
		}
	}

	// Figure out which selectors are no-longer in use.
	removedUids := make(map[string]bool)
	for uid, _ := range knownUids {
		if _, ok := currentSelsByUid[uid]; !ok {
			glog.V(4).Infof("Removed UID: %v", uid)
			removedUids[uid] = true
		}
	}

	// Add the new into the index, triggering events as we discover
	// newly-active selectors.
	if len(addedUids) > 0 {
		if !knownUidsPresent {
			knownUids = make(map[string]bool)
			calc.activeUidsByResource[key] = knownUids
		}
		for uid, _ := range addedUids {
			knownUids[uid] = true
			resources, ok := calc.activeResourcesByUid[uid]
			if !ok {
				glog.V(3).Infof("Selector became active: %v", uid)
				resources = make(map[interface{}]bool)
				calc.activeResourcesByUid[uid] = resources
				sel := currentSelsByUid[uid]
				calc.selectorsByUid[uid] = sel
				// This selector just became active, trigger event.
				calc.OnSelectorActive(sel)
			}
			resources[key] = true
		}
	}

	// And remove the old, triggering events as we clean up unused
	// selectors.
	for uid, _ := range removedUids {
		delete(knownUids, uid)
		resources := calc.activeResourcesByUid[uid]
		delete(resources, key)
		if len(resources) == 0 {
			glog.V(3).Infof("Selector became inactive: %v", uid)
			delete(calc.activeResourcesByUid, uid)
			sel := calc.selectorsByUid[uid]
			delete(calc.selectorsByUid, uid)
			// This selector just became inactive, trigger event.
			calc.OnSelectorInactive(sel)
		}
	}
}

// selByUid is an augmented map with methods to assist in extracting rules from policies.
type selByUid map[string]selector.Selector

func (sbu selByUid) addSelectorsFromRules(rules []backend.Rule) {
	for i, rule := range rules {
		selStrPs := []*string{&rule.SrcSelector,
			&rule.DstSelector,
			&rule.NotSrcSelector,
			&rule.NotDstSelector}
		for _, selStrP := range selStrPs {
			if *selStrP != "" {
				sel, err := selector.Parse(*selStrP)
				if err != nil {
					panic("FIXME: Handle bad selector")
				}
				uid := sel.UniqueId()
				sbu[uid] = sel
				// FIXME: Remove this horrible hack where we update the policy rule
				*selStrP = uid
			}
		}

		tagStrPs := []*string{
			&rule.SrcTag,
			&rule.DstTag,
			&rule.NotSrcTag,
			&rule.NotDstTag,
		}
		for _, tagStrP := range tagStrPs {
			if *tagStrP != "" {
				tag := *tagStrP
				*tagStrP = hash.MakeUniqueID("t", tag)
			}
		}
		rules[i] = rule
	}
}
