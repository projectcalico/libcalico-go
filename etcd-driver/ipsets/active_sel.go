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

type tagOrSel struct {
	tag      string
	selector selector.Selector
}

// RuleScanner calculates the active set of selectors and tags from the current set of policies/profiles.
// It generates events for selectors becoming active/inactive.
type RuleScanner struct {
	// selectorsByUid maps from a selector's UID to the selector itself.
	tagsOrSelsByUID map[string]tagOrSel
	// activeUidsByResource maps from policy or profile ID to "set" of selector UIDs
	rulesIDToUIDs map[interface{}]map[string]bool
	// activeResourcesByUid maps from selector UID back to the "set" of resources using it.
	uidsToRulesIDs map[string]map[interface{}]bool

	OnSelectorActive   func(selector selector.Selector)
	OnSelectorInactive func(selector selector.Selector)
	OnTagActive        func(tag string)
	OnTagInactive      func(tag string)
}

func NewSelectorScanner() *RuleScanner {
	calc := &RuleScanner{
		tagsOrSelsByUID: make(map[string]tagOrSel),
		rulesIDToUIDs:   make(map[interface{}]map[string]bool),
		uidsToRulesIDs:  make(map[string]map[interface{}]bool),
	}
	return calc
}

func (calc *RuleScanner) UpdateRules(key interface{}, inbound, outbound []backend.Rule) {
	// Extract all the new selectors/tags.
	currentUIDToTagOrSel := make(uidToSelector)
	currentUIDToTagOrSel.addSelectorsFromRules(inbound)
	currentUIDToTagOrSel.addSelectorsFromRules(outbound)

	// Find the set of old selectors/tags.
	knownUids, knownUidsPresent := calc.rulesIDToUIDs[key]
	glog.V(4).Infof("Known UIDs for %v: %v", key, knownUids)

	// Figure out which selectors/tags are new.
	addedUids := make(map[string]bool)
	for uid, _ := range currentUIDToTagOrSel {
		if !knownUids[uid] {
			glog.V(4).Infof("Added UID: %v", uid)
			addedUids[uid] = true
		}
	}

	// Figure out which selectors/tags are no-longer in use.
	removedUids := make(map[string]bool)
	for uid, _ := range knownUids {
		if _, ok := currentUIDToTagOrSel[uid]; !ok {
			glog.V(4).Infof("Removed UID: %v", uid)
			removedUids[uid] = true
		}
	}

	// Add the new into the index, triggering events as we discover
	// newly-active tags/selectors.
	if len(addedUids) > 0 {
		if !knownUidsPresent {
			knownUids = make(map[string]bool)
			calc.rulesIDToUIDs[key] = knownUids
		}
		for uid, _ := range addedUids {
			knownUids[uid] = true
			ruleIDs, ok := calc.uidsToRulesIDs[uid]
			if !ok {
				ruleIDs = make(map[interface{}]bool)
				calc.uidsToRulesIDs[uid] = ruleIDs

				tagOrSel := currentUIDToTagOrSel[uid]
				calc.tagsOrSelsByUID[uid] = tagOrSel
				if tagOrSel.selector != nil {
					sel := tagOrSel.selector
					glog.V(3).Infof("Selector became active: %v -> %v",
						uid, sel)
					// This selector just became active, trigger event.
					calc.OnSelectorActive(sel)
				} else {
					tag := tagOrSel.tag
					glog.V(3).Infof("Tag became active: %v -> %v",
						uid, tag)
					calc.OnTagActive(tag)
				}
			}
			ruleIDs[key] = true
		}
	}

	// And remove the old, triggering events as we clean up unused
	// selectors/tags.
	for uid, _ := range removedUids {
		delete(knownUids, uid)
		resources := calc.uidsToRulesIDs[uid]
		delete(resources, key)
		if len(resources) == 0 {
			glog.V(3).Infof("Selector/tag became inactive: %v", uid)
			delete(calc.uidsToRulesIDs, uid)
			tagOrSel := calc.tagsOrSelsByUID[uid]
			if tagOrSel.selector != nil {
				sel := tagOrSel.selector
				delete(calc.tagsOrSelsByUID, uid)
				// This selector just became inactive, trigger event.
				calc.OnSelectorInactive(sel)
			} else {
				tag := tagOrSel.tag
				glog.V(3).Infof("Tag became inactive: %v -> %v",
					uid, tag)
				calc.OnTagInactive(tag)
			}
		}
	}
}

// selByUid is an augmented map with methods to assist in extracting rules from policies.
type uidToSelector map[string]tagOrSel

func (sbu uidToSelector) addSelectorsFromRules(rules []backend.Rule) {
	for i, rule := range rules {
		selStrPs := []*string{&rule.SrcSelector,
			&rule.DstSelector,
			&rule.NotSrcSelector,
			&rule.NotDstSelector}
		for _, selStrP := range selStrPs {
			if *selStrP != "" {
				sel, err := selector.Parse(*selStrP)
				if err != nil {
					glog.Fatalf("FIXME: Handle bad selector %#v", *selStrP)
				}
				uid := sel.UniqueId()
				tos := tagOrSel{selector: sel}
				sbu[uid] = tos
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
				uid := hash.MakeUniqueID("t", tag)
				*tagStrP = uid
				tos := tagOrSel{tag: tag}
				sbu[uid] = tos
			}
		}
		rules[i] = rule
	}
}
