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
	"github.com/tigera/libcalico-go/lib/errors"
	"github.com/tigera/libcalico-go/lib/api/unversioned"
	"github.com/tigera/libcalico-go/lib/backend/model"
	"github.com/tigera/libcalico-go/lib/scope"
	"fmt"
	"strconv"
	"encoding/json"
)

// configInfo contains the conversion info mapping between API and backend.
// config names and values.
type configInfo struct {
	apiName               string
	backendName           string
	validateRegexAPI      string
	validateFuncAPI       func(string) error
	valueConvertToAPI     func(string) (string, error)
	valueConvertToBackend func(string) (string, error)
	unsetValue            *string
}

// We extend the conversionHelper interface to add some of our own conversion
// functions.
type configConversionHelper interface {
	conversionHelper
	registerConfigInfo(configInfo)
	getConfigInfoFromAPIName(string) *configInfo
	getConfigInfoFromBackendName(string) *configInfo
	matchesConfigMetadata(metadata *api.ConfigMetadata) bool
}

// configMap implements part of the configConversionHelper interface.
type configMap struct {
	scope             scope.Scope
	component         api.Component
	apiNameToInfo     map[string]*configInfo
	backendNameToInfo map[string]*configInfo
}

func (m *configMap) registerConfigInfo(info *configInfo) {
	m.apiNameToInfo[info.apiName] = info
	m.backendNameToInfo[info.backendName] = info
}

func (m *configMap) getConfigInfoFromAPIName(name string) *configInfo {
	return m.apiNameToInfo[name]
}


func (m *configMap) getConfigInfoFromBackendName(name string) *configInfo {
	return m.backendNameToInfo[name]
}

func (m *configMap) matchesConfigMetadata(metadata *api.ConfigMetadata) bool {
	// If the Metadata includes a Name field then check if we have that field.
	// We don't do this check if the request is raw.
	if !metadata.Raw && metadata.Name != "" && metadata.Name {
		if _, ok := m.apiNameToInfo[metadata.Name]; !ok {
			return false
		}
	}

	// If the request is raw then the scope and component need to match exactly, otherwise
	// the scope and component may be wildcarded.
	if metadata.Raw {
		if metadata.Scope != m.scope || metadata.Component != m.component {
			return false
		}
	} else {
		if metadata.Scope != scope.Undefined && metadata.Scope != m.scope {
			return false
		}
		if metadata.Component != api.ComponentUndefined && metadata.Component != m.component {
			return false
		}
	}

	return true
}

// configConversionHelpers contains the full list of config conversion helpers
// that can be used to map config values between API and backend formats.
var configConversionHelpers []configConversionHelper

func init() {
	globalBGP := newGlobalBGPConfigConversionHelper()
	hostBGP := newHostBGPConfigConversionHelper()
	globalFelix := newGlobalFelixConfigConversionHelper()
	hostFelix := newHostFelixConfigConversionHelper()

	configConversionHelpers = []configConversionHelper {
		globalBGP,
		hostBGP,
		globalFelix,
		hostFelix,
	}


	// Register logLevel fields.
	globalBGP.registerConfigInfo(configInfo{
		apiName: "logLevel",
		backendName: "loglevel",
		validateRegexAPI: "none|debug|info",
	})
	hostBGP.registerConfigInfo(configInfo{
		apiName: "logLevel",
		backendName: "loglevel",
		validateRegexAPI: "none|debug|info",
	})
	globalFelix.registerConfigInfo(configInfo{
		apiName: "logLevel",
		backendName: "LogSeverityScreen",
		validateRegexAPI: "none|debug|info|warning|error|critical",
	})
	hostFelix.registerConfigInfo(configInfo{
		apiName: "logLevel",
		backendName: "LogSeverityScreen",
		validateRegexAPI: "none|debug|info|warning|error|critical",
	})

	// Register global BGP config fields.
	globalBGP.registerConfigInfo(configInfo{
		apiName: "nodeToNodeMesh",
		backendName: "node_mesh",
		validateRegexAPI: "on|off",
		valueConvertToAPI: nodeToNodeMeshValueConvertToAPI,
		valueConvertToBackend: nodeToNodeMeshValueConvertToBackend,
		unsetValue: "on",
	})
	globalBGP.registerConfigInfo(configInfo{
		apiName: "defaultNodeASNumber",
		backendName: "as_num",
		validateFuncAPI: defaultNodeASNumberValidate,
		unsetValue: 64511,
	})
}

