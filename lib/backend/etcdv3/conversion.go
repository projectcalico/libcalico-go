// Copyright (c) 2016-2017 Tigera, Inc. All rights reserved.

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

package etcdv3

import (
	"strconv"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
	log "github.com/sirupsen/logrus"

	"github.com/projectcalico/libcalico-go/lib/backend/api"
	"github.com/projectcalico/libcalico-go/lib/backend/model"
	"github.com/projectcalico/libcalico-go/lib/errors"
)

// convertListResponse converts etcdv3 Kv to a model.KVPair with parsed values.
// If the etcdv3 key or value does not represent the resource specified by the ListInterface,
// or if value cannot be parsed, this method returns nil.
func convertListResponse(ekv *mvccpb.KeyValue, l model.ListInterface) *model.KVPair {
	log.WithField("etcdv3-etcdKey", string(ekv.Key)).Debug("Processing etcdv3 entry")
	if k := l.KeyFromDefaultPath(string(ekv.Key)); k != nil {
		log.WithField("model-etcdKey", k).Debug("Key is valid and converted to model-etcdKey")
		if v, err := model.ParseValue(k, ekv.Value); err == nil {
			log.Debug("Value is valid - return KVPair with parsed value")
			return &model.KVPair{Key: k, Value: v, Revision: strconv.FormatInt(ekv.ModRevision, 10)}
		}
	}
	return nil
}

// convertWatchEvent converts an etcdv3 watch event to an api.WatchEvent, or nil if the
// event did not correspond to an event that we are interested in.
func convertWatchEvent(e *clientv3.Event, l model.ListInterface) (*api.WatchEvent, error) {
	log.WithField("etcdv3-etcdKey", e.Kv.Key).Debug("Processing etcdv3 event")

	var eventType api.WatchEventType
	switch {
	case e.Type == clientv3.EventTypeDelete:
		eventType = api.WatchDeleted
	case e.IsCreate():
		eventType = api.WatchAdded
	default:
		eventType = api.WatchModified
	}

	var old, new *model.KVPair
	var err error
	if k := l.KeyFromDefaultPath(string(e.Kv.Key)); k != nil {
		log.WithField("model-etcdKey", k).Debug("Key is valid and converted to model-etcdKey")

		if eventType != api.WatchDeleted {
			if new, err = etcdToKVPair(k, e.Kv); err != nil {
				return nil, err
			}
		}
		if eventType != api.WatchAdded && e.PrevKv != nil && len(e.PrevKv.Value) != 0 {
			if old, err = etcdToKVPair(k, e.PrevKv); err != nil {
				return nil, err
			}
		}
	}

	return &api.WatchEvent{
		Old:  old,
		New:  new,
		Type: eventType,
	}, nil
}

// etcdToKVPair converts an etcd KeyValue in to model.KVPair.
func etcdToKVPair(key model.Key, ekv *mvccpb.KeyValue) (*model.KVPair, error) {
	v, err := model.ParseValue(key, ekv.Value)
	if err != nil {
		return nil, errors.ErrorParsingDatastoreEntry{
			RawKey:   string(ekv.Key),
			RawValue: string(ekv.Value),
			Err:      err,
		}
	}

	return &model.KVPair{
		Key:      key,
		Value:    v,
		Revision: strconv.FormatInt(ekv.ModRevision, 10),
	}, nil
}
