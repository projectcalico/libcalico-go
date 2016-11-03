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

package client

import (
	"errors"
	"fmt"
	"math/big"
	"net"
	"reflect"

	log "github.com/Sirupsen/logrus"
	"github.com/projectcalico/libcalico-go/lib/backend/model"
	cnet "github.com/projectcalico/libcalico-go/lib/net"
)

const (
	blockSize = 64
)

type ipVersion struct {
	Number            int
	TotalBits         int
	BlockPrefixLength int
	BlockPrefixMask   net.IPMask
}

var ipv4 ipVersion = ipVersion{
	Number:            4,
	TotalBits:         32,
	BlockPrefixLength: 26,
	BlockPrefixMask:   net.CIDRMask(26, 32),
}

var ipv6 ipVersion = ipVersion{
	Number:            6,
	TotalBits:         128,
	BlockPrefixLength: 122,
	BlockPrefixMask:   net.CIDRMask(122, 128),
}

// Wrap the backend AllocationBlock struct so that we can
// attach methods to it.
type allocationBlock struct {
	*model.AllocationBlock
}

func newBlock(cidr cnet.IPNet) allocationBlock {
	b := model.AllocationBlock{}
	b.Allocations = make([]*int, blockSize)
	b.Unallocated = make([]int, blockSize)
	b.StrictAffinity = false
	b.CIDR = cidr

	// Initialize unallocated ordinals.
	for i := 0; i < blockSize; i++ {
		b.Unallocated[i] = i
	}

	return allocationBlock{&b}
}

func (b *allocationBlock) autoAssign(
	num int, handleID *string, host string, attrs map[string]string, affinityCheck bool) ([]cnet.IP, error) {

	// Determine if we need to check for affinity.
	checkAffinity := b.StrictAffinity || affinityCheck
	if checkAffinity && b.Affinity != nil && !hostAffinityMatches(host, b.AllocationBlock) {
		// Affinity check is enabled but the host does not match - error.
		s := fmt.Sprintf("Block affinity (%s) does not match provided (%s)", *b.Affinity, host)
		return nil, errors.New(s)
	}

	// Walk the allocations until we find enough addresses.
	ordinals := []int{}
	for len(b.Unallocated) > 0 && len(ordinals) < num {
		ordinals = append(ordinals, b.Unallocated[0])
		b.Unallocated = b.Unallocated[1:]
	}

	// Create slice of IPs and perform the allocations.
	ips := []cnet.IP{}
	for _, o := range ordinals {
		attrIndex := b.findOrAddAttribute(handleID, attrs)
		b.Allocations[o] = &attrIndex
		ips = append(ips, incrementIP(cnet.IP{b.CIDR.IP}, big.NewInt(int64(o))))
	}

	log.Debugf("Block %s returned ips: %v", b.CIDR.String(), ips)
	return ips, nil
}

func (b *allocationBlock) assign(address cnet.IP, handleID *string, attrs map[string]string, host string) error {
	if b.StrictAffinity && b.Affinity != nil && !hostAffinityMatches(host, b.AllocationBlock) {
		// Affinity check is enabled but the host does not match - error.
		return errors.New("Block host affinity does not match")
	}

	// Convert to an ordinal.
	ordinal := ipToOrdinal(address, *b)
	if (ordinal < 0) || (ordinal > blockSize) {
		return errors.New("IP address not in block")
	}

	// Check if already allocated.
	if b.Allocations[ordinal] != nil {
		return errors.New("Address already assigned in block")
	}

	// Set up attributes.
	attrIndex := b.findOrAddAttribute(handleID, attrs)
	b.Allocations[ordinal] = &attrIndex

	// Remove from unallocated.
	for i, unallocated := range b.Unallocated {
		if unallocated == ordinal {
			b.Unallocated = append(b.Unallocated[:i], b.Unallocated[i+1:]...)
			break
		}
	}
	return nil
}

// hostAffinityMatches checks if the provided host matches the provided affinity.
func hostAffinityMatches(host string, block *model.AllocationBlock) bool {
	return *block.Affinity == "host:"+host
}

func (b allocationBlock) numFreeAddresses() int {
	return len(b.Unallocated)
}

func (b allocationBlock) empty() bool {
	return b.numFreeAddresses() == blockSize
}

