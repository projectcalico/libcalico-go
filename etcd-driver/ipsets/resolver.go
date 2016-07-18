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

package ipsets

import (
	"github.com/golang/glog"
	"github.com/tigera/libcalico-go/datastructures/labels"
	"github.com/tigera/libcalico-go/datastructures/tags"
	"github.com/tigera/libcalico-go/etcd-driver/store"
	"github.com/tigera/libcalico-go/lib/backend"
	"github.com/tigera/libcalico-go/lib/hash"
	"github.com/tigera/libcalico-go/lib/selector"
)

// Resolver processes datastore updates to calculate the current set of active ipsets.
// It generates events for ipsets being added/removed and IPs being added/removed from them.
type Resolver struct {
	// The tag index keeps track of the dynamic mapping from endpoints to
	// tags.
	tagIndex tags.Index
	// ActiveRulesCalculator matches policies/profiles against local
	// endpoints and notifies the ActiveSelectorCalculator when
	// their rules become active/inactive.
	activeRulesCalculator *ActiveRulesCalculator
	// SelectorScanner scans the active policies/profiles for active
	// selectors...
	selScanner *SelectorScanner
	// ...which we pass to the label inheritance index to calculate the
	// endpoints that match...
	labelIdx labels.LabelInheritanceIndex
	// ...which we pass to the ipset calculator to merge the IPs from
	// different endpoints.
	ipsetCalc *IpsetCalculator

	hostname string

	OnIPSetAdded   func(selID string)
	OnIPAdded      func(selID, ip string)
	OnIPRemoved    func(selID, ip string)
	OnIPSetRemoved func(selID string)
}

func NewResolver(felixSender FelixSender, hostname string) *Resolver {
	resolver := &Resolver{
		OnIPSetAdded:   func(selID string) {},
		OnIPAdded:      func(selID, ip string) {},
		OnIPRemoved:    func(selID, ip string) {},
		OnIPSetRemoved: func(selID string) {},
		hostname:       hostname,
	}

	resolver.tagIndex = tags.NewIndex(resolver.onTagMatchStarted, resolver.onTagMatchStopped)

	resolver.selScanner = NewSelectorScanner()
	resolver.selScanner.OnSelectorActive = resolver.onSelectorActive
	resolver.selScanner.OnSelectorInactive = resolver.onSelectorInactive

	resolver.ipsetCalc = NewIpsetCalculator()
	resolver.ipsetCalc.OnIPAdded = resolver.onIPAdded
	resolver.ipsetCalc.OnIPRemoved = resolver.onIPRemoved

	resolver.labelIdx = labels.NewInheritanceIndex(
		resolver.onSelMatchStarted, resolver.onSelMatchStopped)

	resolver.activeRulesCalculator = NewActiveRulesCalculator(resolver.selScanner, felixSender)

	return resolver
}

type dispatcher interface {
	Register(keyExample backend.KeyInterface, receiver store.ParsedUpdateHandler)
}

// RegisterWith registers the update callbacks that this object requires with the dispatcher.
func (res *Resolver) RegisterWith(disp dispatcher) {
	disp.Register(backend.WorkloadEndpointKey{}, res.onEndpointUpdate)
	disp.Register(backend.PolicyKey{}, res.onPolicyUpdate)
	disp.Register(backend.ProfileTagsKey{}, res.onProfileTagsUpdate)
	disp.Register(backend.ProfileLabelsKey{}, res.onProfileLabelsUpdate)
	disp.Register(backend.ProfileRulesKey{}, res.onProfileRulesUpdate)
}

// Datastore callbacks:

func (res *Resolver) onEndpointUpdate(update *store.ParsedUpdate) {
	if update.Value != nil {
		glog.V(3).Infof("Endpoint %v updated", update.Key)
		glog.V(4).Infof("Endpoint data: %#v", update.Value)
		ep := update.Value.(*backend.WorkloadEndpoint)
		res.ipsetCalc.UpdateEndpointIPs(update.Key, ep.IPv4Nets)
		res.labelIdx.UpdateLabels(update.Key, ep.Labels, ep.ProfileIDs)
		res.tagIndex.UpdateEndpoint(update.Key, ep.ProfileIDs)
	} else {
		glog.V(3).Infof("Endpoint %v deleted", update.Key)
		res.ipsetCalc.DeleteEndpoint(update.Key)
		res.labelIdx.DeleteLabels(update.Key)
		res.tagIndex.DeleteEndpoint(update.Key)
	}
	key := update.Key.(backend.WorkloadEndpointKey)
	if key.Hostname != res.hostname {
		update.SkipSendToFelix = true
	}
}

// onPolicyUpdate is called when we get a policy update from the datastore.
// It passes through to the ActiveSetCalculator, which extracts the active ipsets from its rules.
func (res *Resolver) onPolicyUpdate(update *store.ParsedUpdate) {
	if update.Value != nil {
		glog.V(3).Infof("Policy %v updated", update.Key)
		glog.V(4).Infof("Policy data: %#v", update.Value)
		policy := update.Value.(*backend.Policy)
		res.activeRulesCalculator.UpdatePolicy(update.Key.(backend.PolicyKey), policy)
	} else {
		glog.V(3).Infof("Policy %v deleted", update.Key)
		res.activeRulesCalculator.DeletePolicy(update.Key.(backend.PolicyKey))
	}
	update.SkipSendToFelix = true
}

