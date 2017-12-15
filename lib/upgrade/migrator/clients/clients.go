// Copyright (c) 2017 Tigera, Inc. All rights reserved.

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

package clients

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/kelseyhightower/envconfig"
	log "github.com/sirupsen/logrus"

	"github.com/projectcalico/go-yaml-wrapper"
	"github.com/projectcalico/libcalico-go/lib/apiconfig"
	apiv1 "github.com/projectcalico/libcalico-go/lib/apis/v1"
	"github.com/projectcalico/libcalico-go/lib/apis/v1/unversioned"
	"github.com/projectcalico/libcalico-go/lib/backend"
	bapi "github.com/projectcalico/libcalico-go/lib/backend/api"
	"github.com/projectcalico/libcalico-go/lib/backend/compat"
	"github.com/projectcalico/libcalico-go/lib/backend/etcdv2"
)

const (
	DefaultConfigPathV1 = "/etc/calico/apiconfigv1.cfg"
	DefaultConfigPathV3 = "/etc/calico/apiconfigv3.cfg"
)

// LoadClients loads the v3 and v1 clients required for the migration.
// v3Config and v1Config are the full file paths to the v3 APIConfig
// and the v1 APIConfig respectively.  If either are blank, then this
// loads config from the environments.
//
// If using Kubernetes API as the datastore, only the v3Config, or
// v3 environments need to be specified.  The v1 client uses identical
// configuration for this datastore type.
//
// Returns: the v3 backend client, the v1 backend client, whether backed is KDD
func LoadClients(configV3, configV1 string) (bapi.Client, bapi.Client, bool, error) {
	// If the configV3 or configV1 are the default paths, and those files do not exist, then
	// switch to using environments by settings the path to an empty string.
	if _, err := os.Stat(configV3); err != nil {
		if configV3 != DefaultConfigPathV3 {
			return nil, nil, false, fmt.Errorf("Error reading apiconfigv3 file: %s\n", configV3)
		}
		log.Infof("Config file: %s cannot be read - reading config from environment", configV3)
		configV3 = ""
	}
	if _, err := os.Stat(configV1); err != nil {
		if configV1 != DefaultConfigPathV1 {
			return nil, nil, false, fmt.Errorf("Error reading apiconfigv1 file: %s\n", configV1)
		}
		log.Infof("Config file: %s cannot be read - reading config from environment", configV1)
		configV1 = ""
	}

	// Create the backend clients for v1 and v3.
	var clientV1, clientV3 bapi.Client

	// Load the v3 client config - either from file or environments.
	apiConfigV3, err := apiconfig.LoadClientConfig(configV3)
	if err != nil {
		return nil, nil, false, fmt.Errorf("error with apiconfigv3: %v", err)
	}

	// Create the backend v3 client.
	clientV3, err = backend.NewClient(*apiConfigV3)
	if err != nil {
		return nil, nil, false, fmt.Errorf("error with apiconfigv3: %v", err)
	}

	if apiConfigV3.Spec.DatastoreType == apiconfig.Kubernetes {
		log.Debug("v3 API is using Kubernetes API - use same for v1")
		clientV1 = clientV3
	} else {
		log.Debug("v3 API is not Kubernetes API - create the etcdv2 v1 client")
		// Grab the Calico v1 API config (which must be specified).  The datastore
		// type must be etcdv2.
		apiConfigV1, err := loadClientConfigV1(configV1)
		if apiConfigV1.Spec.DatastoreType != apiv1.EtcdV2 {
			return nil, nil, false, fmt.Errorf("expecting apiconfigv1 datastore to be 'etcdv2', got '%s'", apiConfigV1.Spec.DatastoreType)
		}

		// Create the backend etcdv2 client (v1 API). We wrap this in the compat module to handle
		// multi-key backed resources.
		clientV1, err = etcdv2.NewEtcdClient(&apiConfigV1.Spec.EtcdConfig)
		if err != nil {
			return nil, nil, false, fmt.Errorf("error with apiconfigv1: %v", err)
		}
		clientV1 = compat.NewAdaptor(clientV1)
	}

	return clientV3, clientV1, apiConfigV3.Spec.DatastoreType == apiconfig.Kubernetes, nil
}

// loadClientConfigV1 loads the ClientConfig from the specified file (if specified)
// or from environment variables (if the file is not specified).
func loadClientConfigV1(filename string) (*apiv1.CalicoAPIConfig, error) {

	// Override / merge with values loaded from the specified file.
	if filename != "" {
		b, err := ioutil.ReadFile(filename)
		if err != nil {
			return nil, err
		}

		c, err := loadClientConfigFromBytesV1(b)
		if err != nil {
			return nil, fmt.Errorf("syntax error in %s: %s", filename, err)
		}
		return c, nil
	}
	return loadClientConfigFromEnvironmentV1()
}

// loadClientConfigFromBytesV1 loads the ClientConfig from the supplied bytes containing
// YAML or JSON format data.
func loadClientConfigFromBytesV1(b []byte) (*apiv1.CalicoAPIConfig, error) {
	var c apiv1.CalicoAPIConfig

	// Default the backend type to be etcd v2. This will be overridden if
	// explicitly specified in the file.
	log.Info("Loading config from JSON or YAML data")
	c = apiv1.CalicoAPIConfig{
		Spec: apiv1.CalicoAPIConfigSpec{
			DatastoreType: apiv1.EtcdV2,
		},
	}

	if err := yaml.UnmarshalStrict(b, &c); err != nil {
		return nil, err
	}

	// Validate the version and kind.
	if c.APIVersion != unversioned.VersionCurrent {
		return nil, errors.New("invalid config file: unknown APIVersion '" + c.APIVersion + "'")
	}
	if c.Kind != "calicoApiConfig" {
		return nil, errors.New("invalid config file: expected kind 'calicoApiConfig', got '" + c.Kind + "'")
	}

	log.Info("Datastore type: ", c.Spec.DatastoreType)
	return &c, nil
}

// loadClientConfigFromEnvironmentV1 loads the ClientConfig from the specified file (if specified)
// or from environment variables (if the file is not specified).
func loadClientConfigFromEnvironmentV1() (*apiv1.CalicoAPIConfig, error) {
	c := apiv1.NewCalicoAPIConfig()

	// Load client config from environment variables.
	log.Info("Loading config from environment")
	if err := envconfig.Process("CALICO", &c.Spec); err != nil {
		return nil, err
	}

	return c, nil
}