// getConfigConversionHelpers returns a slice of configConversionHelpers that match
// the supplied Metadata.
func getConfigConversionHelpers(metadata api.ConfigMetadata) []configConversionHelper {
	cchs := []configConversionHelper{}
	for ii := 0; ii < len(configConversionHelpers); ii++ {
		cch := configConversionHelpers[ii]
		if cch.matchesConfigMetadata(metadata) {
			cchs = append(cchs, cch)
		}
	}

	return cchs
}

// defaultNodeASNumberValidate validates the the value of the default node AS number
// configuration field.
func defaultNodeASNumberValidate(value string) error {
	i, err := strconv.Atoi(value)
	if err != nil || i < 0 || i > 4294967295 {
		return fmt.Errorf("the value '%s' is not a valid AS Number", value)
	}
	return nil
}

// nodeToNodeMesh is a struct containing whether node-to-node mesh is enabled.  It can be
// JSON marshalled into the correct structure that is understood by the Calico BGP component.
type nodeToNodeMesh struct {
	Enabled bool `json:"enabled"`
}

// nodeToNodeMeshValueConvertToAPI converts the node-to-node mesh value to
// the equivalent API value.  The backend value is a JSON structure with an enabled
// field set to a true or false boolean - this maps through to an on or off value.
func nodeToNodeMeshValueConvertToAPI(value string) (string, error) {
	n := nodeToNodeMesh{}
	err := json.Unmarshal([]byte(value), &n)
	if err != nil {
		return "", err
	} else if n.Enabled {
		return "on"
	} else {
		return "off"
	}
}

// nodeToNodeMeshValueConvertToBackend converts the node-to-node mesh value to
// the equivalent backend value.  The API takes an on or off value - this maps through
// to a JSON structure with an enabled field set to a true or false boolean.
func nodeToNodeMeshValueConvertToBackend(value string) (string, error) {
	n := nodeToNodeMesh{Enabled: value == "on"}
	b, err := json.Marshal(n)
	if err != nil {
		return "", err
	} else {
		return string(b), nil
	}
}


// ConfigInterface has methods to set, unset and view system configuration.
type ConfigInterface interface {
	List(api.ConfigMetadata) (*api.ConfigList, error)
	Get(api.ConfigMetadata) (*api.Config, error)
	Set(*api.Config) (*api.Config, error)
	Unset(*api.Config) (*api.Config, error)
}

// configs implements ConfigInterface
type configs struct {
	c *Client
}

// newConfigs returns a new ConfigInterface bound to the supplied client.
func newConfigs(c *Client) ConfigInterface {
	return &configs{c: c}
}

