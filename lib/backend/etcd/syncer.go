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
	"net"
)

// defaultEtcdClusterID is default value that an etcd cluster uses if it
// hasn't been bootstrapped with an explicit value.  We warn if we detect that
// case because it implies that the cluster hasn't been properly bootstrapped
// for production.
const defaultEtcdClusterID = "7e27652122e8b2ae"

const clusterIDPollInterval = 10 * time.Second

type actionType uint8

const (
	actionTypeUnknown actionType = iota
	actionSetOrUpdate
	actionDeletion
)

var (
	snapshotGetOpts = client.GetOptions{
		Recursive: true,
	}
	etcdActionToSyncerAction = map[string]actionType{
		"set":            actionSetOrUpdate,
		"compareAndSwap": actionSetOrUpdate,
		"update":         actionSetOrUpdate,
		"create":         actionSetOrUpdate,

		"delete":           actionDeletion,
		"compareAndDelete": actionDeletion,
		"expire":           actionDeletion,
	}
)

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

// Start starts the syncer's background threads.
func (syn *etcdSyncer) Start() {
	// Start a background thread to read events from etcd.  It will
	// queue events onto the etcdEvents channel.  If it drops out of sync,
	// it will signal on the resyncIndex channel.
	log.Info("Starting etcd Syncer")

	watcherUpdateC := make(chan interface{}, 20000)
	if !syn.OneShot {
		log.Info("Syncer not in one-shot mode, starting watcher thread")
		go syn.watchEtcd(watcherUpdateC)
		// In order to make sure that we eventually spot if the etcd
		// cluster is rebuilt, start a thread to poll the etcd
		// Cluster ID.  If we don't spot a cluster rebuild then our
		// watcher will start silently failing.
		go syn.pollClusterID(clusterIDPollInterval)
	}

	// Start a background thread to read snapshots from etcd.  It will
	// read a start-of-day snapshot and then wait to be signalled on the
	// resyncIndex channel.
	snapshotUpdateC := make(chan interface{})
	snapshotRequestC := make(chan snapshotRequest)
	go syn.readSnapshotsFromEtcd(snapshotUpdateC, snapshotRequestC)
	go syn.mergeUpdates(snapshotUpdateC, watcherUpdateC, snapshotRequestC)
}

// readSnapshotsFromEtcd is a goroutine that, when requested, reads a new
// snapshot from etcd and send it to the merge thread.
func (syn *etcdSyncer) readSnapshotsFromEtcd(
	snapshotUpdateC chan<- interface{},
	snapshotRequestC <-chan snapshotRequest,
) {
	log.Info("Syncer snapshot-reading thread started")

	var highestCompletedSnapshotIndex uint64
	var minRequiredSnapshotIndex uint64

	for {
		// Wait to be told to get the snapshot
		snapshotRequest := <-snapshotRequestC
		minRequiredSnapshotIndex = snapshotRequest.minRequiredSnapshotIndex
		log.WithField("newMinIndex", minRequiredSnapshotIndex).Info("Asked for new snapshot")

		for highestCompletedSnapshotIndex < minRequiredSnapshotIndex {
			log.WithFields(log.Fields{
				"requiredIdx": minRequiredSnapshotIndex,
				"currentIdx":  highestCompletedSnapshotIndex,
			}).Info("Newest snapshot is too stale, loading a new one")
			resp, err := syn.keysAPI.Get(context.Background(), "/calico/v1", &snapshotGetOpts)
			if err != nil {
				if syn.OneShot {
					// One-shot mode is used to grab a snapshot and then
					// stop.  We don't want to go into a retry loop.
					log.Fatal("Failed to read snapshot from etcd: ", err)
				}
				log.Warning("Error getting snapshot, retrying...", err)
				time.Sleep(1 * time.Second)
				continue
			}
			if resp.Index < minRequiredSnapshotIndex {
				log.Info("Retrieved stale snapshot, rereading...")
				time.Sleep(100 * time.Millisecond)
				continue
			}

			// If we get here, we should have a good
			// snapshot.  Send it to the merge thread.
			snapshotUpdateC <- snapshotStarting{
				snapshotIndex: resp.Index,
			}
			sendSnapshotNode(resp.Node, snapshotUpdateC, resp)
			highestCompletedSnapshotIndex = resp.Index
			snapshotUpdateC <- snapshotFinished{
				snapshotIndex: resp.Index,
			}
		}
	}
}

