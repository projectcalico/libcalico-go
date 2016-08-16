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
	"github.com/tigera/libcalico-go/lib/backend/api"
	"github.com/tigera/libcalico-go/lib/backend/model"
)

type ConfigBatcher struct {
	hostname        string
	datastoreInSync bool
	configDirty     bool
	globalConfig    map[string]string
	hostConfig      map[string]string
	callbacks       configCallbacks
}

func NewConfigBatcher(hostname string, callbacks configCallbacks) *ConfigBatcher {
	return &ConfigBatcher{
		hostname:     hostname,
		configDirty:  true,
		globalConfig: make(map[string]string),
		hostConfig:   make(map[string]string),
		callbacks:    callbacks,
	}
}

func (cb *ConfigBatcher) OnUpdate(update model.KVPair) (filterOut bool) {
	switch key := update.Key.(type) {
	case model.HostConfigKey:
		if key.Hostname != cb.hostname {
			glog.V(4).Infof("Ignoring host config not for this host: %v", key)
			filterOut = true
			return
		}
		glog.V(2).Infof("Host config update for this host: %v", update)
		if value, ok := update.Value.(string); value != cb.hostConfig[key.Name] {
			if ok {
				cb.hostConfig[key.Name] = value
			} else {
				delete(cb.hostConfig, key.Name)
			}
			cb.configDirty = true
		}
	case model.GlobalConfigKey:
		glog.V(2).Infof("Global config update: %v", update)
		if value, ok := update.Value.(string); value != cb.globalConfig[key.Name] {
			if ok {
				cb.globalConfig[key.Name] = value
			} else {
				delete(cb.globalConfig, key.Name)
			}
			cb.configDirty = true
		}
	default:
		glog.Fatalf("Unexpected update: %#v", update)
	}
	cb.maybeSendCachedConfig()
	return
}

func (cb *ConfigBatcher) OnDatamodelStatus(status api.SyncStatus) {
	if !cb.datastoreInSync && status == api.InSync {
		glog.V(2).Infof("Datamodel in sync, flushing config update")
		cb.datastoreInSync = true
		cb.maybeSendCachedConfig()
	}
}

func (cb *ConfigBatcher) maybeSendCachedConfig() {
	if !cb.configDirty || !cb.datastoreInSync {
		return
	}
	glog.V(2).Infof("Sending config update global: %v, host: %v.",
		cb.globalConfig, cb.hostConfig)
	globalConfigCopy := make(map[string]string)
	hostConfigCopy := make(map[string]string)
	for k, v := range cb.globalConfig {
		globalConfigCopy[k] = v
	}
	for k, v := range cb.hostConfig {
		hostConfigCopy[k] = v
	}
	cb.callbacks.OnConfigUpdate(globalConfigCopy, hostConfigCopy)
}
