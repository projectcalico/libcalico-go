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
	goerrors "errors"
	"fmt"
	"net"
	"reflect"

	"github.com/golang/glog"
	"github.com/tigera/libcalico-go/lib/api"
	"github.com/tigera/libcalico-go/lib/backend/model"
	"github.com/tigera/libcalico-go/lib/errors"
	"github.com/tigera/libcalico-go/lib/types"
)

type blockReaderWriter struct {
	client *Client
}

func (rw blockReaderWriter) getAffineBlocks(host string, ver ipVersion, pool *types.IPNet) ([]types.IPNet, error) {
	// Lookup all blocks by providing an empty BlockListOptions
	// to the List operation.
	opts := model.BlockListOptions{IPVersion: ver.Number}
	datastoreObjs, err := rw.client.backend.List(opts)
	if err != nil {
		if _, ok := err.(errors.ErrorResourceDoesNotExist); ok {
			// The block path does not exist yet.  This is OK - it means
			// there are no affine blocks.
			return []types.IPNet{}, nil

		} else {
			glog.Errorf("Error getting affine blocks: %s", err)
			return nil, err
		}
	}

	// Iterate through and extract the block CIDRs.
	ids := []types.IPNet{}
	for _, o := range datastoreObjs {
		b := o.Value.(model.AllocationBlock)
		ids = append(ids, b.CIDR)
	}
	return ids, nil
}

func (rw blockReaderWriter) claimNewAffineBlock(
	host string, version ipVersion, pool *types.IPNet, config IPAMConfig) (*types.IPNet, error) {

	// If pool is not nil, use the given pool.  Otherwise, default to
	// all configured pools.
	var pools []types.IPNet
	if pool != nil {
		// Validate the given pool is actually configured and matches the version.
		if !rw.isConfiguredPool(*pool) {
			estr := fmt.Sprintf("The given pool (%s) does not exist", pool.String())
			return nil, goerrors.New(estr)
		} else if version.Number != pool.Version() {
			estr := fmt.Sprintf("The given pool (%s) does not match IP version %d", pool.String(), version.Number)
			return nil, goerrors.New(estr)
		}
		pools = []types.IPNet{*pool}
	} else {
		// Default to all configured pools.
		allPools, err := rw.client.Pools().List(api.PoolMetadata{})
		if err != nil {
			glog.Errorf("Error reading configured pools: %s", err)
			return nil, err
		}

		// Grab all the IP networks in these pools.
		for _, p := range allPools.Items {
			// Don't include disabled pools or pools that don't match
			// the requested IP version.
			if !p.Spec.Disabled && version.Number == p.Metadata.CIDR.Version() {
				pools = append(pools, p.Metadata.CIDR)
			}
		}
	}

	// If there are no pools, we cannot assign addresses.
	if len(pools) == 0 {
		return nil, goerrors.New("No configured Calico pools")
	}

	// Iterate through pools to find a new block.
	glog.V(2).Infof("Claiming a new affine block for host '%s'", host)
	for _, pool := range pools {
		// Use a block generator to iterate through all of the blocks
		// that fall within the pool.
		blocks := blockGenerator(pool)
		for subnet := blocks(); subnet != nil; subnet = blocks() {
			// Check if a block already exists for this subnet.
			glog.V(4).Infof("Getting block: %s", subnet.String())
			key := model.BlockKey{CIDR: *subnet}
			_, err := rw.client.backend.Get(key)
			if err != nil {
				if _, ok := err.(errors.ErrorResourceDoesNotExist); ok {
					// The block does not yet exist in etcd.  Try to grab it.
					glog.V(3).Infof("Found free block: %+v", *subnet)
					err = rw.claimBlockAffinity(*subnet, host, config)
					return subnet, err
				} else {
					glog.Errorf("Error getting block: %s", err)
					return nil, err
				}
			}
		}
	}
	return nil, noFreeBlocksError("No Free Blocks")
}

func (rw blockReaderWriter) claimBlockAffinity(subnet types.IPNet, host string, config IPAMConfig) error {
	// Claim the block affinity for this host.
	glog.V(2).Infof("Host %s claiming block affinity for %s", host, subnet)
	obj := model.KVPair{
		Key:   model.BlockAffinityKey{Host: host, CIDR: subnet},
		Value: model.BlockAffinity{},
	}
	_, err := rw.client.backend.Create(&obj)

	// Create the new block.
	block := newBlock(subnet)
	block.HostAffinity = &host
	block.StrictAffinity = config.StrictAffinity

	// Create the new block in the datastore.
	o := model.KVPair{
		Key:   model.BlockKey{block.CIDR},
		Value: block.AllocationBlock,
	}
	_, err = rw.client.backend.Create(&o)
	if err != nil {
		if _, ok := err.(errors.ErrorResourceAlreadyExists); ok {
			// Block already exists, check affinity.
			glog.Warningf("Problem claiming block affinity:", err)
			obj, err := rw.client.backend.Get(model.BlockKey{subnet})
			if err != nil {
				glog.Errorf("Error reading block:", err)
				return err
			}

			// Pull out the allocationBlock object.
			b := allocationBlock{obj.Value.(model.AllocationBlock)}

			if b.HostAffinity != nil && *b.HostAffinity == host {
				// Block has affinity to this host, meaning another
				// process on this host claimed it.
				glog.V(3).Infof("Block %s already claimed by us.  Success", subnet)
				return nil
			}

			// Some other host beat us to this block.  Cleanup and return error.
			err = rw.client.backend.Delete(&model.KVPair{
				Key: model.BlockAffinityKey{Host: host, CIDR: b.CIDR},
			})
			if err != nil {
				glog.Errorf("Error cleaning up block affinity: %s", err)
				return err
			}
			return affinityClaimedError{Block: b}
		} else {
			return err
		}
	}
	return nil
}

