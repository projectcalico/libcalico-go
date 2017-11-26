// Copyright (c) 2016-2017 Tigera, Inc. All rights reserved.

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

package k8s

import (
	"context"
	"fmt"
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	log "github.com/sirupsen/logrus"

	"github.com/projectcalico/libcalico-go/lib/apiconfig"
	apiv3 "github.com/projectcalico/libcalico-go/lib/apis/v3"
	"github.com/projectcalico/libcalico-go/lib/backend/api"
	"github.com/projectcalico/libcalico-go/lib/backend/model"
	"github.com/projectcalico/libcalico-go/lib/numorstring"

	k8sapi "k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

var (
	zeroOrder                  = float64(0.0)
	calicoAllowPolicyModelSpec = apiv3.GlobalNetworkPolicySpec{
		Order: &zeroOrder,
		Ingress: []apiv3.Rule{
			{
				Action: "Allow",
			},
		},
		Egress: []apiv3.Rule{
			{
				Action: "Allow",
			},
		},
	}
	calicoDisallowPolicyModelSpec = apiv3.GlobalNetworkPolicySpec{
		Order: &zeroOrder,
		Ingress: []apiv3.Rule{
			{
				Action: "Deny",
			},
		},
		Egress: []apiv3.Rule{
			{
				Action: "Deny",
			},
		},
	}

	// Used for testing Syncer conversion
	calicoAllowPolicyModelV1 = model.Policy{
		Order: &zeroOrder,
		InboundRules: []model.Rule{
			{
				Action: "allow",
			},
		},
		OutboundRules: []model.Rule{
			{
				Action: "allow",
			},
		},
	}
	calicoDisallowPolicyModelV1 = model.Policy{
		Order: &zeroOrder,
		InboundRules: []model.Rule{
			{
				Action: "deny",
			},
		},
		OutboundRules: []model.Rule{
			{
				Action: "deny",
			},
		},
	}

	// Use a back-off set of intervals for testing deletion of a namespace
	// which can sometimes be slow.
	slowCheck = []interface{}{
		60 * time.Second,
		1 * time.Second,
	}
)

// cb implements the callback interface required for the
// backend Syncer API.
type cb struct {
	// Stores the current state for comparison by the tests.
	State map[string]api.Update
	Lock  *sync.Mutex

	status     api.SyncStatus
	updateChan chan api.Update
}

func (c cb) OnStatusUpdated(status api.SyncStatus) {
	defer GinkgoRecover()

	// Keep latest status up to date.
	log.Warnf("[TEST] Received status update: %+v", status)
	c.status = status

	// Once we get in sync, we don't ever expect to not
	// be in sync.
	if c.status == api.InSync {
		Expect(status).To(Equal(api.InSync))
	}
}

func (c cb) OnUpdates(updates []api.Update) {
	defer GinkgoRecover()

	// Ensure the given updates are valid.
	// We only perform mild validation here.
	for _, u := range updates {
		switch u.UpdateType {
		case api.UpdateTypeKVNew:
			// Sometimes the value is nil (e.g ProfileTags)
			log.Infof("[TEST] Syncer received new: %+v", u)
		case api.UpdateTypeKVUpdated:
			// Sometimes the value is nil (e.g ProfileTags)
			log.Infof("[TEST] Syncer received updated: %+v", u)
		case api.UpdateTypeKVDeleted:
			// Ensure the value is nil for deletes.
			log.Infof("[TEST] Syncer received deleted: %+v", u)
			Expect(u.Value).To(BeNil())
		case api.UpdateTypeKVUnknown:
			panic(fmt.Sprintf("[TEST] Syncer received unkown update: %+v", u))
		}

		// Send the update to a goroutine which will process it.
		c.updateChan <- u
	}
}

func (c cb) ProcessUpdates() {
	for u := range c.updateChan {
		// Store off the update so it can be checked by the test.
		// Use a mutex for safe cross-goroutine reads/writes.
		c.Lock.Lock()
		if u.UpdateType == api.UpdateTypeKVUnknown {
			// We should never get this!
			log.Panic("Received Unknown update type")
		} else if u.UpdateType == api.UpdateTypeKVDeleted {
			// Deleted.
			delete(c.State, u.Key.String())
			log.Infof("[TEST] Delete update %s", u.Key.String())
		} else {
			// Add or modified.
			c.State[u.Key.String()] = u
			log.Infof("[TEST] Stored update (type %d) %s", u.UpdateType, u.Key.String())
		}
		c.Lock.Unlock()
	}
}

func (c cb) ExpectExists(updates []api.Update) {
	// For each Key, wait for it to exist.
	for _, update := range updates {
		log.Infof("[TEST] Expecting key: %s", update.Key)
		matches := false

		wait.PollImmediate(1*time.Second, 60*time.Second, func() (bool, error) {
			// Get the update.
			c.Lock.Lock()
			u, ok := c.State[update.Key.String()]
			c.Lock.Unlock()

			// See if we've got a matching update. For now, we just check
			// that the key exists and that it's the correct type.
			matches = ok && update.UpdateType == u.UpdateType

			log.Infof("[TEST] Key exists? %t matches? %t: %+v", ok, matches, u)
			if matches {
				// Expected the update to be present, and it is.
				return true, nil
			} else {
				// Update is not yet present.
				return false, nil
			}
		})

		// Expect the key to have existed.
		Expect(matches).To(Equal(true), fmt.Sprintf("Expected update not found: %s", update.Key))
	}
}

// ExpectDeleted asserts that the provided KVPairs have been deleted
// via an update over the Syncer.
func (c cb) ExpectDeleted(kvps []model.KVPair) {
	for _, kvp := range kvps {
		log.Infof("[TEST] Not expecting key: %s", kvp.Key)
		exists := true

		wait.PollImmediate(1*time.Second, 60*time.Second, func() (bool, error) {
			// Get the update.
			c.Lock.Lock()
			update, ok := c.State[kvp.Key.String()]
			exists = ok
			c.Lock.Unlock()

			log.Infof("[TEST] Key exists? %t: %+v", ok, update)
			if ok {
				// Expected key to not exist, and it does.
				return false, nil
			} else {
				// Expected key to not exist, and it doesn't.
				return true, nil
			}
		})

		// Expect the key to not exist.
		Expect(exists).To(Equal(false), fmt.Sprintf("Expected key not to exist: %s", kvp.Key))
	}
}

// GetSyncerValueFunc returns a function that can be used to query the value of
// an entry in our syncer state store.  It's useful for performing "Eventually" testing.
//
// The returned function returns the cached entry or nil if the entry does not
// exist in the cache.
func (c cb) GetSyncerValueFunc(key model.Key) func() interface{} {
	return func() interface{} {
		log.Infof("Checking entry in cache: %s", key)
		c.Lock.Lock()
		defer func() {
			c.Lock.Unlock()
		}()
		if entry, ok := c.State[key.String()]; ok {
			return entry.Value
		}
		return nil
	}
}

// GetSyncerValuePresentFunc returns a function that can be used to query whether an entry
// is in our syncer state store.  It's useful for performing "Eventually" testing.
//
// When checking for presence use this function rather than GetSyncerValueFunc() because
// the Value may itself by nil.
//
// The returned function returns true if the entry is present.
func (c cb) GetSyncerValuePresentFunc(key model.Key) func() interface{} {
	return func() interface{} {
		log.Infof("Checking entry in cache: %s", key)
		c.Lock.Lock()
		defer func() { c.Lock.Unlock() }()
		_, ok := c.State[key.String()]
		return ok
	}
}

func CreateClientAndSyncer(cfg apiconfig.KubeConfig) (*KubeClient, *cb, api.Syncer) {
	// First create the client.
	caCfg := apiconfig.CalicoAPIConfigSpec{KubeConfig: cfg}
	c, err := NewKubeClient(&caCfg)
	if err != nil {
		panic(err)
	}

	// Ensure the backend is initialized.
	err = c.EnsureInitialized()
	Expect(err).NotTo(HaveOccurred(), "Failed to initialize the backend.")

	// Start the syncer.
	updateChan := make(chan api.Update)
	callback := cb{
		State:      map[string]api.Update{},
		status:     api.WaitForDatastore,
		Lock:       &sync.Mutex{},
		updateChan: updateChan,
	}
	syncer := c.Syncer(callback)
	return c.(*KubeClient), &callback, syncer
}

var _ = Describe("Test Syncer API for Kubernetes backend", func() {
	var (
		c      *KubeClient
		cb     *cb
		syncer api.Syncer
	)

	ctx := context.Background()

	BeforeEach(func() {
		log.SetLevel(log.DebugLevel)

		// Create a Kubernetes client, callbacks, and a syncer.
		cfg := apiconfig.KubeConfig{K8sAPIEndpoint: "http://localhost:8080"}
		c, cb, syncer = CreateClientAndSyncer(cfg)

		// Start the syncer.
		syncer.Start()

		// Node object is created by applying the mock-node.yaml manifest in advance.

		// Start processing updates.
		go cb.ProcessUpdates()
	})

	It("should handle a Namespace with DefaultDeny (v1beta annotation for namespace isolation)", func() {
		ns := k8sapi.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-syncer-namespace-default-deny",
				Annotations: map[string]string{
					"net.beta.kubernetes.io/network-policy": "{\"ingress\": {\"isolation\": \"DefaultDeny\"}}",
				},
				Labels: map[string]string{"label": "value"},
			},
		}

		// Make sure we clean up.  Don't check for errors since we attempt
		// to delete as part of the test below.
		defer func() {
			c.clientSet.CoreV1().Namespaces().Delete(ns.ObjectMeta.Name, &metav1.DeleteOptions{})
		}()

		By("Creating a namespace", func() {
			_, err := c.clientSet.CoreV1().Namespaces().Create(&ns)
			Expect(err).NotTo(HaveOccurred())
		})

		By("Performing a List of Profiles", func() {
			_, err := c.List(ctx, model.ResourceListOptions{Kind: apiv3.KindProfile}, "")
			Expect(err).NotTo(HaveOccurred())
		})

		By("Performing a List of Policies", func() {
			_, err := c.List(ctx, model.ResourceListOptions{Kind: apiv3.KindNetworkPolicy, Namespace: "test-syncer-namespace-default-deny"}, "")
			Expect(err).NotTo(HaveOccurred())
		})

		By("Performing a Get on the Profile and ensure no error in the Calico API", func() {
			_, err := c.Get(ctx, model.ResourceKey{Name: fmt.Sprintf("kns.%s", ns.ObjectMeta.Name), Kind: apiv3.KindProfile}, "")
			Expect(err).NotTo(HaveOccurred())
		})

		By("Checking the correct entries are in our cache", func() {
			expectedName := "kns.test-syncer-namespace-default-deny"
			Eventually(cb.GetSyncerValuePresentFunc(model.ProfileRulesKey{ProfileKey: model.ProfileKey{expectedName}})).Should(BeTrue())
			Eventually(cb.GetSyncerValuePresentFunc(model.ProfileLabelsKey{ProfileKey: model.ProfileKey{expectedName}})).Should(BeTrue())
		})

		By("Deleting the namespace", func() {
			err := c.clientSet.CoreV1().Namespaces().Delete(ns.ObjectMeta.Name, &metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())
		})

		By("Checking the correct entries are no longer in our cache", func() {
			expectedName := "kns.test-syncer-namespace-default-deny"
			Eventually(cb.GetSyncerValuePresentFunc(model.ProfileRulesKey{ProfileKey: model.ProfileKey{expectedName}}), slowCheck...).Should(BeFalse())
			Eventually(cb.GetSyncerValuePresentFunc(model.ProfileLabelsKey{ProfileKey: model.ProfileKey{expectedName}})).Should(BeFalse())
		})
	})

	It("should handle a Namespace without any annotations", func() {
		ns := k8sapi.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "test-syncer-namespace-no-default-deny",
				Annotations: map[string]string{},
				Labels:      map[string]string{"label": "value"},
			},
		}

		// Make sure we clean up after ourselves.  Don't check for errors since we attempt
		// to delete as part of the test below.
		defer func() {
			c.clientSet.CoreV1().Namespaces().Delete(ns.ObjectMeta.Name, &metav1.DeleteOptions{})
		}()

		// Check to see if the create succeeded.
		By("Creating a namespace", func() {
			_, err := c.clientSet.CoreV1().Namespaces().Create(&ns)
			Expect(err).NotTo(HaveOccurred())
		})

		// Perform a List and ensure it shows up in the Calico API.
		By("listing Profiles", func() {
			_, err := c.List(ctx, model.ResourceListOptions{Kind: apiv3.KindProfile}, "")
			Expect(err).NotTo(HaveOccurred())
		})

		By("listing Policies", func() {
			_, err := c.List(ctx, model.ResourceListOptions{Kind: apiv3.KindNetworkPolicy, Namespace: "test-syncer-namespace-no-default-deny"}, "")
			Expect(err).NotTo(HaveOccurred())
		})

		// Perform a Get and ensure no error in the Calico API.
		By("getting a Profile", func() {
			_, err := c.Get(ctx, model.ResourceKey{Name: fmt.Sprintf("kns.%s", ns.ObjectMeta.Name), Kind: apiv3.KindProfile}, "")
			Expect(err).NotTo(HaveOccurred())
		})

		// Expect corresponding Profile updates over the syncer for this Namespace.
		By("Checking the correct entries are in our cache", func() {
			expectedName := "kns.test-syncer-namespace-no-default-deny"
			Eventually(cb.GetSyncerValuePresentFunc(model.ProfileRulesKey{ProfileKey: model.ProfileKey{expectedName}})).Should(BeTrue())
			Eventually(cb.GetSyncerValuePresentFunc(model.ProfileLabelsKey{ProfileKey: model.ProfileKey{expectedName}})).Should(BeTrue())
		})

		By("deleting a namespace", func() {
			err := c.clientSet.CoreV1().Namespaces().Delete(ns.ObjectMeta.Name, &metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())
		})

		By("Checking the correct entries are in no longer in our cache", func() {
			expectedName := "kns.test-syncer-namespace-no-default-deny"
			Eventually(cb.GetSyncerValuePresentFunc(model.ProfileRulesKey{ProfileKey: model.ProfileKey{expectedName}}), slowCheck...).Should(BeFalse())
			Eventually(cb.GetSyncerValuePresentFunc(model.ProfileLabelsKey{ProfileKey: model.ProfileKey{expectedName}})).Should(BeFalse())
		})
	})

	It("should handle a basic NetworkPolicy", func() {
		np := extensions.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-syncer-basic-net-policy",
			},
			Spec: extensions.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{"label": "value"},
				},
				Ingress: []extensions.NetworkPolicyIngressRule{
					{
						Ports: []extensions.NetworkPolicyPort{
							{},
						},
						From: []extensions.NetworkPolicyPeer{
							{
								PodSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"k": "v",
									},
								},
							},
						},
					},
				},
			},
		}
		res := c.clientSet.ExtensionsV1beta1().RESTClient().
			Post().
			Resource("networkpolicies").
			Namespace("default").
			Body(&np).
			Do()

		// Make sure we clean up after ourselves.
		defer func() {
			res := c.clientSet.ExtensionsV1beta1().RESTClient().
				Delete().
				Resource("networkpolicies").
				Namespace("default").
				Name(np.ObjectMeta.Name).
				Do()
			Expect(res.Error()).NotTo(HaveOccurred())
		}()

		// Check to see if the create succeeded.
		Expect(res.Error()).NotTo(HaveOccurred())

		// Perform a List and ensure it shows up in the Calico API.
		_, err := c.List(ctx, model.ResourceListOptions{Kind: apiv3.KindNetworkPolicy}, "")
		Expect(err).NotTo(HaveOccurred())

		// Perform a Get and ensure no error in the Calico API.
		_, err = c.Get(ctx, model.ResourceKey{
			Name:      fmt.Sprintf("knp.default.%s", np.ObjectMeta.Name),
			Namespace: "default",
			Kind:      apiv3.KindNetworkPolicy,
		}, "")
		Expect(err).NotTo(HaveOccurred())
	})

	// Add a defer to wait for policies to clean up.
	defer func() {
		log.Warnf("[TEST] Waiting for policies to tear down")
		It("should clean up all policies", func() {
			nps := extensions.NetworkPolicyList{}
			err := c.clientSet.ExtensionsV1beta1().RESTClient().
				Get().
				Resource("networkpolicies").
				Namespace("default").
				Timeout(10 * time.Second).
				Do().Into(&nps)
			Expect(err).NotTo(HaveOccurred())

			// Loop until no network policies exist.
			for i := 0; i < 10; i++ {
				if len(nps.Items) == 0 {
					return
				}
				nps := extensions.NetworkPolicyList{}
				err := c.clientSet.ExtensionsV1beta1().RESTClient().
					Get().
					Resource("networkpolicies").
					Namespace("default").
					Timeout(10 * time.Second).
					Do().Into(&nps)
				Expect(err).NotTo(HaveOccurred())
				time.Sleep(1 * time.Second)
			}
			panic(fmt.Sprintf("Failed to clean up policies: %+v", nps))
		})
	}()

	It("should handle a CRUD of Global Network Policy", func() {
		var kvpRes *model.KVPair

		gnpClient := c.getResourceClientFromResourceKind(apiv3.KindGlobalNetworkPolicy)
		kvp1Name := "my-test-gnp"
		kvp1KeyV1 := model.PolicyKey{Name: kvp1Name}
		kvp1a := &model.KVPair{
			Key: model.ResourceKey{Name: kvp1Name, Kind: apiv3.KindGlobalNetworkPolicy},
			Value: &apiv3.GlobalNetworkPolicy{
				TypeMeta: metav1.TypeMeta{
					Kind:       apiv3.KindGlobalNetworkPolicy,
					APIVersion: apiv3.GroupVersionCurrent,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: kvp1Name,
				},
				Spec: calicoAllowPolicyModelSpec,
			},
		}

		kvp1b := &model.KVPair{
			Key: model.ResourceKey{Name: kvp1Name, Kind: apiv3.KindGlobalNetworkPolicy},
			Value: &apiv3.GlobalNetworkPolicy{
				TypeMeta: metav1.TypeMeta{
					Kind:       apiv3.KindGlobalNetworkPolicy,
					APIVersion: apiv3.GroupVersionCurrent,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: kvp1Name,
				},
				Spec: calicoDisallowPolicyModelSpec,
			},
		}

		kvp2Name := "my-test-gnp2"
		kvp2KeyV1 := model.PolicyKey{Name: kvp2Name}
		kvp2a := &model.KVPair{
			Key: model.ResourceKey{Name: kvp2Name, Kind: apiv3.KindGlobalNetworkPolicy},
			Value: &apiv3.GlobalNetworkPolicy{
				TypeMeta: metav1.TypeMeta{
					Kind:       apiv3.KindGlobalNetworkPolicy,
					APIVersion: apiv3.GroupVersionCurrent,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: kvp2Name,
				},
				Spec: calicoAllowPolicyModelSpec,
			},
		}

		kvp2b := &model.KVPair{
			Key: model.ResourceKey{Name: kvp2Name, Kind: apiv3.KindGlobalNetworkPolicy},
			Value: &apiv3.GlobalNetworkPolicy{
				TypeMeta: metav1.TypeMeta{
					Kind:       apiv3.KindGlobalNetworkPolicy,
					APIVersion: apiv3.GroupVersionCurrent,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: kvp2Name,
				},
				Spec: calicoDisallowPolicyModelSpec,
			},
		}

		// Make sure we clean up after ourselves.  We allow this to fail because
		// part of our explicit testing below is to delete the resource.
		defer func() {
			gnpClient.Delete(ctx, kvp1a.Key, "")
			gnpClient.Delete(ctx, kvp2a.Key, "")
		}()

		// Check our syncer has the correct GNP entries for the two
		// System Network Protocols that this test manipulates.  Neither
		// have been created yet.
		By("Checking cache does not have Global Network Policy entries", func() {
			Eventually(cb.GetSyncerValuePresentFunc(kvp1KeyV1)).Should(BeFalse())
			Eventually(cb.GetSyncerValuePresentFunc(kvp2KeyV1)).Should(BeFalse())
		})

		By("Creating a Global Network Policy", func() {
			var err error
			kvpRes, err = gnpClient.Create(ctx, kvp1a)
			Expect(err).NotTo(HaveOccurred())
		})

		By("Checking cache has correct Global Network Policy entries", func() {
			Eventually(cb.GetSyncerValueFunc(kvp1KeyV1)).Should(Equal(&calicoAllowPolicyModelV1))
			Eventually(cb.GetSyncerValuePresentFunc(kvp2KeyV1)).Should(BeFalse())
		})

		By("Attempting to recreate an existing Global Network Policy", func() {
			_, err := gnpClient.Create(ctx, kvp1a)
			Expect(err).To(HaveOccurred())
		})

		By("Updating an existing Global Network Policy", func() {
			kvp1b.Revision = kvpRes.Revision
			_, err := gnpClient.Update(ctx, kvp1b)
			Expect(err).NotTo(HaveOccurred())
		})

		By("Checking cache has correct Global Network Policy entries", func() {
			Eventually(cb.GetSyncerValueFunc(kvp1KeyV1)).Should(Equal(&calicoDisallowPolicyModelV1))
			Eventually(cb.GetSyncerValuePresentFunc(kvp2a.Key)).Should(BeFalse())
		})

		By("Create another Global Network Policy", func() {
			var err error
			kvpRes, err = gnpClient.Create(ctx, kvp2a)
			Expect(err).NotTo(HaveOccurred())
		})

		By("Checking cache has correct Global Network Policy entries", func() {
			Eventually(cb.GetSyncerValueFunc(kvp1KeyV1)).Should(Equal(&calicoDisallowPolicyModelV1))
			Eventually(cb.GetSyncerValueFunc(kvp2KeyV1)).Should(Equal(&calicoAllowPolicyModelV1))
		})

		By("Updating the Global Network Policy created by Create", func() {
			kvp2b.Revision = kvpRes.Revision
			_, err := gnpClient.Update(ctx, kvp2b)
			Expect(err).NotTo(HaveOccurred())
		})

		By("Checking cache has correct Global Network Policy entries", func() {
			Eventually(cb.GetSyncerValueFunc(kvp1KeyV1)).Should(Equal(&calicoDisallowPolicyModelV1))
			Eventually(cb.GetSyncerValueFunc(kvp2KeyV1)).Should(Equal(&calicoDisallowPolicyModelV1))
		})

		By("Deleted the Global Network Policy created by Apply", func() {
			_, err := gnpClient.Delete(ctx, kvp2a.Key, "")
			Expect(err).NotTo(HaveOccurred())
		})

		By("Checking cache has correct Global Network Policy entries", func() {
			Eventually(cb.GetSyncerValueFunc(kvp1KeyV1)).Should(Equal(&calicoDisallowPolicyModelV1))
			Eventually(cb.GetSyncerValuePresentFunc(kvp2KeyV1)).Should(BeFalse())
		})

		By("Getting a Global Network Policy that does noe exist", func() {
			_, err := c.Get(ctx, model.ResourceKey{Name: "my-non-existent-test-gnp", Kind: apiv3.KindGlobalNetworkPolicy}, "")
			Expect(err).To(HaveOccurred())
		})

		By("Listing a missing Global Network Policy", func() {
			kvps, err := c.List(ctx, model.ResourceListOptions{Name: "my-non-existent-test-gnp", Kind: apiv3.KindGlobalNetworkPolicy}, "")
			Expect(err).ToNot(HaveOccurred())
			Expect(kvps.KVPairs).To(HaveLen(0))
		})

		By("Getting an existing Global Network Policy", func() {
			kvp, err := c.Get(ctx, model.ResourceKey{Name: "my-test-gnp", Kind: apiv3.KindGlobalNetworkPolicy}, "")
			Expect(err).ToNot(HaveOccurred())
			Expect(kvp.Key.(model.ResourceKey).Name).To(Equal("my-test-gnp"))
			Expect(kvp.Value.(*apiv3.GlobalNetworkPolicy).Spec).To(Equal(kvp1b.Value.(*apiv3.GlobalNetworkPolicy).Spec))
		})

		By("Listing all Global Network Policies", func() {
			kvps, err := c.List(ctx, model.ResourceListOptions{Kind: apiv3.KindGlobalNetworkPolicy}, "")
			Expect(err).ToNot(HaveOccurred())
			Expect(kvps.KVPairs).To(HaveLen(1))
			Expect(kvps.KVPairs[len(kvps.KVPairs)-1].Key.(model.ResourceKey).Name).To(Equal("my-test-gnp"))
			Expect(kvps.KVPairs[len(kvps.KVPairs)-1].Value.(*apiv3.GlobalNetworkPolicy).Spec).To(Equal(kvp1b.Value.(*apiv3.GlobalNetworkPolicy).Spec))
		})

		By("Deleting an existing Global Network Policy", func() {
			_, err := gnpClient.Delete(ctx, kvp1a.Key, "")
			Expect(err).NotTo(HaveOccurred())
		})

		By("Checking cache has no Global Network Policy entries", func() {
			Eventually(cb.GetSyncerValuePresentFunc(kvp1KeyV1)).Should(BeFalse())
			Eventually(cb.GetSyncerValuePresentFunc(kvp2KeyV1)).Should(BeFalse())
		})
	})

	It("should handle a CRUD of BGP Peer", func() {
		kvp1a := &model.KVPair{
			Key: model.ResourceKey{
				Name: "10-0-0-1",
				Kind: apiv3.KindBGPPeer,
			},
			Value: &apiv3.BGPPeer{
				TypeMeta: metav1.TypeMeta{
					Kind:       apiv3.KindBGPPeer,
					APIVersion: apiv3.GroupVersionCurrent,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "10-0-0-1",
				},
				Spec: apiv3.BGPPeerSpec{
					PeerIP:   "10.0.0.1",
					ASNumber: numorstring.ASNumber(6512),
				},
			},
		}

		kvp1b := &model.KVPair{
			Key: model.ResourceKey{
				Name: "10-0-0-1",
				Kind: apiv3.KindBGPPeer,
			},
			Value: &apiv3.BGPPeer{
				TypeMeta: metav1.TypeMeta{
					Kind:       apiv3.KindBGPPeer,
					APIVersion: apiv3.GroupVersionCurrent,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "10-0-0-1",
				},
				Spec: apiv3.BGPPeerSpec{
					PeerIP:   "10.0.0.1",
					ASNumber: numorstring.ASNumber(6513),
				},
			},
		}

		kvp2a := &model.KVPair{
			Key: model.ResourceKey{
				Name: "aa-bb-cc",
				Kind: apiv3.KindBGPPeer,
			},
			Value: &apiv3.BGPPeer{
				TypeMeta: metav1.TypeMeta{
					Kind:       apiv3.KindBGPPeer,
					APIVersion: apiv3.GroupVersionCurrent,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "aa-bb-cc",
				},
				Spec: apiv3.BGPPeerSpec{
					PeerIP:   "aa:bb::cc",
					ASNumber: numorstring.ASNumber(6514),
				},
			},
		}

		kvp2b := &model.KVPair{
			Key: model.ResourceKey{
				Name: "aa-bb-cc",
				Kind: apiv3.KindBGPPeer,
			},
			Value: &apiv3.BGPPeer{
				TypeMeta: metav1.TypeMeta{
					Kind:       apiv3.KindBGPPeer,
					APIVersion: apiv3.GroupVersionCurrent,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "aa-bb-cc",
				},
				Spec: apiv3.BGPPeerSpec{
					PeerIP: "aa:bb::cc",
				},
			},
		}

		var kvpRes *model.KVPair
		var err error

		// Make sure we clean up after ourselves.  We allow this to fail because
		// part of our explicit testing below is to delete the resource.
		defer func() {
			c.Delete(ctx, kvp1a.Key, "")
			c.Delete(ctx, kvp2a.Key, "")
		}()

		By("Creating a BGP Peer", func() {
			kvpRes, err = c.Create(ctx, kvp1a)
			Expect(err).NotTo(HaveOccurred())
		})

		By("Attempting to recreate an existing BGP Peer", func() {
			_, err := c.Create(ctx, kvp1a)
			Expect(err).To(HaveOccurred())
		})

		By("Updating an existing BGP Peer", func() {
			kvp1b.Revision = kvpRes.Revision
			_, err := c.Update(ctx, kvp1b)
			Expect(err).NotTo(HaveOccurred())
		})

		By("Create a non-existent BGP Peer", func() {
			kvpRes, err = c.Create(ctx, kvp2a)
			Expect(err).NotTo(HaveOccurred())
		})

		By("Updating the BGP Peer created by Create", func() {
			kvp2b.Revision = kvpRes.Revision
			_, err := c.Update(ctx, kvp2b)
			Expect(err).NotTo(HaveOccurred())
		})

		By("Getting a missing BGP Peer", func() {
			_, err := c.Get(ctx, model.ResourceKey{Name: "1-1-1-1", Kind: apiv3.KindBGPPeer}, "")
			Expect(err).To(HaveOccurred())
		})

		By("Listing a missing BGP Peer", func() {
			kvps, err := c.List(ctx, model.ResourceListOptions{Name: "aa-bb-cc-dd-ee", Kind: apiv3.KindBGPPeer}, "")
			Expect(err).ToNot(HaveOccurred())
			Expect(kvps.KVPairs).To(HaveLen(0))
		})

		By("Listing an explicit BGP Peer", func() {
			kvps, err := c.List(ctx, model.ResourceListOptions{Name: "10-0-0-1", Kind: apiv3.KindBGPPeer}, "")
			Expect(err).ToNot(HaveOccurred())
			Expect(kvps.KVPairs).To(HaveLen(1))
			Expect(kvps.KVPairs[0].Key).To(Equal(kvp1b.Key))
			Expect(kvps.KVPairs[0].Value.(*apiv3.BGPPeer).ObjectMeta.Name).To(Equal(kvp1b.Value.(*apiv3.BGPPeer).ObjectMeta.Name))
			Expect(kvps.KVPairs[0].Value.(*apiv3.BGPPeer).Spec).To(Equal(kvp1b.Value.(*apiv3.BGPPeer).Spec))
		})

		By("Listing all BGP Peers (should be 2)", func() {
			kvps, err := c.List(ctx, model.ResourceListOptions{Kind: apiv3.KindBGPPeer}, "")
			Expect(err).ToNot(HaveOccurred())
			Expect(kvps.KVPairs).To(HaveLen(2))
			keys := []model.Key{}
			vals := []interface{}{}
			for _, k := range kvps.KVPairs {
				keys = append(keys, k.Key)
				vals = append(vals, k.Value.(*apiv3.BGPPeer).Spec)
			}
			Expect(keys).To(ContainElement(kvp1b.Key))
			Expect(keys).To(ContainElement(kvp2b.Key))
			Expect(vals).To(ContainElement(kvp1b.Value.(*apiv3.BGPPeer).Spec))
			Expect(vals).To(ContainElement(kvp2b.Value.(*apiv3.BGPPeer).Spec))

		})

		By("Deleting the BGP Peer created by Create", func() {
			_, err := c.Delete(ctx, kvp2a.Key, "")
			Expect(err).NotTo(HaveOccurred())
		})

		By("Listing all BGP Peers (should now be 1)", func() {
			kvps, err := c.List(ctx, model.ResourceListOptions{Kind: apiv3.KindBGPPeer}, "")
			Expect(err).ToNot(HaveOccurred())
			Expect(kvps.KVPairs).To(HaveLen(1))
			Expect(kvps.KVPairs[0].Key).To(Equal(kvp1b.Key))
			Expect(kvps.KVPairs[0].Value.(*apiv3.BGPPeer).ObjectMeta.Name).To(Equal(kvp1b.Value.(*apiv3.BGPPeer).ObjectMeta.Name))
			Expect(kvps.KVPairs[0].Value.(*apiv3.BGPPeer).Spec).To(Equal(kvp1b.Value.(*apiv3.BGPPeer).Spec))
		})

		By("Deleting an existing BGP Peer", func() {
			_, err := c.Delete(ctx, kvp1a.Key, "")
			Expect(err).NotTo(HaveOccurred())
		})
	})

	It("should handle a CRUD of Node BGP Peer", func() {
		var kvp1a, kvp1b, kvp2a, kvp2b, kvpRes *model.KVPair
		var nodename, peername1, peername2 string

		// Make sure we clean up after ourselves.  We allow this to fail because
		// part of our explicit testing below is to delete the resource.
		defer func() {
			log.Debug("Deleting Node BGP Peers")
			if peers, err := c.List(ctx, model.ResourceListOptions{Kind: apiv3.KindBGPPeer}, ""); err == nil {
				log.WithField("Peers", peers).Debug("Deleting resources")
				for _, peer := range peers.KVPairs {
					log.WithField("Key", peer.Key).Debug("Deleting resource")
					peer.Revision = ""
					_, _ = c.Delete(ctx, peer.Key, "")
				}
			}
		}()

		By("Listing all Nodes to find a suitable Node name", func() {
			nodes, err := c.List(ctx, model.ResourceListOptions{Kind: apiv3.KindNode}, "")
			Expect(err).NotTo(HaveOccurred())
			// Get the hostname so we can make a Get call
			kvp := *nodes.KVPairs[0]
			nodename = kvp.Key.(model.ResourceKey).Name
			peername1 = "bgppeer1"
			peername2 = "bgppeer2"
			kvp1a = &model.KVPair{
				Key: model.ResourceKey{
					Name: peername1,
					Kind: apiv3.KindBGPPeer,
				},
				Value: &apiv3.BGPPeer{
					TypeMeta: metav1.TypeMeta{
						Kind:       apiv3.KindBGPPeer,
						APIVersion: apiv3.GroupVersionCurrent,
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: peername1,
					},
					Spec: apiv3.BGPPeerSpec{
						Node:     nodename,
						PeerIP:   "10.0.0.1",
						ASNumber: numorstring.ASNumber(6512),
					},
				},
			}
			kvp1b = &model.KVPair{
				Key: model.ResourceKey{
					Name: peername1,
					Kind: apiv3.KindBGPPeer,
				},
				Value: &apiv3.BGPPeer{
					TypeMeta: metav1.TypeMeta{
						Kind:       apiv3.KindBGPPeer,
						APIVersion: apiv3.GroupVersionCurrent,
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: peername1,
					},
					Spec: apiv3.BGPPeerSpec{
						Node:     nodename,
						PeerIP:   "10.0.0.1",
						ASNumber: numorstring.ASNumber(6513),
					},
				},
			}
			kvp2a = &model.KVPair{
				Key: model.ResourceKey{
					Name: peername2,
					Kind: apiv3.KindBGPPeer,
				},
				Value: &apiv3.BGPPeer{
					TypeMeta: metav1.TypeMeta{
						Kind:       apiv3.KindBGPPeer,
						APIVersion: apiv3.GroupVersionCurrent,
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: peername2,
					},
					Spec: apiv3.BGPPeerSpec{
						Node:     nodename,
						PeerIP:   "aa:bb::cc",
						ASNumber: numorstring.ASNumber(6514),
					},
				},
			}
			kvp2b = &model.KVPair{
				Key: model.ResourceKey{
					Name: peername2,
					Kind: apiv3.KindBGPPeer,
				},
				Value: &apiv3.BGPPeer{
					TypeMeta: metav1.TypeMeta{
						Kind:       apiv3.KindBGPPeer,
						APIVersion: apiv3.GroupVersionCurrent,
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: peername2,
					},
					Spec: apiv3.BGPPeerSpec{
						Node:   nodename,
						PeerIP: "aa:bb::cc",
					},
				},
			}
		})

		By("Creating a Node BGP Peer", func() {
			var err error
			kvpRes, err = c.Create(ctx, kvp1a)
			Expect(err).NotTo(HaveOccurred())
		})

		By("Attempting to recreate an existing Node BGP Peer", func() {
			_, err := c.Create(ctx, kvp1a)
			Expect(err).To(HaveOccurred())
		})

		By("Updating an existing Node BGP Peer", func() {
			kvp1b.Revision = kvpRes.Revision
			_, err := c.Update(ctx, kvp1b)
			Expect(err).NotTo(HaveOccurred())
		})

		By("Applying a non-existent Node BGP Peer", func() {
			var err error
			kvpRes, err = c.Apply(kvp2a)
			Expect(err).NotTo(HaveOccurred())
		})

		By("Updating the Node BGP Peer created by Apply", func() {
			kvp2b.Revision = kvpRes.Revision
			_, err := c.Apply(kvp2b)
			Expect(err).NotTo(HaveOccurred())
		})

		By("Getting a missing Node BGP Peer (wrong name)", func() {
			_, err := c.Get(ctx, model.ResourceKey{
				Name: "foobar",
				Kind: apiv3.KindBGPPeer,
			}, "")
			Expect(err).To(HaveOccurred())
		})

		By("Listing a missing Node BGP Peer (wrong name)", func() {
			kvps, err := c.List(ctx, model.ResourceListOptions{
				Name: "foobar",
				Kind: apiv3.KindBGPPeer,
			}, "")
			Expect(err).ToNot(HaveOccurred())
			Expect(kvps.KVPairs).To(HaveLen(0))
		})

		By("Listing Node BGP Peers should contain Node name", func() {
			kvps, err := c.List(ctx, model.ResourceListOptions{Kind: apiv3.KindBGPPeer}, "")
			Expect(err).ToNot(HaveOccurred())
			Expect(kvps.KVPairs).To(HaveLen(2))
			for _, kvp := range kvps.KVPairs {
				Expect(kvp.Value.(*apiv3.BGPPeer).Spec.Node).To(Equal(nodename))
			}
		})

		By("Listing an explicit Node BGP Peer", func() {
			kvps, err := c.List(ctx, model.ResourceListOptions{Name: peername1, Kind: apiv3.KindBGPPeer}, "")
			Expect(err).ToNot(HaveOccurred())
			Expect(kvps.KVPairs).To(HaveLen(1))
			Expect(kvps.KVPairs[0].Key).To(Equal(kvp1b.Key))
			Expect(kvps.KVPairs[0].Value.(*apiv3.BGPPeer).ObjectMeta.Name).To(Equal(kvp1b.Value.(*apiv3.BGPPeer).ObjectMeta.Name))
			Expect(kvps.KVPairs[0].Value.(*apiv3.BGPPeer).Spec).To(Equal(kvp1b.Value.(*apiv3.BGPPeer).Spec))
		})

		By("Listing all Node BGP Peers (should be 2)", func() {
			kvps, err := c.List(ctx, model.ResourceListOptions{Kind: apiv3.KindBGPPeer}, "")
			Expect(err).ToNot(HaveOccurred())
			Expect(kvps.KVPairs).To(HaveLen(2))
			keys := []model.Key{}
			vals := []interface{}{}
			for _, k := range kvps.KVPairs {
				keys = append(keys, k.Key)
				vals = append(vals, k.Value.(*apiv3.BGPPeer).Spec)
			}
			Expect(keys).To(ContainElement(kvp1b.Key))
			Expect(keys).To(ContainElement(kvp2b.Key))
			Expect(vals).To(ContainElement(kvp1b.Value.(*apiv3.BGPPeer).Spec))
			Expect(vals).To(ContainElement(kvp2b.Value.(*apiv3.BGPPeer).Spec))
		})

		By("Deleting the Node BGP Peer created by Apply", func() {
			_, err := c.Delete(ctx, kvp2a.Key, "")
			Expect(err).NotTo(HaveOccurred())
		})

		By("Listing all Node BGP Peers (should now be 1)", func() {
			kvps, err := c.List(ctx, model.ResourceListOptions{Kind: apiv3.KindBGPPeer}, "")
			Expect(err).ToNot(HaveOccurred())
			Expect(kvps.KVPairs).To(HaveLen(1))
			Expect(kvps.KVPairs[0].Key).To(Equal(kvp1b.Key))
			Expect(kvps.KVPairs[0].Value.(*apiv3.BGPPeer).ObjectMeta.Name).To(Equal(kvp1b.Value.(*apiv3.BGPPeer).ObjectMeta.Name))
			Expect(kvps.KVPairs[0].Value.(*apiv3.BGPPeer).Spec).To(Equal(kvp1b.Value.(*apiv3.BGPPeer).Spec))
		})

		By("Deleting an existing Node BGP Peer", func() {
			_, err := c.Delete(ctx, kvp1a.Key, "")
			Expect(err).NotTo(HaveOccurred())
		})

		By("Deleting a non-existent Node BGP Peer", func() {
			_, err := c.Delete(ctx, kvp1a.Key, "")
			Expect(err).To(HaveOccurred())
		})
	})

	It("should handle a basic Pod", func() {
		pod := k8sapi.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-syncer-basic-pod",
				Namespace: "default",
			},
			Spec: k8sapi.PodSpec{
				NodeName: "127.0.0.1",
				Containers: []k8sapi.Container{
					{
						Name:    "container1",
						Image:   "busybox",
						Command: []string{"sleep", "3600"},
					},
				},
			},
		}
		_, err := c.clientSet.CoreV1().Pods("default").Create(&pod)
		wepName := "127.0.0.1-k8s-test--syncer--basic--pod-eth0"

		// Make sure we clean up after ourselves.  This might fail if we reach the
		// test below which deletes this pod, but that's OK.
		defer func() {
			log.Warnf("[TEST] Cleaning up test pod: %s", pod.ObjectMeta.Name)
			_ = c.clientSet.CoreV1().Pods("default").Delete(pod.ObjectMeta.Name, &metav1.DeleteOptions{})
		}()
		By("Creating a pod", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		By("Assigning an IP", func() {
			// Update the Pod to have an IP and be running.
			pod.Status.PodIP = "192.168.1.1"
			pod.Status.Phase = k8sapi.PodRunning
			_, err = c.clientSet.CoreV1().Pods("default").UpdateStatus(&pod)
			Expect(err).NotTo(HaveOccurred())
		})

		By("Waiting for the pod to start", func() {
			// Wait up to 120s for pod to start running.
			log.Warnf("[TEST] Waiting for pod %s to start", pod.ObjectMeta.Name)
			for i := 0; i < 120; i++ {
				p, err := c.clientSet.CoreV1().Pods("default").Get(pod.ObjectMeta.Name, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				if p.Status.Phase == k8sapi.PodRunning {
					// Pod is running
					break
				}
				time.Sleep(1 * time.Second)
			}
			p, err := c.clientSet.CoreV1().Pods("default").Get(pod.ObjectMeta.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(p.Status.Phase).To(Equal(k8sapi.PodRunning))
		})

		By("Performing a List() operation", func() {
			// Perform List and ensure it shows up in the Calico API.
			weps, err := c.List(ctx, model.ResourceListOptions{Kind: apiv3.KindWorkloadEndpoint}, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(len(weps.KVPairs)).To(BeNumerically(">", 0))
		})

		By("Performing a List(Name=wepName) operation", func() {
			// Perform List, including a workload Name
			weps, err := c.List(ctx, model.ResourceListOptions{Name: wepName, Namespace: "default", Kind: apiv3.KindWorkloadEndpoint}, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(len(weps.KVPairs)).To(Equal(1))
		})

		By("Performing a Get() operation", func() {
			// Perform a Get and ensure no error in the Calico API.
			wep, err := c.Get(ctx, model.ResourceKey{Name: wepName, Namespace: "default", Kind: apiv3.KindWorkloadEndpoint}, "")
			Expect(err).NotTo(HaveOccurred())
			fmt.Printf("Updating Wep %+v\n", wep.Value.(*apiv3.WorkloadEndpoint).Spec)
			_, err = c.Update(ctx, wep)
			Expect(err).NotTo(HaveOccurred())
		})

		expectedKVP := model.KVPair{
			Key: model.WorkloadEndpointKey{
				Hostname:       "127.0.0.1",
				OrchestratorID: "k8s",
				WorkloadID:     fmt.Sprintf("default/%s", pod.ObjectMeta.Name),
				EndpointID:     "eth0",
			},
		}

		By("Expecting an update with type 'KVUpdated' on the Syncer API", func() {
			cb.ExpectExists([]api.Update{
				{KVPair: expectedKVP, UpdateType: api.UpdateTypeKVUpdated},
			})
		})

		By("Expecting a Syncer snapshot to include the update with type 'KVNew'", func() {
			// Create a new syncer / callback pair so that it performs a snapshot.
			cfg := apiconfig.KubeConfig{K8sAPIEndpoint: "http://localhost:8080"}
			_, snapshotCallbacks, snapshotSyncer := CreateClientAndSyncer(cfg)
			go snapshotCallbacks.ProcessUpdates()
			snapshotSyncer.Start()

			// Expect the snapshot to include workload endpoint with type "KVNew".
			snapshotCallbacks.ExpectExists([]api.Update{
				{KVPair: expectedKVP, UpdateType: api.UpdateTypeKVNew},
			})

		})

		By("Deleting the Pod and expecting the wep to be deleted", func() {
			err = c.clientSet.CoreV1().Pods("default").Delete(pod.ObjectMeta.Name, &metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())
			cb.ExpectDeleted([]model.KVPair{expectedKVP})
		})
	})

	// Add a defer to wait for all pods to clean up.
	defer func() {
		It("should clean up all pods", func() {
			log.Warnf("[TEST] Waiting for pods to tear down")
			pods, err := c.clientSet.CoreV1().Pods("default").List(metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())

			// Wait up to 60s for pod cleanup to occur.
			for i := 0; i < 60; i++ {
				if len(pods.Items) == 0 {
					return
				}
				pods, err = c.clientSet.CoreV1().Pods("default").List(metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				time.Sleep(1 * time.Second)
			}
			panic(fmt.Sprintf("Failed to clean up pods: %+v", pods))
		})
	}()

	It("should error on unsupported List() calls", func() {
		objs, err := c.List(ctx, model.BlockAffinityListOptions{}, "")
		Expect(err).To(HaveOccurred())
		Expect(objs).To(BeNil())
	})

	It("should not error on unsupported List() calls", func() {
		var nodename string
		By("Listing all Nodes to find a suitable Node name", func() {
			nodes, err := c.List(ctx, model.ResourceListOptions{Kind: apiv3.KindNode}, "")
			Expect(err).NotTo(HaveOccurred())
			kvp := *nodes.KVPairs[0]
			nodename = kvp.Key.(model.ResourceKey).Name
		})
		By("Listing all BlockAffinity for a specific Node", func() {
			objs, err := c.List(ctx, model.BlockAffinityListOptions{Host: nodename}, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(len(objs.KVPairs)).To(Equal(1))
		})
	})

	It("should support setting and getting FelixConfig", func() {
		fc := &model.KVPair{
			Key: model.ResourceKey{
				Name: "myfelixconfig",
				Kind: apiv3.KindFelixConfiguration,
			},
			Value: &apiv3.FelixConfiguration{
				TypeMeta: metav1.TypeMeta{
					Kind:       apiv3.KindFelixConfiguration,
					APIVersion: apiv3.GroupVersionCurrent,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "myfelixconfig",
				},
				Spec: apiv3.FelixConfigurationSpec{
					InterfacePrefix: "xali-",
				},
			},
		}
		var updFC *model.KVPair
		var err error

		defer func() {
			// Always make sure we tidy up after ourselves.  Ignore
			// errors since the test itself should delete what it created.
			_, _ = c.Delete(ctx, fc.Key, "")
		}()

		By("creating a new object", func() {
			updFC, err = c.Create(ctx, fc)
			Expect(err).NotTo(HaveOccurred())
			Expect(updFC.Key.(model.ResourceKey).Name).To(Equal("myfelixconfig"))
			// Set the ResourceVersion (since it is auto populated by the Kubernetes datastore) to make it easier to compare objects.
			Expect(fc.Value.(*apiv3.FelixConfiguration).GetObjectMeta().GetResourceVersion()).To(Equal(""))
			fc.Value.(*apiv3.FelixConfiguration).GetObjectMeta().SetResourceVersion(updFC.Value.(*apiv3.FelixConfiguration).GetObjectMeta().GetResourceVersion())
			Expect(updFC.Value.(*apiv3.FelixConfiguration)).To(Equal(fc.Value.(*apiv3.FelixConfiguration)))
			Expect(updFC.Revision).NotTo(BeNil())
			// Unset the ResourceVersion for the original resource since we modified it just for the sake of comparing in the tests.
			fc.Value.(*apiv3.FelixConfiguration).GetObjectMeta().SetResourceVersion("")
		})

		By("getting an existing object", func() {
			updFC, err = c.Get(ctx, fc.Key, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(updFC.Value.(*apiv3.FelixConfiguration).Spec).To(Equal(fc.Value.(*apiv3.FelixConfiguration).Spec))
			Expect(updFC.Key.(model.ResourceKey).Name).To(Equal("myfelixconfig"))
			Expect(updFC.Revision).NotTo(BeNil())
		})

		By("updating an existing object", func() {
			updFC.Value.(*apiv3.FelixConfiguration).Spec.InterfacePrefix = "someotherprefix-"
			updFC, err = c.Update(ctx, updFC)
			Expect(err).NotTo(HaveOccurred())
			Expect(updFC.Value.(*apiv3.FelixConfiguration).Spec.InterfacePrefix).To(Equal("someotherprefix-"))
		})

		By("getting the updated object", func() {
			updFC, err = c.Get(ctx, fc.Key, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(updFC.Value.(*apiv3.FelixConfiguration).Spec.InterfacePrefix).To(Equal("someotherprefix-"))
			Expect(updFC.Key.(model.ResourceKey).Name).To(Equal("myfelixconfig"))
			Expect(updFC.Revision).NotTo(BeNil())
		})

		By("applying an existing object", func() {
			val := &apiv3.FelixConfiguration{
				TypeMeta: metav1.TypeMeta{
					Kind:       apiv3.KindFelixConfiguration,
					APIVersion: apiv3.GroupVersionCurrent,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "myfelixconfig",
				},
				Spec: apiv3.FelixConfigurationSpec{
					InterfacePrefix: "somenewprefix-",
				},
			}
			updFC.Value = val
			updFC, err = c.Apply(updFC)
			Expect(err).NotTo(HaveOccurred())
			Expect(updFC.Value.(*apiv3.FelixConfiguration).Spec.InterfacePrefix).To(Equal("somenewprefix-"))
		})

		By("getting the applied object", func() {
			updFC, err = c.Get(ctx, fc.Key, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(updFC.Value.(*apiv3.FelixConfiguration).Spec.InterfacePrefix).To(Equal("somenewprefix-"))
			Expect(updFC.Key.(model.ResourceKey).Name).To(Equal("myfelixconfig"))
			Expect(updFC.Revision).NotTo(BeNil())
		})

		By("deleting an existing object", func() {
			_, err = c.Delete(ctx, fc.Key, "")
			Expect(err).NotTo(HaveOccurred())
		})

		By("deleting a non-existing object", func() {
			_, err = c.Delete(ctx, fc.Key, "")
			Expect(err).To(HaveOccurred())
		})

		By("getting a non-existing object", func() {
			updFC, err = c.Get(ctx, fc.Key, "")
			Expect(err).To(HaveOccurred())
			Expect(updFC).To(BeNil())
		})

		By("applying a new object", func() {
			// Revision should not be specified when creating.
			fc.Revision = ""
			updFC, err = c.Apply(fc)
			Expect(err).NotTo(HaveOccurred())
			Expect(updFC.Value.(*apiv3.FelixConfiguration).Spec).To(Equal(fc.Value.(*apiv3.FelixConfiguration).Spec))
		})

		By("getting the applied object", func() {
			updFC, err = c.Get(ctx, fc.Key, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(updFC.Value.(*apiv3.FelixConfiguration).Spec).To(Equal(fc.Value.(*apiv3.FelixConfiguration).Spec))
			Expect(updFC.Key.(model.ResourceKey).Name).To(Equal("myfelixconfig"))
			Expect(updFC.Revision).NotTo(BeNil())
		})

		By("deleting the existing object", func() {
			_, err = c.Delete(ctx, updFC.Key, "")
			Expect(err).NotTo(HaveOccurred())
		})
	})

	It("should support setting and getting IP Pools", func() {
		By("listing IP pools when none have been created", func() {
			_, err := c.List(ctx, model.ResourceListOptions{Kind: apiv3.KindIPPool}, "")
			Expect(err).NotTo(HaveOccurred())
		})

		By("creating an IP Pool and getting it back", func() {
			cidr := "192.168.0.0/16"
			pool := &model.KVPair{
				Key: model.ResourceKey{
					Name: "192-16-0-0-16",
					Kind: apiv3.KindIPPool,
				},
				Value: &apiv3.IPPool{
					TypeMeta: metav1.TypeMeta{
						Kind:       apiv3.KindIPPool,
						APIVersion: apiv3.GroupVersionCurrent,
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "192-16-0-0-16",
					},
					Spec: apiv3.IPPoolSpec{
						CIDR:     cidr,
						IPIPMode: apiv3.IPIPModeCrossSubnet,
						Disabled: true,
					},
				},
			}
			_, err := c.Create(ctx, pool)
			Expect(err).NotTo(HaveOccurred())

			receivedPool, err := c.Get(ctx, pool.Key, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(receivedPool.Value.(*apiv3.IPPool).Spec.CIDR).To(Equal(cidr))
			Expect(receivedPool.Value.(*apiv3.IPPool).Spec.IPIPMode).To(BeEquivalentTo(apiv3.IPIPModeCrossSubnet))
			Expect(receivedPool.Value.(*apiv3.IPPool).Spec.Disabled).To(Equal(true))
		})

		By("deleting the IP Pool", func() {
			_, err := c.Delete(ctx, model.ResourceKey{
				Name: "192-16-0-0-16",
				Kind: apiv3.KindIPPool,
			}, "")
			Expect(err).NotTo(HaveOccurred())
		})
	})

	It("Should support getting, deleting, and listing Nodes", func() {
		nodeHostname := ""
		var kvp model.KVPair
		ip := "192.168.0.101"

		By("Listing all Nodes", func() {
			nodes, err := c.List(ctx, model.ResourceListOptions{Kind: apiv3.KindNode}, "")
			Expect(err).NotTo(HaveOccurred())
			// Get the hostname so we can make a Get call
			kvp = *nodes.KVPairs[0]
			nodeHostname = kvp.Key.(model.ResourceKey).Name
		})

		By("Listing a specific Node", func() {
			nodes, err := c.List(ctx, model.ResourceListOptions{Name: nodeHostname, Kind: apiv3.KindNode}, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(nodes.KVPairs).To(HaveLen(1))
			Expect(nodes.KVPairs[0].Key).To(Equal(kvp.Key))
			Expect(nodes.KVPairs[0].Value).To(Equal(kvp.Value))
		})

		By("Listing a specific invalid Node", func() {
			nodes, err := c.List(ctx, model.ResourceListOptions{Name: "foobarbaz-node", Kind: apiv3.KindNode}, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(nodes.KVPairs).To(HaveLen(0))
		})

		By("Getting a specific nodeHostname", func() {
			n, err := c.Get(ctx, model.ResourceKey{Name: nodeHostname, Kind: apiv3.KindNode}, "")
			Expect(err).NotTo(HaveOccurred())

			// Check to see we have the right Node
			Expect(nodeHostname).To(Equal(n.Key.(model.ResourceKey).Name))
		})

		By("Creating a new Node", func() {
			_, err := c.Create(ctx, &kvp)
			Expect(err).To(HaveOccurred())
		})

		By("Getting non-existent Node", func() {
			_, err := c.Get(ctx, model.ResourceKey{Name: "Fake", Kind: apiv3.KindNode}, "")
			Expect(err).To(HaveOccurred())
		})

		By("Deleting a Node", func() {
			_, err := c.Delete(ctx, kvp.Key, "")
			Expect(err).To(HaveOccurred())
		})

		By("Updating changes to a node", func() {
			newAsn := numorstring.ASNumber(23455)

			testKvp := model.KVPair{
				Key: model.ResourceKey{
					Name: kvp.Key.(model.ResourceKey).Name,
					Kind: apiv3.KindNode,
				},
				Value: &apiv3.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: kvp.Key.(model.ResourceKey).Name,
					},
					Spec: apiv3.NodeSpec{
						BGP: &apiv3.NodeBGPSpec{
							ASNumber:    &newAsn,
							IPv4Address: ip,
						},
					},
				},
			}
			node, err := c.Update(ctx, &testKvp)
			Expect(err).NotTo(HaveOccurred())
			Expect(*node.Value.(*apiv3.Node).Spec.BGP.ASNumber).To(Equal(newAsn))

			// Also check that Get() returns the changes
			getNode, err := c.Get(ctx, kvp.Key.(model.ResourceKey), "")
			Expect(err).NotTo(HaveOccurred())
			Expect(*getNode.Value.(*apiv3.Node).Spec.BGP.ASNumber).To(Equal(newAsn))
			Expect(getNode.Value.(*apiv3.Node).Spec.BGP.IPv4IPIPTunnelAddr).To(Equal("10.10.10.1"))

			// We do not support creating Nodes, we should see an error
			// if the Node does not exist.
			missingKvp := model.KVPair{
				Key: model.ResourceKey{
					Name: "IDontExist",
					Kind: apiv3.KindNode,
				},
			}
			_, err = c.Create(ctx, &missingKvp)

			Expect(err).To(HaveOccurred())
		})

		By("Updating a Node", func() {
			testKvp := model.KVPair{
				Key: model.ResourceKey{
					Name: kvp.Key.(model.ResourceKey).Name,
					Kind: apiv3.KindNode,
				},
				Value: &apiv3.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: kvp.Key.(model.ResourceKey).Name,
					},
					Spec: apiv3.NodeSpec{
						BGP: &apiv3.NodeBGPSpec{
							IPv4Address: ip,
						},
					},
				},
			}
			node, err := c.Update(ctx, &testKvp)

			Expect(err).NotTo(HaveOccurred())
			Expect(node.Value.(*apiv3.Node).Spec.BGP.ASNumber).To(BeNil())

			// Also check that Get() returns the changes
			getNode, err := c.Get(ctx, kvp.Key.(model.ResourceKey), "")
			Expect(err).NotTo(HaveOccurred())
			Expect(getNode.Value.(*apiv3.Node).Spec.BGP.ASNumber).To(BeNil())
		})

		By("Syncing HostIPs over the Syncer", func() {
			expectExist := []api.Update{
				{model.KVPair{Key: model.HostIPKey{Hostname: nodeHostname}}, api.UpdateTypeKVUpdated},
			}

			// Expect the snapshot to include the right keys.
			cb.ExpectExists(expectExist)
		})

		By("Not syncing Nodes when K8sDisableNodePoll is enabled", func() {
			cfg := apiconfig.KubeConfig{K8sAPIEndpoint: "http://localhost:8080", K8sDisableNodePoll: true}
			_, snapshotCallbacks, snapshotSyncer := CreateClientAndSyncer(cfg)

			go snapshotCallbacks.ProcessUpdates()
			snapshotSyncer.Start()

			expectNotExist := []model.KVPair{
				{Key: model.HostIPKey{Hostname: nodeHostname}},
			}

			// Expect the snapshot to have not received the update.
			snapshotCallbacks.ExpectDeleted(expectNotExist)
		})

		By("Syncing HostConfig for a Node on Syncer start", func() {
			cfg := apiconfig.KubeConfig{K8sAPIEndpoint: "http://localhost:8080", K8sDisableNodePoll: true}
			_, snapshotCallbacks, snapshotSyncer := CreateClientAndSyncer(cfg)

			go snapshotCallbacks.ProcessUpdates()
			snapshotSyncer.Start()

			hostConfigKey := model.KVPair{
				Key: model.HostConfigKey{
					Hostname: "127.0.0.1",
					Name:     "IpInIpTunnelAddr",
				}}

			expectedKeys := []api.Update{
				api.Update{hostConfigKey, api.UpdateTypeKVNew},
			}

			snapshotCallbacks.ExpectExists(expectedKeys)
		})
	})
})