// sendSnapshotNode sends the node and its children over the channel as events.
func sendSnapshotNode(node *client.Node, snapshotUpdates chan<- interface{}, resp *client.Response) {
	if !node.Dir {
		snapshotUpdates <- snapshotUpdate{
			key:           node.Key,
			modifiedIndex: node.ModifiedIndex,
			snapshotIndex: resp.Index,
			value:         node.Value,
		}
	} else {
		for _, child := range node.Nodes {
			sendSnapshotNode(child, snapshotUpdates, resp)
		}
	}
}

// watchEtcd is a goroutine that watches etcd, sending events to the merge
// thread when keys change or it loses sync.
func (syn *etcdSyncer) watchEtcd(watcherUpdateC chan<- interface{}) {
	log.Info("etcd watch thread started.")
	// Each trip around the outer loop establishes the current etcd index
	// of the cluster, triggers a new snapshot read from that index and then
	// starts watching from that index.
	for {
		// Do a non-recursive get on the Ready flag to find out the
		// current etcd index.  We'll trigger a snapshot/start polling from that.
		resp, err := syn.keysAPI.Get(context.Background(), "/calico/v1/Ready", &client.GetOptions{})
		if err != nil {
			log.WithError(err).Warn("Failed to get Ready key from etcd")
			time.Sleep(1 * time.Second)
			continue
		}
		initialClusterIndex := resp.Index
		log.WithField("index", initialClusterIndex).Info("Polled etcd for initial watch index.")

		// We were previously out-of-sync, request a new snapshot at
		// the current cluster index, which is also the index that we'll
		// poll from.
		watcherUpdateC <- watcherNeedsSnapshot{
			minSnapshotIndex: initialClusterIndex,
		}

		// Create the watcher.
		watcherOpts := client.WatcherOptions{
			AfterIndex: initialClusterIndex,
			Recursive:  true,
		}
		watcher := syn.keysAPI.Watcher("/calico/v1", &watcherOpts)
	watchLoop:
		for {
			// Wait for the next event from the watcher.
			resp, err := watcher.Next(context.Background())
			if err != nil {
				if !retryableWatcherError(err) {
					// Break out of the loop to trigger a new resync.
					log.WithError(err).Warning("Lost sync with etcd, restarting watcher.")
					break watchLoop
				}
				// Retryable error, just retry the read.
				log.WithError(err).Debug("Retryable error from etcd")
				continue watchLoop
			}

			// Successful read, interpret the event.
			actionType := etcdActionToSyncerAction[resp.Action]
			if actionType == actionTypeUnknown {
				log.WithField("actionType", resp.Action).Panic("Unknown action type")
			}

			node := resp.Node
			if node.Dir && actionType == actionSetOrUpdate {
				// Creation of a directory, we don't care.
				log.WithField("dir", node.Key).Debug("Ignoring directory creation")
				continue
			}

			if actionType == actionSetOrUpdate {
				watcherUpdateC <- watcherUpdate{
					modifiedIndex: node.ModifiedIndex,
					key:           resp.Node.Key,
					value:         node.Value,
				}
			} else {
				watcherUpdateC <- watcherDeletion{
					modifiedIndex: node.ModifiedIndex,
					key:           resp.Node.Key,
				}
			}

		}
	}
}

