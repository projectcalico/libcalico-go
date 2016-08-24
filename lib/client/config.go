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
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"

	"github.com/tigera/libcalico-go/lib/api"
	"github.com/tigera/libcalico-go/lib/api/unversioned"
	"github.com/tigera/libcalico-go/lib/backend/model"
	"github.com/tigera/libcalico-go/lib/component"
	"github.com/tigera/libcalico-go/lib/errors"
	"github.com/tigera/libcalico-go/lib/scope"
	"github.com/golang/glog"
)

// configInfo contains the conversion info mapping between API and backend.
// config names and values.  This is defined for each valid config field, and is
// stored in a configConversionHelper for which is there is one per valid combination
// of component and scope.
type configInfo struct {
	// The name of the config field in the API.
	apiName               string

	// The name of the config field in the backend structure.
	backendName           string

	// A regex string used to validate the value in the API config.  If blank, we do
	// not validate using this regex.
	validateRegexAPI      string

	// A function used to validate the value in the API config.  If nil, we do not
	// validate using this function.
	validateFuncAPI       func(string) error

	// Function to convert the API value to the equivalent value in the backend.  If
	// nil, the value does not need converting.
	valueConvertToBackend func(string) (string, error)

	// Function to convert the backend value to the equivalent value in the API.  If
	// nil, the value does not need converting.
	valueConvertToAPI     func(string) (string, error)

	// The value used to unset the config option.  If blank, the config option is
	// deleted from the datastore.
	unsetValue            string
}

// We extend the conversionHelper interface to add some of our own conversion
// functions.  Each configConversionHelper is tied to a specific scope and
// component (e.g. global BGP config, or host specific Felix config).
//
// The convertConfigToBackend/convertMetadataToBackend and convertConfigToAPI convert
// an api.Config object or api.ConfigMetadata to use the backend and api representations
// of the Name and Value.  By performing the conversion at this layer rather than in the
// backend processing is useful because:
// 	-  We can group together all of the mapping in a single location.
// 	-  It will be easier to add a "--raw" option to pass values unaltered (as we may want
// 	   to do to handle config options that haven't been added to libcalico yet).
// Note though that this is not particularly architectural - and so this mapping may be later
// pushed out into the backend (which will make more sense longer term to allow us to have
// different data models for different backends).
type configConversionHelper interface {
	conversionHelper
	registerConfigInfo(configInfo)
	matchesConfigMetadata(metadata api.ConfigMetadata) bool
	convertConfigToBackend(a *api.Config) (*api.Config, error)
	convertConfigToAPI(a *api.Config) *api.Config
	convertMetadataToBackend(metadata api.ConfigMetadata) api.ConfigMetadata
	getUnsetValue(metadata api.ConfigMetadata) string
}

// configMap implements part of the configConversionHelper interface.  This provides
// the scope/component specific mappings between API and backlevel versions of the
// field names and values.
type configMap struct {
	scope             scope.Scope
	component         component.Component
	apiNameToInfo     map[string]configInfo
	backendNameToInfo map[string]configInfo
}

// Return the "unset" value for the field indicated in the Metadata.
func (m *configMap) getUnsetValue(metadata api.ConfigMetadata) string {
	// Get the configInfo from the API name.  This method should only be called
	// for valid field names, so no need to check.
	configInfo := m.apiNameToInfo[metadata.Name]
	return configInfo.unsetValue
}

// registerConfigInfo registers a config field with a particular configConversionHelper.
func (m *configMap) registerConfigInfo(info configInfo) {
	if m.apiNameToInfo == nil {
		m.apiNameToInfo = make(map[string]configInfo)
	}
	if m.backendNameToInfo == nil {
		m.backendNameToInfo = make(map[string]configInfo)
	}
	m.apiNameToInfo[info.apiName] = info
	m.backendNameToInfo[info.backendName] = info
}

