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
	"github.com/tigera/libcalico-go/datastructures/ip"
	"github.com/tigera/libcalico-go/datastructures/labels"
	"github.com/tigera/libcalico-go/datastructures/tags"
	"github.com/tigera/libcalico-go/felix/store"
	"github.com/tigera/libcalico-go/lib/backend/model"
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
	// RuleScanner scans the active policies/profiles for active
	// selectors...
	ruleScanner *RuleScanner
	// ...which we pass to the label inheritance index to calculate the
	// endpoints that match...
	labelIdx labels.LabelInheritanceIndex
	// ...which we pass to the ipset calculator to merge the IPs from
	// different endpoints.
	ipsetCalc *IpsetCalculator

	hostname string

	callbacks IPSetUpdateCallbacks
}

type IPSetUpdateCallbacks interface {
	OnIPSetAdded(selID string)
	OnIPAdded(selID string, ip ip.Addr)
	OnIPRemoved(selID string, ip ip.Addr)
	OnIPSetRemoved(selID string)
}

func NewResolver(felixSender FelixSender, hostname string, callbacks IPSetUpdateCallbacks) *Resolver {
	resolver := &Resolver{
		callbacks: callbacks,
		hostname:  hostname,
	}

	resolver.tagIndex = tags.NewIndex(resolver.onTagMatchStarted, resolver.onTagMatchStopped)

	resolver.ruleScanner = NewSelectorScanner()
	resolver.ruleScanner.OnSelectorActive = resolver.onSelectorActive
	resolver.ruleScanner.OnSelectorInactive = resolver.onSelectorInactive
	resolver.ruleScanner.OnTagActive = resolver.tagIndex.SetTagActive
	resolver.ruleScanner.OnTagInactive = resolver.tagIndex.SetTagInactive

	resolver.activeRulesCalculator = NewActiveRulesCalculator(
		resolver.ruleScanner, felixSender, nil)

	resolver.labelIdx = labels.NewInheritanceIndex(
		resolver.onSelMatchStarted, resolver.onSelMatchStopped)

	resolver.ipsetCalc = NewIpsetCalculator()
	resolver.ipsetCalc.OnIPAdded = resolver.onIPAdded
	resolver.ipsetCalc.OnIPRemoved = resolver.onIPRemoved

	return resolver
}

type dispatcher interface {
	Register(keyExample model.Key, receiver store.UpdateHandler)
}

// RegisterWith registers the update callbacks that this object requires with the dispatcher.
func (res *Resolver) RegisterWith(disp dispatcher) {
	disp.Register(model.WorkloadEndpointKey{}, res.onEndpointUpdate)
	disp.Register(model.HostEndpointKey{}, res.onHostEndpointUpdate)
	disp.Register(model.PolicyKey{}, res.activeRulesCalculator.OnUpdate)
	disp.Register(model.PolicyKey{}, res.skipFelix)
	disp.Register(model.ProfileRulesKey{}, res.activeRulesCalculator.OnUpdate)
	disp.Register(model.ProfileRulesKey{}, res.skipFelix)
	disp.Register(model.ProfileTagsKey{}, res.onProfileTagsUpdate)
	disp.Register(model.ProfileLabelsKey{}, res.onProfileLabelsUpdate)
	disp.Register(model.ProfileLabelsKey{}, res.activeRulesCalculator.OnUpdate)
}

// Datastore callbacks:

func (res *Resolver) skipFelix(update model.KVPair) (filteredUpdate model.KVPair, skipFelix bool) {
	filteredUpdate = update
	skipFelix = true
	return
}

func (res *Resolver) onEndpointUpdate(update model.KVPair) (filteredUpdate model.KVPair, skipFelix bool) {
	key := update.Key.(model.WorkloadEndpointKey)
	onThisHost := key.Hostname == res.hostname
	if !onThisHost {
		skipFelix = true
	} else {
		res.activeRulesCalculator.OnUpdate(update)
	}
	if update.Value != nil {
		glog.V(3).Infof("Endpoint %v updated", update.Key)
		glog.V(4).Infof("Endpoint data: %#v", update.Value)
		ep := update.Value.(*model.WorkloadEndpoint)
		// FIXME IPv6
		ips := make([]ip.Addr, len(ep.IPv4Nets))
		for ii, net := range ep.IPv4Nets {
			ips[ii] = ip.FromNetIP(net.IP)
		}
		res.ipsetCalc.UpdateEndpointIPs(update.Key, ips)
		res.labelIdx.UpdateLabels(update.Key, ep.Labels, ep.ProfileIDs)
		res.tagIndex.UpdateEndpoint(update.Key, ep.ProfileIDs)
	} else {
		glog.V(3).Infof("Endpoint %v deleted", update.Key)
		res.ipsetCalc.DeleteEndpoint(update.Key)
		res.labelIdx.DeleteLabels(update.Key)
		res.tagIndex.DeleteEndpoint(update.Key)
	}
	filteredUpdate = update
	return
}

