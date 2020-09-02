// Copyright (c) 2016-2020 Tigera, Inc. All rights reserved.

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

package ipam

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/projectcalico/libcalico-go/lib/apiconfig"
	v3 "github.com/projectcalico/libcalico-go/lib/apis/v3"
	"github.com/projectcalico/libcalico-go/lib/backend"
	bapi "github.com/projectcalico/libcalico-go/lib/backend/api"
	"github.com/projectcalico/libcalico-go/lib/backend/k8s"
	"github.com/projectcalico/libcalico-go/lib/backend/model"
	cerrors "github.com/projectcalico/libcalico-go/lib/errors"
	cnet "github.com/projectcalico/libcalico-go/lib/net"
	"github.com/projectcalico/libcalico-go/lib/testutils"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// Implement an IP pools accessor for the IPAM client.  This is a "mock" version
// of the accessor that we populate directly, rather than requiring the pool
// data to be persisted in etcd.
type ipPoolAccessor struct {
	pools map[string]pool
}

type pool struct {
	cidr         string
	blockSize    int
	enabled      bool
	nodeSelector string
}

func (i *ipPoolAccessor) GetEnabledPools(ipVersion int) ([]v3.IPPool, error) {
	sorted := make([]string, 0)
	// Get a sorted list of enabled pool CIDR strings.
	for p, e := range i.pools {
		if e.enabled {
			sorted = append(sorted, p)
		}
	}
	return i.getPools(sorted, ipVersion, "GetEnabledPools"), nil
}

func (i *ipPoolAccessor) getPools(sorted []string, ipVersion int, caller string) []v3.IPPool {
	sort.Strings(sorted)

	// Convert to IPNets and sort out the correct IP versions.  Sorting the results
	// mimics more closely the behavior of etcd and allows the tests to be
	// deterministic.
	pools := make([]v3.IPPool, 0)
	for _, p := range sorted {
		c := cnet.MustParseCIDR(p)
		if (ipVersion == 0) || (c.Version() == ipVersion) {
			pool := v3.IPPool{Spec: v3.IPPoolSpec{CIDR: p, NodeSelector: i.pools[p].nodeSelector}}
			if i.pools[p].blockSize == 0 {
				if ipVersion == 4 {
					pool.Spec.BlockSize = 26
				} else {
					pool.Spec.BlockSize = 122
				}

			} else {
				pool.Spec.BlockSize = i.pools[p].blockSize
			}
			pools = append(pools, pool)
		}
	}

	log.Infof("%v returns: %v", caller, pools)

	return pools
}

func (i *ipPoolAccessor) GetAllPools() ([]v3.IPPool, error) {
	sorted := make([]string, 0)
	// Get a sorted list of pool CIDR strings.
	for p := range i.pools {
		sorted = append(sorted, p)
	}
	return i.getPools(sorted, 0, "GetAllPools"), nil
}

var (
	ipPools = &ipPoolAccessor{pools: map[string]pool{}}
)

type testArgsClaimAff struct {
	inNet, host                 string
	cleanEnv                    bool
	pool                        []string
	assignIP                    net.IP
	expClaimedIPs, expFailedIPs int
	expError                    error
}

