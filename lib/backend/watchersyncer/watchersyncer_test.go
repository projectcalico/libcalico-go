// Copyright (c) 2017 Tigera, Inc. All rights reserved.

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

package watchersyncer_test

import (
	"context"
	"errors"
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"

	"github.com/projectcalico/libcalico-go/lib/apiv2"
	"github.com/projectcalico/libcalico-go/lib/backend/api"
	"github.com/projectcalico/libcalico-go/lib/backend/model"
	"github.com/projectcalico/libcalico-go/lib/backend/watchersyncer"
	cerrors "github.com/projectcalico/libcalico-go/lib/errors"
	cnet "github.com/projectcalico/libcalico-go/lib/net"
)

var (
	dsError = cerrors.ErrorDatastoreError{Err: errors.New("Generic datastore error")}
	l1Key1  = model.ResourceKey{
		Kind:      apiv2.KindNetworkPolicy,
		Namespace: "namespace1",
		Name:      "policy-1",
	}
	l1Key2 = model.ResourceKey{
		Kind:      apiv2.KindNetworkPolicy,
		Namespace: "namespace1",
		Name:      "policy-2",
	}
	l1Key3 = model.ResourceKey{
		Kind:      apiv2.KindNetworkPolicy,
		Namespace: "namespace2",
		Name:      "policy-1",
	}
	l1Key4 = model.ResourceKey{
		Kind:      apiv2.KindNetworkPolicy,
		Namespace: "namespace2999",
		Name:      "policy-1000",
	}
	l2Key1 = model.ResourceKey{
		Kind: apiv2.KindIPPool,
		Name: "ippool-1",
	}
	l2Key2 = model.ResourceKey{
		Kind: apiv2.KindIPPool,
		Name: "ippool-2",
	}
	l3Key1 = model.BlockAffinityKey{
		CIDR: cnet.MustParseCIDR("1.2.3.0/24"),
		Host: "mynode",
	}
)

