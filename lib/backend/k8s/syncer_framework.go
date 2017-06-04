// Copyright (c) 2016-2017 Tigera, Inc. All rights reserved.
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

package k8s

import (
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/projectcalico/libcalico-go/lib/backend/api"
	"github.com/projectcalico/libcalico-go/lib/backend/model"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/watch"
)

func newGenericSyncer(callbacks api.SyncerCallbacks, syncFuncs GenericSyncFuncs) api.Syncer {
	syn := &genericKubeSyncer{
		callbacks: callbacks,
		tracker:   map[string]model.Key{},
		stopChan:  make(chan int),
		funcs:     syncFuncs,
	}
	return syn
}

type GenericSyncFuncs interface {
	// ListFunc lists existing KVPairs for this resource as well as the latest
	// resourceVesion, or an error.
	ListFunc(opts metav1.ListOptions) ([]model.KVPair, string, error)

	// WatchFunc returns a watch.Interface to watch a particular resource
	// in the Kubernetes API.
	WatchFunc(opts metav1.ListOptions) (watch.Interface, error)

	// ParseFunc takes an incoming event for the watcher and parses it into
	// zero or more KVPair objects to be sent over the syncer.
	ParseFunc(watch.Event) []model.KVPair
}

type genericKubeSyncer struct {
	callbacks api.SyncerCallbacks
	OneShot   bool
	tracker   map[string]model.Key
	stopChan  chan int
	funcs     GenericSyncFuncs
}

func (syn *genericKubeSyncer) Start() {
	// Start a background thread to read snapshots from and watch the Kubernetes API,
	// and pass updates via callbacks.
	go syn.readFromKubernetesAPI()
}

func (syn *genericKubeSyncer) Stop() {
	syn.stopChan <- 1
}

// sendUpdates sends updates to the callback and updates the resource
// tracker.
func (syn *genericKubeSyncer) sendUpdates(kvps []model.KVPair) {
	updates := syn.convertKVPairsToUpdates(kvps)

	// Send to the callback and update the tracker.
	syn.callbacks.OnUpdates(updates)
	syn.updateTracker(updates)
}

// convertKVPairsToUpdates converts a list of KVPairs to the list
// of api.Update objects which should be sent to OnUpdates.  It filters out
// deletes for any KVPairs which we don't know about.
func (syn *genericKubeSyncer) convertKVPairsToUpdates(kvps []model.KVPair) []api.Update {
	updates := []api.Update{}
	for _, kvp := range kvps {
		if _, ok := syn.tracker[kvp.Key.String()]; !ok && kvp.Value == nil {
			// The given KVPair is not in the tracker, and is a delete, so no need to
			// send a delete update.
			continue
		}
		updates = append(updates, api.Update{KVPair: kvp, UpdateType: syn.getUpdateType(kvp)})
	}
	return updates
}

// updateTracker updates the global object tracker with the given update.
// updateTracker should be called after sending a update to the OnUpdates callback.
func (syn *genericKubeSyncer) updateTracker(updates []api.Update) {
	for _, upd := range updates {
		if upd.UpdateType == api.UpdateTypeKVDeleted {
			log.Debugf("Delete from tracker: %+v", upd.KVPair.Key)
			delete(syn.tracker, upd.KVPair.Key.String())
		} else {
			log.Debugf("Update tracker: %+v: %+v", upd.KVPair.Key, upd.KVPair.Revision)
			syn.tracker[upd.KVPair.Key.String()] = upd.KVPair.Key
		}
	}
}

func (syn *genericKubeSyncer) getUpdateType(kvp model.KVPair) api.UpdateType {
	if kvp.Value == nil {
		// If the value is nil, then this is a delete.
		return api.UpdateTypeKVDeleted
	}

	// Not a delete.
	if _, ok := syn.tracker[kvp.Key.String()]; !ok {
		// If not a delete and it does not exist in the tracker, this is an add.
		return api.UpdateTypeKVNew
	} else {
		// If not a delete and it exists in the tracker, this is an update.
		return api.UpdateTypeKVUpdated
	}
}