func (b *allocationBlock) release(addresses []cnet.IP) ([]cnet.IP, map[string]int, error) {
	// Store return values.
	unallocated := []cnet.IP{}
	countByHandle := map[string]int{}

	// Used internally.
	var ordinals []int
	delRefCounts := map[int]int{}
	attrsToDelete := []int{}

	// Determine the ordinals that need to be released and the
	// attributes that need to be cleaned up.
	for _, ip := range addresses {
		// Convert to an ordinal.
		ordinal := ipToOrdinal(ip, *b)
		if (ordinal < 0) || (ordinal > blockSize) {
			return nil, nil, errors.New("IP address not in block")
		}

		// Check if allocated.
		attrIdx := b.Allocations[ordinal]
		if attrIdx == nil {
			log.Debugf("Asked to release address that was not allocated")
			unallocated = append(unallocated, ip)
			continue
		}
		ordinals = append(ordinals, ordinal)

		// Increment referece counting for attributes.
		cnt := 1
		if cur, exists := delRefCounts[*attrIdx]; exists {
			cnt = cur + 1
		}
		delRefCounts[*attrIdx] = cnt

		// Increment count of addresses by handle if a handle
		// exists.
		handleID := b.Attributes[*attrIdx].AttrPrimary
		if handleID != nil {
			handleCount := 0
			if count, ok := countByHandle[*handleID]; !ok {
				handleCount = count
			}
			handleCount += 1
			countByHandle[*handleID] = handleCount
		}
	}

	// Handle cleaning up of attributes.  We do this by
	// reference counting.  If we're deleting the last reference to
	// a given attribute, then it needs to be cleaned up.
	refCounts := b.attributeRefCounts()
	for idx, refs := range delRefCounts {
		if refCounts[idx] == refs {
			attrsToDelete = append(attrsToDelete, idx)
		}
	}
	if len(attrsToDelete) != 0 {
		log.Infof("Deleting attributes: %s", attrsToDelete)
		b.deleteAttributes(attrsToDelete, ordinals)
	}

	// Release requested addresses.
	for _, ordinal := range ordinals {
		b.Allocations[ordinal] = nil
		b.Unallocated = append(b.Unallocated, ordinal)
	}
	return unallocated, countByHandle, nil
}

func (b *allocationBlock) deleteAttributes(delIndexes, ordinals []int) {
	newIndexes := make([]*int, len(b.Attributes))
	newAttrs := []model.AllocationAttribute{}
	y := 0 // Next free slot in the new attributes list.
	for x := range b.Attributes {
		if !intInSlice(x, delIndexes) {
			// Attribute at x is not being deleted.  Build a mapping
			// of old attribute index (x) to new attribute index (y).
			log.Debugf("%d in %s", x, delIndexes)
			newIndex := y
			newIndexes[x] = &newIndex
			y += 1
			newAttrs = append(newAttrs, b.Attributes[x])
		}
	}
	b.Attributes = newAttrs

	// Update attribute indexes for all allocations in this block.
	for i := 0; i < blockSize; i++ {
		if b.Allocations[i] != nil {
			// Get the new index that corresponds to the old index
			// and update the allocation.
			newIndex := newIndexes[*b.Allocations[i]]
			b.Allocations[i] = newIndex
		}
	}
}

func (b allocationBlock) attributeRefCounts() map[int]int {
	refCounts := map[int]int{}
	for _, a := range b.Allocations {
		if a == nil {
			continue
		}

		if count, ok := refCounts[*a]; !ok {
			// No entry for given attribute index.
			refCounts[*a] = 1
		} else {
			refCounts[*a] = count + 1
		}
	}
	return refCounts
}

func (b allocationBlock) attributeIndexesByHandle(handleID string) []int {
	indexes := []int{}
	for i, attr := range b.Attributes {
		if attr.AttrPrimary != nil && *attr.AttrPrimary == handleID {
			indexes = append(indexes, i)
		}
	}
	return indexes
}

func (b *allocationBlock) releaseByHandle(handleID string) int {
	attrIndexes := b.attributeIndexesByHandle(handleID)
	log.Debugf("Attribute indexes to release: %v", attrIndexes)
	if len(attrIndexes) == 0 {
		// Nothing to release.
		log.Debugf("No addresses assigned to handle '%s'", handleID)
		return 0
	}

	// There are addresses to release.
	ordinals := []int{}
	var o int
	for o = 0; o < blockSize; o++ {
		// Only check allocated ordinals.
		if b.Allocations[o] != nil && intInSlice(*b.Allocations[o], attrIndexes) {
			// Release this ordinal.
			ordinals = append(ordinals, o)
		}
	}

	// Clean and reorder attributes.
	b.deleteAttributes(attrIndexes, ordinals)

	// Release the addresses.
	for _, o := range ordinals {
		b.Allocations[o] = nil
		b.Unallocated = append(b.Unallocated, o)
	}
	return len(ordinals)
}

