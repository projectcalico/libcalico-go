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

package endpoint

import (
	"github.com/golang/glog"
	"github.com/tigera/libcalico-go/datastructures/multidict"
	"github.com/tigera/libcalico-go/datastructures/set"
	"github.com/tigera/libcalico-go/lib/backend/model"
)

type PolicyResolver struct {
	policyIDToEndpointIDs multidict.IfaceToIface
	endpointIDToPolicyIDs multidict.IfaceToIface
	sortedTierData        []*TierInfo
	endpoints             map[model.Key]interface{}
	dirtyEndpoints        set.Set
	policySorter          *PolicySorter
	callbacks             PolicyResolverCallbacks
	InSync                bool
}

type PolicyResolverCallbacks interface {
	OnEndpointTierUpdate(endpointKey model.Key, filteredTiers []TierInfo)
}

func NewPolicyResolver(callbacks PolicyResolverCallbacks) *PolicyResolver {
	return &PolicyResolver{
		policyIDToEndpointIDs: multidict.NewIfaceToIface(),
		endpointIDToPolicyIDs: multidict.NewIfaceToIface(),
		endpoints:             make(map[model.Key]interface{}),
		dirtyEndpoints:        set.New(),
		policySorter:          NewPolicySorter(),
		callbacks:             callbacks,
	}
}

func (pr *PolicyResolver) OnUpdate(update model.KVPair) (filteredUpdate model.KVPair, skipFelix bool) {
	policiesDirty := false
	switch key := update.Key.(type) {
	case model.PolicyKey:
		glog.V(3).Infof("Policy update: %v", key)
		policiesDirty = pr.policySorter.OnUpdate(update) &&
			pr.policyIDToEndpointIDs.ContainsKey(update.Key)
		pr.markEndpointsMatchingPolicyDirty(key)
	case model.TierKey:
		glog.V(3).Infof("Tier update: %v", key)
		policiesDirty = pr.policySorter.OnUpdate(update)
		pr.markAllEndpointsDirty()
		//skipFelix = true
	}
	if policiesDirty {
		glog.V(3).Info("Policies dirty, refreshing sort order")
		pr.sortedTierData = pr.policySorter.Sorted()
		glog.V(3).Infof("New sort order: %v", pr.sortedTierData)
		pr.Flush()
	}
	filteredUpdate = update
	return
}

func (pr *PolicyResolver) markAllEndpointsDirty() {
	glog.V(3).Infof("Marking all endpoints dirty")
	pr.endpointIDToPolicyIDs.IterKeys(func(epID interface{}) {
		pr.dirtyEndpoints.Add(epID)
	})
}

func (pr *PolicyResolver) markEndpointsMatchingPolicyDirty(polKey model.PolicyKey) {
	glog.V(3).Infof("Marking all endpoints matching %v dirty", polKey)
	pr.policyIDToEndpointIDs.Iter(polKey, func(epID interface{}) {
		pr.dirtyEndpoints.Add(epID)
	})
}

func (pr *PolicyResolver) OnPolicyMatch(policyKey model.PolicyKey, endpointKey interface{}) {
	glog.V(3).Infof("Storing policy match %v -> %v", policyKey, endpointKey)
	pr.policyIDToEndpointIDs.Put(policyKey, endpointKey)
	pr.endpointIDToPolicyIDs.Put(endpointKey, policyKey)
	pr.dirtyEndpoints.Add(endpointKey)
	pr.Flush()
}

func (pr *PolicyResolver) OnPolicyMatchStopped(policyKey model.PolicyKey, endpointKey interface{}) {
	glog.V(3).Infof("Deleting policy match %v -> %v", policyKey, endpointKey)
	pr.policyIDToEndpointIDs.Discard(policyKey, endpointKey)
	pr.endpointIDToPolicyIDs.Discard(endpointKey, policyKey)
	pr.dirtyEndpoints.Add(endpointKey)
	pr.Flush()
}

func (pr *PolicyResolver) Flush() {
	if !pr.InSync {
		glog.V(3).Infof("Not in sync, skipping flush")
		return
	}
	pr.dirtyEndpoints.Iter(func(endpointID interface{}) error {
		glog.V(3).Infof("Scanning endpoint %v", endpointID)
		applicableTiers := []TierInfo{}
		for _, tier := range pr.sortedTierData {
			if !tier.Valid {
				glog.V(3).Infof("Tier %v invalid, skipping", tier.Name)
				continue
			}
			tierMatches := false
			filteredTier := TierInfo{
				Name:  tier.Name,
				Order: tier.Order,
				Valid: true,
			}
			for _, polKV := range tier.OrderedPolicies {
				glog.V(4).Infof("Checking if policy %v matches %v", polKV.Key, endpointID)
				if pr.endpointIDToPolicyIDs.Contains(endpointID, polKV.Key) {
					glog.V(4).Infof("Policy %v matches %v", polKV.Key, endpointID)
					tierMatches = true
					filteredTier.OrderedPolicies = append(filteredTier.OrderedPolicies,
						polKV)
				}
			}
			if tierMatches {
				glog.V(4).Infof("Tier %v matches %v", tier.Name, endpointID)
				applicableTiers = append(applicableTiers, filteredTier)
			}
		}
		glog.V(4).Infof("Tier update: %v -> %v", endpointID, applicableTiers)
		pr.callbacks.OnEndpointTierUpdate(endpointID.(model.Key), applicableTiers)
		return nil
	})
	pr.dirtyEndpoints = set.New()
}
