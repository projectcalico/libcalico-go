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
	"github.com/tigera/libcalico-go/lib/api/unversioned"
	"github.com/tigera/libcalico-go/lib/component"
	"github.com/tigera/libcalico-go/lib/scope"
)

type ConfigMetadata struct {
	unversioned.ObjectMetadata

	// The config name.
	Name string `json:"name" validate:"omitempty,configkey"`

	// The scope of the config.  This may be global or node.  If the config scope is
	// node, the hostname must also be supplied.
	Scope scope.Scope `json:"scope" validate:"omitempty,scopeglobalornode"`

	// The component that the config operates on.  This may be bgp or felix.
	Component component.Component `json:"component" validate:"omitempty,component"`

	// The hostname of the node that the config applies to.  When modifying config
	// the hostname must be specified when the scope is `node`, and must
	// be omitted when the scope is `global`.
	Hostname string `json:"hostname,omitempty" validate:"omitempty,name"`
}

type ConfigSpec struct {
	// The config value.
	Value string `json:"value"`
}

type Config struct {
	unversioned.TypeMetadata

	// Metadata for Config.
	Metadata ConfigMetadata `json:"metadata,omitempty"`

	// Specification for Config.
	Spec ConfigSpec `json:"spec,omitempty"`
}

func NewConfig() *Config {
	return &Config{TypeMetadata: unversioned.TypeMetadata{Kind: "config", APIVersion: "v1"}}
}

type ConfigList struct {
	unversioned.TypeMetadata
	Metadata unversioned.ListMetadata `json:"metadata,omitempty"`
	Items    []Config                 `json:"items" validate:"dive"`
}

func NewConfigList() *ConfigList {
	return &ConfigList{TypeMetadata: unversioned.TypeMetadata{Kind: "configList", APIVersion: "v1"}}
}
