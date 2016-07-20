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
	"github.com/tigera/libcalico-go/datastructures/hwm"
	"github.com/tigera/libcalico-go/etcd-driver/store"
	"time"

	"github.com/coreos/etcd/client"
	"github.com/golang/glog"
	"golang.org/x/net/context"
)

func init() {
	store.Register("etcd", New)
}

func New(callbacks store.Callbacks, config *store.DriverConfiguration) (store.Driver, error) {
	return &etcdDriver{
		callbacks: callbacks,
		config:    config,
	}, nil
}

type etcdDriver struct {
	callbacks store.Callbacks
	config    *store.DriverConfiguration
}

func (driver *etcdDriver) Start() {
	// Start a background thread to read events from etcd.  It will
	// queue events onto the etcdEvents channel.  If it drops out of sync,
	// it will signal on the resyncIndex channel.
	glog.Info("Starting etcd driver")
	etcdEvents := make(chan event, 20000)
	triggerResync := make(chan uint64, 5)
	initialSnapshotIndex := make(chan uint64)
	if !driver.config.OneShot {
		go driver.watchEtcd(etcdEvents, triggerResync, initialSnapshotIndex)
	}

	// Start a background thread to read snapshots from etcd.  It will
	// read a start-of-day snapshot and then wait to be signalled on the
	// resyncIndex channel.
	snapshotUpdates := make(chan event)
	go driver.readSnapshotsFromEtcd(snapshotUpdates, triggerResync, initialSnapshotIndex)

	go driver.mergeUpdates(snapshotUpdates, etcdEvents)

	// TODO actually send some config
	driver.callbacks.OnConfigLoaded(
		map[string]string{
			"InterfacePrefix": "cali",
		},
		map[string]string{},
	)
}

const (
	actionSet uint8 = iota
	actionDel
	actionSnapFinished
)

// TODO Split this into different types of struct and use a type-switch to unpack.
type event struct {
	action           uint8
	modifiedIndex    uint64
	snapshotIndex    uint64
	key              string
	valueOrNil       string
	snapshotStarting bool
	snapshotFinished bool
}

