// Copyright (c) 2017 Tigera, Inc. All rights reserved.
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
	"net"
	"time"

	"math/rand"

	log "github.com/Sirupsen/logrus"
	"github.com/coreos/etcd/client"
	etcdv3 "github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"github.com/projectcalico/libcalico-go/lib/backend/api"
	"github.com/projectcalico/libcalico-go/lib/backend/model"
	"github.com/projectcalico/libcalico-go/lib/hwm"
	"golang.org/x/net/context"
)

func newSyncerV3(etcdClient *etcdv3.Client, callbacks api.SyncerCallbacks) *etcdSyncerV3 {
	return &etcdSyncerV3{
		etcdClient: etcdClient,
		callbacks:  callbacks,
	}
}

// etcdSyncerV3 loads snapshots from etcd and merges them with events from a
// watch on our directory in etcd. It sends an "eventually consistent" stream of
// events to its callback.
//
// Syncer architecture
//
// The syncer's processing is divided into four goroutines:
//
// The merge goroutine
//
// The merge goroutine receives updates about newly loaded snapshots (from the
// snapshot reeading goroutine) and events (from the watcher goroutine) and
// merges them into a consistent event stream.
//
// Since processing a snapshot takes some time and it happens concurrently with
// polling for new events, the merge goroutine may be receiving updates from
// a snapshot that occurred at etcd index 10, while the event stream is already
// reporting updates at etcd index 11, 12, ...  The merge thread does the
// bookkeeping to squash out-of-date snapshot updates in favour of newer
// information from the watcher and to resolve deletions after losing sync.
//
// The merge goroutine also requests new snapshots when the watcher drops out
// of sync.  While it's the watcher goroutine that detects the loss of sync,
// sending the request via the merge goroutine makes for easier reasoning about
// the thread safety.
//
// The snapshot reading goroutine
//
// When requested by the merge goroutine, the snapshot-reading goroutine
// reads a consistent point-in-time snapshot from etcd and streams it to the
// merge goroutine.  Then it waits for the next request.
//
// The watcher goroutine
//
// The watcher goroutine polls etcd for new events.  A typical use of the
// etcd API would load a snapshot first, then start polling from the snapshot
// index.  However, that approach doesn't work at high event throughput because
// etcd's event buffer can be exhausted before the snapshot is received, leading
// to a resync loop.  We avoid that scenario by having the watcher be free
// running.  If it loses sync, it immediately starts polling again from the
// current etcd index; then it triggers a snapshot read from the point it started
// polling.
//
// The cluster ID poll goroutine
//
// This goroutine simply polls etcd for its cluster ID every few seconds and
// kills the process if it changes.  This ensures that we recover if etcd is
// blown away and rebuilt.  It's such a rare corner case that corrective action
// isn't worth it (and would be likely to be buggy due to lack of exercise).
type etcdSyncerV3 struct {
	callbacks  api.SyncerCallbacks
	etcdClient *etcdv3.Client
	OneShot    bool
}

// Start starts the syncer's background threads.
func (syn *etcdSyncerV3) Start() {
	// Start a background thread to read events from etcd.  It will
	// queue events onto the etcdEvents channel.  If it drops out of sync,
	// it will signal on the resyncIndex channel.
	log.Info("Starting etcd Syncer")

	// Channel used to send updates from the watcher thread to the merge
	// thread.  We give it a large buffer because we want the watcher thread
	// to be free running and only block if we get really backed up.
	watcherUpdateC := make(chan interface{}, 20000)
	// Channel used to send updates from the snapshot thread to the merge
	// thread.  No buffer: we want the snapshot to be slowed down if the
	// merge thread is under load.
	snapshotUpdateC := make(chan interface{})
	// Channel used to signal from the merge thread to the snapshot thread
	// that a new snapshot is required.  To avoid deadlock with the channel
	// above, the merge only sends a new snapshot request once the old
	// snapshot is finished.
	snapshotRequestC := make(chan snapshotRequest)

	if !syn.OneShot {
		log.Info("Syncer not in one-shot mode, starting watcher thread")
		go syn.watchEtcd(watcherUpdateC)
		// In order to make sure that we eventually spot if the etcd
		// cluster is rebuilt, start a thread to poll the etcd
		// Cluster ID.  If we don't spot a cluster rebuild then our
		// watcher will start silently failing.
		go syn.pollClusterID(clusterIDPollInterval)
	}
	go syn.readSnapshotsFromEtcd(snapshotUpdateC, snapshotRequestC)
	go syn.mergeUpdates(snapshotUpdateC, watcherUpdateC, snapshotRequestC)
}