// Convert the config object to have values that are correct for the backend.
func (m *configMap) convertConfigToBackend(a *api.Config) (*api.Config, error) {
	// Get the configInfo from the API name.  This method should only be called
	// for valid field names, so no need to check.
	var err error
	configInfo := m.apiNameToInfo[a.Metadata.Name]

	// Validate the value.
	value := a.Spec.Value
	if configInfo.validateRegexAPI != "" {
		re := regexp.MustCompile(configInfo.validateRegexAPI)
		if !re.MatchString(value) {
			return nil, fmt.Errorf("value '%s' not valid", value)
		}
	}
	if configInfo.validateFuncAPI != nil {
		if err = configInfo.validateFuncAPI(value); err != nil {
			return nil, err
		}
	}

	// If necessary convert the value.
	if configInfo.valueConvertToBackend != nil {
		value, err = configInfo.valueConvertToBackend(value)
		if err != nil {
			return nil, err
		}
	}

	r := api.Config{
		Metadata: api.ConfigMetadata{
			Scope:     m.scope,
			Component: m.component,
			Name:      configInfo.backendName,
			Hostname:  a.Metadata.Hostname,
		},
		Spec: api.ConfigSpec{
			Value: value,
		},
	}
	return &r, nil
}

func (m *configMap) convertConfigToAPI(a *api.Config) *api.Config {
	// Get the configInfo from the backend name.  This method may be called for
	// unrecognised fields, in which case return nothing - we'll filter out the
	// result.
	var err error
	configInfo, ok := m.backendNameToInfo[a.Metadata.Name]
	if !ok {
		return nil
	}

	// If necessary convert the value.
	value := a.Spec.Value
	if configInfo.valueConvertToAPI != nil {
		value, err = configInfo.valueConvertToAPI(value)
		if err != nil {
			return nil
		}
	}

	r := api.Config{
		Metadata: api.ConfigMetadata{
			Scope:     m.scope,
			Component: m.component,
			Name:      configInfo.apiName,
			Hostname:  a.Metadata.Hostname,
		},
		Spec: api.ConfigSpec{
			Value: value,
		},
	}
	return &r
}

func (m *configMap) convertMetadataToBackend(metadata api.ConfigMetadata) api.ConfigMetadata {
	// Get the configInfo from the API name, if supplied.  This method should only be called
	// for valid field names, so no need to check.
	name := metadata.Name
	if name != "" {
		configInfo := m.apiNameToInfo[metadata.Name]
		name = configInfo.backendName
	}

	r := api.ConfigMetadata{
		Scope:     m.scope,
		Component: m.component,
		Name:      name,
		Hostname:  metadata.Hostname,
	}
	return r
}

