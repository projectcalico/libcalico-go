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

package watchersyncer

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/projectcalico/libcalico-go/lib/backend/api"
	"github.com/projectcalico/libcalico-go/lib/backend/model"
	cerrors "github.com/projectcalico/libcalico-go/lib/errors"
)

type watcherCache struct {
	logger       *logrus.Entry
	client       api.Client
	watch        api.WatchInterface
	resources    map[string]resourceKey
	oldResources map[string]resourceKey
	results      chan<- interface{}
	hasSynced    bool
	resourceType ResourceType
}

type resourceKey struct {
	revision string
	key      model.Key
}

// Create a new watcherCache.
// Note that the results channel is untyped.  The watcherSyncer expects one of the following
// types:
// -  An error
// -  An api.Update
// -  A api.SyncStatus (only for the very first InSync notification)
func newWatcherCache(client api.Client, resourceType ResourceType, results chan<- interface{}) *watcherCache {
	wc := &watcherCache{
		logger:       logrus.WithField("ListRoot", model.ListOptionsToDefaultPathRoot(resourceType.ListInterface)),
		client:       client,
		resourceType: resourceType,
		results:      results,
		resources:    make(map[string]resourceKey, 0),
	}
	return wc
}

// run creates the watcher and loops indefinitely reading from the watcher.
func (wc *watcherCache) run() {
	wc.createWatcher()
	for {
		select {
		case event := <-wc.watch.ResultChan():
			switch event.Type {
			case api.WatchSynced:
				wc.handleSyncedWatchEvent()
			case api.WatchAdded, api.WatchModified:
				wc.handleAddedOrModifiedWatchEvent(event)
			case api.WatchDeleted:
				wc.handleDeletedWatchEvent(event)
			case api.WatchError:
				wc.handleErrorWatchEvent(event)
			}
		}
	}
}

// handleSyncedWatchEvent handles a new sync event.  If this watcher has never been synced then
// notify the main watcherSyncer that we've synced.  We also send deleted messages for
// cached resources that were not validated in the sync (i.e. they must have since been
// deleted).
func (wc *watcherCache) handleSyncedWatchEvent() {
	// If this is our first synced event then send a synced notification.  The main
	// watcherSyncer code will send a Synced event when it has received synced events from
	// each cache.
	if !wc.hasSynced {
		wc.logger.Info("Sending synced update")
		wc.results <- api.InSync
		wc.hasSynced = true
	}

	// If the watcher failed at any time, we end up recreating a watcher and storing off
	// the current known resources for revalidation.  Now that we have finished the sync,
	// any of the remaining resources that were not accounted for must have been deleted
	// and we need to send deleted events for them.
	numOldResources := len(wc.oldResources)
	if numOldResources > 0 {
		wc.logger.WithField("Num", numOldResources).Debug("Sending resync deletes")
		updates := make([]api.Update, 0, len(wc.oldResources))
		for _, r := range wc.oldResources {
			updates = append(updates, api.Update{
				UpdateType: api.UpdateTypeKVDeleted,
				KVPair: model.KVPair{
					Key: r.key,
				},
			})
		}
		wc.results <- updates
	}
	wc.oldResources = nil
}

// handleAddedOrModifiedWatchEvent sends a New or Updated sync event depending on whether the resource
// is currently in our cache or not, and whether the revision has changed.
func (wc *watcherCache) handleAddedOrModifiedWatchEvent(event api.WatchEvent) {
	// If we don't have a converter function, just process the KVPair without
	// converting.
	if wc.resourceType.Converter == nil {
		wc.handleAddedOrModifiedUpdate(event.New)
		return
	}

	// Convert the KVPair.  This returns a slice of KVPairs, and any with a nil value
	// should trigger deletion events instead.
	rs, err := wc.resourceType.Converter.ConvertKVPair(event.New)
	for _, r := range rs {
		if r.Value == nil {
			wc.handleDeletedUpdate(r.Key)
		} else {
			wc.handleAddedOrModifiedUpdate(r)
		}
	}

	if err != nil {
		wc.results <- err
	}
}

