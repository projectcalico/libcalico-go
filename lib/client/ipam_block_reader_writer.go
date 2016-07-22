package client

import (
	"errors"
	"fmt"
	"net"

	"github.com/golang/glog"
	"github.com/tigera/libcalico-go/lib/api"
	"github.com/tigera/libcalico-go/lib/backend"
	"github.com/tigera/libcalico-go/lib/common"
)

type blockReaderWriter struct {
	client *Client
}

func (rw blockReaderWriter) getAffineBlocks(host string, ver ipVersion, pool *common.IPNet) ([]common.IPNet, error) {
	// Lookup all blocks by providing an empty BlockListOptions
	// to the List operation.
	opts := backend.BlockListOptions{}
	datastoreObjs, err := rw.client.backend.List(opts)
	if err != nil {
		if _, ok := err.(common.ErrorResourceDoesNotExist); ok {
			// The block path does not exist yet.  This is OK - it means
			// there are no affine blocks.
			return []common.IPNet{}, nil

		} else {
			glog.Errorf("Error getting affine blocks: %s", err)
			return nil, err
		}
	}

	// Iterate through and extract the block CIDRs.
	ids := []common.IPNet{}
	for _, o := range datastoreObjs {
		b, _ := o.Object.(backend.AllocationBlock)
		ids = append(ids, b.CIDR)
	}
	return ids, nil
}

func (rw blockReaderWriter) claimNewAffineBlock(
	host string, version ipVersion, pool *common.IPNet, config IPAMConfig) (*common.IPNet, error) {

	// If pool is not nil, use the given pool.  Otherwise, default to
	// all configured pools.
	var pools []common.IPNet
	if pool != nil {
		// Validate the given pool is actually configured.
		// TODO: Exact match pools check.
		if !rw.withinConfiguredPools(common.IP{pool.IP}) {
			estr := fmt.Sprintf("The given pool (%s) does not exist", pool.String())
			return nil, errors.New(estr)
		}
		pools = []common.IPNet{*pool}
	} else {
		// Default to all configured pools.
		// TODO: Re-implement this.
		allPools, err := rw.client.Pools().List(api.PoolMetadata{})
		if err != nil {
			// TODO: handle error.
			return nil, err
		}

		// Grab all the IP networks in these pools.
		for _, p := range allPools.Items {
			pools = append(pools, p.Metadata.CIDR)
		}
	}

	// If there are no pools, we cannot assign addresses.
	if len(pools) == 0 {
		return nil, errors.New("No configured Calico pools")
	}

	// Iterate through pools to find a new block.
	glog.V(2).Infof("Claiming a new affine block for host '%s'", host)
	for _, pool := range pools {
		for _, subnet := range blocks(pool) {
			// Check if a block already exists for this subnet.
			key := backend.BlockKey{CIDR: subnet}
			_, err := rw.client.backend.Get(key)
			if err != nil {
				if _, ok := err.(common.ErrorResourceDoesNotExist); ok {
					// The block does not yet exist in etcd.  Try to grab it.
					glog.V(3).Infof("Found free block: %+v", subnet)
					err = rw.claimBlockAffinity(subnet, host, config)
					return &subnet, err
				} else {
					glog.Errorf("Error getting block: %s", err)
					return nil, err
				}
			}
		}
	}
	return nil, noFreeBlocksError("No Free Blocks")
}