func (syn *genericKubeSyncer) readFromKubernetesAPI() {
	log.Info("Starting Kubernetes API read worker")

	// Keep track of latest version.
	latestResourceVersion := ""

	// Other watcher vars.
	var eventChannel <-chan watch.Event
	var event watch.Event
	var opts metav1.ListOptions
	var watcher watch.Interface

	// Always perform an initial snapshot.
	needsResync := true

	log.Info("Starting Kubernetes API read loop")
	for {
		// If we need to resync, do so.
		if needsResync {
			// Set status to ResyncInProgress.
			log.Debugf("Resync required - latest version: %+v", latestResourceVersion)
			syn.callbacks.OnStatusUpdated(api.ResyncInProgress)

			// Get snapshot from datastore.
			snap, existingKeys, latestVersions := syn.performSnapshot()
			log.Debugf("Snapshot: %+v, keys: %+v, versions: %+v", snap, existingKeys, latestVersions)

			// Go through and delete anything that existed before, but doesn't anymore.
			syn.performSnapshotDeletes(existingKeys)

			// Send the snapshot through.
			syn.sendUpdates(snap)

			log.Debugf("Snapshot complete - start watch from %+v", latestResourceVersion)
			syn.callbacks.OnStatusUpdated(api.InSync)

			// Don't start watches if we're in oneshot mode.
			if syn.OneShot {
				log.Info("OneShot mode, do not start watches")
				return
			}

			// Close the watch if it's open before starting another one to
			// prevent leaking of watches.
			if watcher != nil {
				watcher.Stop()
			}

			// Instantiate watcher and get the result channel.
			opts = metav1.ListOptions{ResourceVersion: latestResourceVersion}
			watcher, err := syn.funcs.WatchFunc(opts)
			if err != nil {
				log.Warnf("Failed to watch TODO_RESOURCE_HERE, retrying: %s", err)
				time.Sleep(1 * time.Second)
				continue
			}
			eventChannel = watcher.ResultChan()

			// Success - reset the flag.
			needsResync = false
		}

		// Select across the various channels.
		select {
		case <-syn.stopChan:
			log.Info("Syncer told to stop reading")
			if watcher != nil {
				watcher.Stop()
			}
			return
		case event = <-eventChannel:
			log.Debugf("Incoming TODO watch event. Type=%s", event.Type)
			if needsResync = syn.eventTriggersResync(event); needsResync {
				// We need to resync.  Break out into the sync loop.
				log.Warnf("Event triggered resync: %+v", event)
				continue
			}

			// Event is OK - parse it and send any updates.
			kvps := syn.funcs.ParseFunc(event)
			latestResourceVersion = kvps[0].Revision.(string)
			syn.sendUpdates(kvps)
		}
	}
}

func (syn *genericKubeSyncer) performSnapshotDeletes(exists map[string]bool) {
	log.Info("Checking for any deletes for snapshot")
	deletes := []model.KVPair{}
	log.Debugf("Keys in snapshot: %+v", exists)
	for cachedKey, k := range syn.tracker {
		// Check each cached key to see if it exists in the snapshot.  If it doesn't,
		// we need to send a delete for it.
		if _, stillExists := exists[cachedKey]; !stillExists {
			log.Debugf("Cached key not in snapshot: %+v", cachedKey)
			deletes = append(deletes, model.KVPair{Key: k, Value: nil})
		}
	}
	log.Infof("Sending snapshot deletes: %+v", deletes)
	syn.sendUpdates(deletes)
}

// performSnapshot returns a list of existing objects in the datastore,
// a mapping of model.Key objects representing the objects which exist in the datastore, and
// populates the provided resourceVersions with the latest k8s resource version
// for each.
func (syn *genericKubeSyncer) performSnapshot() ([]model.KVPair, map[string]bool, string) {
	opts := metav1.ListOptions{}
	var snap []model.KVPair
	var keys map[string]bool

	// Loop until we successfully are able to accesss the API.
	for {
		// Initialize the values to return.
		snap = []model.KVPair{}
		keys = map[string]bool{}

		// Get the full list of KVPs for this resource and add them
		// to the snapshot.
		log.Info("Syncing TODO RESOURCE HERE")
		kvps, resourceVersion, err := syn.funcs.ListFunc(opts)
		if err != nil {
			log.Warnf("Error syncing TODO RESOURCE HERE, retrying: %s", err)
			time.Sleep(1 * time.Second)
			continue
		}
		log.Info("Received TODO RESOURCE HERE List() response")
		snap = append(snap, kvps...)

		// Ensure that the keys map is populated with each returned
		// key.
		for _, kvp := range kvps {
			keys[kvp.Key.String()] = true
		}

		// Successfully generated snapshot
		return snap, keys, resourceVersion
	}
}

// eventTriggersResync returns true of the given event requires a
// full datastore resync to occur, and false otherwise.
func (syn *genericKubeSyncer) eventTriggersResync(e watch.Event) bool {
	// If we encounter an error, or if the event is nil (which can indicate
	// an unexpected connection close).
	if e.Type == watch.Error || e.Object == nil {
		log.Warnf("Event requires snapshot: %+v", e)
		return true
	}
	return false
}
