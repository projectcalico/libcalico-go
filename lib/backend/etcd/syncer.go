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

package etcd

import (
	"time"

	"math/rand"

	log "github.com/Sirupsen/logrus"
	"github.com/coreos/etcd/client"
	etcd "github.com/coreos/etcd/client"
	"github.com/projectcalico/libcalico-go/lib/backend/api"
	"github.com/projectcalico/libcalico-go/lib/backend/model"
	"github.com/projectcalico/libcalico-go/lib/hwm"
	"golang.org/x/net/context"
)

// defaultEtcdClusterID is default value that an etcd cluster uses if it
// hasn't been bootstrapped with an explicit value.
const defaultEtcdClusterID = "7e27652122e8b2ae"

const clusterIDPollInterval = 10 * time.Second

func newSyncer(keysAPI etcd.KeysAPI, callbacks api.SyncerCallbacks) *etcdSyncer {
	return &etcdSyncer{
		keysAPI:   keysAPI,
		callbacks: callbacks,
	}
}

type etcdSyncer struct {
	callbacks api.SyncerCallbacks
	keysAPI   etcd.KeysAPI
	OneShot   bool
}

func (syn *etcdSyncer) Start() {
	// Start a background thread to read events from etcd.  It will
	// queue events onto the etcdEvents channel.  If it drops out of sync,
	// it will signal on the resyncIndex channel.
	log.Info("Starting etcd Syncer")
	etcdEvents := make(chan interface{}, 20000)
	triggerResync := make(chan uint64, 5)
	initialSnapshotIndex := make(chan uint64)
	if !syn.OneShot {
		log.Info("Syncer not in one-shot mode, starting watcher thread")
		go syn.watchEtcd(etcdEvents, triggerResync, initialSnapshotIndex)
		// In order to make sure that we eventually spot if the etcd
		// cluster is rebuilt, start a thread to poll the etcd
		// Cluster ID.  If we don't spot a cluster rebuild then our
		// watcher will start silently failing.
		go syn.pollClusterID(clusterIDPollInterval)
	}

	// Start a background thread to read snapshots from etcd.  It will
	// read a start-of-day snapshot and then wait to be signalled on the
	// resyncIndex channel.
	snapshotUpdates := make(chan interface{})
	go syn.readSnapshotsFromEtcd(snapshotUpdates, triggerResync, initialSnapshotIndex)
	go syn.mergeUpdates(snapshotUpdates, etcdEvents)
}

const (
	actionSet uint8 = iota
	actionDel
)

// snapshotStarting is the event sent by the snapshot thread to the merge thread when it
// begins processing a snapshot.
type snapshotStarting struct {
	snapshotIndex uint64
}

// snapshotFinished is the event sent by the snapshot thread to the merge thread when it
// finishes processing a snapshot.
type snapshotFinished struct {
	snapshotIndex uint64
}

// snapshotUpdate is the event sent by the snapshot thread when it find a key/value in the
// snapshot.
type snapshotUpdate struct {
	modifiedIndex uint64
	snapshotIndex uint64
	key           string
	value         string
}

// watcherUpdate is sent by the watcher thread to the merge thread when a key is updated.
type watcherUpdate struct {
	modifiedIndex uint64
	key           string
	value         string
}

// watcherDeletion is sent by the watcher thread to the merge thread when a key is removed.
type watcherDeletion struct {
	modifiedIndex uint64
	key           string
}

// watcherNeedsSnapshot is sent by the watcher thread to the merge thread when it drops
// out of sync and it needs a new snapshot.
type watcherNeedsSnapshot struct {
	minSnapshotIndex uint64
}