func (m *configMap) matchesConfigMetadata(metadata api.ConfigMetadata) bool {
	// If the Metadata includes a Name field then check if we have that field.
	if metadata.Name != "" {
		if _, ok := m.apiNameToInfo[metadata.Name]; !ok {
			glog.V(2).Infof("Name %s not found in helper", metadata.Name)
			return false
		}
	}

	if metadata.Scope != scope.Undefined && metadata.Scope != m.scope {
		glog.V(2).Infof("Scope '%s' does not match helper '%s'", metadata.Scope, m.scope)
		return false
	}
	if metadata.Component != component.Undefined && metadata.Component != m.component {
		glog.V(2).Infof("Component '%s' does not match helper '%s'", metadata.Component, m.component)
		return false
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

	configConversionHelpers = []configConversionHelper{
		globalBGP,
		hostBGP,
		globalFelix,
		hostFelix,
	}

	// Register logLevel fields.
	globalBGP.registerConfigInfo(configInfo{
		apiName:          "logLevel",
		backendName:      "loglevel",
		validateRegexAPI: "none|debug|info",
	})
	hostBGP.registerConfigInfo(configInfo{
		apiName:          "logLevel",
		backendName:      "loglevel",
		validateRegexAPI: "none|debug|info",
	})
	globalFelix.registerConfigInfo(configInfo{
		apiName:          "logLevel",
		backendName:      "LogSeverityScreen",
		validateRegexAPI: "none|debug|info|warning|error|critical",
	})
	hostFelix.registerConfigInfo(configInfo{
		apiName:          "logLevel",
		backendName:      "LogSeverityScreen",
		validateRegexAPI: "none|debug|info|warning|error|critical",
	})

	// Register global BGP config fields.
	globalBGP.registerConfigInfo(configInfo{
		apiName:               "nodeToNodeMesh",
		backendName:           "node_mesh",
		validateRegexAPI:      "on|off",
		valueConvertToAPI:     nodeToNodeMeshValueConvertToAPI,
		valueConvertToBackend: nodeToNodeMeshValueConvertToBackend,
		unsetValue:            "on",
	})
	globalBGP.registerConfigInfo(configInfo{
		apiName:         "defaultNodeASNumber",
		backendName:     "as_num",
		validateFuncAPI: defaultNodeASNumberValidate,
		unsetValue:      "64511",
	})
}

// getConfigConversionHelpers returns a slice of configConversionHelpers that match
// the supplied Metadata.  For example, if the Metadata has a config name of "logLevel",
// but does not qualify with scope or component - this will return a list of helpers
// for BGP (global and node scope) and Felix (global and node scope).
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

// getSingleConfigConversionHelper returns the configConversionHelper that
// match the supplied metadata.  There should be a single unique match, otherwise an
// error is returned.
//
// This is used by the Set, Unset and Get methods.  For these methods sufficient identifiers
// must be supplied in the Metadata to uniquely identify the config option.
func getSingleConfigConversionHelper(metadata api.ConfigMetadata) (configConversionHelper, error) {
	// At minimum we need a name.
	if metadata.Name == "" {
		return nil, errors.ErrorInsufficientIdentifiers{Name: "name"}
	}

	cchs := getConfigConversionHelpers(metadata)
	if len(cchs) == 0 {
		return nil, fmt.Errorf("config name not recognised")
	}
	if len(cchs) > 1 {
		return nil, fmt.Errorf("config name is not unique - specify scope and/or component")
	}

	return cchs[0], nil
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
		return "on", nil
	} else {
		return "off", nil
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
	Unset(api.ConfigMetadata) error
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
	glog.V(2).Info("Config set invoked")

	// Get the configConversionHelper for the Set - there should be only one.
	cch, err := getSingleConfigConversionHelper(a.Metadata)
	if err != nil {
		return nil, err
	}

	// Convert the request to backend format name and value.
	b, err := cch.convertConfigToBackend(a)
	if err != nil {
		return nil, err
	}

	// A config Set maps to an apply.
	return a, h.c.apply(*b, cch)
}

// Unset removes a single configuration parameter.  For some parameters this may
// simply reset the value to the original default value.
func (h *configs) Unset(metadata api.ConfigMetadata) error {
	glog.V(2).Info("Config unset invoked")

	// Get the configConversionHelper for the Set - there should be only one.
	cch, err := getSingleConfigConversionHelper(metadata)
	if err != nil {
		return err
	}

	// An unset either deletes the entry or resets to default.
	unsetValue := cch.getUnsetValue(metadata)
	if unsetValue == "" {
		bm := cch.convertMetadataToBackend(metadata)
		err = h.c.delete(bm, cch)
	} else {
		a := api.Config{
			Metadata: metadata,
			Spec: api.ConfigSpec{
				Value: unsetValue,
			},
		}
		_, err = h.Set(&a)
	}

	return err
}

// Get returns information about a particular config parameter.
func (h *configs) Get(metadata api.ConfigMetadata) (*api.Config, error) {
	glog.V(2).Info("Config get invoked")

	// Get the configConversionHelper for the Set - there should be only one.
	cch, err := getSingleConfigConversionHelper(metadata)
	if err != nil {
		return nil, err
	}

	// Convert the Metadata to use the backend name.
	bm := cch.convertMetadataToBackend(metadata)
	br, err := h.c.get(bm, cch)
	if err != nil {
		return nil, err
	}

	// Convert the backend response to the API response.  This should always succeeed
	// but in the event it doesn't, just return the raw backend data.
	bc := br.(*api.Config)
	ac := cch.convertConfigToAPI(bc)
	if ac == nil {
		return bc, nil
	} else {
		return ac, nil
	}
}

// List takes a Metadata, and returns a ConfigList that contains the list of config
// parameters that match the Metadata (wildcarding missing fields).
func (h *configs) List(metadata api.ConfigMetadata) (*api.ConfigList, error) {
	glog.V(2).Info("Config list invoked")

	l := api.NewConfigList()

	// Get a list of all of the configConversionHelpers that are valid for the
	// supplied Metadata.  There may be multiple if the request did not specify
	// scope or component.
	cchs := getConfigConversionHelpers(metadata)
	for _, cch := range cchs {
		// Convert the Metadata to the backend version for the particular
		// config conversion helper and then list config.
		bm := cch.convertMetadataToBackend(metadata)
		cl := api.NewConfigList()
		err := h.c.list(bm, cch, cl)

		if err != nil {
			return nil, err
		}

		// Loop through the config items and convert each one to API version.
		for _, bc := range cl.Items {
			ac := cch.convertConfigToAPI(&bc)
			if ac != nil {
				l.Items = append(l.Items, *ac)
			}
		}
	}

	return l, nil
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
		configMap: configMap{
			scope:     scope.Global,
			component: component.BGP,
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
		Key:   k,
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
		configMap: configMap{
			scope:     scope.Node,
			component: component.BGP,
		},
	}
}

// convertMetadataToListInterface converts a ConfigMetadata to a HostBGPConfigListOptions.
// This is part of the conversionHelper interface.
func (h *hostBGPConfigConversionHelper) convertMetadataToListInterface(m unversioned.ResourceMetadata) (model.ListInterface, error) {
	pm := m.(api.ConfigMetadata)
	l := model.HostBGPConfigListOptions{
		Name: pm.Name,
		Hostname: pm.Hostname,
	}
	return l, nil
}

// convertMetadataToKey converts a ConfigMetadata to a HostBGPConfigKey
// This is part of the conversionHelper interface.
func (h *hostBGPConfigConversionHelper) convertMetadataToKey(m unversioned.ResourceMetadata) (model.Key, error) {
	pm := m.(api.ConfigMetadata)
	k := model.HostBGPConfigKey{
		Name: pm.Name,
		Hostname: pm.Hostname,
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
		Key:   k,
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
		configMap: configMap{
			scope:     scope.Global,
			component: component.Felix,
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
		Key:   k,
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
		configMap: configMap{
			scope:     scope.Node,
			component: component.Felix,
		},
	}
}

// convertMetadataToListInterface converts a ConfigMetadata to a HostConfigListOptions.
// This is part of the conversionHelper interface.
func (h *hostFelixConfigConversionHelper) convertMetadataToListInterface(m unversioned.ResourceMetadata) (model.ListInterface, error) {
	pm := m.(api.ConfigMetadata)
	l := model.HostConfigListOptions{
		Name: pm.Name,
		Hostname: pm.Hostname,
	}
	return l, nil
}

// convertMetadataToKey converts a ConfigMetadata to a HostConfigKey
// This is part of the conversionHelper interface.
func (h *hostFelixConfigConversionHelper) convertMetadataToKey(m unversioned.ResourceMetadata) (model.Key, error) {
	pm := m.(api.ConfigMetadata)
	k := model.HostConfigKey{
		Name: pm.Name,
		Hostname: pm.Hostname,
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
		Key:   k,
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
	apiConfig.Metadata.Hostname = backendConfigKey.Hostname
	apiConfig.Spec.Value = backendConfigValue

	return apiConfig, nil
}
