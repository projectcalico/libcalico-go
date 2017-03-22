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
	goerrors "errors"

	"github.com/projectcalico/libcalico-go/lib/api/unversioned"
	"github.com/projectcalico/libcalico-go/lib/backend/etcd"
	"github.com/projectcalico/libcalico-go/lib/backend/k8s"
)

type DatastoreType string

const (
	EtcdV2     DatastoreType = "etcdv2"
	Kubernetes DatastoreType = "kubernetes"
)

// CalicoAPIConfig contains the connection information fo
type CalicoAPIConfig struct {
	unversioned.TypeMetadata
	Metadata CalicoAPIConfigMetadata `json:"metadata,omitempty"`
	Spec     CalicoAPIConfigSpec     `json:"spec,omitempty"`
}

// CalicoAPIConfigMetadata contains the metadata for a Calico CalicoAPIConfig resource.
type CalicoAPIConfigMetadata struct {
	unversioned.ObjectMetadata
}

// CalicoAPIConfigSpec contains the specification for a Calico CalicoAPIConfig resource.
type CalicoAPIConfigSpec struct {
	DatastoreType DatastoreType `json:"datastoreType" envconfig:"DATASTORE_TYPE" default:"etcdv2"`

	// Inline the ectd config fields
	etcd.EtcdConfig

	// Inline the k8s config fields.
	k8s.KubeConfig
}

// NewCalicoAPIConfig creates a new (zeroed) CalicoAPIConfig struct with the
// TypeMetadata initialised to the current version.
func NewCalicoAPIConfig() *CalicoAPIConfig {
	return &CalicoAPIConfig{
		TypeMetadata: unversioned.TypeMetadata{
			Kind:       "calicoApiConfig",
			APIVersion: unversioned.VersionCurrent,
		},
	}
}

// PriorityMerge will merge the high and low config taking any values from the
// low when the value is not set in the high, will also merge the Datastore
// data for the type selected by DatastoreType. The returned config is a new
// config.
func PriorityMerge(high CalicoAPIConfig, low CalicoAPIConfig) (CalicoAPIConfig, error) {
	c := high
	errorCfg := CalicoAPIConfig{}

	if high.TypeMetadata.APIVersion != low.TypeMetadata.APIVersion {
		return errorCfg, goerrors.New("Unable to merge config " + high.TypeMetadata.APIVersion + " and " + low.TypeMetadata.APIVersion)
	}

	if c.Spec.DatastoreType == "" {
		c.Spec.DatastoreType = low.Spec.DatastoreType
	}

	switch c.Spec.DatastoreType {
	case EtcdV2:
		var err error
		c.Spec.EtcdConfig, err = etcd.PriorityMerge(high.Spec.EtcdConfig, low.Spec.EtcdConfig)
		if err != nil {
			return errorCfg, err
		}
	case Kubernetes:
		var err error
		c.Spec.KubeConfig, err = k8s.PriorityMerge(high.Spec.KubeConfig, low.Spec.KubeConfig)
		if err != nil {
			return errorCfg, err
		}
	}

	return c, nil
}

// UpdateWithDefaults will set defaults on the config and set defaults on
// the set DatastoreType.
func (c *CalicoAPIConfig) UpdateWithDefaults() {
	if c.Spec.DatastoreType == "" {
		c.Spec.DatastoreType = EtcdV2
	}

	switch c.Spec.DatastoreType {
	case EtcdV2:
		c.Spec.EtcdConfig.UpdateWithDefaults()
	case Kubernetes:
		c.Spec.KubeConfig.UpdateWithDefaults()
	}
}