func (syn *etcdSyncer) readSnapshotsFromEtcd(snapshotUpdates chan<- interface{}, triggerResync <-chan uint64, initialSnapshotIndex chan<- uint64) {
	log.Info("Syncer snapshot-reading thread started")
	getOpts := client.GetOptions{
		Recursive: true,
		Sort:      false,
		Quorum:    false,
	}
	var highestSnapshotIndex uint64
	var minIndex uint64

	for {
		if highestSnapshotIndex > 0 {
			// Wait for the watcher thread to tell us what index
			// it starts from.  We need to load a snapshot with
			// an equal or later index, otherwise we could miss
			// some updates.  (Since we may connect to a follower
			// server, it's possible, if unlikely, for us to read
			// a stale snapshot.)
			minIndex = <-triggerResync
			log.Infof("Asked for snapshot > %v; last snapshot was %v",
				minIndex, highestSnapshotIndex)
			if highestSnapshotIndex >= minIndex {
				// We've already read a newer snapshot, no
				// need to re-read.
				log.Info("Snapshot already new enough")
				continue
			}
		}

	readRetryLoop:
		for {
			resp, err := syn.keysAPI.Get(context.Background(),
				"/calico/v1", &getOpts)
			if err != nil {
				if syn.OneShot {
					// One-shot mode is used to grab a snapshot and then
					// stop.  We don't want to go into a retry loop.
					log.Fatal("Failed to read snapshot from etcd: ", err)
				}
				log.Warning("Error getting snapshot, retrying...", err)
				time.Sleep(1 * time.Second)
				continue readRetryLoop
			}

			if resp.Index < minIndex {
				log.Info("Retrieved stale snapshot, rereading...")
				continue readRetryLoop
			}

			// If we get here, we should have a good
			// snapshot.  Send it to the merge thread.
			snapshotUpdates <- snapshotStarting{
				snapshotIndex: resp.Index,
			}
			sendNode(resp.Node, snapshotUpdates, resp)
			snapshotUpdates <- snapshotFinished{
				snapshotIndex: resp.Index,
			}
			if resp.Index > highestSnapshotIndex {
				if highestSnapshotIndex == 0 {
					initialSnapshotIndex <- resp.Index
					close(initialSnapshotIndex)
				}
				highestSnapshotIndex = resp.Index
			}
			break readRetryLoop
		}
	}
}

func sendNode(node *client.Node, snapshotUpdates chan<- interface{}, resp *client.Response) {
	if !node.Dir {
		snapshotUpdates <- snapshotUpdate{
			key:           node.Key,
			modifiedIndex: node.ModifiedIndex,
			snapshotIndex: resp.Index,
			value:         node.Value,
		}
	} else {
		for _, child := range node.Nodes {
			sendNode(child, snapshotUpdates, resp)
		}
	}
}

func (syn *etcdSyncer) watchEtcd(etcdEvents chan<- interface{}, triggerResync chan<- uint64, initialSnapshotIndex <-chan uint64) {
	log.Info("Watcher started, waiting for initial snapshot index...")
	startIndex := <-initialSnapshotIndex
	log.WithField("index", startIndex).Info("Received initial snapshot index")

	watcherOpts := client.WatcherOptions{
		AfterIndex: startIndex,
		Recursive:  true,
	}
	watcher := syn.keysAPI.Watcher("/calico/v1", &watcherOpts)
	inSync := true
	for {
		resp, err := watcher.Next(context.Background())
		if err != nil {
			switch err := err.(type) {
			case client.Error:
				errCode := err.Code
				if errCode == client.ErrorCodeWatcherCleared ||
					errCode == client.ErrorCodeEventIndexCleared {
					log.Warning("Lost sync with etcd, restarting watcher")
					watcherOpts.AfterIndex = 0
					watcher = syn.keysAPI.Watcher("/calico/v1",
						&watcherOpts)
					inSync = false
					// FIXME, we'll only trigger a resync after the next event
					continue
				} else {
					log.Error("Error from etcd", err)
					time.Sleep(1 * time.Second)
				}
			case *client.ClusterError:
				log.Error("Cluster error from etcd", err)
				time.Sleep(1 * time.Second)
			default:
				panic(err)
			}
		} else {
			var actionType uint8
			switch resp.Action {
			case "set", "compareAndSwap", "update", "create":
				actionType = actionSet
			case "delete", "compareAndDelete", "expire":
				actionType = actionDel
			default:
				panic("Unknown action type")
			}

			node := resp.Node
			if node.Dir && actionType == actionSet {
				// Creation of a directory, we don't care.
				continue
			}
			if !inSync {
				// Tell the merge/snapshot threads that we need a new snapshot.
				// The snapshot needs to be from our index or one lower.
				snapIdx := node.ModifiedIndex - 1
				log.WithField("minSnapshotIdx", snapIdx).Info(
					"Out of sync: asking for snapshot")
				etcdEvents <- watcherNeedsSnapshot{
					minSnapshotIndex: node.ModifiedIndex,
				}
				triggerResync <- snapIdx
				inSync = true
			}

			if actionType == actionSet {
				etcdEvents <- watcherUpdate{
					modifiedIndex: node.ModifiedIndex,
					key:           resp.Node.Key,
					value:         node.Value,
				}
			} else {
				etcdEvents <- watcherDeletion{
					modifiedIndex: node.ModifiedIndex,
					key:           resp.Node.Key,
				}
			}
		}
	}
}

