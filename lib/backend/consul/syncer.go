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

package consul

import (
	log "github.com/Sirupsen/logrus"
	consulapi "github.com/hashicorp/consul/api"
	"github.com/projectcalico/libcalico-go/lib/backend/api"
	"github.com/projectcalico/libcalico-go/lib/backend/model"
	"time"
)

var keyToSync = "/calico"

// Consul syncer uses blocking call to consul replica to list all keys
// that have version greater than remembered. Unfortunately, consul will
// send us full snapshot, so we should manage to create our own diffs
//
// Architecture
//
// Syncer runs watcher goroutine which polls consul and report diffs (calculated on client)to api
// We are trying to remove load from consul master server, so we connect to agents which can contain
// stale data. To mitigate that we will connect to master server first time and get an Index of it
// After that we will not consume snapshots with Index lower than ours.
//
// As consul can't report diffs and we can block only on Index-change calls, we will not receive any
// updates on deletion of keys inside `calico` prefix. To mitigate this, every delete operation will
// update ready key with its value (that is a constant, AFAIK) and by doing so will trigger an "update"
// inside `calico` prefix.
func newSyncer(client *consulapi.Client, callbacks api.SyncerCallbacks) *consulSyncer {
	return &consulSyncer{
		callbacks: callbacks,
		client:    client,
	}
}

type consulSyncer struct {
	callbacks api.SyncerCallbacks
	client    *consulapi.Client
	OneShot   bool
}

func (syn *consulSyncer) Start() {
	if !syn.OneShot {
		log.Info("Syncer not in one-shot mode, starting watcher thread")
		go syn.watchConsul()
	}
}

func (syn *consulSyncer) watchConsul() {
	log.Info("consul watch thread started.")

	for {
		syn.callbacks.OnStatusUpdated(api.WaitForDatastore)

		// Do a non-recursive get on the Ready flag to find out the
		// current consul index.  We'll trigger a snapshot/start polling from that.
		_, meta, err := syn.client.KV().Get(calicoPathToConsulPath("/calico/v1/Ready"), nil)
		if err != nil {
			log.WithError(err).Warn("Failed to get Ready key from consul")
			time.Sleep(1 * time.Second)
			continue
		}

		clusterIndex := meta.LastIndex
		log.WithField("index", clusterIndex).Info("Polled consul for initial watch index.")

		results, meta, err := syn.client.KV().List(
			calicoPathToConsulPath(keyToSync),
			getReadFromNonServerOpts(clusterIndex))

		if err != nil {
			log.WithError(err).Warn("Failed to get snapshot from consul")
			continue
		}

		syn.callbacks.OnStatusUpdated(api.ResyncInProgress)

		initialUpdates := make([]api.Update, len(results))
		state := map[model.Key]bool{}
		for i, x := range results {
			key := keyFromDefaultPath(x.Key)
			value, err := model.ParseValue(key, x.Value)
			updateType := api.UpdateTypeKVNew
			if err != nil {
				log.WithField("key", key).Warn("Can't parse value at key.")
				updateType = api.UpdateTypeKVUnknown
				value = nil
			}

			initialUpdates[i] = api.Update{
				UpdateType: updateType,
				KVPair: model.KVPair{
					Key:      key,
					Revision: x.ModifyIndex,
					Value:    value,
				},
			}

			state[key] = true
		}

		syn.callbacks.OnUpdates(initialUpdates)
		syn.callbacks.OnStatusUpdated(api.InSync)

		// second sync will be run in cycle until we have some consul error
	watchLoop:
		for {
			err = repeatableSync(syn, &state, clusterIndex)
			if err != nil {
				log.WithError(err).Warn("Failed to get snapshot from consul")
				break watchLoop
			}
		}
	}
}

func getReadFromNonServerOpts(clusterIndex uint64) *consulapi.QueryOptions {
	return &consulapi.QueryOptions{WaitIndex: clusterIndex, AllowStale: true, RequireConsistent: false}
}

func repeatableSync(syn *consulSyncer, pointerToState *map[model.Key]bool, clusterIndex uint64) error {
	state := *pointerToState
	results, meta, err := syn.client.KV().List(
		calicoPathToConsulPath(keyToSync),
		getReadFromNonServerOpts(clusterIndex))

	if err != nil {
		return err
	}

	updates := make([]api.Update, len(results))
	tombstones := map[model.Key]bool{}

	for k, v := range state {
		tombstones[k] = v
	}

	syn.callbacks.OnStatusUpdated(api.ResyncInProgress)
	for i, x := range results {
		key := keyFromDefaultPath(x.Key)
		value, err := model.ParseValue(key, x.Value)
		updateType := api.UpdateTypeKVNew

		if tombstones[key] {
			delete(tombstones, key)
			updateType = api.UpdateTypeKVUpdated
		}

		if err != nil {
			log.WithField("key", key).Warn("Can't parse value at key.")
			updateType = api.UpdateTypeKVUnknown
			value = nil
		}

		updates[i] = api.Update{
			UpdateType: updateType,
			KVPair: model.KVPair{
				Key:      key,
				Revision: x.ModifyIndex,
				Value:    value,
			},
		}

		state[key] = true
	}

	for x := range tombstones {
		updates = append(updates, api.Update{
			UpdateType: api.UpdateTypeKVDeleted,
			KVPair: model.KVPair{
				Key:      x,
				Revision: meta.LastIndex,
				Value:    nil,
			},
		})

		delete(state, x)
	}

	syn.callbacks.OnUpdates(updates)
	syn.callbacks.OnStatusUpdated(api.InSync)
	return nil
}