var _ = Describe("Test the backend datstore multi-watch syncer", func() {

	r1 := watchersyncer.ResourceType{
		ListInterface: model.ResourceListOptions{Kind: apiv2.KindNetworkPolicy},
	}
	r2 := watchersyncer.ResourceType{
		ListInterface: model.ResourceListOptions{Kind: apiv2.KindIPPool},
	}
	r3 := watchersyncer.ResourceType{
		ListInterface: model.BlockAffinityListOptions{},
	}

	It("Should receive a sync event when the watchers only provide sync events", func() {
		rs := newRawSyncerTester([]watchersyncer.ResourceType{r1, r2, r3})
		rs.startSyncer()
		rs.expectStatus(api.WaitForDatastore)
		rs.expectWatcherCreated(r1)
		rs.expectWatcherCreated(r2)
		rs.expectWatcherCreated(r3)
		rs.expectStatus(api.WaitForDatastore)
		rs.sendEvent(r1, api.WatchEvent{Type: api.WatchSynced})
		rs.expectStatus(api.ResyncInProgress)
		rs.sendEvent(r2, api.WatchEvent{Type: api.WatchSynced})
		rs.expectStatus(api.ResyncInProgress)
		rs.sendEvent(r3, api.WatchEvent{Type: api.WatchSynced})
		rs.expectStatus(api.InSync)
	})

	It("Should discard multiple synced events", func() {
		rs := newRawSyncerTester([]watchersyncer.ResourceType{r1, r2, r3})
		rs.startSyncer()
		rs.expectStatus(api.WaitForDatastore)
		rs.expectWatcherCreated(r1)
		rs.expectWatcherCreated(r2)
		rs.expectWatcherCreated(r3)
		rs.expectStatus(api.WaitForDatastore)
		rs.sendEvent(r1, api.WatchEvent{Type: api.WatchSynced})
		rs.sendEvent(r1, api.WatchEvent{Type: api.WatchSynced})
		rs.sendEvent(r1, api.WatchEvent{Type: api.WatchSynced})
		rs.sendEvent(r1, api.WatchEvent{Type: api.WatchSynced})
		rs.sendEvent(r1, api.WatchEvent{Type: api.WatchSynced})
		rs.sendEvent(r1, api.WatchEvent{Type: api.WatchSynced})
		rs.expectStatus(api.ResyncInProgress)
		rs.sendEvent(r2, api.WatchEvent{Type: api.WatchSynced})
		rs.sendEvent(r2, api.WatchEvent{Type: api.WatchSynced})
		rs.sendEvent(r2, api.WatchEvent{Type: api.WatchSynced})
		rs.sendEvent(r2, api.WatchEvent{Type: api.WatchSynced})
		rs.sendEvent(r2, api.WatchEvent{Type: api.WatchSynced})
		rs.sendEvent(r2, api.WatchEvent{Type: api.WatchSynced})
		rs.expectStatus(api.ResyncInProgress)
		rs.sendEvent(r3, api.WatchEvent{Type: api.WatchSynced})
		rs.sendEvent(r3, api.WatchEvent{Type: api.WatchSynced})
		rs.sendEvent(r3, api.WatchEvent{Type: api.WatchSynced})
		rs.sendEvent(r3, api.WatchEvent{Type: api.WatchSynced})
		rs.sendEvent(r3, api.WatchEvent{Type: api.WatchSynced})
		rs.sendEvent(r3, api.WatchEvent{Type: api.WatchSynced})
		rs.expectStatus(api.InSync)
	})

	It("Should handle reconnection when all watchers initially failed to be created", func() {
		rs := newRawSyncerTester([]watchersyncer.ResourceType{r1, r2, r3})
		rs.setClientErrorWatchCreation(true)
		rs.startSyncer()
		rs.expectStatus(api.WaitForDatastore)
		rs.expectNumWatchCreationErrors(r1, 2)
		rs.expectNumWatchCreationErrors(r2, 2)
		rs.expectNumWatchCreationErrors(r3, 2)
		rs.expectStatus(api.WaitForDatastore)
		rs.setClientErrorWatchCreation(false)
		rs.expectWatcherCreated(r1)
		rs.expectWatcherCreated(r2)
		rs.expectWatcherCreated(r3)
		rs.expectStatus(api.WaitForDatastore)
		rs.sendEvent(r1, api.WatchEvent{Type: api.WatchSynced})
		rs.sendEvent(r2, api.WatchEvent{Type: api.WatchSynced})
		rs.sendEvent(r3, api.WatchEvent{Type: api.WatchSynced})
		rs.expectStatus(api.InSync)
	})

	It("Should handle reconnection and syncing when the watcher sends a watch terminated error", func() {
		rs := newRawSyncerTester([]watchersyncer.ResourceType{r1, r2, r3})
		rs.startSyncer()
		rs.expectWatcherCreated(r1)
		rs.expectWatcherCreated(r2)
		rs.expectWatcherCreated(r3)
		rs.expectStatus(api.WaitForDatastore)
		rs.sendEvent(r1, api.WatchEvent{Type: api.WatchSynced})
		rs.sendEvent(r2, api.WatchEvent{Type: api.WatchSynced})
		rs.sendEvent(r3, api.WatchEvent{
			Type:  api.WatchError,
			Error: cerrors.ErrorWatchTerminated{Err: dsError},
		})
		rs.expectStatus(api.ResyncInProgress)
		rs.expectPreviousWatcherClosedThenReset(r3)
		rs.expectWatcherCreated(r3)
		rs.sendEvent(r3, api.WatchEvent{Type: api.WatchSynced})
		rs.expectStatus(api.InSync)
	})

	It("Should handle receiving events while one watcher fails and fails to reconnect", func() {
		rs := newRawSyncerTester([]watchersyncer.ResourceType{r1, r2, r3})
		eventL1Added1 := addEvent(l1Key1)
		eventL2Added1 := addEvent(l2Key1)
		eventL2Added2 := addEvent(l2Key2)
		eventL3Added1 := addEvent(l3Key1)
		rs.startSyncer()
		rs.expectWatcherCreated(r1)
		rs.expectWatcherCreated(r2)
		rs.expectWatcherCreated(r3)

		rs.expectStatus(api.WaitForDatastore)
		rs.sendEvent(r1, eventL1Added1)
		rs.expectStatus(api.ResyncInProgress)
		rs.sendEvent(r1, api.WatchEvent{Type: api.WatchSynced})
		rs.expectStatus(api.ResyncInProgress)
		rs.sendEvent(r2, api.WatchEvent{Type: api.WatchSynced})
		rs.setClientErrorWatchCreation(true)
		rs.sendEvent(r3, api.WatchEvent{
			Type:  api.WatchError,
			Error: cerrors.ErrorWatchTerminated{Err: dsError},
		})
		rs.expectStatus(api.ResyncInProgress)
		rs.expectPreviousWatcherClosedThenReset(r3)
		rs.sendEvent(r2, eventL2Added1)
		rs.sendEvent(r2, eventL2Added2)
		rs.expectUpdates([]api.Update{
			{
				KVPair:     *eventL1Added1.New,
				UpdateType: api.UpdateTypeKVNew,
			},
			{
				KVPair:     *eventL2Added1.New,
				UpdateType: api.UpdateTypeKVNew,
			},
			{
				KVPair:     *eventL2Added2.New,
				UpdateType: api.UpdateTypeKVNew,
			},
		})
		rs.setClientErrorWatchCreation(false)
		rs.expectWatcherCreated(r3)
		rs.expectStatus(api.ResyncInProgress)
		rs.sendEvent(r3, eventL3Added1)
		rs.expectUpdates([]api.Update{
			{
				KVPair:     *eventL3Added1.New,
				UpdateType: api.UpdateTypeKVNew,
			},
		})
		rs.sendEvent(r3, api.WatchEvent{Type: api.WatchSynced})
		rs.expectStatus(api.InSync)
	})

	It("Should not resend add events during a resync and should delete stale entries", func() {
		rs := newRawSyncerTester([]watchersyncer.ResourceType{r1})
		eventL1Added1 := addEvent(l1Key1)
		eventL1Added2 := addEvent(l1Key2)
		eventL1Added3 := addEvent(l1Key3)
		eventL1Added4 := addEvent(l1Key4)
		eventL1Modified4 := modifiedEvent(l1Key4)
		eventL1Deleted3 := deleteEvent(l1Key3)
		rs.startSyncer()
		rs.expectWatcherCreated(r1)

		// Perform a mix of add/modified/delete events to populate our cache
		// and trigger some interesting events.
		rs.expectStatus(api.WaitForDatastore)
		rs.sendEvent(r1, eventL1Added1)
		rs.sendEvent(r1, eventL1Added2)
		rs.sendEvent(r1, eventL1Added3)
		rs.sendEvent(r1, eventL1Deleted3)
		rs.sendEvent(r1, eventL1Added4)
		rs.sendEvent(r1, eventL1Modified4)
		rs.expectUpdates([]api.Update{
			{
				KVPair:     *eventL1Added1.New,
				UpdateType: api.UpdateTypeKVNew,
			},
			{
				KVPair:     *eventL1Added2.New,
				UpdateType: api.UpdateTypeKVNew,
			},
			{
				KVPair:     *eventL1Added3.New,
				UpdateType: api.UpdateTypeKVNew,
			},
			{
				// Remove the Value for deleted.
				KVPair: model.KVPair{
					Key: eventL1Deleted3.Old.Key,
				},
				UpdateType: api.UpdateTypeKVDeleted,
			},
			{
				KVPair:     *eventL1Added4.New,
				UpdateType: api.UpdateTypeKVNew,
			},
			{
				KVPair:     *eventL1Modified4.New,
				UpdateType: api.UpdateTypeKVUpdated,
			},
		})

		// Watcher fails, new watcher is instantiated and starts to resync.
		// Get as far as entry (1) which is modified.
		rs.sendEvent(r1, api.WatchEvent{
			Type:  api.WatchError,
			Error: cerrors.ErrorWatchTerminated{Err: dsError},
		})
		rs.expectPreviousWatcherClosedThenReset(r1)
		rs.expectWatcherCreated(r1)
		eventL1Added1_2 := addEvent(l1Key1)
		rs.sendEvent(r1, eventL1Added1_2)
		rs.expectUpdates([]api.Update{
			{
				KVPair:     *eventL1Added1_2.New,
				UpdateType: api.UpdateTypeKVUpdated,
			},
		})

		// Watcher fails again, mid-resync.  Recovers.
		// -  Existing entry (1) is modified again (synced from datastore as an add!):  should get modified event
		// -  Existing entry (2) is unchanged:  no update
		// -  Previously deleted entry (3) added back in:  get an add update
		rs.sendEvent(r1, api.WatchEvent{
			Type:  api.WatchError,
			Error: cerrors.ErrorWatchTerminated{Err: dsError},
		})
		rs.expectPreviousWatcherClosedThenReset(r1)
		rs.expectWatcherCreated(r1)
		eventL1Added1_3 := addEvent(l1Key1)
		rs.sendEvent(r1, eventL1Added1_3)
		rs.sendEvent(r1, eventL1Added2)
		rs.sendEvent(r1, eventL1Added3)
		rs.expectUpdates([]api.Update{
			{
				KVPair:     *eventL1Added1_3.New,
				UpdateType: api.UpdateTypeKVUpdated,
			},
			{
				KVPair:     *eventL1Added3.New,
				UpdateType: api.UpdateTypeKVNew,
			},
		})
		rs.expectStatus(api.ResyncInProgress)

		// Datastore finishes it's sync.  We should receive a delete notification for entry
		// 4 that was not in the resync when we recreated the watcher.
		rs.sendEvent(r1, api.WatchEvent{Type: api.WatchSynced})
		rs.expectStatus(api.InSync)
		rs.expectUpdates([]api.Update{
			{
				KVPair: model.KVPair{
					Key: eventL1Modified4.New.Key,
				},
				UpdateType: api.UpdateTypeKVDeleted,
			},
		})
	})

	It("Should accumulate updates into a single update when the handler thread is blocked", func() {
		rs := newRawSyncerTester([]watchersyncer.ResourceType{r1, r2})
		eventL1Added1 := addEvent(l1Key1)
		eventL2Added1 := addEvent(l2Key1)
		eventL2Added2 := addEvent(l2Key2)
		eventL2Modified1 := modifiedEvent(l2Key1)
		eventL2Modified2 := modifiedEvent(l2Key2)
		eventL2Modified1_2 := modifiedEvent(l2Key1)
		eventL1Delete1 := deleteEvent(l1Key1)

		rs.startSyncer()
		rs.expectWatcherCreated(r1)
		rs.expectWatcherCreated(r2)
		rs.expectStatus(api.WaitForDatastore)
		rs.setBlockUpdateHandling(true)
		rs.sendEvent(r1, eventL1Added1)

		// We should get the first update in a single update message, and then the update handler will block.
		rs.expectOnUpdates([][]api.Update{{
			{
				KVPair:     *eventL1Added1.New,
				UpdateType: api.UpdateTypeKVNew,
			},
		}})

		// Send a few more events.
		rs.sendEvent(r2, eventL2Added1)
		rs.sendEvent(r2, eventL2Added2)
		rs.sendEvent(r2, eventL2Modified1)

		// Pause briefly before unblocking the update thread.
		// We should receive three events in 1 OnUpdate message.
		time.Sleep(100 * time.Millisecond)
		rs.setBlockUpdateHandling(false)
		rs.expectOnUpdates([][]api.Update{{
			{
				KVPair:     *eventL2Added1.New,
				UpdateType: api.UpdateTypeKVNew,
			},
			{
				KVPair:     *eventL2Added2.New,
				UpdateType: api.UpdateTypeKVNew,
			},
			{
				KVPair:     *eventL2Modified1.New,
				UpdateType: api.UpdateTypeKVUpdated,
			},
		}})

		// Block the update process again and send in a Delete and wait for the update.
		rs.setBlockUpdateHandling(true)
		rs.sendEvent(r1, eventL1Delete1)
		rs.expectOnUpdates([][]api.Update{{
			{
				KVPair: model.KVPair{
					Key: eventL1Delete1.Old.Key,
				},
				UpdateType: api.UpdateTypeKVDeleted,
			},
		}})
		rs.expectStatus(api.ResyncInProgress)

		// Send in: update, InSync, update, InSync, update, error, update.
		// The first InSync should be ignored since the system is only InSync when
		// all watchers have reported.  Therefore we should get:
		// -  An OnUpdate with 2 updates
		// -  An Insync update
		// -  An OnUpdate with 1 update
		// -  An error
		// -  An OnUpdate with 1 update
		rs.sendEvent(r2, eventL2Modified1_2)
		rs.sendEvent(r1, api.WatchEvent{Type: api.WatchSynced})
		// Pause a little here because the previous watch event is on a different goroutine so
		// we need to ensure it's been processed.
		time.Sleep(100 * time.Millisecond)
		rs.sendEvent(r2, eventL2Modified1)
		rs.sendEvent(r2, api.WatchEvent{Type: api.WatchSynced})
		rs.sendEvent(r2, eventL2Modified2)
		rs.sendEvent(r2, api.WatchEvent{
			Type: api.WatchError,
			Error: cerrors.ErrorParsingDatastoreEntry{
				RawKey:   "abcdef",
				RawValue: "aabbccdd",
			},
		})
		rs.sendEvent(r2, eventL2Modified1_2)

		time.Sleep(100 * time.Millisecond)
		rs.setBlockUpdateHandling(false)
		rs.expectStatus(api.InSync)
		rs.expectParseError("abcdef", "aabbccdd")
		rs.expectOnUpdates([][]api.Update{
			{
				{
					KVPair:     *eventL2Modified1_2.New,
					UpdateType: api.UpdateTypeKVUpdated,
				},
				{
					KVPair:     *eventL2Modified1.New,
					UpdateType: api.UpdateTypeKVUpdated,
				},
			},
			{
				{
					KVPair:     *eventL2Modified2.New,
					UpdateType: api.UpdateTypeKVUpdated,
				},
			},
			{
				{
					KVPair:     *eventL2Modified1_2.New,
					UpdateType: api.UpdateTypeKVUpdated,
				},
			},
		})
	})

	It("Should invoke the supplied converter to alter the update", func() {
		rc1 := watchersyncer.ResourceType{
			Converter:     &fakeConverter{},
			ListInterface: model.ResourceListOptions{Kind: apiv2.KindNetworkPolicy},
		}

		// Since the fake converter doesn't actually look at the incoming event we can
		// send in arbitrary data.  Block the handler thread and send in 1 event and wait for it -
		// this event will block the update handling process.
		// Send in another 6 events to cover the different branches of the fake converter.
		//
		// See fakeConverter for details on what is returned each invocation.
		rs := newRawSyncerTester([]watchersyncer.ResourceType{rc1})
		rs.startSyncer()
		rs.expectWatcherCreated(rc1)
		rs.expectStatus(api.WaitForDatastore)
		rs.setBlockUpdateHandling(true)
		rs.sendEvent(r1, addEvent(l1Key1))
		rs.expectStatus(api.ResyncInProgress)
		rs.expectOnUpdates([][]api.Update{{{
			KVPair:     *fakeConverterKVP1,
			UpdateType: api.UpdateTypeKVNew, // key: l1Key1
		}}})
		rs.sendEvent(r1, addEvent(l1Key1))
		rs.sendEvent(r1, addEvent(l1Key1))
		rs.sendEvent(r1, addEvent(l1Key1))
		rs.sendEvent(r1, addEvent(l1Key1))
		rs.sendEvent(r1, addEvent(l1Key1))
		rs.sendEvent(r1, addEvent(l1Key1))

		// Pause briefly and then unblock the thread.  The events should be collated
		// except that an error will cause the events to be sent immediately.
		time.Sleep(100 * time.Millisecond)
		rs.setBlockUpdateHandling(false)
		rs.expectOnUpdates([][]api.Update{
			{
				{
					KVPair:     *fakeConverterKVP2,
					UpdateType: api.UpdateTypeKVUpdated, // key: l1Key1
				},
				{
					KVPair:     *fakeConverterKVP3,
					UpdateType: api.UpdateTypeKVNew, // key: l1Key2
				},
			},
			{
				{
					KVPair: model.KVPair{
						Key: fakeConverterKVP4.Key,
					},
					UpdateType: api.UpdateTypeKVDeleted, // key: l1Key2
				},
			},
			{
				{
					KVPair:     *fakeConverterKVP5,
					UpdateType: api.UpdateTypeKVUpdated, // key: l1Key1
				},
				{
					KVPair:     *fakeConverterKVP6,
					UpdateType: api.UpdateTypeKVUpdated, // key: l1Key1
				},
			},
		})

		// We should have received a parse error.
		rs.expectParseError("abcdef", "aabbccdd")

		// Send a deleted event.  We should get a single deletion event for l1Key1 since
		// l1Key2 is already deleted.  We should also get an updated Parse error.
		rs.sendEvent(r1, deleteEvent(l1Key1))
		time.Sleep(100 * time.Millisecond)
		rs.expectOnUpdates([][]api.Update{
			{
				{
					KVPair: model.KVPair{
						Key: l1Key1,
					},
					UpdateType: api.UpdateTypeKVDeleted,
				},
			},
		})
		rs.expectParseError("zzzzz", "xxxxx")

	})
})

