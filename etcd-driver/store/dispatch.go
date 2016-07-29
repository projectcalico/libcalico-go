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
	"github.com/golang/glog"
	"github.com/tigera/libcalico-go/lib/backend/model"
	"reflect"
)

type UpdateHandler func(update model.KVPair) (filteredUpdate model.KVPair, skipFelix bool)

type Dispatcher struct {
	listenersByType map[reflect.Type][]UpdateHandler
}

// NewDispatcher creates a Dispatcher with all its event handlers set to no-ops.
func NewDispatcher() *Dispatcher {
	d := Dispatcher{
		listenersByType: make(map[reflect.Type][]UpdateHandler),
	}
	return &d
}

func (d *Dispatcher) Register(keyExample model.Key, receiver UpdateHandler) {
	keyType := reflect.TypeOf(keyExample)
	if keyType.Kind() == reflect.Ptr {
		panic("Register expects a non-pointer")
	}
	glog.Infof("Registering listener for type %v: %#v", keyType, receiver)
	d.listenersByType[keyType] = append(d.listenersByType[keyType], receiver)
}

func (d *Dispatcher) DispatchUpdate(update model.KVPair) (model.KVPair, bool) {
	glog.V(3).Infof("Dispatching %v", update)
	keyType := reflect.TypeOf(update.Key)
	glog.V(4).Info("Type: ", keyType)
	listeners := d.listenersByType[keyType]
	glog.V(4).Infof("Listeners: %#v", listeners)
	skipFelix := false
	skip := false
	for _, recv := range listeners {
		update, skip = recv(update)
		skipFelix = skipFelix || skip
	}
	return update, skipFelix
}
