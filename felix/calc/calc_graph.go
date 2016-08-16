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

package calc

import (
	"github.com/golang/glog"
	"github.com/tigera/libcalico-go/datastructures/ip"
	"github.com/tigera/libcalico-go/datastructures/labels"
	"github.com/tigera/libcalico-go/datastructures/tags"
	"github.com/tigera/libcalico-go/felix/endpoint"
	"github.com/tigera/libcalico-go/felix/proto"
	"github.com/tigera/libcalico-go/felix/store"
	"github.com/tigera/libcalico-go/lib/backend/api"
	"github.com/tigera/libcalico-go/lib/backend/model"
	"github.com/tigera/libcalico-go/lib/hash"
	"github.com/tigera/libcalico-go/lib/selector"
)

type ipSetUpdateCallbacks interface {
	OnIPSetAdded(setID string)
	OnIPAdded(setID string, ip ip.Addr)
	OnIPRemoved(setID string, ip ip.Addr)
	OnIPSetRemoved(setID string)
}

type rulesUpdateCallbacks interface {
	OnPolicyActive(model.PolicyKey, *proto.Rules)
	OnPolicyInactive(model.PolicyKey)
	OnProfileActive(model.ProfileRulesKey, *proto.Rules)
	OnProfileInactive(model.ProfileRulesKey)
}

type endpointCallbacks interface {
	OnEndpointTierUpdate(endpointKey model.Key,
		endpoint interface{},
		filteredTiers []endpoint.TierInfo)
}

type configCallbacks interface {
	OnConfigUpdate(globalConfig, hostConfig map[string]string)
}

type PipelineCallbacks interface {
	ipSetUpdateCallbacks
	rulesUpdateCallbacks
	endpointCallbacks
	configCallbacks
}