func (rw blockReaderWriter) claimBlockAffinity(subnet common.IPNet, host string, config IPAMConfig) error {
	// Claim the block affinity for this host.
	glog.V(2).Infof("Host %s claiming block affinity for %s", host, subnet)
	obj := backend.DatastoreObject{
		Key:    backend.BlockAffinityKey{Host: host, CIDR: subnet},
		Object: backend.BlockAffinity{},
	}
	_, err := rw.client.backend.Create(&obj)

	// Create the new block.
	block := newBlock(subnet)
	block.HostAffinity = &host
	block.StrictAffinity = config.StrictAffinity

	// Create the new block in the datastore.
	o := backend.DatastoreObject{
		Key:    backend.BlockKey{block.CIDR},
		Object: block.AllocationBlock,
	}
	_, err = rw.client.backend.Create(&o)
	if err != nil {
		if _, ok := err.(casError); ok {
			// Block already exists, check affinity.
			glog.Warningf("Problem claiming block affinity:", err)
			o, err := rw.client.backend.Get(backend.BlockKey{subnet})
			if err != nil {
				glog.Errorf("Error reading block:", err)
				return err
			}

			// Pull out the block object.
			bb, _ := o.Object.(backend.AllocationBlock)
			b := allocationBlock{bb}

			if b.HostAffinity != nil && *b.HostAffinity == host {
				// Block has affinity to this host, meaning another
				// process on this host claimed it.
				glog.V(3).Infof("Block %s already claimed by us.  Success", subnet)
				return nil
			}

			// Some other host beat us to this block.  Cleanup and return error.
			err = rw.client.backend.Delete(backend.BlockAffinityKey{Host: host, CIDR: b.CIDR})
			if err != nil {
				// TODO: Handle this error.
				return err
			}

			return affinityClaimedError{Block: b}
		} else {
			return err
		}
	}
	return nil
}

func (rw blockReaderWriter) releaseBlockAffinity(host string, blockCidr common.IPNet) error {
	for i := 0; i < ipamEtcdRetries; i++ {
		// Read the backend.DatastoreObject containing the block
		// and pull out the allocationBlock object.  We need to hold on to this
		// so that we can pass it back to the datastore on Update.
		o, err := rw.client.backend.Get(backend.BlockKey{CIDR: blockCidr})
		if err != nil {
			// TODO: Better error handling here.
			return err
		}
		bb, _ := o.Object.(backend.AllocationBlock)
		b := allocationBlock{bb}

		// Check that the block affinity matches the given affinity.
		if b.HostAffinity != nil && *b.HostAffinity != host {
			// TODO: Handle this error.
			glog.Errorf("Mismatched affinity: %s != %s", *b.HostAffinity, host)
			// return affinityClaimedError{Block: *b}
		}

		if b.empty() {
			// If the block is empty, we can delete it.
			err := rw.client.backend.Delete(backend.BlockKey{CIDR: b.CIDR})
			if err != nil {
				if err != nil {
					// Block already deleted.  Carry on.
					// TODO: Check type of error to make sure this is true!

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

			// Pass back the original DatastoreObject with the new
			// block information so we can do a CAS.
			o.Object = b
			_, err = rw.client.backend.Update(o)
			if err != nil {
				if _, ok := err.(casError); ok {
					// CASError - continue.
					continue
				} else {
					return err
				}
			}
		}

		// We've removed / updated the block, so update the host config
		// to remove the CIDR.
		err = rw.client.backend.Delete(backend.BlockAffinityKey{Host: host, CIDR: b.CIDR})
		// TODO: Handle errors.
		// if err != nil {
		// 	if eerr, ok := err.(client.Error); ok && eerr.Code == client.ErrorCodeNodeExist {
		// 		// Already deleted.  Carry on.

		// 	} else {
		// 		return err
		// 	}
		// }
		return nil

	}
	return errors.New("Max retries hit")
}

func (rw blockReaderWriter) withinConfiguredPools(ip common.IP) bool {
	// TODO: Re-implement this.
	// ver := getIPVersion(ip)
	// allPools := libcalico.GetPools(rw.etcd, fmt.Sprintf("%d", ver.Number))
	// for _, p := range allPools {
	// 	_, c, _ := net.ParseCIDR(p.Cidr)
	// 	if c.Contains(ip) {
	// 		return true
	// 	}
	// }
	// return false
	return true
}

// Return the list of block CIDRs which fall within
// the given pool.
func blocks(pool common.IPNet) []common.IPNet {
	// Determine the IP type to use.
	ipVersion := getIPVersion(common.IP{pool.IP})
	nets := []common.IPNet{}
	ip := common.IP{pool.IP}
	for pool.Contains(ip.IP) {
		netIP := net.IPNet{ip.IP, ipVersion.BlockPrefixMask}
		nets = append(nets, common.IPNet{netIP})
		ip = incrementIP(ip, blockSize)
	}
	return nets
}