// handleAddedOrModifiedUpdate handles a single Added or Modified update request.
// Whether we send an Added or Modified depends on whether we have already sent
// an added notification for this resource.
func (wc *watcherCache) handleAddedOrModifiedUpdate(kvp *model.KVPair) {
	thisKey := kvp.Key
	thisKeyString := thisKey.String()
	thisRevision := kvp.Revision
	wc.handleResync(thisKeyString)

	// If the resource is already in our map, then this is a modified event.  Check the
	// revision to see if we actually need to send an update.
	if resource, ok := wc.resources[thisKeyString]; ok {
		if resource.revision == thisRevision {
			// No update to revision, so no event to send.
			wc.logger.WithField("Key", thisKeyString).Debug("Entry not modified")
			return
		}
		// Resource is modified, send an update event and store the latest revision.
		wc.logger.WithField("Key", thisKeyString).Debug("Entry modified")
		wc.results <- []api.Update{{
			UpdateType: api.UpdateTypeKVUpdated,
			KVPair:     *kvp,
		}}
		resource.revision = thisRevision
		wc.resources[thisKeyString] = resource
		return
	}

	// The resource has not been seen before, so send a new event, and store the
	// current revision.
	wc.logger.WithField("Key", thisKeyString).Debug("Entry added")
	wc.results <- []api.Update{{
		UpdateType: api.UpdateTypeKVNew,
		KVPair:     *kvp,
	}}
	wc.resources[thisKeyString] = resourceKey{
		revision: thisRevision,
		key:      thisKey,
	}
}

// handleDeletedWatchEvent sends a deleted event and removes the resource key from our cache.
func (wc *watcherCache) handleDeletedWatchEvent(event api.WatchEvent) {
	// If we don't have a converter function, just process the event without
	// converting.
	if wc.resourceType.Converter == nil {
		wc.handleDeletedUpdate(event.Old.Key)
		return
	}

	// Convert the Key.  This returns a slice of Keys which should all be deleted.  We only actually
	// send delete updates if we have previously send an Added event.
	ks, err := wc.resourceType.Converter.ConvertKey(event.Old.Key)
	for _, k := range ks {
		wc.handleDeletedUpdate(k)
	}

	if err != nil {
		wc.results <- err
	}
}

// handleDeletedWatchEvent sends a deleted event and removes the resource key from our cache.
func (wc *watcherCache) handleDeletedUpdate(key model.Key) {
	thisKeyString := key.String()
	wc.handleResync(thisKeyString)

	// If we have seen an added event for this key then send a deleted event and remove
	// from the cache.
	if _, ok := wc.resources[thisKeyString]; ok {
		wc.logger.WithField("Key", thisKeyString).Debug("Entry deleted")
		wc.results <- []api.Update{{
			UpdateType: api.UpdateTypeKVDeleted,
			KVPair: model.KVPair{
				Key: key,
			},
		}}
		delete(wc.resources, thisKeyString)
	}
}

// handleErrorWatchEvent sends the error in the errors channel, and if necessary will
// trigger a watcher restart.
func (wc *watcherCache) handleErrorWatchEvent(event api.WatchEvent) {
	wc.results <- event.Error
	if _, ok := event.Error.(cerrors.ErrorWatchTerminated); ok {
		wc.logger.Info("Received watch terminated error - recreate watcher")
		wc.createWatcher()
	}
}

// handleResync performs common processing for events required if a resync is
// in progress.  This is a no-op if a resync is not in progress, otherwise, it moves
// the cached resource data from the stored set of resources into the current cache.
// This allows us to do a mark-and-sweep to send deleted events once the resync is complete
// for resources that were deleted while we weren't watching.
func (wc *watcherCache) handleResync(resourceKey string) {
	if wc.oldResources != nil {
		if oldResource, ok := wc.oldResources[resourceKey]; ok {
			wc.logger.WithField("Key", resourceKey).Debug("Marking key as re-processed")
			wc.resources[resourceKey] = oldResource
			delete(wc.oldResources, resourceKey)
		}
	}
}

// createWatcher creates a new watcher using the supplied ListOptions
func (wc *watcherCache) createWatcher() {
	// Make sure any previous watcher is stopped.
	if wc.watch != nil {
		wc.logger.Info("Stopping previous watcher")
		wc.watch.Stop()
		wc.watch = nil
	}

	// Move the current resources over to the oldResources - when we reconnect we'll
	// move entries back again when we rediscover them.  Whatever is left in the oldResources
	// after the sync is complete indicates entries that were deleted during our reconnection.
	if len(wc.oldResources) == 0 {
		wc.logger.Debug("Storing current resource revisions to validate during resync")
		wc.oldResources = wc.resources
	} else {
		wc.logger.Info("Appending current resource revisions unvalidated revision to validate during resync")
		for k, v := range wc.resources {
			wc.oldResources[k] = v
		}
	}
	wc.resources = make(map[string]resourceKey, 0)

	for {
		w, err := wc.client.Watch(context.Background(), wc.resourceType.ListInterface, "")

		// We created the watcher, so return.
		if err == nil {
			wc.logger.Info("Created watcher")
			wc.watch = w
			return
		}

		// Sleep so that we don't tight loop.
		wc.logger.WithError(err).Debug("Failed to create watcher")
		time.Sleep(1 * time.Second)
	}
}