// pollClusterID polls etcd for its current cluster ID.  If the cluster ID changes
// it terminates the process.
func (syn *etcdSyncer) pollClusterID(interval time.Duration) {
	log.Info("Cluster ID poll thread started")
	lastSeenClusterID := ""
	opts := client.GetOptions{}
	for {
		resp, err := syn.keysAPI.Get(context.Background(), "/calico/", &opts)
		if err != nil {
			log.WithError(err).Warn("Failed to poll etcd server cluster ID")
		} else {
			log.WithField("clusterID", resp.ClusterID).Debug(
				"Polled etcd for cluster ID.")
			if lastSeenClusterID == "" {
				log.WithField("clusterID", resp.ClusterID).Info("etcd cluster ID now known")
				lastSeenClusterID = resp.ClusterID
				if resp.ClusterID == defaultEtcdClusterID {
					log.Error("etcd server is using the default cluster ID; " +
						"will not be able to spot if etcd is replaced with " +
						"another cluster using the default cluster ID. " +
						"Pass a unique --initial-cluster-token when creating " +
						"your etcd cluster to set the cluster ID.")
				}
			} else if resp.ClusterID != "" && lastSeenClusterID != resp.ClusterID {
				// The Syncer doesn't currently support this (hopefully rare)
				// scenario.  Terminate the process rather than carry on with
				// possibly out-of-sync etcd index.
				log.WithFields(log.Fields{
					"oldID": lastSeenClusterID,
					"newID": resp.ClusterID,
				}).Fatal("etcd cluster ID changed; must exit.")
			}
		}
		// Jitter by 10% of interval.
		time.Sleep(time.Duration(float64(interval) * (1 + (0.1 * rand.Float64()))))
	}
}