func (b allocationBlock) ipsByHandle(handleID string) []cnet.IP {
	ips := []cnet.IP{}
	attrIndexes := b.attributeIndexesByHandle(handleID)
	var o int
	for o = 0; o < blockSize; o++ {
		if b.Allocations[o] != nil && intInSlice(*b.Allocations[o], attrIndexes) {
			ip := ordinalToIP(o, b)
			ips = append(ips, ip)
		}
	}
	return ips
}

func (b allocationBlock) attributesForIP(ip cnet.IP) (map[string]string, error) {
	// Convert to an ordinal.
	ordinal := ipToOrdinal(ip, b)
	if (ordinal < 0) || (ordinal > blockSize) {
		return nil, errors.New(fmt.Sprintf("IP %s not in block %s", ip, b.AllocationBlock.CIDR))
	}

	// Check if allocated.
	attrIndex := b.Allocations[ordinal]
	if attrIndex == nil {
		return nil, errors.New(fmt.Sprintf("IP %s is not currently assigned in block", ip))
	}
	return b.Attributes[*attrIndex].AttrSecondary, nil
}

func (b *allocationBlock) findOrAddAttribute(handleID *string, attrs map[string]string) int {
	attr := model.AllocationAttribute{handleID, attrs}
	for idx, existing := range b.Attributes {
		if reflect.DeepEqual(attr, existing) {
			log.Debugf("Attribute '%+v' already exists", attr)
			return idx
		}
	}

	// Does not exist - add it.
	log.Infof("New allocation attribute: %+v", attr)
	attrIndex := len(b.Attributes)
	b.Attributes = append(b.Attributes, attr)
	return attrIndex
}

func getBlockCIDRForAddress(addr cnet.IP) cnet.IPNet {
	var mask net.IPMask
	if addr.Version() == 6 {
		// This is an IPv6 address.
		mask = ipv6.BlockPrefixMask
	} else {
		// This is an IPv4 address.
		mask = ipv4.BlockPrefixMask
	}
	masked := addr.Mask(mask)
	return cnet.IPNet{net.IPNet{IP: masked, Mask: mask}}
}

func getIPVersion(ip cnet.IP) ipVersion {
	if ip.To4() == nil {
		return ipv6
	}
	return ipv4
}

func largerThanOrEqualToBlock(blockCIDR cnet.IPNet) bool {
	ones, _ := blockCIDR.Mask.Size()
	ipVersion := getIPVersion(cnet.IP{blockCIDR.IP})
	return ones <= ipVersion.BlockPrefixLength
}

func intInSlice(searchInt int, slice []int) bool {
	for _, v := range slice {
		if v == searchInt {
			return true
		}
	}
	return false
}

func ipToInt(ip cnet.IP) *big.Int {
	if ip.To4() != nil {
		return big.NewInt(0).SetBytes(ip.To4())
	} else {
		return big.NewInt(0).SetBytes(ip.To16())
	}
}

func intToIP(ipInt *big.Int) cnet.IP {
	ip := cnet.IP{net.IP(ipInt.Bytes())}
	return ip
}

func incrementIP(ip cnet.IP, increment *big.Int) cnet.IP {
	sum := big.NewInt(0).Add(ipToInt(ip), increment)
	return intToIP(sum)
}

func ipToOrdinal(ip cnet.IP, b allocationBlock) int {
	ip_int := ipToInt(ip)
	base_int := ipToInt(cnet.IP{b.CIDR.IP})
	ord := big.NewInt(0).Sub(ip_int, base_int).Int64()
	if ord < 0 || ord >= blockSize {
		// IP address not in the given block.
		log.Fatalf("IP %s not in block %s", ip, b.CIDR)
	}
	return int(ord)
}

func ordinalToIP(ord int, b allocationBlock) cnet.IP {
	sum := big.NewInt(0).Add(ipToInt(cnet.IP{b.CIDR.IP}), big.NewInt(int64(ord)))
	return intToIP(sum)
}
