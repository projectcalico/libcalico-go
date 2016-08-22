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

package client

import (
	"github.com/tigera/libcalico-go/lib/api"
	"github.com/tigera/libcalico-go/lib/api/unversioned"
	"github.com/tigera/libcalico-go/lib/backend/model"
	"github.com/tigera/libcalico-go/lib/scope"
)

// ConfigInterface has methods to set, unset and view system configuration.
type ConfigInterface interface {
	List(api.ConfigMetadata) (*api.ConfigList, error)
	Get(api.ConfigMetadata) (*api.Config, error)
	Set(*api.Config) (*api.Config, error)
	Unset(*api.Config) (*api.Config, error)
}

// peers implements ConfigInterface
type configs struct {
	c *Client
	globalBGP globalBGPConfigHelper
	hostBGP hostBGPConfigHelper
	globalFelix globalFelixConfigHelper
	hostFelix hostFelixConfigHelper
}

// newConfigs returns a new ConfigInterface bound to the supplied client.
func newConfigs(c *Client) ConfigInterface {
	return &configs{c: c}
}

// Set sets a single configuration parameter.
func (h *configs) Set(a *api.Config) (*api.Config, error) {
	return a, h.c.create(*a, h)
}

// Unset removes a single configuration parameter.  For some parameters this may
// simply reset the value to the original default value.
func (h *configs) Unset(a *api.Config) (*api.Config, error) {
	return a, h.c.update(*a, h)
}

// Get returns information about a particular config parameter.
func (h *configs) Get(metadata api.ConfigMetadata) (*api.Config, error) {
	if a, err := h.c.get(metadata, h); err != nil {
		return nil, err
	} else {
		return a.(*api.Config), nil
	}
}

// List takes a Metadata, and returns a ConfigList that contains the list of config
// parameters that match the Metadata (wildcarding missing fields).
func (h *configs) List(metadata api.ConfigMetadata) (*api.ConfigList, error) {
	l := api.NewConfigList()
	err := h.c.list(metadata, h, l)
	return l, err
}

// The config management interface actually operates on multiple different backend
// models.  We define a conversionHelper for each backend type.

type globalBGPConfigHelper struct {}
type hostBGPConfigHelper struct {}
type globalFelixConfigHelper struct {}
type hostFelixConfigHelper struct {}

// convertMetadataToListInterface converts a ConfigMetadata to a GlobalBGPConfigListOptions.
// This is part of the conversionHelper interface.
func (h *globalBGPConfigHelper) convertMetadataToListInterface(m unversioned.ResourceMetadata) (model.ListInterface, error) {
	pm := m.(api.ConfigMetadata)
	l := model.GlobalBGPConfigListOptions{
		Name: pm.Name,
	}
	return l, nil
}

// convertMetadataToKey converts a ConfigMetadata to a GlobalBGPConfigKey
// This is part of the conversionHelper interface.
func (h *globalBGPConfigHelper) convertMetadataToKey(m unversioned.ResourceMetadata) (model.Key, error) {
	pm := m.(api.ConfigMetadata)
	k := model.GlobalBGPConfigKey{
		Name: pm.Name,
	}
	return k, nil
}

// convertAPIToKVPair converts an API Config structure to a KVPair containing a
// string value and GlobalBGPConfigKey.
// This is part of the conversionHelper interface.
func (h *globalBGPConfigHelper) convertAPIToKVPair(a unversioned.Resource) (*model.KVPair, error) {
	ap := a.(api.Config)
	k, err := h.convertMetadataToKey(ap.Metadata)
	if err != nil {
		return nil, err
	}

	d := model.KVPair{
		Key: k,
		Value: ap.Spec.Value,
	}

	return &d, nil
}

// convertKVPairToAPI converts a KVPair containing a string value and GlobalBGPConfigKey
// to an API Config structure.
// This is part of the conversionHelper interface.
func (h *globalBGPConfigHelper) convertKVPairToAPI(d *model.KVPair) (unversioned.Resource, error) {
	backendConfigValue := d.Value.(string)
	backendConfigKey := d.Key.(model.GlobalBGPConfigKey)

	apiConfig := api.NewConfig()
	apiConfig.Metadata.Name = backendConfigKey.Name
	apiConfig.Metadata.Scope = scope.Global
	apiConfig.Metadata.Hostname = ""
	apiConfig.Spec.Value = backendConfigValue

	return apiConfig, nil
}

// convertMetadataToListInterface converts a ConfigMetadata to a HostBGPConfigListOptions.
// This is part of the conversionHelper interface.
func (h *hostBGPConfigHelper) convertMetadataToListInterface(m unversioned.ResourceMetadata) (model.ListInterface, error) {
	pm := m.(api.ConfigMetadata)
	l := model.HostBGPConfigListOptions{
		Name: pm.Name,
	}
	return l, nil
}

// convertMetadataToKey converts a ConfigMetadata to a HostBGPConfigKey
// This is part of the conversionHelper interface.
func (h *hostBGPConfigHelper) convertMetadataToKey(m unversioned.ResourceMetadata) (model.Key, error) {
	pm := m.(api.ConfigMetadata)
	k := model.HostBGPConfigKey{
		Name: pm.Name,
	}
	return k, nil
}