// Set sets a single configuration parameter.
func (h *configs) Set(a *api.Config) (*api.Config, error) {
	// We at minimum need a name.
	if a.Metadata.Name == "" {
		return "", errors.ErrorInsufficientIdentifiers{Name: "name"}
	}

	// Get a slice of configConversionHelpers that match the request.  If there are
	// more than one then insufficient identifiers have been supplied.
	cchs := getConfigConversionHelpers(a.Metadata)
	if len(cchs) == 0 {
		return "", fmt.Errorf("config name not recognised")
	}
	if len(cchs) > 1 {
		return "", errors.ErrorInsufficientIdentifiers{Name: "name"}
	}

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

// globalBGPConfigConversionHelper implements the configConversionHelper interface for
// global BGP configuration parameters.
type globalBGPConfigConversionHelper struct {
	configMap
}

// newGlobalBGPConfigConversionHelper is used to instantiate a new globalBGPConfigConversionHelper
// instance.
func newGlobalBGPConfigConversionHelper() *globalBGPConfigConversionHelper {
	return &globalBGPConfigConversionHelper{
		configMap: configMap {
			scope: scope.Global,
			component: api.ComponentBGP,
		},
	}
}

// convertMetadataToListInterface converts a ConfigMetadata to a GlobalBGPConfigListOptions.
// This is part of the conversionHelper interface.
func (h *globalBGPConfigConversionHelper) convertMetadataToListInterface(m unversioned.ResourceMetadata) (model.ListInterface, error) {
	pm := m.(api.ConfigMetadata)
	l := model.GlobalBGPConfigListOptions{
		Name: pm.Name,
	}
	return l, nil
}

// convertMetadataToKey converts a ConfigMetadata to a GlobalBGPConfigKey
// This is part of the conversionHelper interface.
func (h *globalBGPConfigConversionHelper) convertMetadataToKey(m unversioned.ResourceMetadata) (model.Key, error) {
	pm := m.(api.ConfigMetadata)
	k := model.GlobalBGPConfigKey{
		Name: pm.Name,
	}
	return k, nil
}

// convertAPIToKVPair converts an API Config structure to a KVPair containing a
// string value and GlobalBGPConfigKey.
// This is part of the conversionHelper interface.
func (h *globalBGPConfigConversionHelper) convertAPIToKVPair(a unversioned.Resource) (*model.KVPair, error) {
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
func (h *globalBGPConfigConversionHelper) convertKVPairToAPI(d *model.KVPair) (unversioned.Resource, error) {
	backendConfigValue := d.Value.(string)
	backendConfigKey := d.Key.(model.GlobalBGPConfigKey)

	apiConfig := api.NewConfig()
	apiConfig.Metadata.Name = backendConfigKey.Name
	apiConfig.Metadata.Scope = scope.Global
	apiConfig.Metadata.Hostname = ""
	apiConfig.Spec.Value = backendConfigValue

	return apiConfig, nil
}

// hostBGPConfigConversionHelper implements the configConversionHelper interface for
// host BGP configuration parameters.
type hostBGPConfigConversionHelper struct {
	configMap
}

// newHostBGPConfigConversionHelper is used to instantiate a new globalBGPConfigConversionHelper
// instance.
func newHostBGPConfigConversionHelper() *hostBGPConfigConversionHelper {
	return &hostBGPConfigConversionHelper{
		configMap: configMap {
			scope: scope.Node,
			component: api.ComponentBGP,
		},
	}
}

// convertMetadataToListInterface converts a ConfigMetadata to a HostBGPConfigListOptions.
// This is part of the conversionHelper interface.
func (h *hostBGPConfigConversionHelper) convertMetadataToListInterface(m unversioned.ResourceMetadata) (model.ListInterface, error) {
	pm := m.(api.ConfigMetadata)
	l := model.HostBGPConfigListOptions{
		Name: pm.Name,
	}
	return l, nil
}

// convertMetadataToKey converts a ConfigMetadata to a HostBGPConfigKey
// This is part of the conversionHelper interface.
func (h *hostBGPConfigConversionHelper) convertMetadataToKey(m unversioned.ResourceMetadata) (model.Key, error) {
	pm := m.(api.ConfigMetadata)
	k := model.HostBGPConfigKey{
		Name: pm.Name,
	}
	return k, nil
}

// convertAPIToKVPair converts an API Config structure to a KVPair containing a
// string value and HostBGPConfigKey.
// This is part of the conversionHelper interface.
func (h *hostBGPConfigConversionHelper) convertAPIToKVPair(a unversioned.Resource) (*model.KVPair, error) {
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
func (h *hostBGPConfigConversionHelper) convertKVPairToAPI(d *model.KVPair) (unversioned.Resource, error) {
	backendConfigValue := d.Value.(string)
	backendConfigKey := d.Key.(model.HostBGPConfigKey)

	apiConfig := api.NewConfig()
	apiConfig.Metadata.Name = backendConfigKey.Name
	apiConfig.Metadata.Scope = scope.Node
	apiConfig.Metadata.Hostname = backendConfigKey.Hostname
	apiConfig.Spec.Value = backendConfigValue

	return apiConfig, nil
}

// globalFelixConfigConversionHelper implements the configConversionHelper interface for
// global Felix configuration parameters.
type globalFelixConfigConversionHelper struct {
	configMap
}

// newGlobalFelixConfigConversionHelper is used to instantiate a new globalFelixConfigConversionHelper
// instance.
func newGlobalFelixConfigConversionHelper() *globalFelixConfigConversionHelper {
	return &globalFelixConfigConversionHelper{
		configMap: configMap {
			scope: scope.Global,
			component: api.ComponentFelix,
		},
	}
}

// convertMetadataToListInterface converts a ConfigMetadata to a GlobalConfigListOptions.
// This is part of the conversionHelper interface.
func (h *globalFelixConfigConversionHelper) convertMetadataToListInterface(m unversioned.ResourceMetadata) (model.ListInterface, error) {
	pm := m.(api.ConfigMetadata)
	l := model.GlobalConfigListOptions{
		Name: pm.Name,
	}
	return l, nil
}

// convertMetadataToKey converts a ConfigMetadata to a GlobalConfigKey
// This is part of the conversionHelper interface.
func (h *globalFelixConfigConversionHelper) convertMetadataToKey(m unversioned.ResourceMetadata) (model.Key, error) {
	pm := m.(api.ConfigMetadata)
	k := model.GlobalConfigKey{
		Name: pm.Name,
	}
	return k, nil
}

// convertAPIToKVPair converts an API Config structure to a KVPair containing a
// string value and GlobalConfigKey.
// This is part of the conversionHelper interface.
func (h *globalFelixConfigConversionHelper) convertAPIToKVPair(a unversioned.Resource) (*model.KVPair, error) {
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
func (h *globalFelixConfigConversionHelper) convertKVPairToAPI(d *model.KVPair) (unversioned.Resource, error) {
	backendConfigValue := d.Value.(string)
	backendConfigKey := d.Key.(model.GlobalConfigKey)

	apiConfig := api.NewConfig()
	apiConfig.Metadata.Name = backendConfigKey.Name
	apiConfig.Metadata.Scope = scope.Global
	apiConfig.Metadata.Hostname = ""
	apiConfig.Spec.Value = backendConfigValue

	return apiConfig, nil
}

// hostFelixConfigConversionHelper implements the configConversionHelper interface for
// host Felix configuration parameters.
type hostFelixConfigConversionHelper struct {
	configMap
}

// newHostFelixConfigConversionHelper is used to instantiate a new hostFelixConfigConversionHelper
// instance.
func newHostFelixConfigConversionHelper() *hostFelixConfigConversionHelper {
	return &hostFelixConfigConversionHelper{
		configMap: configMap {
			scope: scope.Node,
			component: api.ComponentFelix,
		},
	}
}

// convertMetadataToListInterface converts a ConfigMetadata to a HostConfigListOptions.
// This is part of the conversionHelper interface.
func (h *hostFelixConfigConversionHelper) convertMetadataToListInterface(m unversioned.ResourceMetadata) (model.ListInterface, error) {
	pm := m.(api.ConfigMetadata)
	l := model.HostConfigListOptions{
		Name: pm.Name,
	}
	return l, nil
}

// convertMetadataToKey converts a ConfigMetadata to a HostConfigKey
// This is part of the conversionHelper interface.
func (h *hostFelixConfigConversionHelper) convertMetadataToKey(m unversioned.ResourceMetadata) (model.Key, error) {
	pm := m.(api.ConfigMetadata)
	k := model.HostConfigKey{
		Name: pm.Name,
	}
	return k, nil
}

// convertAPIToKVPair converts an API Config structure to a KVPair containing a
// string value and HostConfigKey.
// This is part of the conversionHelper interface.
func (h *hostFelixConfigConversionHelper) convertAPIToKVPair(a unversioned.Resource) (*model.KVPair, error) {
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
func (h *hostFelixConfigConversionHelper) convertKVPairToAPI(d *model.KVPair) (unversioned.Resource, error) {
	backendConfigValue := d.Value.(string)
	backendConfigKey := d.Key.(model.HostConfigKey)

	apiConfig := api.NewConfig()
	apiConfig.Metadata.Name = backendConfigKey.Name
	apiConfig.Metadata.Scope = scope.Node
	apiConfig.Metadata.Hostname = backendConfigKey.Hostname
	apiConfig.Spec.Value = backendConfigValue

	return apiConfig, nil
}
