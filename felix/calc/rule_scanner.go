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
package calc

import (
	"github.com/golang/glog"
	"github.com/tigera/libcalico-go/datastructures/multidict"
	"github.com/tigera/libcalico-go/datastructures/set"
	"github.com/tigera/libcalico-go/felix/proto"
	"github.com/tigera/libcalico-go/lib/backend/model"
	"github.com/tigera/libcalico-go/lib/hash"
	"github.com/tigera/libcalico-go/lib/selector"
)

// RuleScanner calculates the active set of selectors and tags from the current set of policies/profiles.
// It generates events for selectors becoming active/inactive.
type RuleScanner struct {
	// selectorsByUid maps from a selector's UID to the selector itself.
	tagsOrSelsByUID map[string]tagOrSel
	// activeUidsByResource maps from policy or profile ID to "set" of selector UIDs
	rulesIDToUIDs multidict.IfaceToString
	// activeResourcesByUid maps from selector UID back to the "set" of resources using it.
	uidsToRulesIDs multidict.StringToIface

	OnSelectorActive   func(selector selector.Selector)
	OnSelectorInactive func(selector selector.Selector)
	OnTagActive        func(tag string)
	OnTagInactive      func(tag string)

	RulesUpdateCallbacks rulesUpdateCallbacks
}

func NewRuleScanner() *RuleScanner {
	calc := &RuleScanner{
		tagsOrSelsByUID: make(map[string]tagOrSel),
		rulesIDToUIDs:   multidict.NewIfaceToString(),
		uidsToRulesIDs:  multidict.NewStringToIface(),
	}
	return calc
}

func (calc *RuleScanner) OnProfileActive(key model.ProfileRulesKey, profile *model.ProfileRules) {
	parsedRules := calc.updateRules(key, profile.InboundRules, profile.OutboundRules)
	calc.RulesUpdateCallbacks.OnProfileActive(key, parsedRules)
}

func (calc *RuleScanner) OnProfileInactive(key model.ProfileRulesKey) {
	calc.updateRules(key, nil, nil)
	calc.RulesUpdateCallbacks.OnProfileInactive(key)
}

func (calc *RuleScanner) OnPolicyActive(key model.PolicyKey, policy *model.Policy) {
	parsedRules := calc.updateRules(key, policy.InboundRules, policy.OutboundRules)
	calc.RulesUpdateCallbacks.OnPolicyActive(key, parsedRules)
}

func (calc *RuleScanner) OnPolicyInactive(key model.PolicyKey) {
	calc.updateRules(key, nil, nil)
	calc.RulesUpdateCallbacks.OnPolicyInactive(key)
}

