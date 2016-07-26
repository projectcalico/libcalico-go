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

package ipsets_test

import (
	. "github.com/tigera/libcalico-go/etcd-driver/ipsets"

	"flag"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"os"
)

type ipUpdate struct {
	kind  string
	ipSet string
	ip    string
}

func init() {
	if os.Getenv("GLOG") != "" {
		flag.Parse()
		flag.Lookup("logtostderr").Value.Set("true")
		flag.Lookup("v").Value.Set("10")
	}
}

var _ = Describe("An empty IpsetCalculator", func() {
	var calc *IpsetCalculator
	var updates []ipUpdate
	BeforeEach(func() {
		updates = nil
		calc = NewIpsetCalculator()
		calc.OnIPAdded = func(ipSetID string, ip string) {
			updates = append(updates, ipUpdate{"add", ipSetID, ip})
		}
		calc.OnIPRemoved = func(ipSetID string, ip string) {
			updates = append(updates, ipUpdate{"remove", ipSetID, ip})
		}
	})
	Describe("with a match added", func() {
		BeforeEach(func() {
			calc.MatchStarted("ep0", "ipset1")
		})
		It("should trigger events ehen adding IPs", func() {
			calc.UpdateEndpointIPs("ep0", []string{"10.0.0.1", "10.0.0.2"})
			Expect(updates).To(Equal([]ipUpdate{
				{"add", "ipset1", "10.0.0.1"},
				{"add", "ipset1", "10.0.0.2"},
			}))
		})
		Describe("and then removed", func() {
			BeforeEach(func() {
				calc.MatchStopped("ep0", "ipset1")
			})
			It("should not trigger an event ehen adding IPs", func() {
				calc.UpdateEndpointIPs("ep0", []string{"10.0.0.1", "10.0.0.2"})
				Expect(updates).To(BeEmpty())
			})
		})
	})
	Describe("with endpoints added", func() {
		BeforeEach(func() {
			calc.UpdateEndpointIPs("ep0", []string{})
			calc.UpdateEndpointIPs("ep1", []string{"10.0.0.1", "10.0.0.2"})
			calc.UpdateEndpointIPs("ep2", []string{"10.0.0.1", "10.0.1.2"})
		})
		It("Should emit no events", func() {
			Expect(updates).To(BeEmpty())
		})
		Describe("and a match on an endpoint with no IPs", func() {
			BeforeEach(func() {
				calc.MatchStarted("ep0", "ipset1")
			})
			It("should emit no events", func() {
				Expect(updates).To(BeEmpty())
			})
			Describe("and update to add an IP", func() {
				BeforeEach(func() {
					calc.UpdateEndpointIPs("ep0", []string{"10.0.2.2"})
				})
				It("should generate an event", func() {
					Expect(updates).To(Equal([]ipUpdate{
						{"add", "ipset1", "10.0.2.2"},
					}))
				})
				Describe("and 2nd update to add an IP", func() {
					BeforeEach(func() {
						calc.UpdateEndpointIPs("ep0", []string{"10.0.2.2", "10.0.2.3"})
					})
					It("should generate an event", func() {
						Expect(updates).To(Equal([]ipUpdate{
							{"add", "ipset1", "10.0.2.2"},
							{"add", "ipset1", "10.0.2.3"},
						}))
					})
				})
				Describe("and 2nd update to change IP", func() {
					BeforeEach(func() {
						calc.UpdateEndpointIPs("ep0", []string{"10.0.2.3"})
					})
					It("should generate an event", func() {
						Expect(updates).To(Equal([]ipUpdate{
							{"add", "ipset1", "10.0.2.2"},
							{"add", "ipset1", "10.0.2.3"},
							{"remove", "ipset1", "10.0.2.2"},
						}))
					})
				})
				Describe("and 2nd update to remove IPs", func() {
					BeforeEach(func() {
						calc.UpdateEndpointIPs("ep0", []string{})
					})
					It("should generate an event", func() {
						Expect(updates).To(Equal([]ipUpdate{
							{"add", "ipset1", "10.0.2.2"},
							{"remove", "ipset1", "10.0.2.2"},
						}))
					})
				})
			})
		})
		Describe("and a single match", func() {
			BeforeEach(func() {
				calc.MatchStarted("ep1", "ipset1")
			})
			It("should emit events for each IP", func() {
				Expect(updates).To(Equal([]ipUpdate{
					{"add", "ipset1", "10.0.0.1"},
					{"add", "ipset1", "10.0.0.2"},
				}))
			})
			Describe("and a second match", func() {
				BeforeEach(func() {
					updates = []ipUpdate{}
					calc.MatchStarted("ep2", "ipset1")
				})
				It("should emit events for new IPs only", func() {
					Expect(updates).To(Equal([]ipUpdate{
						{"add", "ipset1", "10.0.1.2"},
					}))
				})
				Describe("and removing the 1st match", func() {
					BeforeEach(func() {
						updates = []ipUpdate{}
						calc.MatchStopped("ep1", "ipset1")
					})
					It("should remove ep1's unique IP only", func() {
						Expect(updates).To(Equal([]ipUpdate{
							{"remove", "ipset1", "10.0.0.2"},
						}))
					})
					Describe("and removing the 2nd match", func() {
						BeforeEach(func() {
							updates = []ipUpdate{}
							calc.MatchStopped("ep2", "ipset1")
						})
						It("should remove other IPs", func() {
							Expect(updates).To(Equal([]ipUpdate{
								{"remove", "ipset1", "10.0.0.1"},
								{"remove", "ipset1", "10.0.1.2"},
							}))
						})
						Describe("and deleting endpoints", func() {
							BeforeEach(func() {
								calc.DeleteEndpoint("ep0")
								calc.DeleteEndpoint("ep1")
								calc.DeleteEndpoint("ep2")
							})
							It("should be empty", func() {
								Expect(calc.Empty()).To(BeTrue())
							})
						})
					})
				})
				Describe("and removing the 2st match", func() {
					BeforeEach(func() {
						updates = []ipUpdate{}
						calc.MatchStopped("ep2", "ipset1")
					})
					It("should remove ep2's unique IP only", func() {
						Expect(updates).To(Equal([]ipUpdate{
							{"remove", "ipset1", "10.0.1.2"},
						}))
					})
					Describe("and removing the 1st match", func() {
						BeforeEach(func() {
							updates = []ipUpdate{}
							calc.MatchStopped("ep1", "ipset1")
						})
						It("should remove other IPs", func() {
							Expect(updates).To(Equal([]ipUpdate{
								{"remove", "ipset1", "10.0.0.1"},
								{"remove", "ipset1", "10.0.0.2"},
							}))
						})
						Describe("and deleting endpoints", func() {
							BeforeEach(func() {
								calc.DeleteEndpoint("ep0")
								calc.DeleteEndpoint("ep1")
								calc.DeleteEndpoint("ep2")
							})
							It("should be empty", func() {
								Expect(calc.Empty()).To(BeTrue())
							})
						})
					})
				})
				Describe("and deleting the 1st endpoint", func() {
					BeforeEach(func() {
						updates = []ipUpdate{}
						calc.DeleteEndpoint("ep1")
					})
					It("should remove ep1's unique IP only", func() {
						Expect(updates).To(Equal([]ipUpdate{
							{"remove", "ipset1", "10.0.0.2"},
						}))
					})
					Describe("and deleting the 2nd endpoint", func() {
						BeforeEach(func() {
							updates = []ipUpdate{}
							calc.DeleteEndpoint("ep2")
						})
						It("should remove other IPs", func() {
							Expect(updates).To(Equal([]ipUpdate{
								{"remove", "ipset1", "10.0.0.1"},
								{"remove", "ipset1", "10.0.1.2"},
							}))
						})
					})
				})
			})
		})
	})
})