var _ = testutils.E2eDatastoreDescribe("IPAM tests", testutils.DatastoreAll, func(config apiconfig.CalicoAPIConfig) {
	// Create a new backend client and an IPAM Client using the IP Pools Accessor.
	// Tests that need to ensure a clean datastore should invoke Clean() on the datastore at the start of the
	// tests.
	var bc bapi.Client
	var ic Interface
	var kc *kubernetes.Clientset
	BeforeEach(func() {
		var err error
		bc, err = backend.NewClient(config)
		Expect(err).NotTo(HaveOccurred())
		ic = NewIPAMClient(bc, ipPools)

		// If running in KDD mode, extract the k8s clientset.
		if config.Spec.DatastoreType == "kubernetes" {
			kc = bc.(*k8s.KubeClient).ClientSet
		}
	})

	Context("Measuring allocation performance", func() {
		var pa *ipPoolAccessor
		var hostname string
		var err error
		var pool20, pool32, pool26 []cnet.IPNet

		BeforeEach(func() {
			// Remove all data in the datastore.
			bc, err = backend.NewClient(config)
			Expect(err).NotTo(HaveOccurred())
			bc.Clean()

			// Build a new pool accessor for these tests.
			pa = &ipPoolAccessor{pools: map[string]pool{}}

			// Create many pools
			for i := 0; i < 100; i++ {
				cidr := fmt.Sprintf("10.%d.0.0/16", i)
				pa.pools[cidr] = pool{enabled: true, blockSize: 26}
				pool26 = append(pool26, cnet.MustParseCIDR(cidr))
			}

			for i := 0; i < 100; i++ {
				cidr := fmt.Sprintf("11.%d.0.0/16", i)
				pa.pools[cidr] = pool{enabled: true, blockSize: 32}
				pool32 = append(pool32, cnet.MustParseCIDR(cidr))
			}

			for i := 0; i < 50; i++ {
				cidr := fmt.Sprintf("12.%d.0.0/16", i)
				pa.pools[cidr] = pool{enabled: true, blockSize: 20}
				pool20 = append(pool20, cnet.MustParseCIDR(cidr))
			}

			// Create the node object.
			hostname = "host-perf"
			applyNode(bc, kc, hostname, map[string]string{"foo": "bar"})
		})

		AfterEach(func() {
			deleteNode(bc, kc, hostname)
		})

		Measure("It should be able to allocate a single address quickly - blocksize 32", func(b Benchmarker) {
			runtime := b.Time("runtime", func() {
				// Build a new backend client. We use a different client for each iteration of the test
				// so that the k8s QPS /burst limits don't carry across tests. This is more realistic.
				bc, err = backend.NewClient(config)
				Expect(err).NotTo(HaveOccurred())
				ic = NewIPAMClient(bc, pa)

				v4, _, outErr := ic.AutoAssign(context.Background(), AutoAssignArgs{Num4: 1, IPv4Pools: pool32, Hostname: hostname})
				Expect(outErr).NotTo(HaveOccurred())
				Expect(len(v4)).To(Equal(1))
			})

			Expect(runtime.Seconds()).Should(BeNumerically("<", 1))
		}, 100)

		Measure("It should be able to allocate a single address quickly - blocksize 26", func(b Benchmarker) {
			runtime := b.Time("runtime", func() {
				// Build a new backend client. We use a different client for each iteration of the test
				// so that the k8s QPS /burst limits don't carry across tests. This is more realistic.
				bc, err = backend.NewClient(config)
				Expect(err).NotTo(HaveOccurred())
				ic = NewIPAMClient(bc, pa)

				v4, _, outErr := ic.AutoAssign(context.Background(), AutoAssignArgs{Num4: 1, IPv4Pools: pool26, Hostname: hostname})
				Expect(outErr).NotTo(HaveOccurred())
				Expect(len(v4)).To(Equal(1))
			})

			Expect(runtime.Seconds()).Should(BeNumerically("<", 1))
		}, 100)

		Measure("It should be able to allocate a single address quickly - blocksize 20", func(b Benchmarker) {
			runtime := b.Time("runtime", func() {
				// Build a new backend client. We use a different client for each iteration of the test
				// so that the k8s QPS /burst limits don't carry across tests. This is more realistic.
				bc, err = backend.NewClient(config)
				Expect(err).NotTo(HaveOccurred())
				ic = NewIPAMClient(bc, pa)

				v4, _, outErr := ic.AutoAssign(context.Background(), AutoAssignArgs{Num4: 1, IPv4Pools: pool20, Hostname: hostname})
				Expect(outErr).NotTo(HaveOccurred())
				Expect(len(v4)).To(Equal(1))
			})

			Expect(runtime.Seconds()).Should(BeNumerically("<", 1))
		}, 100)

		Measure("It should be able to allocate a lot of addresses quickly", func(b Benchmarker) {
			runtime := b.Time("runtime", func() {
				// Build a new backend client. We use a different client for each iteration of the test
				// so that the k8s QPS /burst limits don't carry across tests. This is more realistic.
				bc, err = backend.NewClient(config)
				Expect(err).NotTo(HaveOccurred())
				ic = NewIPAMClient(bc, pa)

				v4, _, outErr := ic.AutoAssign(context.Background(), AutoAssignArgs{Num4: 64, IPv4Pools: pool20, Hostname: hostname})
				Expect(outErr).NotTo(HaveOccurred())
				Expect(len(v4)).To(Equal(64))
			})

			Expect(runtime.Seconds()).Should(BeNumerically("<", 1))
		}, 20)

		Measure("It should be able to allocate and release addresses quickly", func(b Benchmarker) {
			runtime := b.Time("runtime", func() {
				// Build a new backend client. We use a different client for each iteration of the test
				// so that the k8s QPS /burst limits don't carry across tests. This is more realistic.
				bc, err = backend.NewClient(config)
				Expect(err).NotTo(HaveOccurred())
				ic = NewIPAMClient(bc, pa)

				v4, _, outErr := ic.AutoAssign(context.Background(), AutoAssignArgs{Num4: 1, Hostname: hostname})
				v4IP := make([]cnet.IP, 0, 0)
				for _, ipNets := range v4 {
					IP, _, _ := cnet.ParseCIDR(ipNets.String())
					v4IP = append(v4IP, *IP)
				}
				Expect(outErr).NotTo(HaveOccurred())
				Expect(len(v4IP)).To(Equal(1))
				v4IP, outErr = ic.ReleaseIPs(context.Background(), v4IP)
				Expect(outErr).NotTo(HaveOccurred())
				Expect(len(v4IP)).To(Equal(0))
			})

			Expect(runtime.Seconds()).Should(BeNumerically("<", 1))
		}, 20)
	})

	Describe("RemoveIPAMHost tests", func() {
		It("should succeed if the host already doesn't exist", func() {
			err := ic.RemoveIPAMHost(context.Background(), "randomhost")
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("Allocation attributes tests", func() {
		var hostname string

		sentinelIP := net.ParseIP("10.0.0.1")

		It("Should return ResourceNotExist on no valid pool", func() {
			attrs, handle, err := ic.GetAssignmentAttributes(context.Background(), cnet.IP{sentinelIP})
			Expect(err).To(BeAssignableToTypeOf(cerrors.ErrorResourceDoesNotExist{}))
			Expect(attrs).To(BeEmpty())
			Expect(handle).To(BeNil())
		})

		Context("With valid pool", func() {
			BeforeEach(func() {
				// Remove all data in the datastore.
				bc.Clean()

				// Create an IP pool
				applyPool("10.0.0.0/24", true, "all()")

				// Create the node object.
				hostname = "allocation-attributes"
				applyNode(bc, kc, hostname, nil)
			})

			AfterEach(func() {
				bc.Clean()
			})

			It("Should return ResourceNotExist error on no block", func() {
				attrs, handle, err := ic.GetAssignmentAttributes(context.Background(), cnet.IP{sentinelIP})
				Expect(err).To(BeAssignableToTypeOf(cerrors.ErrorResourceDoesNotExist{}))
				Expect(attrs).To(BeEmpty())
				Expect(handle).To(BeNil())
			})

			It("Should return correct attributes on allocated ip", func() {
				handle := "my-test-handle"
				ipAttr := map[string]string{
					AttributeNode: hostname,
					AttributeType: AttributeTypeVXLAN,
				}
				args := AssignIPArgs{
					IP:       cnet.IP{sentinelIP},
					Hostname: hostname,
					Attrs:    ipAttr,
					HandleID: &handle,
				}
				err := ic.AssignIP(context.Background(), args)
				Expect(err).NotTo(HaveOccurred())

				attrs, returnedHandle, err := ic.GetAssignmentAttributes(context.Background(), cnet.IP{sentinelIP})
				Expect(err).NotTo(HaveOccurred())
				Expect(attrs).To(Equal(ipAttr))
				Expect(returnedHandle).NotTo(BeNil())
				Expect(*returnedHandle).To(Equal(handle))
			})

			It("Should return ResourceNotExist on unallocated ip", func() {
				// Allocate an ip in same block
				ipAttr := map[string]string{
					AttributeNode: hostname,
					AttributeType: AttributeTypeVXLAN,
				}
				args := AssignIPArgs{
					IP:       cnet.IP{net.ParseIP("10.0.0.2")},
					Hostname: hostname,
					Attrs:    ipAttr,
				}
				err := ic.AssignIP(context.Background(), args)
				Expect(err).NotTo(HaveOccurred())

				// Block exists but sentinel ip is not allocated.
				attrs, handle, err := ic.GetAssignmentAttributes(context.Background(), cnet.IP{sentinelIP})
				Expect(err).To(BeAssignableToTypeOf(cerrors.ErrorResourceDoesNotExist{}))
				Expect(attrs).To(BeEmpty())
				Expect(handle).To(BeNil())
			})
		})
	})

	Describe("Affinity FV tests", func() {
		var err error
		var hostname string
		BeforeEach(func() {
			// Remove all data in the datastore.
			bc, err = backend.NewClient(config)
			Expect(err).NotTo(HaveOccurred())
			bc.Clean()

			// Create an IP pool
			applyPoolWithBlockSize("10.0.0.0/24", true, "all()", 30)

			// Create the node object.
			hostname = "host-affinity-fvs"
			applyNode(bc, kc, hostname, nil)
		})

		It("should only release empty blocks", func() {
			// Allocate an IP address in a block.
			handle := "test-handle"
			v4, _, err := ic.AutoAssign(context.Background(), AutoAssignArgs{Num4: 1, Hostname: hostname, HandleID: &handle})
			Expect(err).NotTo(HaveOccurred())
			Expect(len(v4)).To(Equal(1))

			// Get the block that was allocated. It should have an affinity matching the host.
			blocks, err := bc.List(context.Background(), model.BlockListOptions{}, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(len(blocks.KVPairs)).To(Equal(1))
			Expect(*blocks.KVPairs[0].Value.(*model.AllocationBlock).Affinity).To(Equal(fmt.Sprintf("host:%s", hostname)))

			// Try to release the block's affinity, requiring it to be empty. It should fail.
			err = ic.ReleaseAffinity(context.Background(), blocks.KVPairs[0].Value.(*model.AllocationBlock).CIDR, hostname, true)
			Expect(err).To(HaveOccurred())

			// The block should still have an affinity to the host.
			blocks, err = bc.List(context.Background(), model.BlockListOptions{}, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(len(blocks.KVPairs)).To(Equal(1))
			Expect(*blocks.KVPairs[0].Value.(*model.AllocationBlock).Affinity).To(Equal(fmt.Sprintf("host:%s", hostname)))

			// Release the IP.
			err = ic.ReleaseByHandle(context.Background(), handle)
			Expect(err).NotTo(HaveOccurred())

			// Try to release the block's affinity, requiring it to be empty. This time, the block is empty
			// and it should succeed.
			err = ic.ReleaseAffinity(context.Background(), blocks.KVPairs[0].Value.(*model.AllocationBlock).CIDR, hostname, true)
			Expect(err).NotTo(HaveOccurred())

			// Releasing the block affinity should have deleted it.
			blocks, err = bc.List(context.Background(), model.BlockListOptions{}, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(len(blocks.KVPairs)).To(Equal(0))
		})

		It("should release all non-empty blocks if there are multiple", func() {
			// Allocate several blocks to the node. The pool is a /30, so 4 addresses
			// per each block.
			handle := "test-handle"
			for i := 0; i < 12; i++ {
				v4, _, err := ic.AutoAssign(context.Background(), AutoAssignArgs{Num4: 1, Hostname: hostname, HandleID: &handle})
				Expect(err).NotTo(HaveOccurred())
				Expect(len(v4)).To(Equal(1))
			}

			// Release them all, leaving just the empty blocks.
			err := ic.ReleaseByHandle(context.Background(), handle)
			Expect(err).NotTo(HaveOccurred())

			// Expect three empty blocks.
			blocks, err := bc.List(context.Background(), model.BlockListOptions{}, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(len(blocks.KVPairs)).To(Equal(3))

			// Allocate a single address, which will make one of the blocks non empty.
			v4, _, err := ic.AutoAssign(context.Background(), AutoAssignArgs{Num4: 1, Hostname: hostname, HandleID: &handle})
			Expect(err).NotTo(HaveOccurred())
			Expect(len(v4)).To(Equal(1))

			// Release host affinities. It should clean up the two empty blocks, but leave the block with an address allocated.
			// It should return an error because it cannot release all three.
			err = ic.ReleaseHostAffinities(context.Background(), hostname, true)
			Expect(err).To(HaveOccurred())

			// Expect one remaining block.
			blocks, err = bc.List(context.Background(), model.BlockListOptions{}, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(len(blocks.KVPairs)).To(Equal(1))
		})
	})

	Describe("IPAM ReleaseIPs with duplicates in the request should be safe", func() {
		host := "host-a"
		pool1 := cnet.MustParseNetwork("10.0.0.0/26")
		// Single out an IP to attempt a double-release and double-assign with
		sentinelIP := net.ParseIP("10.0.0.1")

		It("Should setup a pool with no free addresses", func() {
			bc.Clean()
			deleteAllPools()

			applyNode(bc, kc, host, nil)
			applyPool("10.0.0.0/26", true, "")

			args := AutoAssignArgs{
				Num4:      64,
				Hostname:  host,
				IPv4Pools: []cnet.IPNet{pool1},
			}
			ips, _, bulkAssignErr := ic.AutoAssign(context.Background(), args)

			Expect(bulkAssignErr).NotTo(HaveOccurred())
			Expect(len(ips)).To(Equal(64))
		})

		It("Should release with sentinel IP duplicated in the request args", func() {
			// Releasing the same IP multiple times in a single request
			// should be handled gracefully by the IPAM Block allocator
			_, releaseErr := ic.ReleaseIPs(context.Background(), []cnet.IP{
				cnet.IP{sentinelIP},
				cnet.IP{sentinelIP},
			})
			Expect(releaseErr).NotTo(HaveOccurred())
		})

		It("Should be able to re-assign the sentinel IP", func() {
			assignIPutil(ic, sentinelIP, host)
			attrs, handle, attrErr := ic.GetAssignmentAttributes(context.Background(), cnet.IP{sentinelIP})
			Expect(attrErr).NotTo(HaveOccurred())
			Expect(attrs).To(BeEmpty())
			Expect(handle).To(BeNil())
		})

		It("Should fail to assign any more addresses", func() {
			args := AutoAssignArgs{
				Num4:      1,
				Hostname:  host,
				IPv4Pools: []cnet.IPNet{pool1},
			}
			ips, _, err := ic.AutoAssign(context.Background(), args)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(ips)).To(Equal(0), "An IP has been assigned twice!")
		})
	})

	// We're assigning one IP which should be from the only ipPool created at the time, second one
	// should be from the same /26 block since they're both from the same host, then delete
	// the ipPool and create a new ipPool, and AutoAssign 1 more IP for the same host - expect the
	// assigned IP to be from the new ipPool that was created, this is to make sure the assigned IP
	// doesn't come from the old affinedBlock even after the ipPool was deleted.
	Describe("IPAM AutoAssign from the default pool then delete the pool and assign again", func() {
		hostA := "host-a"
		hostB := "host-b"
		pool1 := cnet.MustParseNetwork("10.0.0.0/24")
		pool2 := cnet.MustParseNetwork("20.0.0.0/24")
		var block cnet.IPNet

		Context("AutoAssign a single IP without specifying a pool", func() {

			It("should auto-assign from the only available pool", func() {
				bc.Clean()
				deleteAllPools()

				err := applyNode(bc, kc, hostA, nil)
				Expect(err).NotTo(HaveOccurred())
				err = applyNode(bc, kc, hostB, nil)
				Expect(err).NotTo(HaveOccurred())
				applyPool("10.0.0.0/24", true, "")

				args := AutoAssignArgs{
					Num4:     1,
					Num6:     0,
					Hostname: hostA,
				}

				v4, _, outErr := ic.AutoAssign(context.Background(), args)

				blocks := getAffineBlocks(bc, hostA)
				for _, b := range blocks {
					if pool1.Contains(b.IPNet.IP) {
						block = b
					}
				}
				Expect(outErr).NotTo(HaveOccurred())
				Expect(pool1.IPNet.Contains(v4[0].IP)).To(BeTrue())
			})

			It("should auto-assign another IP from the same pool into the same allocation block", func() {
				args := AutoAssignArgs{
					Num4:     1,
					Num6:     0,
					Hostname: hostA,
				}

				v4, _, outErr := ic.AutoAssign(context.Background(), args)
				Expect(outErr).NotTo(HaveOccurred())
				Expect(block.IPNet.Contains(v4[0].IP)).To(BeTrue())
			})

			It("should assign from a new pool for a new host (old pool is removed)", func() {
				deleteAllPools()
				applyPool("20.0.0.0/24", true, "")

				p, _ := ipPools.GetEnabledPools(4)
				Expect(len(p)).To(Equal(1))
				Expect(p[0].Spec.CIDR).To(Equal(pool2.String()))
				p, _ = ipPools.GetEnabledPools(6)
				Expect(len(p)).To(BeZero())

				args := AutoAssignArgs{
					Num4:     1,
					Num6:     0,
					Hostname: hostB,
				}
				v4, _, outErr := ic.AutoAssign(context.Background(), args)
				Expect(outErr).NotTo(HaveOccurred())
				Expect(pool2.IPNet.Contains(v4[0].IP)).To(BeTrue())
			})

			It("should not assign from an existing affine block for the first host since the pool is removed)", func() {
				args := AutoAssignArgs{
					Num4:     1,
					Num6:     0,
					Hostname: hostA,
				}
				v4, _, outErr := ic.AutoAssign(context.Background(), args)
				Expect(outErr).NotTo(HaveOccurred())
				Expect(pool2.IPNet.Contains(v4[0].IP)).To(BeTrue())
			})
		})
	})

	Describe("IPAM handle tests", func() {
		It("should support querying and releasing an IP address by handle", func() {
			By("creating a node", func() {
				applyNode(bc, kc, "test-host", nil)
			})

			By("setting up an IP pool", func() {
				deleteAllPools()
				applyPool("10.0.0.0/24", true, "")
			})

			handle := "test-handle"
			ctx := context.Background()

			By("Querying the IP by handle and expecting none", func() {
				_, err := ic.IPsByHandle(ctx, handle)
				Expect(err).To(HaveOccurred())
			})

			By("Assigning an IP address", func() {
				args := AutoAssignArgs{Num4: 1, HandleID: &handle, Hostname: "test-host"}
				v4, _, err := ic.AutoAssign(context.Background(), args)
				Expect(err).NotTo(HaveOccurred())
				Expect(len(v4)).To(Equal(1))
			})

			By("Querying the IP by handle and expecting one", func() {
				ips, err := ic.IPsByHandle(ctx, handle)
				Expect(err).NotTo(HaveOccurred())
				Expect(len(ips)).To(Equal(1))
			})

			By("Releasing the IP address using its handle", func() {
				err := ic.ReleaseByHandle(ctx, handle)
				Expect(err).NotTo(HaveOccurred())
			})

			By("Querying the IP by handle and expecting none", func() {
				_, err := ic.IPsByHandle(ctx, handle)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("IPAM IP borrowing", func() {
		node1 := "host1"
		node2 := "host2"

		It("should respect IPAMConfig.StrictAffinity when it is changed", func() {
			ctx := context.Background()

			bc.Clean()
			deleteAllPools()

			err := applyNode(bc, kc, node1, map[string]string{"foo": "bar"})
			Expect(err).NotTo(HaveOccurred())

			err = applyNode(bc, kc, node2, map[string]string{"foo": "bar"})
			Expect(err).NotTo(HaveOccurred())

			// Only one block can be created out of the pool.
			// When StrictAffinity is false, both nodes will be able to assign IP addresses
			// When StrictAffinity is true, only one node, the ones that gets the block, will
			// be able to assign IP address

			// StrictAffinity is false
			cfg := IPAMConfig{AutoAllocateBlocks: true, StrictAffinity: false}
			err = ic.SetIPAMConfig(ctx, cfg)
			Expect(err).NotTo(HaveOccurred())

			applyPoolWithBlockSize("10.0.0.0/28", true, `foo == "bar"`, 28)

			v4Node0, _, errNode0 := ic.AutoAssign(context.Background(), AutoAssignArgs{
				Num4:     1,
				Num6:     0,
				Hostname: node1,
			})
			Expect(errNode0).ToNot(HaveOccurred())
			Expect(len(v4Node0)).To(Equal(1))

			v4Node1, _, errNode1 := ic.AutoAssign(context.Background(), AutoAssignArgs{
				Num4:     1,
				Num6:     0,
				Hostname: node2,
			})
			Expect(errNode1).ToNot(HaveOccurred())
			Expect(len(v4Node1)).To(Equal(1))

			// StrictAffinity is true
			cfg = IPAMConfig{AutoAllocateBlocks: true, StrictAffinity: true}
			err = ic.SetIPAMConfig(ctx, cfg)
			Expect(err).NotTo(HaveOccurred())

			v4Node0, _, errNode0 = ic.AutoAssign(context.Background(), AutoAssignArgs{
				Num4:     1,
				Num6:     0,
				Hostname: node1,
			})

			v4Node1, _, errNode1 = ic.AutoAssign(context.Background(), AutoAssignArgs{
				Num4:     1,
				Num6:     0,
				Hostname: node2,
			})

			if errNode0 != nil {
				Expect(errNode1).NotTo(HaveOccurred())
				Expect(len(v4Node1)).To(Equal(1))
				Expect(len(v4Node0)).To(Equal(0))
			} else {
				Expect(errNode0).NotTo(HaveOccurred())
				Expect(len(v4Node0)).To(Equal(1))
				Expect(len(v4Node1)).To(Equal(0))
			}

			// StrictAffinity is false
			cfg = IPAMConfig{AutoAllocateBlocks: true, StrictAffinity: false}
			err = ic.SetIPAMConfig(ctx, cfg)
			Expect(err).NotTo(HaveOccurred())

			v4Node0, _, err = ic.AutoAssign(context.Background(), AutoAssignArgs{
				Num4:     1,
				Num6:     0,
				Hostname: node1,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(len(v4Node0)).To(Equal(1))

			v4Node1, _, err = ic.AutoAssign(context.Background(), AutoAssignArgs{
				Num4:     1,
				Num6:     0,
				Hostname: node2,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(len(v4Node1)).To(Equal(1))
		})

		It("should borrow and release borrowed IPs as normal", func() {
			ctx := context.Background()

			bc.Clean()
			deleteAllPools()

			err := applyNode(bc, kc, node1, map[string]string{"foo": "bar"})
			Expect(err).NotTo(HaveOccurred())

			err = applyNode(bc, kc, node2, map[string]string{"foo": "bar"})
			Expect(err).NotTo(HaveOccurred())

			// Only one block can be created out of the pool.
			// When StrictAffinity is false, both nodes will be able to assign IP addresses
			// When StrictAffinity is true, only one node, the ones that gets the block, will
			// be able to assign IP address

			// StrictAffinity is false
			cfg := IPAMConfig{AutoAllocateBlocks: true, StrictAffinity: false}
			err = ic.SetIPAMConfig(ctx, cfg)
			Expect(err).NotTo(HaveOccurred())

			applyPoolWithBlockSize("10.0.0.0/28", true, `foo == "bar"`, 28)

			handle1 := "handle-1"
			handle2 := "handle-2"
			v4Node0, _, errNode0 := ic.AutoAssign(context.Background(), AutoAssignArgs{
				Num4:     1,
				Num6:     0,
				Hostname: node1,
				HandleID: &handle1,
			})
			Expect(errNode0).ToNot(HaveOccurred())
			Expect(len(v4Node0)).To(Equal(1))

			v4Node1, _, errNode1 := ic.AutoAssign(context.Background(), AutoAssignArgs{
				Num4:     1,
				Num6:     0,
				Hostname: node2,
				HandleID: &handle2,
			})
			Expect(errNode1).ToNot(HaveOccurred())
			Expect(len(v4Node1)).To(Equal(1))

			err = ic.ReleaseByHandle(context.Background(), handle2)
			Expect(err).ToNot(HaveOccurred())

			err = ic.ReleaseByHandle(context.Background(), handle1)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("IPAM AutoAssign from any pool", func() {
		var args AutoAssignArgs
		var longHostname, longHostname2 string

		BeforeEach(func() {
			args = AutoAssignArgs{
				Num4:     1,
				Num6:     0,
				Hostname: "test-host",
			}

			// Build a hostname that is longer than the Kubernetes limit.
			// 12 characters * 21 = 252 characters. Kubernetes max is 253.
			longHostname = strings.Repeat("long--hostna", 21)
			Expect(len(longHostname)).To(BeNumerically("==", 252))
			longHostname2 = fmt.Sprintf("%s-two", longHostname[:4])
			Expect(len(longHostname)).To(BeNumerically("==", 252))

			err := applyNode(bc, kc, args.Hostname, nil)
			Expect(err).NotTo(HaveOccurred())
			err = applyNode(bc, kc, longHostname, nil)
			Expect(err).NotTo(HaveOccurred())
			err = applyNode(bc, kc, longHostname2, nil)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			deleteNode(bc, kc, args.Hostname)
			deleteNode(bc, kc, longHostname)
			deleteNode(bc, kc, longHostname2)
		})

		It("should handle long hostnames", func() {
			deleteAllPools()

			applyPool("10.0.0.0/24", true, "")
			applyPool("fe80:ba:ad:beef::00/120", true, "")
			args.Hostname = longHostname
			args.Num6 = 1

			v4, v6, err := ic.AutoAssign(context.Background(), args)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(v4)).To(Equal(1))
			Expect(len(v6)).To(Equal(1))

			// The block should have an affinity to the host.
			opts := model.BlockAffinityListOptions{Host: longHostname, IPVersion: 6}
			affs, err := bc.List(context.Background(), opts, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(len(affs.KVPairs)).To(Equal(1))
			k := affs.KVPairs[0].Key.(model.BlockAffinityKey)
			Expect(k.Host).To(Equal(longHostname))

			// It should also handle hostnames which are the same after
			// truncation occurs. Perform the same query with a really similar
			// hostname, with only the last few characters changed.
			// In KDD mode, this could be a problem if we don't handle long hostnames correctly.

			args.Hostname = longHostname2
			args.Num6 = 1
			v4, v6, err = ic.AutoAssign(context.Background(), args)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(v4)).To(Equal(1))
			Expect(len(v6)).To(Equal(1))

			// Expect two block affinities.
			opts = model.BlockAffinityListOptions{IPVersion: 6}
			affs, err = bc.List(context.Background(), opts, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(len(affs.KVPairs)).To(Equal(2))

			// The block should be affine to the second host.
			opts = model.BlockAffinityListOptions{Host: longHostname2, IPVersion: 6}
			affs, err = bc.List(context.Background(), opts, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(len(affs.KVPairs)).To(Equal(1))
			k = affs.KVPairs[0].Key.(model.BlockAffinityKey)
			Expect(k.Host).To(Equal(longHostname2))
		})

		// Call once in order to assign an IP address and create a block.
		It("should have assigned an IP address with no error", func() {
			deleteAllPools()

			applyPool("10.0.0.0/24", true, "")
			applyPool("20.0.0.0/24", true, "")

			v4, _, err := ic.AutoAssign(context.Background(), args)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(v4) == 1).To(BeTrue())
		})

		// Call again to trigger an assignment from the newly created block.
		It("should have assigned an IP address with no error", func() {
			v4, _, err := ic.AutoAssign(context.Background(), args)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(v4)).To(Equal(1))
		})
	})

	Describe("IPAM AutoAssign from different pools", func() {
		host := "host-a"
		pool1 := cnet.MustParseNetwork("10.0.0.0/24")
		pool2 := cnet.MustParseNetwork("20.0.0.0/24")
		var block1, block2 cnet.IPNet

		findInUse := func(usage []*PoolUtilization, cidr string, expectedInUse int) bool {
			for _, poolUse := range usage {
				for _, blockUse := range poolUse.Blocks {
					if (blockUse.CIDR.String() == cidr) &&
						(blockUse.Available == blockUse.Capacity-expectedInUse) {
						return true
					}
				}
			}
			return false
		}

		It("should get an IP from pool1 when explicitly requesting from that pool", func() {
			bc.Clean()
			deleteAllPools()

			err := applyNode(bc, kc, host, nil)
			Expect(err).NotTo(HaveOccurred())
			applyPool("10.0.0.0/24", true, "")
			applyPool("20.0.0.0/24", true, "")

			args := AutoAssignArgs{
				Num4:      1,
				Num6:      0,
				Hostname:  host,
				IPv4Pools: []cnet.IPNet{pool1},
			}

			v4, _, outErr := ic.AutoAssign(context.Background(), args)
			blocks := getAffineBlocks(bc, host)
			for _, b := range blocks {
				if pool1.Contains(b.IPNet.IP) {
					block1 = b
				}
			}

			Expect(outErr).NotTo(HaveOccurred())
			Expect(pool1.IPNet.Contains(v4[0].IP)).To(BeTrue())

			usage, err := ic.GetUtilization(context.Background(), GetUtilizationArgs{})
			Expect(err).NotTo(HaveOccurred())
			Expect(findInUse(usage, "10.0.0.0/26", 1)).To(BeTrue())

			usage, err = ic.GetUtilization(context.Background(), GetUtilizationArgs{
				Pools: []string{"20.0.0.0/24"},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(findInUse(usage, "10.0.0.0/26", 1)).To(BeFalse())
		})

		It("should get an IP from pool2 when explicitly requesting from that pool", func() {
			args := AutoAssignArgs{
				Num4:      1,
				Num6:      0,
				Hostname:  host,
				IPv4Pools: []cnet.IPNet{pool2},
			}

			v4, _, outErr := ic.AutoAssign(context.Background(), args)
			blocks := getAffineBlocks(bc, host)
			for _, b := range blocks {
				if pool2.Contains(b.IPNet.IP) {
					block2 = b
				}
			}

			Expect(outErr).NotTo(HaveOccurred())
			Expect(block2.IPNet.Contains(v4[0].IP)).To(BeTrue())

			usage, err := ic.GetUtilization(context.Background(), GetUtilizationArgs{})
			Expect(err).NotTo(HaveOccurred())
			Expect(findInUse(usage, "20.0.0.0/26", 1)).To(BeTrue())

			usage, err = ic.GetUtilization(context.Background(), GetUtilizationArgs{
				Pools: []string{"20.0.0.0/24"},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(findInUse(usage, "20.0.0.0/26", 1)).To(BeTrue())
		})

		It("should get an IP from pool1 in the same allocation block as the first IP from pool1", func() {
			args := AutoAssignArgs{
				Num4:      1,
				Num6:      0,
				Hostname:  host,
				IPv4Pools: []cnet.IPNet{pool1},
			}
			v4, _, outErr := ic.AutoAssign(context.Background(), args)
			Expect(outErr).NotTo(HaveOccurred())
			Expect(block1.IPNet.Contains(v4[0].IP)).To(BeTrue())
		})

		It("should get an IP from pool2 in the same allocation block as the first IP from pool2", func() {
			args := AutoAssignArgs{
				Num4:      1,
				Num6:      0,
				Hostname:  host,
				IPv4Pools: []cnet.IPNet{pool2},
			}

			v4, _, outErr := ic.AutoAssign(context.Background(), args)
			Expect(outErr).NotTo(HaveOccurred())
			Expect(block2.IPNet.Contains(v4[0].IP)).To(BeTrue())
		})

		It("should have strict IP pool affinity", func() {
			// Assign the rest of the addresses in pool2.
			// A /24 has 256 addresses. We've assigned 2 already, so assign 254 more.
			args := AutoAssignArgs{
				Num4:      254,
				Num6:      0,
				Hostname:  host,
				IPv4Pools: []cnet.IPNet{pool2},
			}

			By("allocating the rest of the IPs in the pool", func() {
				v4, _, outErr := ic.AutoAssign(context.Background(), args)
				Expect(outErr).NotTo(HaveOccurred())
				Expect(len(v4)).To(Equal(254))

				// Expect all the IPs to be in pool2.
				for _, a := range v4 {
					Expect(pool2.IPNet.Contains(a.IP)).To(BeTrue(), fmt.Sprintf("%s not in pool %s", a.IP, pool2))
				}
			})

			By("attempting to allocate an IP when there are no more left in the pool", func() {
				args.Num4 = 1
				v4, _, outErr := ic.AutoAssign(context.Background(), args)
				Expect(outErr).NotTo(HaveOccurred())
				Expect(len(v4)).To(Equal(0))
			})
		})
	})

	Describe("IPAM AutoAssign using ip pool node selectors", func() {
		It("should only assign ips from the ip pool whose node selector matches the host's node labels", func() {
			host := "host"
			pool1 := cnet.MustParseNetwork("10.0.0.0/24")
			pool2 := cnet.MustParseNetwork("20.0.0.0/24")

			bc.Clean()
			deleteAllPools()

			applyNode(bc, kc, host, map[string]string{"foo": "bar"})
			applyPool(pool1.String(), true, `foo == "bar"`)
			applyPool(pool2.String(), true, `foo != "bar"`)

			// Attempt to assign 300 ips but only the 256 ips from pool1 should be used.
			v4, _, outErr := ic.AutoAssign(context.Background(), AutoAssignArgs{
				Num4:     300,
				Num6:     0,
				Hostname: host,
			})
			Expect(outErr).NotTo(HaveOccurred())
			Expect(len(v4)).To(Equal(256))

			// Expect all the IPs to be from pool1.
			for _, a := range v4 {
				Expect(pool1.IPNet.Contains(a.IP)).To(BeTrue(), fmt.Sprintf("%s not in pool %s", a.IP, pool1))
			}
		})

		// This test validates the behavior of changing node selectors on IP pools. Specifically, it
		// ensures that when a node used to be selected by an IP pool but is no longer selected, we
		// properly release block affinities so that the block can be reassigned to a node that is
		// actually selected by the IP pool.
		It("should handle changing node selectors and release affinity appropriately (ReleaseIPs)", func() {
			host := "host"
			pool1 := cnet.MustParseNetwork("10.0.0.0/24")

			bc.Clean()
			deleteAllPools()

			applyNode(bc, kc, host, map[string]string{"foo": "bar"})
			applyPool(pool1.String(), true, `foo == "bar"`)

			// Assign three addresses to the node.
			v4nets, _, err := ic.AutoAssign(context.Background(), AutoAssignArgs{
				Num4:     3,
				Num6:     0,
				Hostname: host,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(len(v4nets)).To(Equal(3))

			// Should have one affine block to this host.
			blocks := getAffineBlocks(bc, host)
			Expect(len(blocks)).To(Equal(1))

			// Expect all the IPs to be from pool1.
			var v4IPs []cnet.IP
			for _, a := range v4nets {
				Expect(pool1.IPNet.Contains(a.IP)).To(BeTrue(), fmt.Sprintf("%s not in pool %s", a.IP, pool1))
				v4IPs = append(v4IPs, cnet.IP{a.IP})
			}

			// Release one of the IPs.
			unallocated, err := ic.ReleaseIPs(context.Background(), v4IPs[0:1])
			Expect(len(unallocated)).To(Equal(0))
			Expect(err).NotTo(HaveOccurred())

			// Should still have one affine block to this host.
			blocks = getAffineBlocks(bc, host)
			Expect(len(blocks)).To(Equal(1))

			// Change the selector for the IP pool so that it no longer matches node1.
			applyPool(pool1.String(), true, `foo != "bar"`)

			// Release another one of the IPs.
			unallocated, err = ic.ReleaseIPs(context.Background(), v4IPs[1:2])
			Expect(len(unallocated)).To(Equal(0))
			Expect(err).NotTo(HaveOccurred())

			// The block still have an affinity to this host.
			Expect(len(getAffineBlocks(bc, host))).To(Equal(1))

			// And it should still exist.
			opts := model.BlockListOptions{IPVersion: 4}
			out, err := bc.List(context.Background(), opts, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(len(out.KVPairs)).To(Equal(1))

			// Release the last IP.
			unallocated, err = ic.ReleaseIPs(context.Background(), v4IPs[2:3])
			Expect(len(unallocated)).To(Equal(0))
			Expect(err).NotTo(HaveOccurred())

			// The block now has no affinity, and no IPs, so it should be deleted.
			out, err = bc.List(context.Background(), opts, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(len(out.KVPairs)).To(Equal(0))
		})

		// Same test as above but using the ReleaseByHandle IPAM method
		It("should handle changing node selectors and release affinity appropriately (ReleaseByHandle)", func() {
			host := "host"
			pool1 := cnet.MustParseNetwork("10.0.0.0/24")

			bc.Clean()
			deleteAllPools()

			applyNode(bc, kc, host, map[string]string{"foo": "bar"})
			applyPool(pool1.String(), true, `foo == "bar"`)

			handleID1 := "handle1"
			handleID2 := "handle2"
			handleID3 := "handle3"

			// Assign three addresses to the node.
			v4net1, _, err := ic.AutoAssign(context.Background(), AutoAssignArgs{
				Num4:     1,
				Num6:     0,
				Hostname: host,
				HandleID: &handleID1,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(len(v4net1)).To(Equal(1))

			v4net2, _, err := ic.AutoAssign(context.Background(), AutoAssignArgs{
				Num4:     1,
				Num6:     0,
				Hostname: host,
				HandleID: &handleID2,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(len(v4net2)).To(Equal(1))

			v4net3, _, err := ic.AutoAssign(context.Background(), AutoAssignArgs{
				Num4:     1,
				Num6:     0,
				Hostname: host,
				HandleID: &handleID3,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(len(v4net3)).To(Equal(1))

			// Should have one affine block to this host.
			blocks := getAffineBlocks(bc, host)
			Expect(len(blocks)).To(Equal(1))

			// Expect all the IPs to be from pool1.
			Expect(pool1.IPNet.Contains(v4net1[0].IP)).To(BeTrue(), fmt.Sprintf("%s not in pool %s", v4net1[0].IP, pool1))
			Expect(pool1.IPNet.Contains(v4net2[0].IP)).To(BeTrue(), fmt.Sprintf("%s not in pool %s", v4net2[0].IP, pool1))
			Expect(pool1.IPNet.Contains(v4net3[0].IP)).To(BeTrue(), fmt.Sprintf("%s not in pool %s", v4net3[0].IP, pool1))

			// Release one of the IPs.
			err = ic.ReleaseByHandle(context.Background(), handleID1)
			Expect(err).NotTo(HaveOccurred())

			// Should still have one affine block to this host.
			blocks = getAffineBlocks(bc, host)
			Expect(len(blocks)).To(Equal(1))

			// Change the selector for the IP pool so that it no longer matches node1.
			applyPool(pool1.String(), true, `foo != "bar"`)

			// Release another one of the IPs.
			err = ic.ReleaseByHandle(context.Background(), handleID2)
			Expect(err).NotTo(HaveOccurred())

			// The block still have an affinity to this host.
			Expect(len(getAffineBlocks(bc, host))).To(Equal(1))

			// And it should still exist.
			opts := model.BlockListOptions{IPVersion: 4}
			out, err := bc.List(context.Background(), opts, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(len(out.KVPairs)).To(Equal(1))

			// Release the last IP.
			err = ic.ReleaseByHandle(context.Background(), handleID3)
			Expect(err).NotTo(HaveOccurred())

			// The block now has no affinity, and no IPs, so it should be deleted.
			out, err = bc.List(context.Background(), opts, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(len(out.KVPairs)).To(Equal(0))
		})

		// Tests behavior when there are no more blocks available. For nodes which are selected by an
		// IP pool, addresses should be borrowed from other blocks within the pool. For nodes which are
		// not selected by that IP pool, an error should be returned and addresses should no be borrowed.
		It("should handle changing node selectors between two nodes with no available blocks", func() {
			node1 := "host1"
			node2 := "host2"
			pool1 := cnet.MustParseNetwork("10.0.0.0/30")

			bc.Clean()
			deleteAllPools()

			applyNode(bc, kc, node1, map[string]string{"foo": "bar"})
			applyNode(bc, kc, node2, nil)
			applyPoolWithBlockSize(pool1.String(), true, `foo == "bar"`, 30)

			// Assign 3 of the 4 total addresses to node1.
			v4, _, err := ic.AutoAssign(context.Background(), AutoAssignArgs{
				Num4:     3,
				Num6:     0,
				Hostname: node1,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(len(v4)).To(Equal(3))

			// Should have one affine block to node1.
			blocks := getAffineBlocks(bc, node1)
			Expect(len(blocks)).To(Equal(1))

			// Switch labels so that ip pool selects node2.
			applyNode(bc, kc, node1, nil)
			applyNode(bc, kc, node2, map[string]string{"foo": "bar"})

			// Assign 1 address to node1, expect an error.
			v4, _, err = ic.AutoAssign(context.Background(), AutoAssignArgs{
				Num4:     1,
				Num6:     0,
				Hostname: node1,
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("no configured Calico pools for node host1"))
			Expect(len(v4)).To(Equal(0))

			// Assign 1 address to node2.
			v4, _, err = ic.AutoAssign(context.Background(), AutoAssignArgs{
				Num4:     1,
				Num6:     0,
				Hostname: node2,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(len(v4)).To(Equal(1))

			// The block should still be affine to node 1.
			blocks = getAffineBlocks(bc, node1)
			Expect(len(blocks)).To(Equal(1))

			// The address assigned to node2 should come from the block affine to node1.
			node2IP := v4[0].IP
			Expect(pool1.IPNet.Contains(node2IP)).To(BeTrue(), fmt.Sprintf("%s not in pool %s", node2IP, pool1))
		})

		// Allocates IPs from a pool that has a matching node selector,
		// deallocates them all, deselects the pool from the node,
		// tests that the block affinity should be released.
		It("should release affinity and block when pool is empty and node selector is deselected", func() {
			host := "host"
			pool1 := cnet.MustParseNetwork("10.0.0.0/24")

			bc.Clean()
			deleteAllPools()

			applyNode(bc, kc, host, map[string]string{"foo": "bar"})
			applyPool(pool1.String(), true, `foo == "bar"`)

			// Assign three addresses to the node.
			v4nets, _, err := ic.AutoAssign(context.Background(), AutoAssignArgs{
				Num4:     3,
				Num6:     0,
				Hostname: host,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(len(v4nets)).To(Equal(3))

			// Should have one affine block to this host.
			blocks := getAffineBlocks(bc, host)
			Expect(len(blocks)).To(Equal(1))

			// Expect all the IPs to be from pool1.
			var v4IPs []cnet.IP
			for _, a := range v4nets {
				Expect(pool1.IPNet.Contains(a.IP)).To(BeTrue(), fmt.Sprintf("%s not in pool %s", a.IP, pool1))
				v4IPs = append(v4IPs, cnet.IP{a.IP})
			}

			// Release all IPs.
			unallocated, err := ic.ReleaseIPs(context.Background(), v4IPs)
			Expect(len(unallocated)).To(Equal(0))
			Expect(err).NotTo(HaveOccurred())

			// Change the selector for the IP pool so that it no longer matches node1.
			applyPool(pool1.String(), true, `foo != "bar"`)

			// The block should still have an affinity to this host.
			Expect(len(getAffineBlocks(bc, host))).To(Equal(1))

			// The allocation block should still exist.
			opts := model.BlockListOptions{IPVersion: 4}
			out, err := bc.List(context.Background(), opts, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(len(out.KVPairs)).To(Equal(1))

			// Create a second pool and assign a new address to the node.
			pool2 := cnet.MustParseNetwork("20.0.0.0/24")
			applyPool(pool2.String(), true, "all()")

			// Assign three addresses to the node.
			v4nets, _, err = ic.AutoAssign(context.Background(), AutoAssignArgs{
				Num4:     3,
				Num6:     0,
				Hostname: host,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(len(v4nets)).To(Equal(3))

			// Expect all the IPs to be from pool2.
			v4IPs = []cnet.IP{}
			for _, a := range v4nets {
				Expect(pool2.IPNet.Contains(a.IP)).To(BeTrue(), fmt.Sprintf("%s not in pool %s", a.IP, pool2))
				v4IPs = append(v4IPs, cnet.IP{a.IP})
			}

			// The block should only have one affinity to this host.
			blocks = getAffineBlocks(bc, host)
			Expect(len(blocks)).To(Equal(1))
			Expect(pool2.IPNet.Contains(blocks[0].IP)).To(BeTrue())

			// The block should only have one affinity.
			out, err = bc.List(context.Background(), opts, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(len(out.KVPairs)).To(Equal(1))

		})

		// Create one ip pool, call AutoAssign, call ReleaseIPs,
		// create another ip pool, call AutoAssign explicitly passing the second pool,
		// ensure that the block affinity from the first ip pool is not released.
		It("should not release blocks when the ips within are released but still selects the node while a different pool is explicitly requested", func() {
			host := "host"
			pool1 := cnet.MustParseNetwork("10.0.0.0/24")

			bc.Clean()
			deleteAllPools()

			applyNode(bc, kc, host, map[string]string{"foo": "bar"})
			applyPool(pool1.String(), true, `foo == "bar"`)

			// Assign three addresses to the node.
			v4nets, _, err := ic.AutoAssign(context.Background(), AutoAssignArgs{
				Num4:     3,
				Num6:     0,
				Hostname: host,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(len(v4nets)).To(Equal(3))

			// Should have one affine block to this host.
			blocks := getAffineBlocks(bc, host)
			Expect(len(blocks)).To(Equal(1))

			// Expect all the IPs to be from pool1.
			var v4IPs []cnet.IP
			for _, a := range v4nets {
				Expect(pool1.IPNet.Contains(a.IP)).To(BeTrue(), fmt.Sprintf("%s not in pool %s", a.IP, pool1))
				v4IPs = append(v4IPs, cnet.IP{a.IP})
			}

			// Release all IPs.
			unallocated, err := ic.ReleaseIPs(context.Background(), v4IPs)
			Expect(len(unallocated)).To(Equal(0))
			Expect(err).NotTo(HaveOccurred())

			// The block should still have an affinity to this host.
			Expect(len(getAffineBlocks(bc, host))).To(Equal(1))

			// The allocation block should still exist.
			opts := model.BlockListOptions{IPVersion: 4}
			out, err := bc.List(context.Background(), opts, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(len(out.KVPairs)).To(Equal(1))

			// Create a second pool and assign a new address to the node.
			pool2 := cnet.MustParseNetwork("20.0.0.0/24")
			applyPool(pool2.String(), true, "all()")

			// Assign three addresses to the node.
			v4nets, _, err = ic.AutoAssign(context.Background(), AutoAssignArgs{
				Num4:      3,
				Num6:      0,
				Hostname:  host,
				IPv4Pools: []cnet.IPNet{pool2},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(len(v4nets)).To(Equal(3))

			// The block should still have an affinity to this host.
			Expect(len(getAffineBlocks(bc, host))).To(Equal(2))

			// The allocation block should still exist.
			out, err = bc.List(context.Background(), opts, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(len(out.KVPairs)).To(Equal(2))
		})
	})

	Describe("IPAM AutoAssign from different pools - multi", func() {
		host := "host-a"
		pool1 := cnet.MustParseNetwork("10.0.0.0/24")
		pool2 := cnet.MustParseNetwork("20.0.0.0/24")
		pool3 := cnet.MustParseNetwork("30.0.0.0/24")
		pool4_v6 := cnet.MustParseNetwork("fe80::11/120")
		pool5_doesnot_exist := cnet.MustParseNetwork("40.0.0.0/24")

		It("should fail to AutoAssign 1 IPv4 when requesting a disabled IPv4 in the list of requested pools", func() {
			args := AutoAssignArgs{
				Num4:      1,
				Num6:      0,
				Hostname:  host,
				IPv4Pools: []cnet.IPNet{pool1, pool3},
			}
			bc.Clean()
			deleteAllPools()

			applyNode(bc, kc, host, nil)
			applyPool(pool1.String(), true, "")
			applyPool(pool2.String(), true, "")
			applyPool(pool3.String(), false, "")
			applyPool(pool4_v6.String(), true, "")
			_, _, outErr := ic.AutoAssign(context.Background(), args)
			Expect(outErr).To(HaveOccurred())
		})

		It("should fail to AutoAssign when specifying an IPv6 pool in the IPv4 requested pools", func() {
			args := AutoAssignArgs{
				Num4:      0,
				Num6:      1,
				Hostname:  host,
				IPv6Pools: []cnet.IPNet{pool4_v6, pool1},
			}
			_, _, outErr := ic.AutoAssign(context.Background(), args)
			Expect(outErr).To(HaveOccurred())
		})

		It("should allocate an IP from the first requested pool when two valid pools are requested", func() {
			args := AutoAssignArgs{
				Num4:      1,
				Num6:      0,
				Hostname:  host,
				IPv4Pools: []cnet.IPNet{pool1, pool2},
			}
			v4, _, outErr := ic.AutoAssign(context.Background(), args)
			log.Printf("IPAM returned: %v\n", v4)

			Expect(outErr).NotTo(HaveOccurred())
			Expect(len(v4)).To(Equal(1))
			Expect(pool1.Contains(v4[0].IP)).To(BeTrue())
		})

		It("should allocate 300 IP addresses from two enabled pools that contain sufficient addresses", func() {
			args := AutoAssignArgs{
				Num4:      300,
				Num6:      0,
				Hostname:  host,
				IPv4Pools: []cnet.IPNet{pool1, pool2},
			}
			v4, _, outErr := ic.AutoAssign(context.Background(), args)
			log.Printf("v4: %d IPs\n", len(v4))

			Expect(outErr).NotTo(HaveOccurred())
			Expect(len(v4)).To(Equal(300))
		})

		It("should fail to allocate another 300 IP addresses from the same pools due to lack of addresses (partial allocation)", func() {
			args := AutoAssignArgs{
				Num4:      300,
				Num6:      0,
				Hostname:  host,
				IPv4Pools: []cnet.IPNet{pool1, pool2},
			}
			v4, _, outErr := ic.AutoAssign(context.Background(), args)
			log.Printf("v4: %d IPs\n", len(v4))

			// Expect 211 entries since we have a total of 512, we requested 1 + 300 already.
			Expect(outErr).NotTo(HaveOccurred())
			Expect(v4).To(HaveLen(211))
		})

		It("should fail to allocate any address when requesting an invalid pool and a valid pool", func() {
			args := AutoAssignArgs{
				Num4:      1,
				Num6:      0,
				Hostname:  host,
				IPv4Pools: []cnet.IPNet{pool1, pool5_doesnot_exist},
			}
			v4, _, err := ic.AutoAssign(context.Background(), args)
			log.Printf("v4: %d IPs\n", len(v4))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).Should(Equal("the given pool (40.0.0.0/24) does not exist, or is not enabled"))
			Expect(len(v4)).To(Equal(0))
		})
	})

	Describe("IPAM EnsureBlock from different pools - multi", func() {
		host := "host-a"
		pool1 := cnet.MustParseNetwork("10.0.0.0/24")
		pool2 := cnet.MustParseNetwork("20.0.0.0/24")
		pool3 := cnet.MustParseNetwork("30.0.0.0/24")
		pool4_v6 := cnet.MustParseNetwork("fe80::11/120")
		pool5_doesnot_exist := cnet.MustParseNetwork("40.0.0.0/24")
		pool_big_block_size := cnet.MustParseNetwork("90.0.0.0/24")

		It("should fail to EnsureBlock when requesting a disabled IPv4 in the list of requested pools", func() {
			args := BlockArgs{
				Hostname:              host,
				IPv4Pools:             []cnet.IPNet{pool1, pool3},
				HostReservedAttrIPv4s: rsvdAttrWindows,
			}
			bc.Clean()
			deleteAllPools()

			applyNode(bc, kc, host, nil)
			applyPool(pool1.String(), true, "")
			applyPool(pool2.String(), true, "")
			applyPool(pool3.String(), false, "")
			applyPool(pool4_v6.String(), true, "")
			ipPools.pools[pool_big_block_size.String()] = pool{enabled: true, nodeSelector: "", blockSize: 31}
			_, _, outErr := ic.EnsureBlock(context.Background(), args)
			Expect(outErr).To(HaveOccurred())
		})

		It("should fail to EnsureBlock when specifying an IPv6 pool in the IPv4 requested pools", func() {
			args := BlockArgs{
				Hostname:              host,
				IPv4Pools:             []cnet.IPNet{pool4_v6},
				HostReservedAttrIPv4s: rsvdAttrWindows,
			}
			_, _, outErr := ic.EnsureBlock(context.Background(), args)
			Expect(outErr).To(HaveOccurred())
		})

		It("should fail to allocate a block when requesting an invalid pool which does not satisfy HostReserveAttr", func() {
			args := BlockArgs{
				Hostname:              host,
				IPv4Pools:             []cnet.IPNet{pool_big_block_size, pool1},
				HostReservedAttrIPv4s: rsvdAttrWindows,
			}
			_, _, err := ic.EnsureBlock(context.Background(), args)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).Should(Equal("the given pool (90.0.0.0/24) does not exist, or is not enabled"))
		})

		It("should fail to allocate a block when requesting an invalid pool and a valid pool", func() {
			args := BlockArgs{
				Hostname:              host,
				IPv4Pools:             []cnet.IPNet{pool1, pool5_doesnot_exist},
				HostReservedAttrIPv4s: rsvdAttrWindows,
			}
			_, _, err := ic.EnsureBlock(context.Background(), args)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).Should(Equal("the given pool (40.0.0.0/24) does not exist, or is not enabled"))
		})

		It("should allocate a block with no required pool specified", func() {
			args := BlockArgs{
				Hostname:              host,
				HostReservedAttrIPv4s: rsvdAttrWindows,
			}
			v4, _, outErr := ic.EnsureBlock(context.Background(), args)
			log.Printf("IPAM returned: %v\n", v4)

			Expect(outErr).NotTo(HaveOccurred())
			Expect(pool1.Contains(v4.IP)).To(BeTrue())
		})

		It("should allocate a block from the first requested pool when two valid pools are requested", func() {
			args := BlockArgs{
				Hostname:              host,
				IPv4Pools:             []cnet.IPNet{pool1, pool2},
				HostReservedAttrIPv4s: rsvdAttrWindows,
			}
			v4, _, outErr := ic.EnsureBlock(context.Background(), args)
			log.Printf("IPAM returned: %v\n", v4)

			Expect(outErr).NotTo(HaveOccurred())
			Expect(pool1.Contains(v4.IP)).To(BeTrue())
		})

		It("should return existing block after calling EnsureBlock again", func() {
			args := BlockArgs{
				Hostname:              host,
				IPv4Pools:             []cnet.IPNet{pool1, pool2},
				HostReservedAttrIPv4s: rsvdAttrWindows,
			}
			v4, _, outErr := ic.EnsureBlock(context.Background(), args)
			log.Printf("IPAM returned: %v\n", v4)

			Expect(outErr).NotTo(HaveOccurred())
			Expect(pool1.Contains(v4.IP)).To(BeTrue())

			v4_again, _, outErr := ic.EnsureBlock(context.Background(), args)
			Expect(outErr).NotTo(HaveOccurred())
			Expect(v4_again).To(Equal(v4))
		})
	})

	Describe("IPAM findOrClaimBlock test", func() {
		host := "host-a"
		pool1 := cnet.MustParseNetwork("10.0.0.0/24")
		rsvdAttr := &HostReservedAttr{
			StartOfBlock: 2,
			EndOfBlock:   1,
			Handle:       "findOrClaimBlock",
			Note:         "ipam ut",
		}
		var pools []v3.IPPool
		var ctx context.Context
		var affBlocks []cnet.IPNet
		var s *blockAssignState

		BeforeEach(func() {
			bc.Clean()
			deleteAllPools()

			applyNode(bc, kc, host, nil)
			// ippool with 4 ips
			ipPools.pools[pool1.String()] = pool{enabled: true, nodeSelector: "", blockSize: 30}
			pools, _ = ipPools.GetEnabledPools(4)
			Expect(len(pools)).To(Equal(1))

			ctx = context.Background()

			// initiate two block cidr
			affBlocks = []cnet.IPNet{cnet.MustParseNetwork("10.0.0.0/30"), cnet.MustParseNetwork("10.0.0.4/30")}

			cfg, err := ic.GetIPAMConfig(context.Background())
			Expect(err).NotTo(HaveOccurred())

			// Claim affinity on two blocks
			for _, blockCIDR := range affBlocks {
				pa, err := ic.(*ipamClient).blockReaderWriter.getPendingAffinity(ctx, host, blockCIDR)
				Expect(err).NotTo(HaveOccurred())

				_, err = ic.(*ipamClient).blockReaderWriter.claimAffineBlock(ctx, pa, *cfg, rsvdAttr)
				Expect(err).NotTo(HaveOccurred())
			}

			s = &blockAssignState{
				client:                *ic.(*ipamClient),
				version:               4,
				host:                  host,
				pools:                 pools,
				remainingAffineBlocks: affBlocks,
				hostReservedAttr:      rsvdAttr,
				allowNewClaim:         true,
			}
		})

		It("Should find or claim blocks", func() {
			b, newlyClaimed, outErr := s.findOrClaimBlock(ctx, 1)
			Expect(outErr).NotTo(HaveOccurred())
			// Should allocate from host-affine blocks.
			Expect(newlyClaimed).To(BeFalse())
			// Should find first block.
			Expect(b.Key.(model.BlockKey).CIDR.String()).To(Equal("10.0.0.0/30"))
			// uncheckedAffBlocks has single element which is the second block.
			Expect(len(s.remainingAffineBlocks)).To(Equal(1))
			Expect(s.remainingAffineBlocks[0].String()).To(Equal("10.0.0.4/30"))

			b, newlyClaimed, outErr = s.findOrClaimBlock(ctx, 1)
			Expect(outErr).NotTo(HaveOccurred())
			// Should allocate from host-affine blocks
			Expect(newlyClaimed).To(BeFalse())
			// find first block of outClaimed.
			Expect(b.Key.(model.BlockKey).CIDR.String()).To(Equal("10.0.0.4/30"))
			// uncheckedAffBlocks has single element which is the second block of outClaimed.
			Expect(len(s.remainingAffineBlocks)).To(Equal(0))

			b, newlyClaimed, outErr = s.findOrClaimBlock(ctx, 1)
			Expect(outErr).NotTo(HaveOccurred())
			// Should allocate from host-affine blocks
			Expect(newlyClaimed).To(BeTrue())
			Expect(len(s.remainingAffineBlocks)).To(Equal(0))
		})

		It("Should skip blocks having insufficient ip", func() {
			// Assign an IP from first block
			args := AssignIPArgs{
				IP:       cnet.IP{net.ParseIP("10.0.0.2")},
				Hostname: host,
			}
			outErr := ic.AssignIP(context.Background(), args)
			Expect(outErr).NotTo(HaveOccurred())

			b, newlyClaimed, outErr := s.findOrClaimBlock(ctx, 1)
			Expect(outErr).NotTo(HaveOccurred())
			// Should allocate from host-affine blocks.
			Expect(newlyClaimed).To(BeFalse())
			// Should find second block.
			Expect(b.Key.(model.BlockKey).CIDR.String()).To(Equal("10.0.0.4/30"))
			// uncheckedAffBlocks has no elements.
			Expect(len(s.remainingAffineBlocks)).To(Equal(0))
			Expect(s.datastoreRetryCount).To(Equal(0))

			// Assign an IP from second block
			args = AssignIPArgs{
				IP:       cnet.IP{net.ParseIP("10.0.0.6")},
				Hostname: host,
			}
			outErr = ic.AssignIP(context.Background(), args)
			Expect(outErr).NotTo(HaveOccurred())

			b, newlyClaimed, outErr = s.findOrClaimBlock(ctx, 1)
			Expect(outErr).NotTo(HaveOccurred())
			// Should claim new block.
			Expect(newlyClaimed).To(BeTrue())
			Expect(len(s.remainingAffineBlocks)).To(Equal(0))
			Expect(s.datastoreRetryCount).To(Equal(0))

			// Should return error if allowNewClaim is false.
			s.allowNewClaim = false
			b, newlyClaimed, outErr = s.findOrClaimBlock(ctx, 1)
			Expect(outErr).To(Equal(ErrBlockLimit))
		})

		It("Should return same block after been called multiple times", func() {
			// Clone current blockAssignState
			sCopy := *s
			sCopyPtr := &sCopy

			b, newlyClaimed, outErr := s.findOrClaimBlock(ctx, 1)
			Expect(outErr).NotTo(HaveOccurred())
			// Should allocate from host-affine blocks.
			Expect(newlyClaimed).To(BeFalse())
			// Should find first block.
			Expect(b.Key.(model.BlockKey).CIDR.String()).To(Equal("10.0.0.0/30"))
			// uncheckedAffBlocks has single element which is the second block.
			Expect(len(s.remainingAffineBlocks)).To(Equal(1))
			Expect(s.remainingAffineBlocks[0].String()).To(Equal("10.0.0.4/30"))

			b, newlyClaimed, outErr = sCopyPtr.findOrClaimBlock(ctx, 1)
			Expect(outErr).NotTo(HaveOccurred())
			// Should allocate from host-affine blocks.
			Expect(newlyClaimed).To(BeFalse())
			// Should find first block.
			Expect(b.Key.(model.BlockKey).CIDR.String()).To(Equal("10.0.0.0/30"))
			// uncheckedAffBlocks has single element which is the second block.
			Expect(len(s.remainingAffineBlocks)).To(Equal(1))
			Expect(s.remainingAffineBlocks[0].String()).To(Equal("10.0.0.4/30"))
		})
	})

	DescribeTable("AutoAssign: requested IPs vs returned IPs",
		func(host string, cleanEnv bool, pools []pool, usePool string, inv4, inv6, expv4, expv6 int, blockLimit int, expError error) {

			if cleanEnv {
				bc.Clean()
				deleteAllPools()
			}
			applyNode(bc, kc, host, nil)
			defer deleteNode(bc, kc, host)

			for _, v := range pools {
				ipPools.pools[v.cidr] = pool{cidr: v.cidr, enabled: v.enabled, blockSize: v.blockSize}
			}

			fromPool := cnet.MustParseNetwork(usePool)
			args := AutoAssignArgs{
				Num4:             inv4,
				Num6:             inv6,
				Hostname:         host,
				IPv4Pools:        []cnet.IPNet{fromPool},
				MaxBlocksPerHost: blockLimit,
			}

			outv4, outv6, err := ic.AutoAssign(context.Background(), args)
			if expError != nil {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).ToNot(HaveOccurred())
			}
			Expect(outv4).To(HaveLen(expv4))
			Expect(outv6).To(HaveLen(expv6))
		},

		// Test 1a: AutoAssign 1 IPv4, 1 IPv6 with tiny block - expect one of each to be returned.
		Entry("1 v4 1 v6 - tiny block", "test-host", true, []pool{{"192.168.1.0/24", 32, true, ""}, {"fd80:24e2:f998:72d6::/120", 128, true, ""}}, "192.168.1.0/24", 1, 1, 1, 1, 0, nil),

		// Test 1b: AutoAssign 1 IPv4, 1 IPv6 with massive block - expect one of each to be returned.
		Entry("1 v4 1 v6 - big block", "test-host", true, []pool{{"192.168.0.0/16", 20, true, ""}, {"fd80:24e2:f998:72d6::/110", 116, true, ""}}, "192.168.0.0/16", 1, 1, 1, 1, 0, nil),

		// Test 1c: AutoAssign 1 IPv4, 1 IPv6 with default block - expect one of each to be returned.
		Entry("1 v4 1 v6 - default block", "test-host", true, []pool{{"192.168.1.0/24", 26, true, ""}, {"fd80:24e2:f998:72d6::/120", 122, true, ""}}, "192.168.1.0/24", 1, 1, 1, 1, 0, nil),

		// Test 2a: AutoAssign 256 IPv4, 256 IPv6 with default blocksize- expect 256 IPv4 + IPv6 addresses.
		Entry("256 v4 256 v6", "test-host", true, []pool{{"192.168.1.0/24", 26, true, ""}, {"fd80:24e2:f998:72d6::/120", 122, true, ""}}, "192.168.1.0/24", 256, 256, 256, 256, 0, nil),

		// Test 2b: AutoAssign 256 IPv4, 256 IPv6 with small blocksize- expect 256 IPv4 + IPv6 addresses.
		Entry("256 v4 256 v6 - small blocks", "test-host", true, []pool{{"192.168.1.0/24", 30, true, ""}, {"fd80:24e2:f998:72d6::/120", 126, true, ""}}, "192.168.1.0/24", 256, 256, 256, 256, 0, nil),

		// Test 2a: AutoAssign 256 IPv4, 256 IPv6 with num blocks limit expect 64 IPv4 + IPv6 addresses.
		Entry("256 v4 0 v6 block limit", "test-host", true, []pool{{"192.168.1.0/24", 26, true, ""}, {"fd80:24e2:f998:72d6::/120", 122, true, ""}}, "192.168.1.0/24", 256, 0, 64, 0, 1, ErrBlockLimit),
		Entry("256 v4 0 v6 block limit 2", "test-host", true, []pool{{"192.168.1.0/24", 26, true, ""}, {"fd80:24e2:f998:72d6::/120", 122, true, ""}}, "192.168.1.0/24", 256, 0, 128, 0, 2, ErrBlockLimit),
		Entry("0 v4 256 v6 block limit", "test-host", true, []pool{{"192.168.1.0/24", 26, true, ""}, {"fd80:24e2:f998:72d6::/120", 122, true, ""}}, "192.168.1.0/24", 0, 256, 0, 64, 1, ErrBlockLimit),

		// Test 3: AutoAssign 257 IPv4, 0 IPv6 - expect 256 IPv4 addresses, no IPv6, and no error.
		Entry("257 v4 0 v6", "test-host", true, []pool{{"192.168.1.0/24", 26, true, ""}, {"fd80:24e2:f998:72d6::/120", 122, true, ""}}, "192.168.1.0/24", 257, 0, 256, 0, 0, nil),

		// Test 4: AutoAssign 0 IPv4, 257 IPv6 - expect 256 IPv6 addresses, no IPv6, and no error.
		Entry("0 v4 257 v6", "test-host", true, []pool{{"192.168.1.0/24", 26, true, ""}, {"fd80:24e2:f998:72d6::/120", 122, true, ""}}, "192.168.1.0/24", 0, 257, 0, 256, 0, nil),

		// Test 5: (use pool of size /25 so only two blocks are contained):
		// - Assign 1 address on host A (Expect 1 address).
		Entry("1 v4 0 v6 host-a", "host-a", true, []pool{{"10.0.0.0/25", 26, true, ""}, {"fd80:24e2:f998:72d6::/121", 122, true, ""}}, "10.0.0.0/25", 1, 0, 1, 0, 0, nil),

		// - Assign 1 address on host B (Expect 1 address, different block).
		Entry("1 v4 0 v6 host-b", "host-b", false, []pool{{"10.0.0.0/25", 26, true, ""}, {"fd80:24e2:f998:72d6::/121", 122, true, ""}}, "10.0.0.0/25", 1, 0, 1, 0, 0, nil),

		// - Assign 64 more addresses on host A (Expect 63 addresses from host A's block, 1 address from host B's block).
		Entry("64 v4 0 v6 host-a", "host-a", false, []pool{{"10.0.0.0/25", 26, true, ""}, {"fd80:24e2:f998:72d6::/121", 122, true, ""}}, "10.0.0.0/25", 64, 0, 64, 0, 0, nil),
	)

	DescribeTable("AssignIP: requested IP vs returned error",
		func(inIP net.IP, host string, cleanEnv bool, pool []string, expError error) {
			args := AssignIPArgs{
				IP:       cnet.IP{inIP},
				Hostname: host,
			}
			if cleanEnv {
				bc.Clean()
				deleteAllPools()
			}

			applyNode(bc, kc, host, nil)
			defer deleteNode(bc, kc, host)

			for _, v := range pool {
				applyPool(v, true, "")
			}

			outError := ic.AssignIP(context.Background(), args)
			if expError != nil {
				Expect(outError).To(HaveOccurred())
				Expect(outError).To(Equal(expError))
			} else {
				Expect(outError).ToNot(HaveOccurred())
			}
		},

		// Test 1: Assign 1 IPv4 from a configured pool - expect no error returned.
		Entry("Assign 1 IPv4 from a configured pool", net.ParseIP("192.168.1.0"), "test-host", true, []string{"192.168.1.0/24", "fd80:24e2:f998:72d6::/120"}, nil),

		// Test 2: Assign 1 IPv6 from a configured pool - expect no error returned.
		Entry("Assign 1 IPv6 from a configured pool", net.ParseIP("fd80:24e2:f998:72d6::"), "test-host", true, []string{"192.168.1.0/24", "fd80:24e2:f998:72d6::/120"}, nil),

		// Test 3: Assign 1 IPv4 from a non-configured pool - expect an error returned.
		Entry("Assign 1 IPv4 from a non-configured pool", net.ParseIP("1.1.1.1"), "test-host", true, []string{"192.168.1.0/24", "fd80:24e2:f998:72d6::/120"}, errors.New("The provided IP address is not in a configured pool\n")),

		// Test 4: Assign 1 IPv4 from a configured pool twice:
		// - Expect no error returned while assigning the IP for the first time.
		Entry("Assign 1 IPv4 from a configured pool twice (first time)", net.ParseIP("192.168.1.0"), "test-host", true, []string{"192.168.1.0/24", "fd80:24e2:f998:72d6::/120"}, nil),

		// - Expect an error returned while assigning the SAME IP again.
		Entry("Assign 1 IPv4 from a configured pool twice (second time)", net.ParseIP("192.168.1.0"), "test-host", false, []string{"192.168.1.0/24", "fd80:24e2:f998:72d6::/120"}, cerrors.ErrorResourceAlreadyExists{Err: errors.New("Address already assigned in block"), Identifier: "192.168.1.0"}),
	)

	DescribeTable("ReleaseIPs: requested IPs to be released vs actual unallocated IPs",
		func(inIP net.IP, cleanEnv bool, pool []string, assignIP net.IP, autoAssignNumIPv4 int, expUnallocatedIPs []cnet.IP, expError error) {
			inIPs := []cnet.IP{{inIP}}
			hostname := "host-release"

			// If we cleaned the datastore then recreate the pools.
			if cleanEnv {
				bc.Clean()
				deleteAllPools()
			}

			applyNode(bc, kc, hostname, nil)
			defer deleteNode(bc, kc, hostname)

			for _, v := range pool {
				applyPool(v, true, "")
			}

			if len(assignIP) != 0 {
				err := ic.AssignIP(context.Background(), AssignIPArgs{
					IP: cnet.IP{assignIP},
				})
				if err != nil {
					Fail(fmt.Sprintf("Error assigning IP %s", assignIP))
				}

				// Re-initialize it to an empty slice to flush out any IP if passed in by mistake.
				inIPs = []cnet.IP{}

				inIPs = append(inIPs, cnet.IP{assignIP})

			}

			if autoAssignNumIPv4 != 0 {
				assignedIPv4, _, err := ic.AutoAssign(context.Background(), AutoAssignArgs{
					Num4:     autoAssignNumIPv4,
					Hostname: hostname,
				})
				Expect(err).ToNot(HaveOccurred())
				for _, ipnet := range assignedIPv4 {
					inIPs = append(inIPs, cnet.MustParseIP(ipnet.IP.String()))
				}
				inIPs = inIPs[1:]
			}

			unallocatedIPs, outErr := ic.ReleaseIPs(context.Background(), inIPs)
			if outErr != nil {
				log.Println(outErr)
			}

			// Expect returned slice of unallocatedIPs to be equal to expected expUnallocatedIPs.
			Expect(unallocatedIPs).To(Equal(expUnallocatedIPs))

			// Assert if an error was expected.
			if expError != nil {
				Expect(outErr).To(HaveOccurred())
				Expect(outErr).To(Equal(expError))
			} else {
				Expect(outErr).ToNot(HaveOccurred())
			}
		},

		// Test cases (ReleaseIPs):
		// Test 1: release an IP that's not configured in any pools - expect a slice with the same IP as unallocatedIPs and no error.
		Entry("Release an IP that's not configured in any pools", net.ParseIP("1.1.1.1"), true, []string{"192.168.1.0/24", "fd80:24e2:f998:72d6::/120"}, net.IP{}, 0, []cnet.IP{{net.ParseIP("1.1.1.1")}}, nil),

		// Test 2: release an IP that's not allocated in the pool - expect a slice with one (unallocatedIPs) and no error.
		Entry("Release an IP that's not allocated in the pool", net.ParseIP("192.168.1.0"), true, []string{"192.168.1.0/24", "fd80:24e2:f998:72d6::/120"}, net.IP{}, 0, []cnet.IP{{net.ParseIP("192.168.1.0")}}, nil),

		// Test 3: Assign 1 IPv4 with AssignIP from a configured pool and then release it.
		// - Assign should not return an error.
		// - ReleaseIP should return empty slice of IPs (unallocatedIPs) and no error.
		Entry("Assign 1 IPv4 with AssignIP from a configured pool and then release it", net.IP{}, true, []string{"192.168.1.0/24", "fd80:24e2:f998:72d6::/120"}, net.ParseIP("192.168.1.0"), 0, []cnet.IP{}, nil),

		// Test 4: Assign 66 IPs (across 2 blocks) with AutoAssign from a configured pool then release them.
		// - Assign should not return an error.
		// - ReleaseIPs should return an empty slice of IPs (unallocatedIPs) and no error.
		Entry("Assign 66 IPs (across 2 blocks) with AutoAssign from a configured pool then release them", net.IP{}, true, []string{"192.168.1.0/24", "fd80:24e2:f998:72d6::/120"}, net.IP{}, 66, []cnet.IP{}, nil),

		// Test 5: Assign 1 IPv4 address with AssignIP then try to release 2 IPs.
		// - Assign should not return no error.
		// - ReleaseIPs should return a slice with one (unallocatedIPs) and no error.
		Entry("Assign 1 IPv4 address with AssignIP then try to release 2 IPs (assign one and release it)", net.IP{}, true, []string{"192.168.1.0/24", "fd80:24e2:f998:72d6::/120"}, net.ParseIP("192.168.1.0"), 0, []cnet.IP{}, nil),
		Entry("Assign 1 IPv4 address with AssignIP then try to release 2 IPs (release a second one)", net.ParseIP("192.168.1.1"), false, []string{"192.168.1.0/24", "fd80:24e2:f998:72d6::/120"}, net.IP{}, 0, []cnet.IP{cnet.MustParseIP("192.168.1.1")}, nil),
	)

	DescribeTable("ClaimAffinity: claim IPNet vs actual number of blocks claimed",
		func(args testArgsClaimAff) {
			inIPNet := cnet.MustParseNetwork(args.inNet)

			if args.cleanEnv {
				bc.Clean()
				deleteAllPools()
			}

			applyNode(bc, kc, args.host, nil)
			defer deleteNode(bc, kc, args.host)

			for _, v := range args.pool {
				applyPool(v, true, "")
			}

			assignIPutil(ic, args.assignIP, "host-a")

			outClaimed, outFailed, outError := ic.ClaimAffinity(context.Background(), inIPNet, args.host)
			log.Println("Claimed IP blocks: ", outClaimed)
			log.Println("Failed to claim IP blocks: ", outFailed)

			// Expect returned slice of claimed IPNet to be equal to expected claimed.
			Expect(len(outClaimed)).To(Equal(args.expClaimedIPs))

			// Expect returned slice of failed IPNet to be equal to expected failed.
			Expect(len(outFailed)).To(Equal(args.expFailedIPs))

			// Assert if an error was expected.
			if args.expError != nil {
				Expect(outError).To(HaveOccurred())
				Expect(outError.Error()).To(Equal(args.expError.Error()))
			}
		},

		// Test cases (ClaimAffinity):
		// Test 1: claim affinity for an unclaimed IPNet of size 64 - expect 1 claimed blocks, 0 failed and expect no error.
		Entry("Claim affinity for an unclaimed IPNet of size 64", testArgsClaimAff{"192.168.1.0/26", "host-a", true, []string{"192.168.1.0/24", "fd80:24e2:f998:72d6::/120"}, net.IP{}, 1, 0, nil}),

		// Test 2: claim affinity for an unclaimed IPNet of size smaller than 64 - expect 0 claimed blocks, 0 failed and expect an error.
		Entry("Claim affinity for an unclaimed IPNet of size smaller than 64", testArgsClaimAff{"192.168.1.0/27", "host-a", true, []string{"192.168.1.0/24", "fd80:24e2:f998:72d6::/120"}, net.IP{}, 0, 0, errors.New("The requested CIDR (192.168.1.0/27) is smaller than the minimum.")}),

		// Test 3: claim affinity for an IPNet that has an IP already assigned to another host.
		// - Assign an IP with AssignIP to "host-a" from a configured pool
		// - Claim affinity for "host-b" to the block that IP belongs to - expect 3 claimed blocks and 1 failed.
		Entry("Claim affinity for an IPNet that has an IP already assigned to another host (Claim affinity for host-b)", testArgsClaimAff{"10.0.0.0/24", "host-b", true, []string{"10.0.0.0/24", "fd80:24e2:f998:72d6::/120"}, net.ParseIP("10.0.0.1"), 3, 1, nil}),

		// Test 4: claim affinity to a block twice from different hosts.
		// - Claim affinity to an unclaimed block for "host-a" - expect 4 claimed blocks, 0 failed and expect no error.
		Entry("Claim affinity to an unclaimed block for host-a", testArgsClaimAff{"10.0.0.0/24", "host-a", true, []string{"10.0.0.0/24", "fd80:24e2:f998:72d6::/120"}, net.IP{}, 4, 0, nil}),

		// - Claim affinity to the same block again but for "host-b" this time - expect 0 claimed blocks, 4 failed and expect no error.
		Entry("Claim affinity to the same block again but for host-b this time", testArgsClaimAff{"10.0.0.0/24", "host-b", false, []string{"10.0.0.0/24", "fd80:24e2:f998:72d6::/120"}, net.IP{}, 0, 4, nil}),
	)

	Describe("ensure that GetIPAMConfig and SetIPAMConfig work as expected", func() {
		ctx := context.Background()

		BeforeEach(func() {
			bc.Clean()
		})

		It("should get the default IPAMConfig if one doesn't exist", func() {
			cfg, err := ic.GetIPAMConfig(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(cfg.AutoAllocateBlocks).To(Equal(true))
			Expect(cfg.StrictAffinity).To(Equal(false))
		})

		It("should set an IPAMConfig resource that is different from the default", func() {
			cfg := IPAMConfig{AutoAllocateBlocks: false, StrictAffinity: true}
			err := ic.SetIPAMConfig(ctx, cfg)
			Expect(err).NotTo(HaveOccurred())

			cfg2, err := ic.GetIPAMConfig(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(*cfg2).To(Equal(cfg))
		})
	})
})

// Tests for determining IP pools to use.
var _ = DescribeTable("determinePools tests",
	func(pool1Enabled, pool2Enabled bool, pool1Selector, pool2Selector string, requestPool1, requestPool2 bool, expectation []string, expectErr bool) {
		// Seed data
		ipPools.pools = map[string]pool{
			"10.0.0.0/24": pool{enabled: pool1Enabled, nodeSelector: pool1Selector},
			"20.0.0.0/24": pool{enabled: pool2Enabled, nodeSelector: pool2Selector},
		}
		// Create a new IPAM client, giving a nil datastore client since determining pools
		// doesn't require datastore access (we mock out the IP pool accessor).
		ic := NewIPAMClient(nil, ipPools)

		// Create a node object for the test.
		node := v3.Node{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"foo": "bar"}}}

		// Prep input data
		reqPools := []cnet.IPNet{}
		if requestPool1 {
			cidr := cnet.MustParseCIDR("10.0.0.0/24")
			reqPools = append(reqPools, cidr)
		}
		if requestPool2 {
			cidr := cnet.MustParseCIDR("20.0.0.0/24")
			reqPools = append(reqPools, cidr)
		}

		// Call determinePools
		pools, _, err := ic.(*ipamClient).determinePools(context.Background(), reqPools, 4, node, 32)

		// Assert on any returned error.
		if expectErr {
			Expect(err).To(HaveOccurred())
		} else {
			Expect(err).NotTo(HaveOccurred())
		}

		// Check that the expected pools match the returned.
		actual := []string{}
		for _, pool := range pools {
			actual = append(actual, pool.Spec.CIDR)
		}
		Expect(actual).To(Equal(expectation))
	},
	Entry("Both pools enabled, none with node selector, no requested pools", true, true, "", "", false, false, []string{"10.0.0.0/24", "20.0.0.0/24"}, false),
	Entry("Both pools enabled, none with node selector, pool1 requested", true, true, "", "", true, false, []string{"10.0.0.0/24"}, false),

	Entry("Both pools enabled, pool1 matching selector, no requested pools", true, true, `foo == "bar"`, `foo != "bar"`, false, false, []string{"10.0.0.0/24"}, false),
	Entry("Both pools enabled, pool1 matching node selector, pool1 requested", true, true, `foo == "bar"`, `foo != "bar"`, true, false, []string{"10.0.0.0/24"}, false),

	Entry("Both pools enabled, pool1 mismatching node selector, no requested pools", true, true, `foo != "bar"`, "all()", false, false, []string{"20.0.0.0/24"}, false),
	Entry("Both pools enabled, pool1 mismatching node selector, pool1 requested", true, true, `foo != "bar"`, "", true, false, []string{"10.0.0.0/24"}, false),

	Entry("Both pools enabled, pool1 matching node selector, pool2 requested", true, true, `foo == "bar"`, "", false, true, []string{"20.0.0.0/24"}, false),

	Entry("pool1 disabled, none with node selector, no requested pools", false, true, "", "", false, false, []string{"20.0.0.0/24"}, false),
	Entry("pool1 disabled, none with node selector, pool1 requested", false, true, "", "", true, false, []string{}, true),
	Entry("pool1 disabled, none with node selector, pool2 requested", false, true, "", "", false, true, []string{"20.0.0.0/24"}, false),

	Entry("pool1 disabled, pool2 matching node selector, no requested pools", false, true, "", `foo == "bar"`, false, false, []string{"20.0.0.0/24"}, false),
	Entry("pool1 disabled, pool2 matching node selector, pool2 requested", false, true, "", `foo == "bar"`, false, true, []string{"20.0.0.0/24"}, false),
	Entry("pool1 disabled, pool2 mismatching node selector, no requested pools", false, true, "", `foo != "bar"`, false, false, []string{}, false),
	Entry("pool1 disabled, pool2 mismatching node selector, pool2 requested", false, true, "", `foo != "bar"`, false, true, []string{"20.0.0.0/24"}, false),
)

// assignIPutil is a utility function to help with assigning a single IP address to a hostname passed in.
func assignIPutil(ic Interface, assignIP net.IP, host string) {
	if len(assignIP) != 0 {
		err := ic.AssignIP(context.Background(), AssignIPArgs{
			IP:       cnet.IP{assignIP},
			Hostname: host,
		})
		log.Printf("Assigning IP: %s\n", assignIP)
		if err != nil {
			Fail(fmt.Sprintf("Error assigning IP %s", assignIP))
		}
	}
}

// getAffineBlocks gets all the blocks affined to the host passed in.
func getAffineBlocks(backend bapi.Client, host string) []cnet.IPNet {
	opts := model.BlockAffinityListOptions{Host: host, IPVersion: 4}
	datastoreObjs, err := backend.List(context.Background(), opts, "")
	if err != nil {
		if _, ok := err.(cerrors.ErrorResourceDoesNotExist); ok {
			log.Printf("No affined blocks found")
		} else {
			Expect(err).NotTo(HaveOccurred(), "Error getting affine blocks: %v", err)
		}
	}

	// Iterate through and extract the block CIDRs.
	var blocks []cnet.IPNet
	for _, o := range datastoreObjs.KVPairs {
		k := o.Key.(model.BlockAffinityKey)
		blocks = append(blocks, k.CIDR)
	}
	return blocks
}
func deleteAllPools() {
	log.Infof("Deleting all pools")
	ipPools.pools = map[string]pool{}
}

func applyPool(cidr string, enabled bool, nodeSelector string) {
	ipPools.pools[cidr] = pool{enabled: enabled, nodeSelector: nodeSelector}
}

func applyPoolWithBlockSize(cidr string, enabled bool, nodeSelector string, blockSize int) {
	ipPools.pools[cidr] = pool{enabled: enabled, nodeSelector: nodeSelector, blockSize: blockSize}
}

func applyNode(c bapi.Client, kc *kubernetes.Clientset, host string, labels map[string]string) error {
	if kc != nil {
		// If a k8s clientset was provided, create the node in Kubernetes.
		n := corev1.Node{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Node",
				APIVersion: "v1",
			},
		}
		n.Name = host
		n.Labels = labels

		// Create/Update the node
		newNode, err := kc.CoreV1().Nodes().Create(&n)
		if err != nil {
			if kerrors.IsAlreadyExists(err) {
				oldNode, _ := kc.CoreV1().Nodes().Get(host, metav1.GetOptions{})
				oldNode.Labels = labels

				newNode, err = kc.CoreV1().Nodes().Update(oldNode)
				if err != nil {
					return nil
				}
			} else {
				return err
			}
		}
		log.WithField("node", newNode).WithError(err).Info("node applied")
	} else {
		// Otherwise, create it in Calico.
		_, err := c.Apply(context.Background(), &model.KVPair{
			Key: model.ResourceKey{Name: host, Kind: v3.KindNode},
			Value: v3.Node{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: v3.NodeSpec{OrchRefs: []v3.OrchRef{
					v3.OrchRef{Orchestrator: "k8s", NodeName: host},
				}},
			},
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func deleteNode(c bapi.Client, kc *kubernetes.Clientset, host string) {
	if kc != nil {
		kc.CoreV1().Nodes().Delete(host, &metav1.DeleteOptions{})
	} else {
		c.Delete(context.Background(), &model.ResourceKey{Name: host, Kind: v3.KindNode}, "")
	}
}