var (
	// Test events for the conversion code.
	fakeConverterKVP1 = &model.KVPair{
		Key:      l1Key1,
		Value:    "abcdef",
		Revision: "abcdefg",
	}
	fakeConverterKVP2 = &model.KVPair{
		Key:      l1Key1,
		Value:    "abcdefgh",
		Revision: "abcdfg",
	}
	fakeConverterKVP3 = &model.KVPair{
		Key:      l1Key2,
		Value:    "abcdef",
		Revision: "abcdefg",
	}
	fakeConverterKVP4 = &model.KVPair{
		Key:      l1Key2,
		Revision: "abfdgscdfg",
	}
	fakeConverterKVP5 = &model.KVPair{
		Key:      l1Key1,
		Value:    "abcdeddfgh",
		Revision: "abfdgffffscdfg",
	}
	fakeConverterKVP6 = &model.KVPair{
		Key:      l1Key1,
		Value:    "abcdeddgjdfgjdfgdfgh",
		Revision: "abfdgscdfg",
	}
)

// Fake converter used to cover error and update handling paths.
type fakeConverter struct {
	i int
}

func (fc *fakeConverter) ConvertKVPair(kvp *model.KVPair) ([]*model.KVPair, error) {
	fc.i++
	switch fc.i {
	case 1: // First update used to block update thread in test.
		return []*model.KVPair{
			fakeConverterKVP1,
		}, nil
	case 2: // Second contains two updates.
		return []*model.KVPair{
			fakeConverterKVP2,
			fakeConverterKVP3,
		}, nil
	case 3: // Third contains an error, which will result in the update event.
		return nil, errors.New("Fake error that we should handle gracefully")
	case 4: // Fourth contains event and error, event will be sent and parse error will be stored.
		return []*model.KVPair{
				fakeConverterKVP4,
			}, cerrors.ErrorParsingDatastoreEntry{
				RawKey:   "abcdef",
				RawValue: "aabbccdd",
			}
	case 5: // Fifth contains an update.
		return []*model.KVPair{
			fakeConverterKVP5,
		}, nil
	case 6: // Sixth contains nothing.
		return nil, nil
	case 7: // Seventh contains another update that will be appended to the one in case 5.
		return []*model.KVPair{
			fakeConverterKVP6,
		}, nil
	}
	return nil, nil
}