func (driver *etcdDriver) readSnapshotsFromEtcd(snapshotUpdates chan<- event, triggerResync <-chan uint64, initialSnapshotIndex chan<- uint64) {
	cfg := client.Config{
		Endpoints:               []string{"http://127.0.0.1:2379"},
		Transport:               client.DefaultTransport,
		HeaderTimeoutPerRequest: 10 * time.Second,
	}
	c, err := client.New(cfg)
	if err != nil {
		glog.Fatal(err)
	}
	kapi := client.NewKeysAPI(c)
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
			glog.Infof("Asked for snapshot > %v; last snapshot was %v",
				minIndex, highestSnapshotIndex)
			if highestSnapshotIndex >= minIndex {
				// We've already read a newer snapshot, no
				// need to re-read.
				glog.Info("Snapshot already new enough")
				continue
			}
		}

	readRetryLoop:
		for {
			resp, err := kapi.Get(context.Background(),
				"/calico/v1", &getOpts)
			if err != nil {
				if driver.config.OneShot {
					// One-shot mode is used to grab a snapshot and then
					// stop.  We don't want to go into a retry loop.
					glog.Fatal("Failed to read snapshot from etcd: ", err)
				}
				glog.Warning("Error getting snapshot, retrying...", err)
				time.Sleep(1 * time.Second)
				continue readRetryLoop
			}

			if resp.Index < minIndex {
				glog.Info("Retrieved stale snapshot, rereading...")
				continue readRetryLoop
			}

			// If we get here, we should have a good
			// snapshot.  Send it to the merge thread.
			sendNode(resp.Node, snapshotUpdates, resp)
			snapshotUpdates <- event{
				action:        actionSnapFinished,
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

func sendNode(node *client.Node, snapshotUpdates chan<- event, resp *client.Response) {
	if !node.Dir {
		snapshotUpdates <- event{
			key:           node.Key,
			modifiedIndex: node.ModifiedIndex,
			snapshotIndex: resp.Index,
			valueOrNil:    node.Value,
			action:        actionSet,
		}
	} else {
		for _, child := range node.Nodes {
			sendNode(child, snapshotUpdates, resp)
		}
	}
}

func (driver *etcdDriver) watchEtcd(etcdEvents chan<- event, triggerResync chan<- uint64, initialSnapshotIndex <-chan uint64) {
	cfg := client.Config{
		Endpoints: []string{"http://127.0.0.1:2379"},
		Transport: client.DefaultTransport,
		// set timeout per request to fail fast when the target endpoint is unavailable
		HeaderTimeoutPerRequest: time.Second,
	}
	glog.V(1).Infof("Watcher connecting to etcd with config: %v", cfg)
	c, err := client.New(cfg)
	if err != nil {
		glog.Fatal(err)
	}
	kapi := client.NewKeysAPI(c)

	start_index := <-initialSnapshotIndex

	watcherOpts := client.WatcherOptions{
		AfterIndex: start_index + 1,
		Recursive:  true,
	}
	watcher := kapi.Watcher("/calico/v1", &watcherOpts)
	inSync := true
	for {
		resp, err := watcher.Next(context.Background())
		if err != nil {
			switch err := err.(type) {
			case client.Error:
				errCode := err.Code
				if errCode == client.ErrorCodeWatcherCleared ||
					errCode == client.ErrorCodeEventIndexCleared {
					glog.Warning("Lost sync with etcd, restarting watcher")
					watcherOpts.AfterIndex = 0
					watcher = kapi.Watcher("/calico/v1",
						&watcherOpts)
					inSync = false
					// FIXME, we'll only trigger a resync after the next event
					continue
				} else {
					glog.Error("Error from etcd", err)
					time.Sleep(1 * time.Second)
				}
			case *client.ClusterError:
				glog.Error("Cluster error from etcd", err)
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
				// Tell the snapshot thread that we need a
				// new snapshot.  The snapshot needs to be
				// from our index or one lower.
				snapIdx := node.ModifiedIndex - 1
				glog.V(1).Infof("Asking for snapshot @ %v",
					snapIdx)
				triggerResync <- snapIdx
				inSync = true
			}
			etcdEvents <- event{
				action:           actionType,
				modifiedIndex:    node.ModifiedIndex,
				key:              resp.Node.Key,
				valueOrNil:       node.Value,
				snapshotStarting: !inSync,
			}
		}
	}
}

func (driver *etcdDriver) mergeUpdates(snapshotUpdates <-chan event, watcherUpdates <-chan event) {
	var e event
	var minSnapshotIndex uint64
	hwms := hwm.NewHighWatermarkTracker()

	driver.callbacks.OnStatusUpdated(store.WaitForDatastore)
	for {
		select {
		case e = <-snapshotUpdates:
			glog.V(4).Infof("Snapshot update %v @ %v\n", e.key, e.modifiedIndex)
		case e = <-watcherUpdates:
			glog.V(4).Infof("Watcher update %v @ %v\n", e.key, e.modifiedIndex)
		}
		if e.snapshotStarting {
			// Watcher lost sync, need to track deletions until
			// we get a snapshot from after this index.
			glog.V(1).Infof("Watcher out-of-sync, starting to track deletions")
			minSnapshotIndex = e.modifiedIndex
			driver.callbacks.OnStatusUpdated(store.ResyncInProgress)
		}
		switch e.action {
		case actionSet:
			var indexToStore uint64
			if e.snapshotIndex != 0 {
				// Store the snapshot index in the trie so that
				// we can scan the trie later looking for
				// prefixes that are older than the snapshot
				// (and hence must have been deleted while
				// we were out-of-sync).
				indexToStore = e.snapshotIndex
			} else {
				indexToStore = e.modifiedIndex
			}
			oldIdx := hwms.StoreUpdate(e.key, indexToStore)
			//glog.V(2).Infof("%v update %v -> %v\n",
			//	e.key, oldIdx, e.modifiedIndex)
			if oldIdx < e.modifiedIndex {
				// Event is newer than value for that key.
				// Send the update to Felix.
				update := store.Update{
					Key:        e.key,
					ValueOrNil: &e.valueOrNil,
				}
				driver.callbacks.OnKeysUpdated(
					[]store.Update{update})
			}
		case actionDel:
			deletedKeys := hwms.StoreDeletion(e.key,
				e.modifiedIndex)
			glog.V(3).Infof("Prefix %v deleted; %v keys",
				e.key, len(deletedKeys))
			updates := make([]store.Update, 0, len(deletedKeys))
			for _, child := range deletedKeys {
				updates = append(updates, store.Update{
					Key:        child,
					ValueOrNil: nil,
				})
			}
			driver.callbacks.OnKeysUpdated(updates)
		case actionSnapFinished:
			if e.snapshotIndex >= minSnapshotIndex {
				// Now in sync.
				hwms.StopTrackingDeletions()
				keys := hwms.DeleteOldKeys(e.snapshotIndex)
				glog.Infof("Snapshot finished at index %v; "+
					"%v keys deleted.\n",
					e.snapshotIndex, len(keys))
				for _, key := range keys {
					updates := []store.Update{{
						Key:        key,
						ValueOrNil: nil,
					}}
					driver.callbacks.OnKeysUpdated(updates)
				}
			}
			driver.callbacks.OnStatusUpdated(store.InSync)
		}
	}
}