// readSnapshotsFromEtcd is a goroutine that, when requested, reads a new
// snapshot from etcd and send it to the merge thread.  A snapshot request
// includes the required etcd index for the snapshot; in case of a read from a
// stale replica, the snapshot thread retries until it reads a snapshot that is
// new enough.
func (syn *etcdSyncerV3) readSnapshotsFromEtcd(
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

		// In case of read from stale replica, loop until we read a
		// new-enough snapshot.  We don't do a quorum read to avoid
		// pinning the read load to the etcd master.
		for highestCompletedSnapshotIndex < minRequiredSnapshotIndex {
			log.WithFields(log.Fields{
				"requiredIdx": minRequiredSnapshotIndex,
				"currentIdx":  highestCompletedSnapshotIndex,
			}).Info("Newest snapshot is too stale, loading a new one")
			resp, err := syn.etcdClient.Get(context.Background(), "/calico/v1", etcdv3.WithPrefix())
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

			index := uint64(resp.Header.Revision)
			if index < minRequiredSnapshotIndex {
				log.Info("Retrieved stale snapshot, rereading...")
				time.Sleep(100 * time.Millisecond)
				continue
			}

			// If we get here, we should have a good snapshot.
			// Send it to the merge thread.
			snapshotUpdateC <- snapshotStarting{
				snapshotIndex: index,
			}
			sendSnapshotResp(resp, snapshotUpdateC)
			highestCompletedSnapshotIndex = index
		}
		// Defensive: just in case we somehow got called without needing
		// to execute the loop, send the snapshotFinished from outside
		// the loop. This ensures that we always pair a snapshot
		// finished to every snapshotRequest.
		snapshotUpdateC <- snapshotFinished{
			snapshotIndex: highestCompletedSnapshotIndex,
		}
	}
}

// sendSnapshotResp sends the node and its children over the channel as events.
func sendSnapshotResp(resp *etcdv3.GetResponse, snapshotUpdates chan<- interface{}) {
	for _, kv := range resp.Kvs {
		snapshotUpdates <- snapshotUpdate{
			snapshotIndex: uint64(resp.Header.Revision),
			kv: kvPair{
				key:           string(kv.Key),
				value:         string(kv.Value),
				modifiedIndex: uint64(kv.ModRevision),
			},
		}
	}
}

// watchEtcd is a goroutine that polls etcd for new events.  As described in the
// comment for the etcdSyncerV3, the watcher goroutine is free-running; it always
// tries to keep up with etcd but it emits events when it drops out of sync
// so that the merge goroutine can trigger a new resync via snapshot.
func (syn *etcdSyncerV3) watchEtcd(watcherUpdateC chan<- interface{}) {
	log.Info("etcd watch thread started.")
	// Each trip around the outer loop establishes the current etcd index
	// of the cluster, triggers a new snapshot read from that index (via
	// message to the merge goroutine) and starts watching from that index.
	for {
		// Do a non-recursive get on the Ready flag to find out the
		// current etcd index.  We'll trigger a snapshot/start polling from that.
		resp, err := syn.etcdClient.Get(context.Background(), "/calico/v1/Ready")
		if err != nil || resp.Count == 0 {
			log.WithError(err).Warn("Failed to get Ready key from etcd")
			time.Sleep(1 * time.Second)
			continue
		}

		initialClusterIndex := uint64(resp.Header.Revision)
		log.WithField("index", initialClusterIndex).Info("Polled etcd for initial watch index.")

		// We were previously out-of-sync, request a new snapshot at
		// the current cluster index, which is also the index that we'll
		// poll from.
		watcherUpdateC <- watcherNeedsSnapshot{
			minSnapshotIndex: initialClusterIndex,
		}

	watchLoop: // We'll stay in this poll loop unless we drop out of sync.
		watcher := syn.etcdClient.Watch(context.Background(), "/calico/v1", etcdv3.WithPrefix(), etcdv3.WithRev(int64(initialClusterIndex)))
		for wresp := range watcher {
			if err := wresp.Err(); err != nil {
				log.WithError(err).Warn("Unexpected error type from etcd")
				goto watchLoop
			}

			for _, ev := range wresp.Events {
				switch ev.Type {
				case mvccpb.PUT:
					watcherUpdateC <- watcherUpdate{kv: kvPair{
						modifiedIndex: uint64(ev.Kv.ModRevision),
						key:           string(ev.Kv.Key),
						value:         string(ev.Kv.Value),
					}}
				case mvccpb.DELETE:
					watcherUpdateC <- watcherDeletion{
						modifiedIndex: uint64(ev.Kv.ModRevision),
						key:           string(ev.Kv.Key),
					}
				default:
					log.WithField("actionType", ev.Type).Panic("Unknown action type")
				}
			}
		}
	}
}