func (fc *fakeConverter) ConvertKey(model.Key) ([]model.Key, error) {
	return []model.Key{l1Key1, l1Key2}, cerrors.ErrorParsingDatastoreEntry{
		RawKey:   "zzzzz",
		RawValue: "xxxxx",
	}
}

// Create a delete event from a Key. The value types don't need to match the
// Key types since we aren't unmarshaling/marshaling them in this package.
func deleteEvent(key model.Key) api.WatchEvent {
	return api.WatchEvent{
		Type: api.WatchDeleted,
		Old: &model.KVPair{
			Key:      key,
			Value:    uuid.NewV4().String(),
			Revision: uuid.NewV4().String(),
		},
	}
}

// Create an add event from a Key. The value types don't need to match the
// Key types since we aren't unmarshaling/marshaling them in this package.
func addEvent(key model.Key) api.WatchEvent {
	return api.WatchEvent{
		Type: api.WatchAdded,
		New: &model.KVPair{
			Key:      key,
			Value:    uuid.NewV4().String(),
			Revision: uuid.NewV4().String(),
		},
	}
}

// Create a modified event from a Key. The value types don't need to match the
// Key types since we aren't unmarshaling/marshaling them in this package.
func modifiedEvent(key model.Key) api.WatchEvent {
	return api.WatchEvent{
		Type: api.WatchModified,
		New: &model.KVPair{
			Key:      key,
			Value:    uuid.NewV4().String(),
			Revision: uuid.NewV4().String(),
		},
	}
}