func (res *Resolver) onProfileRulesUpdate(update *store.ParsedUpdate) {
	if update.Value != nil {
		glog.V(3).Infof("Profile rules %v updated", update.Key)
		glog.V(4).Infof("Rules data: %#v", update.Value)
		profile := update.Value.(*backend.ProfileRules)
		res.activeRulesCalculator.UpdateProfileRules(update.Key.(backend.ProfileRulesKey), profile)
		update.ValueUpdated = true
	} else {
		glog.V(3).Infof("Profile rules %v deleted", update.Key)
		res.activeRulesCalculator.DeleteProfileRules(update.Key.(backend.ProfileRulesKey))
	}
	update.SkipSendToFelix = true
}

// onProfileLabelsUpdate updates the index when the labels attached to a profile change.
func (res *Resolver) onProfileLabelsUpdate(update *store.ParsedUpdate) {
	// The profile IDs in the endpoint are simple strings; convert the
	// key to string so that it matches.
	profileKey := update.Key.(backend.ProfileLabelsKey)
	profileID := profileKey.Name
	if update.Value != nil {
		// Update.
		glog.V(3).Infof("Profile labels %v updated", update.Key)
		glog.V(4).Infof("Profile labels data: %#v", update.Value)
		labels := update.Value.(map[string]string)
		res.labelIdx.UpdateParentLabels(profileID, labels)
	} else {
		// Deletion.
		glog.V(3).Infof("Profile labels %v deleted", update.Key)
		res.labelIdx.DeleteParentLabels(profileID)
	}
	update.SkipSendToFelix = true
}

func (res *Resolver) onProfileTagsUpdate(update *store.ParsedUpdate) {
	glog.V(3).Infof("Profile tags %v updated", update)
	// The profile IDs in the endpoint are simple strings; convert the
	// key to string so that it matches.
	profileKey := update.Key.(backend.ProfileTagsKey)
	profileID := profileKey.Name
	if update.Value != nil {
		// Update.
		glog.V(3).Infof("Profile tags %v updated", update.Key)
		glog.V(4).Infof("Profile tags data: %#v", update.Value)
		tags := update.Value.([]string)
		res.tagIndex.UpdateProfileTags(profileID, tags)
	} else {
		// Deletion.
		glog.V(3).Infof("Profile tags %v deleted", update.Key)
		res.tagIndex.DeleteProfileTags(profileID)
	}
	update.SkipSendToFelix = true
}

//// OnProfileUpdate is called when we get a profile update from the datastore.
//// It passes through to the ActiveSetCalculator, which extracts the active ipsets from its rules.
//func (res *Resolver) OnProfileUpdate(key backend.ProfileKey, policy *backend.Profile) {
//	glog.Infof("Profile %v updated", key)
//	res.activeSelCalc.UpdateProfile(key, policy)
//}

// IpsetCalculator callbacks:

// onIPAdded is called when an IP is now present in an active selector.
func (res *Resolver) onIPAdded(selID, ip string) {
	glog.V(3).Infof("IP set %v now contains %v", selID, ip)
	res.OnIPAdded(selID, ip)
}

// onIPRemoved is called when an IP is no longer present in a selector.
func (res *Resolver) onIPRemoved(selID, ip string) {
	glog.V(3).Infof("IP set %v no longer contains %v", selID, ip)
	res.OnIPRemoved(selID, ip)
}

// LabelIndex callbacks:

// onMatchStarted is called when an endpoint starts matching an active selector.
func (res *Resolver) onSelMatchStarted(selId, labelId interface{}) {
	glog.V(3).Infof("Endpoint %v now matches selector %v", labelId, selId)
	res.ipsetCalc.MatchStarted(labelId.(backend.KeyInterface), selId.(string))
}

// onMatchStopped is called when an endpoint stops matching an active selector.
func (res *Resolver) onSelMatchStopped(selId, labelId interface{}) {
	glog.V(3).Infof("Endpoint %v no longer matches selector %v", labelId, selId)
	res.ipsetCalc.MatchStopped(labelId.(backend.KeyInterface), selId.(string))
}

func (res *Resolver) onTagMatchStarted(key tags.EndpointKey, tagID string) {
	glog.V(3).Infof("Endpoint %v now matches tag %v", key, tagID)
	res.ipsetCalc.MatchStarted(key, hash.MakeUniqueID("t", tagID))
}

func (res *Resolver) onTagMatchStopped(key tags.EndpointKey, tagID string) {
	glog.V(3).Infof("Endpoint %v no longer matches selector %v", key, tagID)
	res.ipsetCalc.MatchStopped(key, hash.MakeUniqueID("t", tagID))
}

// ActiveSelectorCalculator callbacks:

// onSelectorActive is called when a selector starts being used in a rule.
// It adds the selector to the label index and starts tracking it.
func (res *Resolver) onSelectorActive(sel selector.Selector) {
	glog.Infof("Selector %v now active", sel)
	res.OnIPSetAdded(sel.UniqueId())
	res.labelIdx.UpdateSelector(sel.UniqueId(), sel)
}

// onSelectorActive is called when a selector stops being used in a rule.
// It removes the selector to the label index and stops tracking it.
func (res *Resolver) onSelectorInactive(sel selector.Selector) {
	glog.Infof("Selector %v now inactive", sel)
	res.labelIdx.DeleteSelector(sel.UniqueId())
	res.OnIPSetRemoved(sel.UniqueId())
}
