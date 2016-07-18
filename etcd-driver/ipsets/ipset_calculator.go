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

import "github.com/golang/glog"

// endpointKeys are expected to be WorkloadEndpointKey or HostEndpointKey
// objects but all we require is that they're hashable objects.
type endpointKey interface{}

type IpsetCalculator struct {
	keyToIPs              map[endpointKey][]string
	keyToMatchingIPSetIDs map[endpointKey]map[string]bool
	ipSetIDToIPToKey      map[string]map[string]map[endpointKey]bool

	// Callbacks.
	OnIPAdded   func(ipSetID string, ip string)
	OnIPRemoved func(ipSetID string, ip string)
}

func NewIpsetCalculator() *IpsetCalculator {
	calc := &IpsetCalculator{
		keyToIPs:              make(map[endpointKey][]string),
		keyToMatchingIPSetIDs: make(map[endpointKey]map[string]bool),
		ipSetIDToIPToKey:      make(map[string]map[string]map[endpointKey]bool),
	}
	return calc
}

// MatchStarted tells this object that an endpoint now belongs to an IP set.
func (calc *IpsetCalculator) MatchStarted(key endpointKey, ipSetID string) {
	matchingIDs, ok := calc.keyToMatchingIPSetIDs[key]
	if !ok {
		matchingIDs = make(map[string]bool)
		calc.keyToMatchingIPSetIDs[key] = matchingIDs
	}
	matchingIDs[ipSetID] = true

	ips := calc.keyToIPs[key]
	calc.addMatchToIndex(ipSetID, key, ips)
}

// MatchStopped tells this object that an endpoint no longer belongs to an IP set.
func (calc *IpsetCalculator) MatchStopped(key endpointKey, ipSetID string) {
	matchingIDs := calc.keyToMatchingIPSetIDs[key]
	delete(matchingIDs, ipSetID)
	if len(matchingIDs) == 0 {
		delete(calc.keyToMatchingIPSetIDs, key)
	}

	ips := calc.keyToIPs[key]
	calc.removeMatchFromIndex(ipSetID, key, ips)
}

// UpdateEndpointIPs tells this object that an endpoint has a new set of IP addresses.
func (calc *IpsetCalculator) UpdateEndpointIPs(endpointKey endpointKey, ips []string) {
	glog.V(4).Infof("Endpoint %v IPs updated to %v", endpointKey, ips)
	oldIPs := calc.keyToIPs[endpointKey]
	if len(ips) == 0 {
		delete(calc.keyToIPs, endpointKey)
	} else {
		calc.keyToIPs[endpointKey] = ips
	}

	oldIPsSet := make(map[string]bool)
	for _, ip := range oldIPs {
		oldIPsSet[ip] = true
	}

	addedIPs := make([]string, 0)
	currentIPs := make(map[string]bool)
	for _, ip := range ips {
		if !oldIPsSet[ip] {
			addedIPs = append(addedIPs, ip)
		}
		currentIPs[ip] = true
	}

	removedIPs := make([]string, 0)
	for _, ip := range oldIPs {
		if !currentIPs[ip] {
			removedIPs = append(removedIPs, ip)
		}
	}

	matchingSels := calc.keyToMatchingIPSetIDs[endpointKey]
	glog.V(4).Infof("Updating IPs in matching selectors: %v", matchingSels)
	for ipSetID, _ := range matchingSels {
		calc.addMatchToIndex(ipSetID, endpointKey, addedIPs)
		calc.removeMatchFromIndex(ipSetID, endpointKey, removedIPs)
	}
}

// DeleteEndpoint removes an endpoint from the index.
func (calc *IpsetCalculator) DeleteEndpoint(endpointKey endpointKey) {
	calc.UpdateEndpointIPs(endpointKey, []string{})
}

func (calc *IpsetCalculator) addMatchToIndex(ipSetID string, key endpointKey, ips []string) {
	glog.V(3).Infof("Selector %v now matches IPs %v via %v", ipSetID, ips, key)
	ipToKeys, ok := calc.ipSetIDToIPToKey[ipSetID]
	if !ok {
		ipToKeys = make(map[string]map[endpointKey]bool)
		calc.ipSetIDToIPToKey[ipSetID] = ipToKeys
	}

	for _, ip := range ips {
		keys, ok := ipToKeys[ip]
		if !ok {
			keys = make(map[endpointKey]bool)
			ipToKeys[ip] = keys
			calc.OnIPAdded(ipSetID, ip)
		}
		keys[key] = true
	}
}

func (calc *IpsetCalculator) removeMatchFromIndex(ipSetID string, key endpointKey, ips []string) {
	glog.V(3).Infof("Selector %v no longer matches IPs %v via %v", ipSetID, ips, key)
	ipToKeys := calc.ipSetIDToIPToKey[ipSetID]
	for _, ip := range ips {
		keys := ipToKeys[ip]
		delete(keys, key)
		if len(keys) == 0 {
			calc.OnIPRemoved(ipSetID, ip)
			delete(ipToKeys, ip)
			if len(ipToKeys) == 0 {
				delete(calc.ipSetIDToIPToKey, ipSetID)
			}
		}
	}
}
