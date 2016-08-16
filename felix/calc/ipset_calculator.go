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
	"github.com/tigera/libcalico-go/lib/backend/api"
	"github.com/tigera/libcalico-go/lib/backend/model"
)

// endpointKeys are expected to be WorkloadEndpointKey or HostEndpointKey
// objects but all we require is that they're hashable objects.
type endpointKey interface{}

type IPAddRemoveCallbacks interface {
	OnIPAdded(ipSetID string, ip ip.Addr)
	OnIPRemoved(ipSetID string, ip ip.Addr)
}

type MemberCalculator struct {
	keyToIPs              map[endpointKey][]ip.Addr
	keyToMatchingIPSetIDs multidict.IfaceToString
	ipSetIDToIPToKey      map[string]multidict.IfaceToIface

	callbacks IPAddRemoveCallbacks
}

func NewMemberCalculator() *MemberCalculator {
	calc := &MemberCalculator{
		keyToIPs:              make(map[endpointKey][]ip.Addr),
		keyToMatchingIPSetIDs: multidict.NewIfaceToString(),
		ipSetIDToIPToKey:      make(map[string]multidict.IfaceToIface),
	}
	return calc
}

// MatchStarted tells this object that an endpoint now belongs to an IP set.
func (calc *MemberCalculator) MatchStarted(key endpointKey, ipSetID string) {
	glog.V(4).Infof("Adding endpoint %v to IP set %v", key, ipSetID)
	calc.keyToMatchingIPSetIDs.Put(key, ipSetID)
	ips := calc.keyToIPs[key]
	calc.addMatchToIndex(ipSetID, key, ips)
}

// MatchStopped tells this object that an endpoint no longer belongs to an IP set.
func (calc *MemberCalculator) MatchStopped(key endpointKey, ipSetID string) {
	glog.V(4).Infof("Removing endpoint %v from IP set %v", key, ipSetID)
	calc.keyToMatchingIPSetIDs.Discard(key, ipSetID)
	ips := calc.keyToIPs[key]
	calc.removeMatchFromIndex(ipSetID, key, ips)
}

func (calc *MemberCalculator) OnUpdate(update model.KVPair) (filterOut bool) {
	if update.Value == nil {
		calc.updateEndpointIPs(update.Key, []ip.Addr{})
		return
	}
	switch update.Key.(type) {
	case model.WorkloadEndpointKey:
		ep := update.Value.(model.WorkloadEndpoint)
		ips := make([]ip.Addr, 0, len(ep.IPv4Nets)+len(ep.IPv6Nets))
		for _, net := range ep.IPv4Nets {
			ips = append(ips, ip.FromNetIP(net.IP))
		}
		for _, net := range ep.IPv6Nets {
			ips = append(ips, ip.FromNetIP(net.IP))
		}
		calc.updateEndpointIPs(update.Key, ips)
	case model.HostEndpointKey:
		ep := update.Value.(*model.HostEndpoint)
		ips := make([]ip.Addr, 0,
			len(ep.ExpectedIPv4Addrs)+len(ep.ExpectedIPv6Addrs))
		for _, netIP := range ep.ExpectedIPv4Addrs {
			ips = append(ips, ip.FromNetIP(netIP.IP))
		}
		for _, netIP := range ep.ExpectedIPv6Addrs {
			ips = append(ips, ip.FromNetIP(netIP.IP))
		}
		calc.updateEndpointIPs(update.Key, ips)
	}
	return
}

func (l *MemberCalculator) OnDatamodelStatus(status api.SyncStatus) {
}

// UpdateEndpointIPs tells this object that an endpoint has a new set of IP addresses.
func (calc *MemberCalculator) updateEndpointIPs(endpointKey endpointKey, ips []ip.Addr) {
	glog.V(4).Infof("Endpoint %v IPs updated to %v", endpointKey, ips)
	oldIPs := calc.keyToIPs[endpointKey]
	if len(ips) == 0 {
		delete(calc.keyToIPs, endpointKey)
	} else {
		calc.keyToIPs[endpointKey] = ips
	}

	oldIPsSet := set.New()
	for _, ip := range oldIPs {
		oldIPsSet.Add(ip)
	}

	addedIPs := make([]ip.Addr, 0)
	currentIPs := set.New()
	for _, ip := range ips {
		if !oldIPsSet.Contains(ip) {
			glog.V(4).Infof("Added IP: %v", ip)
			addedIPs = append(addedIPs, ip)
		}
		currentIPs.Add(ip)
	}

	removedIPs := make([]ip.Addr, 0)
	for _, ip := range oldIPs {
		if !currentIPs.Contains(ip) {
			glog.V(4).Infof("Removed IP: %v", ip)
			removedIPs = append(removedIPs, ip)
		}
	}

	calc.keyToMatchingIPSetIDs.Iter(endpointKey, func(ipSetID string) {
		glog.V(4).Infof("Updating matching IP set: %v", ipSetID)
		calc.addMatchToIndex(ipSetID, endpointKey, addedIPs)
		calc.removeMatchFromIndex(ipSetID, endpointKey, removedIPs)
	})
}

func (calc *MemberCalculator) Empty() bool {
	if len(calc.keyToIPs) != 0 {
		return false
	}
	if !calc.keyToMatchingIPSetIDs.Empty() {
		return false
	}
	if len(calc.ipSetIDToIPToKey) != 0 {
		return false
	}
	return true
}

func (calc *MemberCalculator) addMatchToIndex(ipSetID string, key endpointKey, ips []ip.Addr) {
	glog.V(3).Infof("IP set %v now matches IPs %v via %v", ipSetID, ips, key)
	ipToKeys, ok := calc.ipSetIDToIPToKey[ipSetID]
	if !ok {
		ipToKeys = multidict.NewIfaceToIface()
		calc.ipSetIDToIPToKey[ipSetID] = ipToKeys
	}

	for _, ip := range ips {
		if !ipToKeys.ContainsKey(ip) {
			glog.V(3).Infof("New IP in IP set %v: %v", ipSetID, ip)
			calc.callbacks.OnIPAdded(ipSetID, ip)
		}
		ipToKeys.Put(ip, key)
	}
}

func (calc *MemberCalculator) removeMatchFromIndex(ipSetID string, key endpointKey, ips []ip.Addr) {
	glog.V(3).Infof("IP set %v no longer matches IPs %v via %v", ipSetID, ips, key)
	ipToKeys := calc.ipSetIDToIPToKey[ipSetID]
	for _, ip := range ips {
		ipToKeys.Discard(ip, key)
		if !ipToKeys.ContainsKey(ip) {
			glog.V(3).Infof("IP no longer in IP set %v: %v", ipSetID, ip)
			calc.callbacks.OnIPRemoved(ipSetID, ip)
			if ipToKeys.Len() == 0 {
				delete(calc.ipSetIDToIPToKey, ipSetID)
			}
		}
	}
}