func (rw blockReaderWriter) releaseBlockAffinity(host string, blockCIDR types.IPNet) error {
	for i := 0; i < ipamEtcdRetries; i++ {
		// Read the model.KVPair containing the block
		// and pull out the allocationBlock object.  We need to hold on to this
		// so that we can pass it back to the datastore on Update.
		obj, err := rw.client.backend.Get(model.BlockKey{CIDR: blockCIDR})
		if err != nil {
			glog.Errorf("Error getting block %s: %s", blockCIDR.String(), err)
			return err
		}
		b := allocationBlock{obj.Value.(model.AllocationBlock)}

		// Check that the block affinity matches the given affinity.
		if b.HostAffinity != nil && *b.HostAffinity != host {
			glog.Errorf("Mismatched affinity: %s != %s", *b.HostAffinity, host)
			return affinityClaimedError{Block: b}
		}

		if b.empty() {
			// If the block is empty, we can delete it.
			err := rw.client.backend.Delete(&model.KVPair{
				Key: model.BlockKey{CIDR: b.CIDR},
			})
			if err != nil {
				if _, ok := err.(errors.ErrorResourceDoesNotExist); ok {
					// Block already deleted.  Carry on.
				} else {
					glog.Errorf("Error deleting block: %s", err)
					return err
				}
			}
		} else {
			// Otherwise, we need to remove affinity from it.
			// This prevents the host from automatically assigning
			// from this block unless we're allowed to overflow into
			// non-affine blocks.
			b.HostAffinity = nil

			// Pass back the original KVPair with the new
			// block information so we can do a CAS.
			obj.Value = b
			_, err = rw.client.backend.Update(obj)
			if err != nil {
				if _, ok := err.(errors.ErrorResourceUpdateConflict); ok {
					// CASError - continue.
					continue
				} else {
					return err
				}
			}
		}

		// We've removed / updated the block, so update the host config
		// to remove the CIDR.
		err = rw.client.backend.Delete(&model.KVPair{
			Key: model.BlockAffinityKey{Host: host, CIDR: b.CIDR},
		})
		if err != nil {
			if _, ok := err.(errors.ErrorResourceDoesNotExist); ok {
				// Already deleted - carry on.
			} else {
				glog.Errorf("Error deleting block affinity: %s", err)
			}
		}
		return nil

	}
	return goerrors.New("Max retries hit")
}

// withinConfiguredPools returns true if the given IP is within a configured
// Calico pool, and false otherwise.
func (rw blockReaderWriter) withinConfiguredPools(ip types.IP) bool {
	allPools, _ := rw.client.Pools().List(api.PoolMetadata{})
	for _, p := range allPools.Items {
		// Compare any enabled pools.
		if !p.Spec.Disabled && p.Metadata.CIDR.Contains(ip.IP) {
			return true
		}
	}
	return false
}

// isConfiguredPool returns true if the given IPNet is a configured
// Calico pool, and false otherwise.
func (rw blockReaderWriter) isConfiguredPool(cidr types.IPNet) bool {
	allPools, _ := rw.client.Pools().List(api.PoolMetadata{})
	for _, p := range allPools.Items {
		// Compare any enabled pools.
		if !p.Spec.Disabled && reflect.DeepEqual(p.Metadata.CIDR, cidr) {
			return true
		}
	}
	return false
}

// Generator to get list of block CIDRs which
// fall within the given pool. Returns nil when no more
// blocks can be generated.
func blockGenerator(pool types.IPNet) func() *types.IPNet {
	// Determine the IP type to use.
	version := getIPVersion(types.IP{pool.IP})
	ip := types.IP{pool.IP}
	return func() *types.IPNet {
		returnIP := ip
		ip = incrementIP(ip, blockSize)
		if pool.Contains(ip.IP) {
			ipnet := net.IPNet{returnIP.IP, version.BlockPrefixMask}
			cidr := types.IPNet{ipnet}
			return &cidr
		} else {
			return nil
		}
	}
}