// Create a new rawSyncerTester.
func newRawSyncerTester(l []watchersyncer.ResourceType) *rawSyncerTester {
	fc := &fakeClient{
		currentWatchers:  make(map[string]*fakeWatcher, 0),
		previousWatchers: make(map[string]*fakeWatcher, 0),
	}
	sr := &syncerReceiver{
		status:    api.SyncStatus(9),
		updates:   make([]api.Update, 0),
		onUpdates: make([][]api.Update, 0),
	}
	rst := &rawSyncerTester{
		fc:        fc,
		sr:        sr,
		rawSyncer: watchersyncer.New(fc, l, sr),
	}
	return rst
}

// rawSyncerTester is used to create, start and validate a rawSyncer.  It
// contains a number of useful methods used for asserting current state.
type rawSyncerTester struct {
	fc        *fakeClient
	sr        *syncerReceiver
	rawSyncer api.Syncer
}

// Start the syncer.
func (rst *rawSyncerTester) startSyncer() {
	rst.rawSyncer.Start()
}

// Call to test the expected current status.
func (rst *rawSyncerTester) expectStatus(status api.SyncStatus) {
	// Pause a fraction before testing, to make sure we aren't in transition.
	log.Infof("Expecting status of %d (%s)", status, status)
	time.Sleep(10 * time.Millisecond)
	Eventually(rst.sr.getStatus).Should(Equal(status))
}