func (syn *etcdSyncer) mergeUpdates(snapshotUpdates <-chan interface{}, watcherUpdates <-chan interface{}) {
	var e interface{}
	var minSnapshotIndex uint64
	hwms := hwm.NewHighWatermarkTracker()

	syn.callbacks.OnStatusUpdated(api.WaitForDatastore)
	for {
		select {
		case e = <-snapshotUpdates:
			log.WithField("event", e).Debug("Snapshot update")
		case e = <-watcherUpdates:
			log.WithField("event", e).Debug("Watcher update")
		}

		var updatedLastSeenIndex, updatedModifiedIndex uint64
		var updatedKey string
		var updatedValue string

		switch e := e.(type) {
		case watcherNeedsSnapshot:
			// Watcher lost sync, need to track deletions until we get a snapshot from
			// after this index.  Note: we might receive snapshot updates before this
			// message but it's OK that we only start tracking deletions now because
			// we only get deletions from the watcher.
			log.Infof("Watcher out-of-sync, starting to track deletions")
			minSnapshotIndex = e.minSnapshotIndex
			syn.callbacks.OnStatusUpdated(api.ResyncInProgress)
		case snapshotStarting:
			// Informational message from the snapshot thread.  Ensures that we get
			// a clear sequence of logs.
			log.WithField("snapshotIndex", e.snapshotIndex).Info("Started receiving snapshot")
		case snapshotFinished:
			// Snapshot is ending, we need to check if this snapshot is new enough to
			// mean that we're really in sync (because the watcher may have fallen
			// out of sync again afterwards).
			logCxt := log.WithFields(log.Fields{
				"snapshotIndex":    e.snapshotIndex,
				"minSnapshotIndex": minSnapshotIndex,
			})
			logCxt.Info("Finished receiving snapshot")
			if e.snapshotIndex >= minSnapshotIndex {
				// Now in sync.
				hwms.StopTrackingDeletions()
				deletedKeys := hwms.DeleteOldKeys(e.snapshotIndex)
				logCxt.WithField("numDeletedKeys", len(deletedKeys)).Info(
					"Snapshot was fresh enough, now in sync.")
				syn.sendDeletions(deletedKeys, e.snapshotIndex)
			} else {
				log.WithFields(log.Fields{
					"snapshotIndex":    e.snapshotIndex,
					"minSnapshotIndex": minSnapshotIndex,
				}).Warn("Snapshot not fresh enough, still out-of-sync.")
			}
			syn.callbacks.OnStatusUpdated(api.InSync)
		case snapshotUpdate:
			// Update from a snapshot.  We update the HWM to be the
			// index of the snapshot, not the modified index.  This
			// lets us spot keys that have been deleted once the snapshot
			// is finished because any keys that were deleted will have
			// last seen indexes that are lower than the snapshot index.
			updatedLastSeenIndex = e.snapshotIndex
			updatedModifiedIndex = e.modifiedIndex
			updatedKey = e.key
			updatedValue = e.value
		case watcherUpdate:
			// Normal watcher update, last-seen equal to the modified
			// index.
			updatedLastSeenIndex = e.modifiedIndex
			updatedModifiedIndex = e.modifiedIndex
			updatedKey = e.key
			updatedValue = e.value
		case watcherDeletion:
			deletedKeys := hwms.StoreDeletion(e.key, e.modifiedIndex)
			log.Debugf("Prefix %v deleted; %v keys",
				e.key, len(deletedKeys))
			syn.sendDeletions(deletedKeys, e.modifiedIndex)
		}

		if updatedLastSeenIndex != 0 {
			// Common update processing, shared between snapshot and
			// watcher updates.
			log.WithFields(log.Fields{
				"indexToStore": updatedLastSeenIndex,
				"modIdx":       updatedModifiedIndex,
				"key":          updatedKey,
				"value":        updatedValue,
			}).Debug("Snapshot/watcher update to store")
			oldIdx := hwms.StoreUpdate(updatedKey, updatedLastSeenIndex)
			if oldIdx < updatedModifiedIndex {
				// Event is newer than value for that key.
				// Send the update.
				var updateType api.UpdateType
				if oldIdx > 0 {
					log.WithField("oldIdx", oldIdx).Debug("Set updates known key")
					updateType = api.UpdateTypeKVUpdated
				} else {
					log.WithField("oldIdx", oldIdx).Debug("Set is a new key")
					updateType = api.UpdateTypeKVNew
				}
				syn.sendUpdate(updatedKey, &updatedValue, updatedModifiedIndex, updateType)
			}
		}
	}
}

func (syn *etcdSyncer) sendUpdate(key string, value *string, revision uint64, updateType api.UpdateType) {
	log.Debugf("Parsing etcd key %#v", key)
	parsedKey := model.KeyFromDefaultPath(key)
	if parsedKey == nil {
		log.Debugf("Failed to parse key %v", key)
		if cb, ok := syn.callbacks.(api.SyncerParseFailCallbacks); ok {
			cb.ParseFailed(key, value)
		}
		return
	}
	log.Debugf("Parsed etcd key: %v", parsedKey)

	var parsedValue interface{}
	var err error
	if value != nil {
		parsedValue, err = model.ParseValue(parsedKey, []byte(*value))
		if err != nil {
			log.Warningf("Failed to parse value for %v: %#v", key, *value)
		}
		log.Debugf("Parsed value: %#v", parsedValue)
	}
	updates := []api.Update{
		{
			KVPair: model.KVPair{
				Key:      parsedKey,
				Value:    parsedValue,
				Revision: revision,
			},
			UpdateType: updateType,
		},
	}
	syn.callbacks.OnUpdates(updates)
}

func (syn *etcdSyncer) sendDeletions(deletedKeys []string, revision uint64) {
	updates := make([]api.Update, 0, len(deletedKeys))
	for _, key := range deletedKeys {
		parsedKey := model.KeyFromDefaultPath(key)
		if parsedKey == nil {
			log.Debugf("Failed to parse key %v", key)
			if cb, ok := syn.callbacks.(api.SyncerParseFailCallbacks); ok {
				cb.ParseFailed(key, nil)
			}
			continue
		}
		updates = append(updates, api.Update{
			KVPair: model.KVPair{
				Key:      parsedKey,
				Value:    nil,
				Revision: revision,
			},
			UpdateType: api.UpdateTypeKVDeleted,
		})
	}
	syn.callbacks.OnUpdates(updates)
}
