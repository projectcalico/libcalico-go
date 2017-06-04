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
	log "github.com/Sirupsen/logrus"
	"github.com/projectcalico/libcalico-go/lib/backend/api"
)

func newSyncer(callbacks api.SyncerCallbacks, kc *KubeClient, disableNodePoll bool) *parentSyncer {
	syn := &parentSyncer{
		kubeClient:      kc,
		callbacks:       callbacks,
		disableNodePoll: disableNodePoll,
		stopChan:        make(chan int),
		childStatus:     make(chan api.SyncStatus),
		childUpdates:    make(chan api.Update),
		children:        []api.Syncer{},
	}
	return syn
}

type parentSyncer struct {
	kubeClient      *KubeClient
	callbacks       api.SyncerCallbacks
	OneShot         bool
	disableNodePoll bool
	stopChan        chan int
	childStatus     chan api.SyncStatus
	childUpdates    chan []api.Update
	children        []api.Syncer
}

func (syn *parentSyncer) Start() {
	// Kick off a number of different "child" syncers, each of which will
	// monitor its own state and send updates to this "parent" syncer over
	// the provided callbacks.
	ccb := childCallbacks{
		updateChannel: syn.childUpdates,
		statusChannel: syn.childStatus,
	}
	children := []api.Syncer{}

	// Add a Pod syncer as a child.
	syn.children = append(syn.children, PodSyncer(ccb, syn.kubeClient))

	// Start all the children.
	for _, child := range children {
		go child.Start()
	}

	// Watch for updates from children.
	syn.acceptChildUpdates()
}

func (syn *parentSyncer) Stop() {
	for _, c := range syn.children {
		c.Stop()
	}
}

func (syn *parentSyncer) acceptChildUpdates() {
	for {
		select {
		case status := <-syn.childStatus:
			// Received a status update from a child.
			log.Infof("Received sync status from child: %s", status)
		case updates := <-syn.childUpdates:
			// Received an update from a child.
			log.Infof("Received an update from a child")
		}
	}
}

// childCallbacks implements the SyncerCallbacks.  It sends all updates
// received from children over the appropriate channels to its parent.
type childCallbacks struct {
	updateChannel chan []api.Update
	statusChannel chan api.SyncStatus
}

func (ccb childCallbacks) OnStatusUpdated(status api.SyncStatus) {
	ccb.statusChannel <- status
}

func (ccb childCallbacks) OnUpdates(u []api.Update) {
	// When we receive an update from one of our children,
	// just pass it straight through to our callbacks.
	ccb.updateChannel <- u
}