// Call to test the received and collated current events.  This removes the collated
// updates from the receiver, so the next invocation only needs to specify the subsequent updates.
func (rst *rawSyncerTester) expectUpdates(updates []api.Update) {
	log.Infof("Expecting updates of %v", updates)
	pollInterval := 50 * time.Millisecond
	waitInterval := 20 * pollInterval
	Eventually(rst.sr.getUpdates, waitInterval, pollInterval).Should(Equal(updates))
	rst.sr.removeSyncUpdates(len(updates))
}

// Call to test the actual OnUpdate requests that were expected.
// This removes the onUpdate events from the receiver.
func (rst *rawSyncerTester) expectOnUpdates(onUpdates [][]api.Update) {
	log.Infof("Expecting updates of %v", onUpdates)
	pollInterval := 50 * time.Millisecond
	waitInterval := 20 * pollInterval
	Eventually(rst.sr.getOnUpdates, waitInterval, pollInterval).Should(Equal(onUpdates))
	rst.sr.removeOnUpdates()
}

// Call to test the actual OnUpdate requests that were expected.
// This removes the onUpdate events from the receiver.
func (rst *rawSyncerTester) expectParseError(key, value string) {
	log.Infof("Expecting parse error: %v=%v", key, value)
	pollInterval := 50 * time.Millisecond
	waitInterval := 20 * pollInterval
	Eventually(rst.sr.getErrorRawKey, waitInterval, pollInterval).Should(Equal(key))
	Expect(rst.sr.getErrorRawValue()).To(Equal(value))
}

// Call to send an event via a particular watcher.
func (rst *rawSyncerTester) sendEvent(r watchersyncer.ResourceType, event api.WatchEvent) {
	path := model.ListOptionsToDefaultPathRoot(r.ListInterface)
	log.WithField("Name", path).Infof("Sending event")
	w := rst.fc.getCurrentWatcher(r)
	Expect(w).ToNot(BeNil())
	w.sendEvent(event)
}

// Call to test that the previous watcher was closed.  This resets the previous watcher
// so that we can re-use this test later on.
func (rst *rawSyncerTester) expectPreviousWatcherClosedThenReset(r watchersyncer.ResourceType) {
	path := model.ListOptionsToDefaultPathRoot(r.ListInterface)
	log.WithField("Name", path).Info("Expecting previous watcher closed")
	pollInterval := 50 * time.Millisecond
	waitInterval := 20 * pollInterval
	Eventually(func() bool {
		w := rst.fc.getPreviousWatcher(r)
		if w == nil {
			return false
		}
		if !w.HasTerminated() {
			return false
		}
		rst.fc.removePreviousWatcher(r)
		return true
	}, waitInterval, pollInterval).Should(BeTrue())
}

// Get the current number of watch creation failures.  This tests the watcher
// creation retry processing.  Don't make the num too high since the rawSyncer waits for
// 1s between creation attempts.
func (rst *rawSyncerTester) expectWatcherCreated(r watchersyncer.ResourceType) {
	path := model.ListOptionsToDefaultPathRoot(r.ListInterface)
	log.WithField("Name", path).Info("Expecting watcher created")
	pollInterval := 50 * time.Millisecond
	waitInterval := time.Second + pollInterval
	Eventually(func() int {
		w := rst.fc.getCurrentWatcher(r)
		if w == nil {
			return -1
		}
		return w.getNumWatchCreationErrors()
	}, waitInterval, pollInterval).Should(Equal(0))
}

// Get the current number of watch creation failures.  This tests the watcher
// creation retry processing.  Don't make the num too high since the rawSyncer waits for
// 1s between creation attempts.
func (rst *rawSyncerTester) expectNumWatchCreationErrors(r watchersyncer.ResourceType, num int) {
	path := model.ListOptionsToDefaultPathRoot(r.ListInterface)
	log.WithField("Name", path).Info("Expecting watch creation errors")
	pollInterval := 50 * time.Millisecond
	waitInterval := time.Duration(num)*time.Second + pollInterval
	Eventually(func() int {
		w := rst.fc.getCurrentWatcher(r)
		if w == nil {
			return -1
		}
		return w.getNumWatchCreationErrors()
	}, waitInterval, pollInterval).Should(BeNumerically(">=", num))
}

// Tell the client to start failing to create watchers.
func (rst *rawSyncerTester) setClientErrorWatchCreation(fail bool) {
	log.WithField("Fail", fail).Info("Setting client watch creation fail flag")
	rst.fc.setClientErrorWatchCreation(fail)
}

