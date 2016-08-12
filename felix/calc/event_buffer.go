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
	"github.com/tigera/libcalico-go/datastructures/ip"
	"github.com/tigera/libcalico-go/datastructures/multidict"
	"github.com/tigera/libcalico-go/datastructures/set"
	"github.com/tigera/libcalico-go/felix/endpoint"
	"github.com/tigera/libcalico-go/felix/proto"
	"github.com/tigera/libcalico-go/lib/backend/model"
	"github.com/tigera/libcalico-go/lib/net"
)

type EventHandler func(message interface{})

type EventBuffer struct {
	ipSetsAdded   set.Set
	ipSetsRemoved set.Set
	ipsAdded      multidict.StringToIface
	ipsRemoved    multidict.StringToIface

	pendingUpdates []interface{}

	callback EventHandler
}

func NewEventBuffer() *EventBuffer {
	buf := &EventBuffer{
		ipSetsAdded:   set.New(),
		ipSetsRemoved: set.New(),
		ipsAdded:      multidict.NewStringToIface(),
		ipsRemoved:    multidict.NewStringToIface(),
	}
	return buf
}

func (buf *EventBuffer) OnIPSetAdded(setID string) {
	glog.V(3).Infof("IP set %v now active", setID)
	buf.ipSetsAdded.Add(setID)
	buf.ipSetsRemoved.Discard(setID)
}

func (buf *EventBuffer) OnIPSetRemoved(setID string) {
	glog.V(3).Infof("IP set %v no longer active", setID)
	buf.ipSetsAdded.Discard(setID)
	buf.ipSetsRemoved.Add(setID)
}

func (buf *EventBuffer) OnIPAdded(setID string, ip ip.Addr) {
	glog.V(4).Infof("IP set %v now contains %v", setID, ip)
	buf.ipsAdded.Put(setID, ip)
	buf.ipsRemoved.Discard(setID, ip)
}

func (buf *EventBuffer) OnIPRemoved(setID string, ip ip.Addr) {
	glog.V(4).Infof("IP set %v no longer contains %v", setID, ip)
	buf.ipsAdded.Discard(setID, ip)
	buf.ipsRemoved.Put(setID, ip)
}

func (buf *EventBuffer) Flush() {
	buf.ipSetsRemoved.Iter(func(item interface{}) (err error) {
		setID := item.(string)
		glog.V(3).Infof("Flushing IP set remove: %v", setID)
		buf.callback(&proto.IPSetRemove{
			SetID: setID,
		})
		buf.ipsRemoved.DiscardKey(setID)
		buf.ipsAdded.DiscardKey(setID)
		buf.ipSetsRemoved.Discard(item)
		return
	})
	buf.ipSetsAdded.Iter(func(item interface{}) (err error) {
		setID := item.(string)
		glog.V(3).Infof("Flushing IP set added: %v", setID)
		members := make([]net.IP, 0)
		buf.ipsAdded.Iter(setID, func(value interface{}) {
			members = append(members, value.(ip.Addr).AsCalicoNetIP())
		})
		buf.ipsAdded.DiscardKey(setID)
		buf.callback(&proto.IPSetUpdate{
			SetID:   setID,
			Members: members,
		})
		buf.ipSetsAdded.Discard(item)
		return
	})
	buf.ipsRemoved.IterKeys(buf.flushAddsOrRemoves)
	buf.ipsAdded.IterKeys(buf.flushAddsOrRemoves)

	for _, update := range buf.pendingUpdates {
		buf.callback(update)
	}
	buf.pendingUpdates = make([]interface{}, 0)
}

func (buf *EventBuffer) flushAddsOrRemoves(setID string) {
	glog.V(3).Infof("Flushing IP set deltas: %v", setID)
	deltaUpdate := proto.IPSetDeltaUpdate{
		SetID: setID,
	}
	buf.ipsAdded.Iter(setID, func(item interface{}) {
		ip := item.(ip.Addr).AsCalicoNetIP()
		deltaUpdate.AddedIPs = append(deltaUpdate.AddedIPs, ip)
	})
	buf.ipsRemoved.Iter(setID, func(item interface{}) {
		ip := item.(ip.Addr).AsCalicoNetIP()
		deltaUpdate.RemovedIPs = append(deltaUpdate.RemovedIPs, ip)
	})
	buf.ipsAdded.DiscardKey(setID)
	buf.ipsRemoved.DiscardKey(setID)
	buf.callback(&deltaUpdate)
}

