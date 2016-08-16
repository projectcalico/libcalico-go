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

package calc

import (
	"github.com/golang/glog"
	"github.com/tigera/libcalico-go/felix/proto"
	"github.com/tigera/libcalico-go/felix/store"
	"github.com/tigera/libcalico-go/lib/backend/api"
	"github.com/tigera/libcalico-go/lib/backend/model"
)

type AsyncCalcGraph struct {
	Dispatcher   *store.Dispatcher
	inputEvents  chan interface{}
	outputEvents chan<- interface{}
	eventBuffer  *EventBuffer
}

func NewAsyncCalcGraph(hostname string, outputEvents chan<- interface{}) *AsyncCalcGraph {
	eventBuffer := NewEventBuffer()
	dispatcher := NewCalculationGraph(eventBuffer, hostname)
	g := &AsyncCalcGraph{
		inputEvents:  make(chan interface{}, 10),
		outputEvents: outputEvents,
		Dispatcher:   dispatcher,
		eventBuffer:  eventBuffer,
	}
	eventBuffer.callback = g.onEvent
	return g
}

func (b *AsyncCalcGraph) OnUpdates(updates []model.KVPair) {
	glog.V(4).Infof("Got %v updates; queueing", len(updates))
	b.inputEvents <- updates
}

func (b *AsyncCalcGraph) OnStatusUpdated(status api.SyncStatus) {
	glog.V(4).Infof("Status updated: %v; queueing", status)
	b.inputEvents <- status
}

func (b *AsyncCalcGraph) loop() {
	glog.V(1).Info("AsyncCalcGraph running")
	for {
		update := <-b.inputEvents
		switch update := update.(type) {
		case []model.KVPair:
			glog.V(4).Info("Pulled []KVPair off channel")
			b.Dispatcher.OnUpdates(update)
		case api.SyncStatus:
			glog.V(4).Info("Pulled status update off channel")
			b.Dispatcher.OnStatusUpdated(update)
			b.onEvent(&proto.DatastoreStatus{
				Status: update.String(),
			})
		default:
			glog.Fatal("Unexpected update: %#v", update)
		}
		glog.V(4).Info("Flushing updates")
		b.eventBuffer.Flush()
		glog.V(4).Info("Flushed updates")
	}
}

func (b *AsyncCalcGraph) onEvent(event interface{}) {
	glog.V(4).Info("Sending output event on channel")
	b.outputEvents <- event
	glog.V(4).Info("Sent output event on channel")
}

func (b *AsyncCalcGraph) Start() {
	glog.V(1).Info("Starting AsyncCalcGraph")
	go b.loop()
}