// Tell the receiver to block or unblock its status processing.
func (rst *rawSyncerTester) setBlockUpdateHandling(block bool) {
	rst.sr.setBlockUpdateHandling(block)
}

// syncerReceiver implements the api.SyncerCallbacks and api.SyncerParseFailCallbacks
// interface and stores the events.  Test code should access this state through the
// rawSyncerTester.
type syncerReceiver struct {
	lock          sync.Mutex
	status        api.SyncStatus
	onUpdates     [][]api.Update
	updates       []api.Update
	errorRawKey   string
	errorRawValue string
	wg            sync.WaitGroup
}

func (s *syncerReceiver) OnStatusUpdated(status api.SyncStatus) {
	log.WithField("Status", status).Info("OnStatusUpdates called - may be blocked")
	s.lock.Lock()
	s.status = status
	s.lock.Unlock()
	log.Info("OnStatusUpdates processing complete")
}

func (s *syncerReceiver) OnUpdates(updates []api.Update) {
	log.WithField("Updates", updates).Info("OnUpdates called - may be blocked")
	s.lock.Lock()
	s.updates = append(s.updates, updates...)
	s.onUpdates = append(s.onUpdates, updates)
	s.lock.Unlock()

	// We've updated the data and released the lock, but we may need to block.
	s.wg.Wait()
	log.Info("OnUpdates processing complete")
}

func (s *syncerReceiver) ParseFailed(rawKey string, rawValue string) {
	log.WithFields(log.Fields{
		"Key":   rawKey,
		"Value": rawValue,
	}).Info("ParseFailed called")
	s.lock.Lock()
	defer s.lock.Unlock()
	s.errorRawKey = rawKey
	s.errorRawValue = rawValue
}

// Block or unblock the status and update processing handler.
func (s *syncerReceiver) setBlockUpdateHandling(block bool) {
	if block {
		log.Info("Blocking update handling thread")
		s.wg.Add(1)
	} else {
		log.Info("Unblocking update handling thread")
		s.wg.Done()
	}
}

// Get the last reported status.
func (s *syncerReceiver) getStatus() api.SyncStatus {
	s.lock.Lock()
	defer s.lock.Unlock()
	log.WithField("Status", s.status).Info("getStatus()")
	return s.status
}

// Get the raw error key from the last parse error.
func (s *syncerReceiver) getErrorRawKey() string {
	s.lock.Lock()
	defer s.lock.Unlock()
	log.WithField("errorRawKey", s.errorRawKey).Info("getErrorRawKey()")
	return s.errorRawKey
}

// Get the raw error value from the last parse error.
func (s *syncerReceiver) getErrorRawValue() string {
	s.lock.Lock()
	defer s.lock.Unlock()
	log.WithField("errorRawValue", s.errorRawValue).Info("getErrorRawValue()")
	return s.errorRawValue
}

// Get a copy of the current set of updates.
func (s *syncerReceiver) getUpdates() []api.Update {
	s.lock.Lock()
	defer s.lock.Unlock()
	log.WithField("Updates", s.updates).Info("getUpdates()")
	return s.updates[:]
}

// Get the separate onUpdate sets we received.
func (s *syncerReceiver) getOnUpdates() [][]api.Update {
	s.lock.Lock()
	defer s.lock.Unlock()
	log.WithField("onUpdates", s.onUpdates).Info("getOnUpdates()")
	return s.onUpdates[:]
}

// Remove the current set of onUpdate messages we received.
func (s *syncerReceiver) removeOnUpdates() {
	s.lock.Lock()
	defer s.lock.Unlock()
	log.Info("removeOnUpdates()")
	s.onUpdates = make([][]api.Update, 0)
}

// Remove the first <num> entries from the updates - this is used to clear out updates that have
// already been validated.  We also reset the number of on update messages we received - this is
// only a valid thing to do if there are no more messages being receive (so test code beware!).
func (s *syncerReceiver) removeSyncUpdates(num int) {
	s.lock.Lock()
	defer s.lock.Unlock()
	log.WithField("Num", num).Info("removeSyncUpdates()")
	s.updates = s.updates[num:]
}

// fakeClient implements the api.Client interface.  We mock this out so that we can control
// the events
type fakeClient struct {
	lock               sync.Mutex
	errorWatchCreation bool
	currentWatchers    map[string]*fakeWatcher
	previousWatchers   map[string]*fakeWatcher
}

// We don't implement any of the CRUD related methods, just the Watch method to return
// a fake watcher that the test code will drive.
func (c *fakeClient) Create(ctx context.Context, object *model.KVPair) (*model.KVPair, error) {
	panic("should not be called")
	return nil, nil
}
func (c *fakeClient) Update(ctx context.Context, object *model.KVPair) (*model.KVPair, error) {
	panic("should not be called")
	return nil, nil
}
func (c *fakeClient) Apply(object *model.KVPair) (*model.KVPair, error) {
	panic("should not be called")
	return nil, nil
}
func (c *fakeClient) Delete(ctx context.Context, key model.Key, revision string) (*model.KVPair, error) {
	panic("should not be called")
	return nil, nil
}
func (c *fakeClient) Get(ctx context.Context, key model.Key, revision string) (*model.KVPair, error) {
	panic("should not be called")
	return nil, nil
}
func (c *fakeClient) List(ctx context.Context, list model.ListInterface, revision string) (*model.KVPairList, error) {
	panic("should not be called")
	return nil, nil
}
func (c *fakeClient) Syncer(callbacks api.SyncerCallbacks) api.Syncer {
	panic("should not be called")
	return nil
}
func (c *fakeClient) EnsureInitialized() error {
	panic("should not be called")
	return nil
}
func (c *fakeClient) Clean() error {
	panic("should not be called")
	return nil
}