func (res *Resolver) onHostEndpointUpdate(update model.KVPair) (filteredUpdate model.KVPair, skipFelix bool) {
	key := update.Key.(model.HostEndpointKey)
	onThisHost := key.Hostname == res.hostname
	if !onThisHost {
		skipFelix = true
	}
	if update.Value != nil {
		glog.V(3).Infof("Endpoint %v updated", update.Key)
		glog.V(4).Infof("Endpoint data: %#v", update.Value)
		ep := update.Value.(*model.HostEndpoint)
		if onThisHost {
			res.activeRulesCalculator.OnUpdate(update)
		}
		// FIXME IPv6
		ips := make([]ip.Addr, len(ep.ExpectedIPv4Addrs))
		for ii, netIP := range ep.ExpectedIPv4Addrs {
			ips[ii] = ip.FromNetIP(netIP.IP)
		}
		res.ipsetCalc.UpdateEndpointIPs(update.Key, ips)
		res.labelIdx.UpdateLabels(update.Key, ep.Labels, ep.ProfileIDs)
		res.tagIndex.UpdateEndpoint(update.Key, ep.ProfileIDs)
	} else {
		glog.V(3).Infof("Endpoint %v deleted", update.Key)
		if onThisHost {
			res.activeRulesCalculator.OnUpdate(update)
		}
		res.ipsetCalc.DeleteEndpoint(update.Key)
		res.labelIdx.DeleteLabels(update.Key)
		res.tagIndex.DeleteEndpoint(update.Key)
	}
	filteredUpdate = update
	return
}

// onProfileLabelsUpdate updates the index when the labels attached to a profile change.
func (res *Resolver) onProfileLabelsUpdate(update model.KVPair) (filteredUpdate model.KVPair, skipFelix bool) {
	// The profile IDs in the endpoint are simple strings; convert the
	// key to string so that it matches.
	profileKey := update.Key.(model.ProfileLabelsKey)
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
	filteredUpdate = update
	skipFelix = true
	return
}

func (res *Resolver) onProfileTagsUpdate(update model.KVPair) (filteredUpdate model.KVPair, skipFelix bool) {
	glog.V(3).Infof("Profile tags %v updated", update)
	// The profile IDs in the endpoint are simple strings; convert the
	// key to string so that it matches.
	profileKey := update.Key.(model.ProfileTagsKey)
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
	filteredUpdate = update
	skipFelix = true
	return
}

//// OnProfileUpdate is called when we get a profile update from the datastore.
//// It passes through to the ActiveSetCalculator, which extracts the active ipsets from its rules.
//func (res *Resolver) OnProfileUpdate(key model.ProfileKey, policy *model.Profile) {
//	glog.Infof("Profile %v updated", key)
//	res.activeSelCalc.UpdateProfile(key, policy)
//}

// LabelIndex callbacks:

// onMatchStarted is called when an endpoint starts matching an active selector.
func (res *Resolver) onSelMatchStarted(selId, labelId interface{}) {
	glog.V(3).Infof("Endpoint %v now matches selector %v", labelId, selId)
	res.ipsetCalc.MatchStarted(labelId.(model.Key), selId.(string))
}

// onMatchStopped is called when an endpoint stops matching an active selector.
func (res *Resolver) onSelMatchStopped(selId, labelId interface{}) {
	glog.V(3).Infof("Endpoint %v no longer matches selector %v", labelId, selId)
	res.ipsetCalc.MatchStopped(labelId.(model.Key), selId.(string))
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
	res.callbacks.OnIPSetAdded(sel.UniqueId())
	res.labelIdx.UpdateSelector(sel.UniqueId(), sel)
}

// onSelectorActive is called when a selector stops being used in a rule.
// It removes the selector from the label index and stops tracking it.
func (res *Resolver) onSelectorInactive(sel selector.Selector) {
	glog.Infof("Selector %v now inactive", sel)
	res.labelIdx.DeleteSelector(sel.UniqueId())
	res.callbacks.OnIPSetRemoved(sel.UniqueId())
}

// IpsetCalculator callbacks:

// onIPAdded is called when an IP is now present in an active selector.
func (res *Resolver) onIPAdded(selID string, ip ip.Addr) {
	glog.V(3).Infof("IP set %v now contains %v", selID, ip)
	res.callbacks.OnIPAdded(selID, ip)
}

// onIPRemoved is called when an IP is no longer present in a selector.
func (res *Resolver) onIPRemoved(selID string, ip ip.Addr) {
	glog.V(3).Infof("IP set %v no longer contains %v", selID, ip)
	res.callbacks.OnIPRemoved(selID, ip)
}