// retryableWatcherError returns true if the given etcd error is worth
// retrying in the context of a watch.
func retryableWatcherError(err error) bool {
	// Unpack any nested errors.
	var errs []error
	if clusterErr, ok := err.(*client.ClusterError); ok {
		errs = clusterErr.Errors
	} else {
		errs = []error{err}
	}
	for _, err = range errs {
		switch err := err.(type) {
		case client.Error:
			errCode := err.Code
			if errCode == client.ErrorCodeWatcherCleared ||
				errCode == client.ErrorCodeEventIndexCleared {
				// This means that our watch has failed and needs
				// to be restarted.
				return false
			}
		case net.Error:
			// We expect timeouts if there are no events from etcd
			// so only log if we hit something unusual.
			if !err.Timeout() {
				log.WithError(err).Warn("Net error from etcd")
			}
		default:
			log.WithError(err).Warn("Unexpected error type from etcd")
		}
	}
	// Didn't find any non-retryable errors.
	return true
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

// mergeUpdates is a goroutine that processes updates from the snapshot and wathcer threads,
// merging them into an eventually-consistent stream of updates.
//
// The merging includes resolving deletions where the watcher may be ahead of the snapshot
// and delete a key that later arrives in a snapshot.  The key would then be suppressed
// and no update generated.
//
// It also handles deletions due to a resync by doing a mark and sweep of keys that are seen
// in the snapshot.
//
// Thread safety:  mergeUpdates both sends to and receives from channels to the snapshot
// reading thread.  Thread safety is ensured by tracking whether a snapshot is in progress
// and waiting until it finishes (and hence the snapshot thread is no longer sending)
// before sending it a request for a new snapshot.
func (syn *etcdSyncer) mergeUpdates(
	snapshotUpdateC <-chan interface{},
	watcherUpdateC <-chan interface{},
	snapshotRequestC chan<- snapshotRequest,
) {
	var minRequiredSnapshotIndex uint64
	var highestCompletedSnapshotIndex uint64
	snapshotInProgress := false
	hwms := hwm.NewHighWatermarkTracker()

	syn.callbacks.OnStatusUpdated(api.WaitForDatastore)
	for {
		var event interface{}
		select {
		case event = <-snapshotUpdateC:
			log.WithField("event", event).Debug("Snapshot update")
		case event = <-watcherUpdateC:
			log.WithField("event", event).Debug("Watcher update")
		}

		var updatedLastSeenIndex, updatedModifiedIndex uint64
		var updatedKey string
		var updatedValue string

		switch e := event.(type) {
		case watcherNeedsSnapshot:
			// Watcher has lost sync.  Record the snapshot index
			// that we now require to bring us into sync.  We'll start
			// a new snapshot below if we can.
			log.Info("Watcher out-of-sync, starting to track deletions")
			minRequiredSnapshotIndex = e.minSnapshotIndex
		case snapshotStarting:
			// Informational message from the snapshot thread.  Makes the logs clearer.
			log.WithField("snapshotIndex", e.snapshotIndex).Info("Started receiving snapshot")
		case snapshotFinished:
			// Snapshot is ending, we need to check if this snapshot is still new enough to
			// mean that we're really in sync (because the watcher may have fallen
			// out of sync again after it requested the snapshot).
			logCxt := log.WithFields(log.Fields{
				"snapshotIndex":    e.snapshotIndex,
				"minSnapshotIndex": minRequiredSnapshotIndex,
			})
			logCxt.Info("Finished receiving snapshot, cleaning up old keys.")
			snapshotInProgress = false
			hwms.StopTrackingDeletions()
			deletedKeys := hwms.DeleteOldKeys(e.snapshotIndex)
			logCxt.WithField("numDeletedKeys", len(deletedKeys)).Info(
				"Deleted old keys that weren't seen in snapshot.")
			syn.sendDeletions(deletedKeys, e.snapshotIndex)

			if e.snapshotIndex >= minRequiredSnapshotIndex {
				// Now in sync.
				logCxt.Info("Snapshot brought us into sync.")
				syn.callbacks.OnStatusUpdated(api.InSync)
			} else {
				// Watcher is already out-of-sync.  We'll restart the
				// snapshot below.
				logCxt.Warn("Snapshot was stale before it finished.")
			}
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
			log.WithFields(log.Fields{
				"prefix":  e.key,
				"numKeys": len(deletedKeys),
			}).Debug("Prefix deleted")
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

		if !snapshotInProgress && highestCompletedSnapshotIndex < minRequiredSnapshotIndex {
			log.Info("Watcher is out-of-sync but no snapshot in progress, starting one.")
			snapshotRequestC <- snapshotRequest{
				minRequiredSnapshotIndex: minRequiredSnapshotIndex,
			}
			// Track that the snapshot is in progress; it's not safe to start a
			// new snapshot until the old one finishes (or we'll deadlock with the
			// snapshot trying to send us updates).
			snapshotInProgress = true
			hwms.StartTrackingDeletions()
			syn.callbacks.OnStatusUpdated(api.ResyncInProgress)
		}
	}
}

// sendUpdate parses and sends an update to the callback API.
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

// sendDeletions sends a series of deletions on the callback API.
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

// snapshotRequest is sent by the merge thread to the snapshot thread when a new snapshot
// is required.  Thread safety: the merge thead should only send this message when the
// snapshot thread is quiesced, I.e. after it receives teh snapshotFinished message from
// the previous snapshot.
type snapshotRequest struct {
	minRequiredSnapshotIndex uint64
}

// watcherNeedsSnapshot is sent by the watcher thread to the merge thread when it drops
// out of sync and it needs a new snapshot.
type watcherNeedsSnapshot struct {
	minSnapshotIndex uint64
}