func (c *fakeClient) Watch(ctx context.Context, list model.ListInterface, revision string) (api.WatchInterface, error) {
	// Create a fake watcher keyed off the ListOptions (root path).
	c.lock.Lock()
	defer c.lock.Unlock()
	path := model.ListOptionsToDefaultPathRoot(list)
	log.WithField("Name", path).Info("Creating new watcher")

	if cur, ok := c.currentWatchers[path]; ok {
		// If we are testing watch instantiation errors, then first check to see if the current watch
		// is an errored watcher and if so increment it's count and then return an error.
		if c.errorWatchCreation {
			numErrs := cur.getNumWatchCreationErrors()
			if numErrs > 0 {
				log.WithField("Name", path).Info("Failed to create watcher; incrementing count on existing failed watcher")
				cur.incrementNumCreationErrors()
				return nil, dsError
			}
		}

		// We are creating a new watcher, so store the old one so that we can check its state.
		c.previousWatchers[path] = cur
	}

	// Create a new watcher either to track instantiation errors or to actually use as
	// a fake watcher.
	if c.errorWatchCreation {
		c.currentWatchers[path] = &fakeWatcher{
			name: path,
			numWatchCreationErrors: 1,
		}
		log.WithField("Name", path).Info("Failed to create watcher")
		return nil, dsError
	}

	c.currentWatchers[path] = &fakeWatcher{
		name:    path,
		results: make(chan api.WatchEvent, 100),
	}
	return c.currentWatchers[path], nil
}

// Controls whether the mock client will error when attempting to create a Watch client.
func (c *fakeClient) setClientErrorWatchCreation(fail bool) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.errorWatchCreation = fail
}

// Return the current watcher data for the specified ListInterface.
func (c *fakeClient) getCurrentWatcher(r watchersyncer.ResourceType) *fakeWatcher {
	path := model.ListOptionsToDefaultPathRoot(r.ListInterface)
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.currentWatchers[path]
}

// Return the previous watcher data for the specified ListInterface.  This is updated
// from the current watcher data when the watcher is closed.
func (c *fakeClient) getPreviousWatcher(r watchersyncer.ResourceType) *fakeWatcher {
	path := model.ListOptionsToDefaultPathRoot(r.ListInterface)
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.previousWatchers[path]
}

// Remove the previous watcher data.  This is called once the previous watcher has
// been validated.
func (c *fakeClient) removePreviousWatcher(r watchersyncer.ResourceType) {
	path := model.ListOptionsToDefaultPathRoot(r.ListInterface)
	c.lock.Lock()
	defer c.lock.Unlock()
	delete(c.previousWatchers, path)
}

// fakeWatcher is used both to track watcher instantation errors, and to act as a
// fake watcher that can be driven by the test code.
type fakeWatcher struct {
	name                   string
	lock                   sync.Mutex
	numWatchCreationErrors int
	results                chan api.WatchEvent
	stopCalled             bool
}

func (fw *fakeWatcher) Stop() {
	log.WithField("Name", fw.name).Info("Stop called on watcher")
	fw.lock.Lock()
	defer fw.lock.Unlock()
	fw.stopCalled = true
}

func (fw *fakeWatcher) ResultChan() <-chan api.WatchEvent {
	// This should never be called for a failed watcher.
	Expect(fw.results).ToNot(BeNil())
	return fw.results
}

// Send an event for this watcher.
func (fw *fakeWatcher) sendEvent(e api.WatchEvent) {
	log.WithField("Name", fw.name).Infof("Send event %s", e.Type)
	fw.results <- e
}

// Increment the number of creation errors for this watcher type.
func (fw *fakeWatcher) incrementNumCreationErrors() {
	fw.lock.Lock()
	defer fw.lock.Unlock()
	fw.numWatchCreationErrors++
	log.WithField("Name", fw.name).Infof("Incrementing failed count to %d", fw.numWatchCreationErrors)
}

// Get the number of creation errors for this watcher type.
func (fw *fakeWatcher) getNumWatchCreationErrors() int {
	fw.lock.Lock()
	defer fw.lock.Unlock()
	log.WithField("Name", fw.name).Infof("getNumWatchCreationErrors called on watcher: %v", fw.numWatchCreationErrors)
	return fw.numWatchCreationErrors
}

// HasTerminated returns true if the watcher has terminated and released all
// resources.  This is used for test purposes.
func (fw *fakeWatcher) HasTerminated() bool {
	fw.lock.Lock()
	defer fw.lock.Unlock()
	log.WithField("Name", fw.name).Infof("HasTerminated called on watcher: %v", fw.stopCalled)
	return fw.stopCalled
}
