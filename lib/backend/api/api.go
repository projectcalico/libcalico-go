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

package api

import (
	"fmt"
	. "github.com/tigera/libcalico-go/lib/backend/model"
)

type SyncStatus uint8

const (
	WaitForDatastore SyncStatus = iota
	ResyncInProgress
	InSync
)

func (s SyncStatus) String() string {
	switch s {
	case WaitForDatastore:
		return "wait-for-ready"
	case InSync:
		return "in-sync"
	case ResyncInProgress:
		return "resync"
	default:
		return fmt.Sprintf("Unknown<%v>", uint8(s))
	}
}

type Client interface {
	Create(object *KVPair) (*KVPair, error)
	Update(object *KVPair) (*KVPair, error)
	Apply(object *KVPair) (*KVPair, error)
	Delete(object *KVPair) error
	Get(key Key) (*KVPair, error)
	List(list ListInterface) ([]*KVPair, error)

	// Syncer creates an object that generates a series of KVPair updates,
	// which paint an eventually-consistent picture of the full state of
	// the datastore and then generates subsequent KVPair updates for
	// changes to the datastore.
	Syncer(callbacks SyncerCallbacks) Syncer
}

type Syncer interface {
	Start()
}

type SyncerCallbacks interface {
	OnStatusUpdated(status SyncStatus)
	OnUpdates(updates []KVPair)
}

type SyncerParseFailCallbacks interface {
	ParseFailed(rawKey string, rawValue *string)
}