func (buf *EventBuffer) OnPolicyActive(key model.PolicyKey, rules *proto.Rules) {
	buf.pendingUpdates = append(buf.pendingUpdates, &proto.ActivePolicyUpdate{
		Tier:   key.Tier,
		Name:   key.Name,
		Policy: *rules,
	})
}

func (buf *EventBuffer) OnPolicyInactive(key model.PolicyKey) {
	buf.pendingUpdates = append(buf.pendingUpdates, &proto.ActivePolicyRemove{
		Tier: key.Tier,
		Name: key.Name,
	})
}

func (buf *EventBuffer) OnProfileActive(key model.ProfileRulesKey, rules *proto.Rules) {
	buf.pendingUpdates = append(buf.pendingUpdates, &proto.ActiveProfileUpdate{
		Name:    key.Name,
		Profile: *rules,
	})
}

func (buf *EventBuffer) OnProfileInactive(key model.ProfileRulesKey) {
	buf.pendingUpdates = append(buf.pendingUpdates, &proto.ActiveProfileRemove{
		Name: key.Name,
	})
}

func (buf *EventBuffer) OnEndpointTierUpdate(endpointKey model.Key,
	endpoint interface{},
	filteredTiers []endpoint.TierInfo) {
	tiers := convertBackendTierInfo(filteredTiers)
	switch key := endpointKey.(type) {
	case model.WorkloadEndpointKey:
		if endpoint == nil {
			buf.pendingUpdates = append(buf.pendingUpdates,
				&proto.WorkloadEndpointRemove{
					Hostname:       key.Hostname,
					OrchestratorID: key.OrchestratorID,
					WorkloadID:     key.WorkloadID,
					EndpointID:     key.EndpointID,
				})
			return
		}
		ep := endpoint.(*model.WorkloadEndpoint)
		buf.pendingUpdates = append(buf.pendingUpdates,
			&proto.WorkloadEndpointUpdate{
				Hostname:       key.Hostname,
				OrchestratorID: key.OrchestratorID,
				WorkloadID:     key.WorkloadID,
				EndpointID:     key.EndpointID,

				Endpoint: proto.WorkloadEndpoint{
					State:      ep.State,
					Name:       ep.Name,
					Mac:        ep.Mac,
					ProfileIDs: ep.ProfileIDs,
					IPv4Nets:   ep.IPv4Nets,
					IPv6Nets:   ep.IPv6Nets,
					Tiers:      tiers,
				},
			})
	case model.HostEndpointKey:
		if endpoint == nil {
			buf.pendingUpdates = append(buf.pendingUpdates,
				&proto.HostEndpointRemove{
					Hostname:   key.Hostname,
					EndpointID: key.EndpointID,
				})
			return
		}
		ep := endpoint.(*model.HostEndpoint)
		buf.pendingUpdates = append(buf.pendingUpdates,
			&proto.HostEndpointUpdate{
				Hostname:   key.Hostname,
				EndpointID: key.EndpointID,
				Endpoint: proto.HostEndpoint{
					Name:              ep.Name,
					ExpectedIPv4Addrs: ep.ExpectedIPv4Addrs,
					ExpectedIPv6Addrs: ep.ExpectedIPv6Addrs,
					ProfileIDs:        ep.ProfileIDs,
					Tiers:             tiers,
				},
			})
	}
}

func convertBackendTierInfo(filteredTiers []endpoint.TierInfo) []proto.TierInfo {
	tiers := make([]proto.TierInfo, len(filteredTiers))
	if len(filteredTiers) > 0 {
		for ii, ti := range filteredTiers {
			pols := make([]string, len(ti.OrderedPolicies))
			for jj, pol := range ti.OrderedPolicies {
				pols[jj] = pol.Key.Name
			}
			tiers[ii] = proto.TierInfo{ti.Name, pols}
		}
	}
	return tiers
}
