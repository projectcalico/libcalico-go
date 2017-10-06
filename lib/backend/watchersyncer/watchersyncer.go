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
	log "github.com/sirupsen/logrus"

	"github.com/projectcalico/libcalico-go/lib/backend/api"
	"github.com/projectcalico/libcalico-go/lib/backend/model"
	cerrors "github.com/projectcalico/libcalico-go/lib/errors"
)

// ResourceType groups together the watch and conversion information for a
// specific resource type.
type ResourceType struct {
	// The ListInterface used to perform a watch on this resource type.
	ListInterface model.ListInterface

	// An optional converter function used to convert the update event prior to
	// it being sent in the Syncer.
	Converter SyncerUpdateConverter
}

// SyncerUpdateConverter is used to convert a Watch update into one or more additional
// Syncer updates.
type SyncerUpdateConverter interface {
	// ConvertKey converts the original Key from a Watcher update to the equivalent full set
	// of Keys for the mapped-resources returned on the Syncer.  Used for delete processing
	// to delete all of the possible mapped keys.  The converter may return any combination
	// of the return values (including nil for both).
	ConvertKey(model.Key) ([]model.Key, error)

	// ConvertKVPair converts the original KVPair from a Watcher update to the equivalent set
	// of KVParis for the mapped-resources returned on the Syncer.  In the case where the configuration
	// in the original KVPair would indicate deletion of some of the mapped resources, those resources
	// should still be included in the mapped-KVPairs, but with a nil Value indicating (not present).
	// The full set of Keys returned in the mapped KVPairs should match those that would be returned
	// by ConvertKey (for the same resource as input).  The converter may return any combination
	// of the return values (including nil for both).
	ConvertKVPair(*model.KVPair) ([]*model.KVPair, error)
}

// New creates a new multiple Watcher-backed api.Syncer.
func New(client api.Client, resourceTypes []ResourceType, callbacks api.SyncerCallbacks) api.Syncer {
	rs := &watcherSyncer{
		watcherCaches: make([]*watcherCache, len(resourceTypes)),
		results:       make(chan interface{}, 2000),
		callbacks:     callbacks,
	}
	for i, r := range resourceTypes {
		rs.watcherCaches[i] = newWatcherCache(client, r, rs.results)
	}
	return rs
}

// watcherSyncer implements the api.Syncer interface.
type watcherSyncer struct {
	status        api.SyncStatus
	watcherCaches []*watcherCache
	results       chan interface{}
	numSynced     int
	callbacks     api.SyncerCallbacks
}

func (rs *watcherSyncer) Start() {
	go rs.run()
}

// Send a status update and store the status.
func (rs *watcherSyncer) sendStatusUpdate(status api.SyncStatus) {
	log.WithField("Status", status).Info("Sending status update")
	rs.callbacks.OnStatusUpdated(status)
	rs.status = status
}

// run implements the main syncer loop that loops forever receiving watch events and translating
// to syncer updates.
func (rs *watcherSyncer) run() {
	rs.sendStatusUpdate(api.WaitForDatastore)
	for _, wc := range rs.watcherCaches {
		go wc.run()
	}

	var updates []api.Update
	for {
		// Block until there is data.
		result := <-rs.results

		// Process the data - this will append the data in subsequent calls, and action
		// it if we hit a non-update event.
		updates := rs.processResult(updates, result)

		// Process remaining entries in the channel in one go.
		remaining := len(rs.results)
		for i := 0; i < remaining; i++ {
			next := <-rs.results
			updates = rs.processResult(updates, next)
		}

		// Perform final processing (pass in a nil result) before we hit the blocking
		// call.
		updates = rs.processResult(updates, nil)
	}

}

// Process a result from the result channel.  We don't immediately action updates, but
// instead start grouping them together so that we can send a larger single update to
// Felix.
func (rs *watcherSyncer) processResult(updates []api.Update, result interface{}) []api.Update {

	// Final call of the interation, send any updates if we've collated some.
	if result == nil {
		if len(updates) > 0 {
			rs.callbacks.OnUpdates(updates)
		}
		return nil
	}

	// Switch on the result type.
	switch r := result.(type) {
	case []api.Update:
		// This is an update.  If we don't have previous updates then also check to see
		// if we need to shift the status into Resync.
		// We append these updates to the previous if there were any.
		if len(updates) == 0 {
			if rs.status == api.WaitForDatastore {
				rs.sendStatusUpdate(api.ResyncInProgress)
			}
			updates = r
		} else {
			updates = append(updates, r...)
		}

	case error:
		// Received an error.  Firstly, send any updates that we have grouped.
		if len(updates) > 0 {
			rs.callbacks.OnUpdates(updates)
			updates = nil
		}

		// If this is a parsing error, and if the callbacks support
		// it, then send the error update.
		log.WithError(r).Info("Error received in syncer")
		if ec, ok := rs.callbacks.(api.SyncerParseFailCallbacks); ok {
			if pe, ok := r.(cerrors.ErrorParsingDatastoreEntry); ok {
				ec.ParseFailed(pe.RawKey, pe.RawValue)
			}
		}

	case api.SyncStatus:
		// Received a synced event.  If we are still waiting for datastore, send a
		// ResyncInProgress since at least one watcher has connected.
		log.Info("Received sync event from watcher")
		if rs.status == api.WaitForDatastore {
			rs.sendStatusUpdate(api.ResyncInProgress)
		}

		// Increment the count of synced events.
		rs.numSynced++

		// If we have now received synced events from all of our watchers then we are in
		// sync.  If we have any updates, send them first and then send the status update.
		if rs.numSynced == len(rs.watcherCaches) {
			if len(updates) > 0 {
				rs.callbacks.OnUpdates(updates)
				updates = nil
			}
			rs.sendStatusUpdate(api.InSync)
		}
	}

	// Return the accumulated or processed updated.
	return updates
}