func NewCalculationGraph(callbacks PipelineCallbacks, hostname string) (input *store.Dispatcher) {
	// The source of the processing graph, this dispatcher will be fed all
	// the updates from the datastore, fanning them out to the registered
	// handlers.
	sourceDispatcher := store.NewDispatcher()

	// Some of the handlers only need to know about local endpoints.
	// Create a second dispatcher which will filter out non-local endpoints.
	localEndpointFilter := &endpointHostnameFilter{hostname: hostname}
	localEndpointDispatcher := store.NewDispatcher()
	sourceDispatcher.Register(model.WorkloadEndpointKey{}, localEndpointDispatcher)
	sourceDispatcher.Register(model.HostEndpointKey{}, localEndpointDispatcher)
	localEndpointDispatcher.Register(model.WorkloadEndpointKey{}, localEndpointFilter)
	localEndpointDispatcher.Register(model.HostEndpointKey{}, localEndpointFilter)

	// The active rules calculator matches local endpoints against policies
	// and profiles to figure out which policies/profiles are active on this
	// host.
	activeRulesCalc := NewActiveRulesCalculator()
	// It needs the filtered endpoints...
	localEndpointDispatcher.Register(model.WorkloadEndpointKey{}, activeRulesCalc)
	localEndpointDispatcher.Register(model.HostEndpointKey{}, activeRulesCalc)
	// ...as well as all the policies and profiles.
	sourceDispatcher.Register(model.PolicyKey{}, activeRulesCalc)
	sourceDispatcher.Register(model.ProfileRulesKey{}, activeRulesCalc)
	sourceDispatcher.Register(model.ProfileLabelsKey{}, activeRulesCalc)

	// The rule scanner takes the output from the active rules calculator
	// and scans the individual rules for selectors and tags.  It generates
	// events when a new selector/tag starts/stops being used.
	ruleScanner := NewRuleScanner()
	activeRulesCalc.RuleScanner = ruleScanner
	ruleScanner.RulesUpdateCallbacks = callbacks

	// The active selector index matches the active selectors found by the
	// rule scanner against *all* endpoints.  It emits events when an
	// endpoint starts/stops matching one of the active selectors.  We
	// send the events to the membership calculator, which will extract the
	// ip addresses of the endpoints.  The member calculator handles tags
	// and selectors uniformly but we need to shim the interface because
	// it expects a string ID.
	var memberCalc *MemberCalculator
	activeSelectorIndex := labels.NewInheritIndex(
		func(selId, labelId interface{}) {
			// Match started callback.
			memberCalc.MatchStarted(labelId, selId.(string))
		},
		func(selId, labelId interface{}) {
			// Match stopped callback.
			memberCalc.MatchStopped(labelId, selId.(string))
		},
	)
	ruleScanner.OnSelectorActive = func(sel selector.Selector) {
		glog.Infof("Selector %v now active", sel)
		callbacks.OnIPSetAdded(sel.UniqueId())
		activeSelectorIndex.UpdateSelector(sel.UniqueId(), sel)
	}
	ruleScanner.OnSelectorInactive = func(sel selector.Selector) {
		glog.Infof("Selector %v now inactive", sel)
		callbacks.OnIPSetRemoved(sel.UniqueId())
		activeSelectorIndex.DeleteSelector(sel.UniqueId())
	}
	sourceDispatcher.Register(model.ProfileLabelsKey{}, activeSelectorIndex)
	sourceDispatcher.Register(model.WorkloadEndpointKey{}, activeSelectorIndex)
	sourceDispatcher.Register(model.HostEndpointKey{}, activeSelectorIndex)

	// The active tag index does the same for tags.  Calculating which
	// endpoints match each tag.
	tagIndex := tags.NewIndex(
		func(key tags.EndpointKey, tagID string) {
			memberCalc.MatchStarted(key, hash.MakeUniqueID("t", tagID))
		},
		func(key tags.EndpointKey, tagID string) {
			memberCalc.MatchStopped(key, hash.MakeUniqueID("t", tagID))
		},
	)
	ruleScanner.OnTagActive = tagIndex.SetTagActive
	ruleScanner.OnTagInactive = tagIndex.SetTagInactive
	sourceDispatcher.Register(model.WorkloadEndpointKey{}, tagIndex)
	sourceDispatcher.Register(model.HostEndpointKey{}, tagIndex)

	// The member calculator merges the IPs from different endpoints to
	// calculate the actual IPs that should be in each IP set.  It deals
	// with corner cases, such as having the same IP on multiple endpoints.
	memberCalc = NewMemberCalculator()
	// It needs to know about *all* endpoints to do the calculation.
	sourceDispatcher.Register(model.WorkloadEndpointKey{}, memberCalc)
	sourceDispatcher.Register(model.HostEndpointKey{}, memberCalc)
	// Hook it up to the output.
	memberCalc.callbacks = callbacks

	// The endpoint policy resolver marries up the active policies with
	// local endpoints and calculates the complete, ordered set of
	// policies that apply to each endpoint.
	polResolver := endpoint.NewPolicyResolver()
	// Hook up the inputs to the policy resolver.
	activeRulesCalc.PolicyMatchListener = polResolver
	sourceDispatcher.Register(model.PolicyKey{}, polResolver)
	sourceDispatcher.Register(model.TierKey{}, polResolver)
	localEndpointDispatcher.Register(model.WorkloadEndpointKey{}, polResolver)
	localEndpointDispatcher.Register(model.HostEndpointKey{}, polResolver)
	// And hook its output to the callbacks.
	polResolver.Callbacks = callbacks

	// Register for config updates.
	configBatcher := NewConfigBatcher(hostname, callbacks)
	sourceDispatcher.Register(model.GlobalConfigKey{}, configBatcher)
	sourceDispatcher.Register(model.HostConfigKey{}, configBatcher)

	return sourceDispatcher
}

// endpointHostnameFilter provides an UpdateHandler that filters out endpoints
// that are not on the given host.
type endpointHostnameFilter struct {
	hostname string
}

func (f *endpointHostnameFilter) OnUpdate(update model.KVPair) (filterOut bool) {
	switch key := update.Key.(type) {
	case model.WorkloadEndpointKey:
		if key.Hostname != f.hostname {
			filterOut = true
		}
	case model.HostEndpointKey:
		if key.Hostname != f.hostname {
			filterOut = true
		}
	}
	return
}

func (f *endpointHostnameFilter) OnDatamodelStatus(status api.SyncStatus) {
}
