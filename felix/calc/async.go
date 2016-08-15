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
	b.inputEvents <- updates
}

func (b *AsyncCalcGraph) OnStatusUpdated(status api.SyncStatus) {
	b.inputEvents <- status
}

func (b *AsyncCalcGraph) loop() {
	for {
		update := <-b.inputEvents
		switch update := update.(type) {
		case []model.KVPair:
			b.Dispatcher.OnUpdates(update)
		case api.SyncStatus:
			b.Dispatcher.OnStatusUpdated(update)
			b.onEvent(&proto.DatastoreStatus{
				Status: update.String(),
			})
		}
		b.eventBuffer.Flush()
	}
}

func (b *AsyncCalcGraph) onEvent(event interface{}) {
	b.outputEvents <- event
}

func (b *AsyncCalcGraph) Start() {
	go b.loop()
}

type GlobalConfigUpdate struct {
	Name       string
	ValueOrNil *string
}

type HostConfigUpdate struct {
	Name       string
	ValueOrNil *string
}
