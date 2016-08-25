// Copyright (c) 2016 Tigera, Inc. All rights reserved.

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
	"github.com/golang/glog"
	"github.com/tigera/libcalico-go/lib/backend/etcd"
)

type BackendType string

const (
	EtcdV2 BackendType = "etcdv2"
)

// NewConfig returns a pointer to a new config struct for the relevant datastore.
func (b BackendType) NewConfig() interface{} {
	switch b {
	case EtcdV2:
		return &etcd.EtcdConfig{}
	default:
		glog.Errorf("Unknown backend type: %v", b)
		return nil
	}
}

// Client configuration required to instantiate a Calico client interface.
type ClientConfig struct {
	BackendType   BackendType `json:"datastoreType" envconfig:"DATASTORE_TYPE" default:"etcdv2"`
	BackendConfig interface{} `json:"-"`
}