// retryableWatcherError returns true if the given etcd error is worth
// retrying in the context of a watch.
func retryableWatcherV3Error(err error) bool {
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
func (syn *etcdSyncerV3) pollClusterID(interval time.Duration) {
	log.Info("Cluster ID poll thread started")
	lastSeenClusterID := uint64(0)
	for {
		resp, err := syn.etcdClient.Get(context.Background(), "/calico/v1/Ready")
		if err != nil {
			log.WithError(err).Warn("Failed to poll etcd server cluster ID")
			syn.sleepBeforeContinue(interval)
			continue
		}

		clusterID := resp.Header.ClusterId
		log.WithField("clusterID", clusterID).Debug("Polled etcd for cluster ID.")

		if lastSeenClusterID == 0 {
			log.WithField("clusterID", clusterID).Info("etcd cluster ID now known")
			lastSeenClusterID = resp.Header.ClusterId
		} else if lastSeenClusterID != clusterID {
			// The Syncer doesn't currently support this (hopefully rare)
			// scenario.  Terminate the process rather than carry on with
			// possibly out-of-sync etcd index.
			log.WithFields(log.Fields{
				"oldID": lastSeenClusterID,
				"newID": clusterID,
			}).Fatal("etcd cluster ID changed; must exit.")
		}

		syn.sleepBeforeContinue(interval)
	}
}

//sleepBeforeContinue Jitter by 10% of interval.
func (syn *etcdSyncerV3) sleepBeforeContinue(interval time.Duration) {
	time.Sleep(time.Duration(float64(interval) * (1 + (0.1 * rand.Float64()))))
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
func (syn *etcdSyncerV3) mergeUpdates(
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

		switch event := event.(type) {
		case update:
			// Common update processing, shared between snapshot and watcher updates.
			updatedLastSeenIndex := event.lastSeenIndex()
			kv := event.kvPair()
			log.WithFields(log.Fields{
				"indexToStore": updatedLastSeenIndex,
				"kv":           kv,
			}).Debug("Snapshot/watcher update")
			oldIdx := hwms.StoreUpdate(kv.key, updatedLastSeenIndex)
			if oldIdx < kv.modifiedIndex {
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
				syn.sendUpdate(kv.key, kv.value, kv.modifiedIndex, updateType)
			}
		case watcherDeletion:
			deletedKeys := hwms.StoreDeletion(event.key, event.modifiedIndex)
			log.WithFields(log.Fields{
				"prefix":  event.key,
				"numKeys": len(deletedKeys),
			}).Debug("Prefix deleted")
			syn.sendDeletions(deletedKeys, event.modifiedIndex)
		case watcherNeedsSnapshot:
			// Watcher has lost sync.  Record the snapshot index
			// that we now require to bring us into sync.  We'll start
			// a new snapshot below if we can.
			log.Info("Watcher out-of-sync, starting to track deletions")
			minRequiredSnapshotIndex = event.minSnapshotIndex
		case snapshotStarting:
			// Informational message from the snapshot thread.  Makes the logs clearer.
			log.WithField("snapshotIndex", event.snapshotIndex).Info("Started receiving snapshot")
		case snapshotFinished:
			// Snapshot is ending, we need to check if this snapshot is still new enough to
			// mean that we're really in sync (because the watcher may have fallen
			// out of sync again after it requested the snapshot).
			logCxt := log.WithFields(log.Fields{
				"snapshotIndex":    event.snapshotIndex,
				"minSnapshotIndex": minRequiredSnapshotIndex,
			})
			logCxt.Info("Finished receiving snapshot, cleaning up old keys.")
			snapshotInProgress = false
			hwms.StopTrackingDeletions()
			deletedKeys := hwms.DeleteOldKeys(event.snapshotIndex)
			logCxt.WithField("numDeletedKeys", len(deletedKeys)).Info(
				"Deleted old keys that weren't seen in snapshot.")
			syn.sendDeletions(deletedKeys, event.snapshotIndex)

			if event.snapshotIndex > highestCompletedSnapshotIndex {
				highestCompletedSnapshotIndex = event.snapshotIndex
			}
			if event.snapshotIndex >= minRequiredSnapshotIndex {
				// Now in sync.
				logCxt.Info("Snapshot brought us into sync.")
				syn.callbacks.OnStatusUpdated(api.InSync)
			} else {
				// Watcher is already out-of-sync.  We'll restart the
				// snapshot below.
				logCxt.Warn("Snapshot was stale before it finished.")
			}
		default:
			log.WithField("event", event).Panic("Unknown event type")
		}

		outOfSync := highestCompletedSnapshotIndex < minRequiredSnapshotIndex
		if outOfSync && !snapshotInProgress {
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
func (syn *etcdSyncerV3) sendUpdate(key string, value string, revision uint64, updateType api.UpdateType) {
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

	parsedValue, err = model.ParseValue(parsedKey, []byte(value))
	if err != nil {
		log.Warningf("Failed to parse value for %v: %#v", key, value)
	}
	log.Debugf("Parsed value: %#v", parsedValue)

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
func (syn *etcdSyncerV3) sendDeletions(deletedKeys []string, revision uint64) {
	updates := make([]api.Update, 0, len(deletedKeys))
	for _, key := range deletedKeys {
		parsedKey := model.KeyFromDefaultPath(key)
		if parsedKey == nil {
			log.Debugf("Failed to parse key %v", key)
			if cb, ok := syn.callbacks.(api.SyncerParseFailCallbacks); ok {
				cb.ParseFailed(key, "")
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
