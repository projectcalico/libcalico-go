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

package store

import (
	"encoding/json"
	"github.com/golang/glog"
	"github.com/tigera/libcalico-go/lib/backend"
	"reflect"
)

type ParsedUpdateHandler func(update *ParsedUpdate)

type Dispatcher struct {
	listenersByType map[reflect.Type][]ParsedUpdateHandler
}

// NewDispatcher creates a Dispatcher with all its event handlers set to no-ops.
func NewDispatcher() *Dispatcher {
	d := Dispatcher{
		listenersByType: make(map[reflect.Type][]ParsedUpdateHandler),
	}
	return &d
}

type ParsedUpdate struct {
	Key      backend.KeyInterface
	Value    interface{}
	ParseErr error
	// RawUpdate is the Update that will be passed to Felix, mutable!
	RawUpdate       *Update
	ValueUpdated    bool
	SkipSendToFelix bool
}

func (d *Dispatcher) Register(keyExample backend.KeyInterface, receiver ParsedUpdateHandler) {
	keyType := reflect.TypeOf(keyExample)
	if keyType.Kind() == reflect.Ptr {
		panic("Register expects a non-pointer")
	}
	glog.Infof("Registering listener for type %v: %v", keyType, receiver)
	d.listenersByType[keyType] = append(d.listenersByType[keyType], receiver)
}

func (d *Dispatcher) DispatchUpdate(update *Update) bool {
	glog.V(3).Infof("Dispatching %v", update)
	key := backend.ParseKey(update.Key)
	if key == nil {
		return false
	}

	glog.V(4).Info("Key ", key)
	var value interface{}
	var err error
	if update.ValueOrNil != nil {
		value, err = backend.ParseValue(key, []byte(*update.ValueOrNil))
	}

	parsedUpdate := &ParsedUpdate{
		Key:       key,
		Value:     value,
		ParseErr:  err,
		RawUpdate: update,
	}

	keyType := reflect.TypeOf(key)
	glog.V(4).Info("Type: ", keyType)
	listeners := d.listenersByType[keyType]
	glog.V(4).Infof("Listeners: %#v", listeners)
	for _, recv := range listeners {
		recv(parsedUpdate)
	}

	if parsedUpdate.ValueUpdated {
		// A handler has tweaked the value, update the JSON.
		rawJSON, err := json.Marshal(parsedUpdate.Value)
		if err != nil {
			update.ValueOrNil = nil
		} else {
			str := string(rawJSON)
			update.ValueOrNil = &str
		}
	}

	return parsedUpdate.SkipSendToFelix
}