func (calc *RuleScanner) updateRules(key interface{}, inbound, outbound []model.Rule) (parsedRules *proto.Rules) {
	glog.V(4).Infof("Scanning rules (%v in, %v out) for key %v",
		len(inbound), len(outbound), key)
	// Extract all the new selectors/tags.
	currentUIDToTagOrSel := make(map[string]tagOrSel)
	parsedInbound := make([]*proto.Rule, len(inbound))
	for ii, rule := range inbound {
		parsed, allToS, err := ruleToParsedRule(&rule)
		if err != nil {
			glog.Warningf("Bad selector in %v: %v", key, err)
			panic("Bad selector")
		}
		parsedInbound[ii] = parsed
		for _, tos := range allToS {
			currentUIDToTagOrSel[tos.uid] = tos
		}
	}
	parsedOutbound := make([]*proto.Rule, len(outbound))
	for ii, rule := range outbound {
		parsed, allToS, err := ruleToParsedRule(&rule)
		if err != nil {
			glog.Warningf("Bad selector in %v: %v", key, err)
			panic("Bad selector")
		}
		parsedOutbound[ii] = parsed
		for _, tos := range allToS {
			currentUIDToTagOrSel[tos.uid] = tos
		}
	}
	parsedRules = &proto.Rules{parsedInbound, parsedOutbound}

	// Figure out which selectors/tags are new.
	addedUids := set.New()
	for uid, _ := range currentUIDToTagOrSel {
		glog.V(4).Infof("Checking if UID %v is new.", uid)
		if !calc.rulesIDToUIDs.Contains(key, uid) {
			glog.V(4).Infof("UID %v is new", uid)
			addedUids.Add(uid)
		}
	}

	// Figure out which selectors/tags are no-longer in use.
	removedUids := set.New()
	calc.rulesIDToUIDs.Iter(key, func(uid string) {
		if _, ok := currentUIDToTagOrSel[uid]; !ok {
			glog.V(4).Infof("Removed UID: %v", uid)
			removedUids.Add(uid)
		}
	})

	// Add the new into the index, triggering events as we discover
	// newly-active tags/selectors.
	addedUids.Iter(func(item interface{}) error {
		uid := item.(string)
		calc.rulesIDToUIDs.Put(key, uid)
		if !calc.uidsToRulesIDs.ContainsKey(uid) {
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
		calc.uidsToRulesIDs.Put(uid, key)
		return nil
	})

	// And remove the old, triggering events as we clean up unused
	// selectors/tags.
	addedUids.Iter(func(item interface{}) error {
		uid := item.(string)
		calc.rulesIDToUIDs.Discard(key, uid)
		if !calc.uidsToRulesIDs.ContainsKey(uid) {
			glog.V(3).Infof("Selector/tag became inactive: %v", uid)
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
		return nil
	})
	return
}

func ruleToParsedRule(rule *model.Rule) (parsedRule *proto.Rule, allTagOrSels []tagOrSel, err error) {
	src, dst, notSrc, notDst, err := extractTagsAndSelectors(rule)
	if err != nil {
		return
	}

	parsedRule = &proto.Rule{
		Action: rule.Action,

		Protocol: rule.Protocol,

		SrcNet:      rule.SrcNet,
		SrcPorts:    rule.SrcPorts,
		DstNet:      rule.DstNet,
		DstPorts:    rule.DstPorts,
		ICMPType:    rule.ICMPType,
		ICMPCode:    rule.ICMPCode,
		SrcIPSetIDs: tosSlice(src).ToUIDs(),
		DstIPSetIDs: tosSlice(dst).ToUIDs(),

		NotProtocol:    rule.NotProtocol,
		NotSrcNet:      rule.NotSrcNet,
		NotSrcPorts:    rule.NotSrcPorts,
		NotDstNet:      rule.NotDstNet,
		NotDstPorts:    rule.NotDstPorts,
		NotICMPType:    rule.NotICMPType,
		NotICMPCode:    rule.NotICMPCode,
		NotSrcIPSetIDs: tosSlice(notSrc).ToUIDs(),
		NotDstIPSetIDs: tosSlice(notDst).ToUIDs(),
	}

	allTagOrSels = make([]tagOrSel, 0, len(src)+len(dst)+len(notSrc)+len(notDst))
	allTagOrSels = append(allTagOrSels, src...)
	allTagOrSels = append(allTagOrSels, dst...)
	allTagOrSels = append(allTagOrSels, notSrc...)
	allTagOrSels = append(allTagOrSels, notDst...)

	return
}

func extractTagsAndSelectors(rule *model.Rule) (src, dst, notSrc, notDst []tagOrSel, err error) {
	if rule.SrcTag != "" {
		src = append(src, tagOrSelFromTag(rule.SrcTag))
	}
	if rule.DstTag != "" {
		dst = append(src, tagOrSelFromTag(rule.DstTag))
	}
	if rule.NotSrcTag != "" {
		notSrc = append(src, tagOrSelFromTag(rule.NotSrcTag))
	}
	if rule.NotDstTag != "" {
		notDst = append(src, tagOrSelFromTag(rule.NotDstTag))
	}
	var tos tagOrSel
	if rule.SrcSelector != "" {
		tos, err = tagOrSelFromSel(rule.SrcSelector)
		if err != nil {
			return
		}
		src = append(src, tos)
	}
	if rule.DstSelector != "" {
		tos, err = tagOrSelFromSel(rule.DstSelector)
		if err != nil {
			return
		}
		dst = append(dst, tos)
	}
	if rule.NotSrcSelector != "" {
		tos, err = tagOrSelFromSel(rule.NotSrcSelector)
		if err != nil {
			return
		}
		notSrc = append(notSrc, tos)
	}
	if rule.NotDstSelector != "" {
		tos, err = tagOrSelFromSel(rule.NotDstSelector)
		if err != nil {
			return
		}
		notDst = append(notDst, tos)
	}
	return
}

type tagOrSel struct {
	tag      string
	selector selector.Selector
	uid      string
}

func tagOrSelFromTag(tag string) tagOrSel {
	return tagOrSel{tag: tag, uid: hash.MakeUniqueID("t", tag)}
}

func tagOrSelFromSel(sel string) (tos tagOrSel, err error) {
	selector, err := selector.Parse(sel)
	if err == nil {
		tos = tagOrSel{selector: selector, uid: selector.UniqueId()}
	}
	return
}

type tosSlice []tagOrSel

func (t tosSlice) ToUIDs() []string {
	uids := make([]string, len(t))
	for ii, tos := range t {
		uids[ii] = tos.uid
	}
	return uids
}
