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

package backend

import (
	"errors"
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/projectcalico/libcalico-go/lib/api"
	bapi "github.com/projectcalico/libcalico-go/lib/backend/api"
	"github.com/projectcalico/libcalico-go/lib/backend/compat"
	"github.com/projectcalico/libcalico-go/lib/backend/consul"
	"github.com/projectcalico/libcalico-go/lib/backend/etcd"
	"github.com/projectcalico/libcalico-go/lib/backend/k8s"
)

// NewClient creates a new backend datastore client.
func NewClient(config api.CalicoAPIConfig) (c bapi.Client, err error) {
	log.Debugf("Using datastore type '%s'", config.Spec.DatastoreType)
	c, err = NewDatastoreClient(config)
	if c != nil {
		// Wrap the backend, which deals only in raw KV pairs with an
		// adaptor that handles aggregate datatypes.  This allows for
		// reading and writing Profile objects, which are composed
		// of multiple backend keys.
		c = compat.NewAdaptor(c)
	}
	return
}

func NewDatastoreClient(config api.CalicoAPIConfig) (bapi.Client, error) {
	switch config.Spec.DatastoreType {
	case api.EtcdV2:
		return etcd.NewEtcdClient(&config.Spec.EtcdConfig)
	case api.Kubernetes:
		return k8s.NewKubeClient(&config.Spec.KubeConfig)
	case api.ConsulV1:
		return consul.NewConsulClient(&config.Spec.ConsulConfig)
	default:
		return nil, errors.New(fmt.Sprintf("Unknown datastore type: %v",
			config.Spec.DatastoreType))
	}
}