// convertAPIToKVPair converts an API Config structure to a KVPair containing a
// string value and HostBGPConfigKey.
// This is part of the conversionHelper interface.
func (h *hostBGPConfigHelper) convertAPIToKVPair(a unversioned.Resource) (*model.KVPair, error) {
	ap := a.(api.Config)
	k, err := h.convertMetadataToKey(ap.Metadata)
	if err != nil {
		return nil, err
	}

	d := model.KVPair{
		Key: k,
		Value: ap.Spec.Value,
	}

	return &d, nil
}

// convertKVPairToAPI converts a KVPair containing a string value and HostBGPConfigKey
// to an API Config structure.
// This is part of the conversionHelper interface.
func (h *hostBGPConfigHelper) convertKVPairToAPI(d *model.KVPair) (unversioned.Resource, error) {
	backendConfigValue := d.Value.(string)
	backendConfigKey := d.Key.(model.HostBGPConfigKey)

	apiConfig := api.NewConfig()
	apiConfig.Metadata.Name = backendConfigKey.Name
	apiConfig.Metadata.Scope = scope.Node
	apiConfig.Metadata.Hostname = ""
	apiConfig.Spec.Value = backendConfigValue

	return apiConfig, nil
}

// convertMetadataToListInterface converts a ConfigMetadata to a GlobalConfigListOptions.
// This is part of the conversionHelper interface.
func (h *globalFelixConfigHelper) convertMetadataToListInterface(m unversioned.ResourceMetadata) (model.ListInterface, error) {
	pm := m.(api.ConfigMetadata)
	l := model.GlobalConfigListOptions{
		Name: pm.Name,
	}
	return l, nil
}

// convertMetadataToKey converts a ConfigMetadata to a GlobalConfigKey
// This is part of the conversionHelper interface.
func (h *globalFelixConfigHelper) convertMetadataToKey(m unversioned.ResourceMetadata) (model.Key, error) {
	pm := m.(api.ConfigMetadata)
	k := model.GlobalConfigKey{
		Name: pm.Name,
	}
	return k, nil
}

// convertAPIToKVPair converts an API Config structure to a KVPair containing a
// string value and GlobalConfigKey.
// This is part of the conversionHelper interface.
func (h *globalFelixConfigHelper) convertAPIToKVPair(a unversioned.Resource) (*model.KVPair, error) {
	ap := a.(api.Config)
	k, err := h.convertMetadataToKey(ap.Metadata)
	if err != nil {
		return nil, err
	}

	d := model.KVPair{
		Key: k,
		Value: ap.Spec.Value,
	}

	return &d, nil
}

// convertKVPairToAPI converts a KVPair containing a string value and GlobalConfigKey
// to an API Config structure.
// This is part of the conversionHelper interface.
func (h *globalFelixConfigHelper) convertKVPairToAPI(d *model.KVPair) (unversioned.Resource, error) {
	backendConfigValue := d.Value.(string)
	backendConfigKey := d.Key.(model.GlobalConfigKey)

	apiConfig := api.NewConfig()
	apiConfig.Metadata.Name = backendConfigKey.Name
	apiConfig.Metadata.Scope = scope.Global
	apiConfig.Metadata.Hostname = ""
	apiConfig.Spec.Value = backendConfigValue

	return apiConfig, nil
}

// convertMetadataToListInterface converts a ConfigMetadata to a HostConfigListOptions.
// This is part of the conversionHelper interface.
func (h *hostFelixConfigHelper) convertMetadataToListInterface(m unversioned.ResourceMetadata) (model.ListInterface, error) {
	pm := m.(api.ConfigMetadata)
	l := model.HostConfigListOptions{
		Name: pm.Name,
	}
	return l, nil
}

// convertMetadataToKey converts a ConfigMetadata to a HostConfigKey
// This is part of the conversionHelper interface.
func (h *hostFelixConfigHelper) convertMetadataToKey(m unversioned.ResourceMetadata) (model.Key, error) {
	pm := m.(api.ConfigMetadata)
	k := model.HostConfigKey{
		Name: pm.Name,
	}
	return k, nil
}

// convertAPIToKVPair converts an API Config structure to a KVPair containing a
// string value and HostConfigKey.
// This is part of the conversionHelper interface.
func (h *hostFelixConfigHelper) convertAPIToKVPair(a unversioned.Resource) (*model.KVPair, error) {
	ap := a.(api.Config)
	k, err := h.convertMetadataToKey(ap.Metadata)
	if err != nil {
		return nil, err
	}

	d := model.KVPair{
		Key: k,
		Value: ap.Spec.Value,
	}

	return &d, nil
}

// convertKVPairToAPI converts a KVPair containing a string value and HostConfigKey
// to an API Config structure.
// This is part of the conversionHelper interface.
func (h *hostFelixConfigHelper) convertKVPairToAPI(d *model.KVPair) (unversioned.Resource, error) {
	backendConfigValue := d.Value.(string)
	backendConfigKey := d.Key.(model.HostConfigKey)

	apiConfig := api.NewConfig()
	apiConfig.Metadata.Name = backendConfigKey.Name
	apiConfig.Metadata.Scope = scope.Node
	apiConfig.Metadata.Hostname = backendConfigKey.Hostname
	apiConfig.Spec.Value = backendConfigValue

	return apiConfig, nil
}
