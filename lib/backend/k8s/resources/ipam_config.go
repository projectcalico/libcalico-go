// Copyright (c) 2019 Tigera, Inc. All rights reserved.

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

package resources

import (
	"context"
	"reflect"

	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	apiv3 "github.com/projectcalico/libcalico-go/lib/apis/v3"
	"github.com/projectcalico/libcalico-go/lib/backend/api"
	"github.com/projectcalico/libcalico-go/lib/backend/model"
	"github.com/projectcalico/libcalico-go/lib/converter"
	cerrors "github.com/projectcalico/libcalico-go/lib/errors"
)

const (
	IPAMConfigResourceName = "IPAMConfigs"
	IPAMConfigCRDName      = "ipamconfigs.crd.projectcalico.org"
)

func NewIPAMConfigClient(c *kubernetes.Clientset, r *rest.RESTClient) K8sResourceClient {
	return &ipamConfigClient{
		rc: customK8sResourceClient{
			clientSet:       c,
			restClient:      r,
			name:            IPAMConfigCRDName,
			resource:        IPAMConfigResourceName,
			description:     "Calico IPAM configuration",
			k8sResourceType: reflect.TypeOf(apiv3.IPAMConfig{}),
			k8sResourceTypeMeta: metav1.TypeMeta{
				Kind:       apiv3.KindIPAMConfig,
				APIVersion: apiv3.GroupVersionCurrent,
			},
			k8sListType:  reflect.TypeOf(apiv3.IPAMConfigList{}),
			resourceKind: apiv3.KindIPAMConfig}}
}

// NewV3IPAMConfigClient returns a resource client that accepts IPAMConfig resources
// in V3 format, as opposed to the NewIPAMConfigClient that accepts V1 resources.
func NewV3IPAMConfigClient(c *kubernetes.Clientset, r *rest.RESTClient) K8sResourceClient {
	return &customK8sResourceClient{
		clientSet:       c,
		restClient:      r,
		name:            IPAMConfigCRDName,
		resource:        IPAMConfigResourceName,
		description:     "Calico IPAM configuration",
		k8sResourceType: reflect.TypeOf(apiv3.IPAMConfig{}),
		k8sResourceTypeMeta: metav1.TypeMeta{
			Kind:       apiv3.KindIPAMConfig,
			APIVersion: apiv3.GroupVersionCurrent,
		},
		k8sListType:  reflect.TypeOf(apiv3.IPAMConfigList{}),
		resourceKind: apiv3.KindIPAMConfig}
}

// ipamConfigClient implements the api.Client interface for IPAMConfig objects. It
// handles the translation between v1 objects understood by the IPAM codebase in lib/ipam,
// and the CRDs which are used to actually store the data in the Kubernetes API.
// It uses a customK8sResourceClient under the covers to perform CRUD operations on
// kubernetes CRDs.
type ipamConfigClient struct {
	rc customK8sResourceClient
}

var ipamConfigConverter = converter.IPAMConfigConverter{}

func (c *ipamConfigClient) Create(ctx context.Context, kvp *model.KVPair) (*model.KVPair, error) {
	log.Debug("Received Create request on IPAMConfig type")
	nkvp, err := c.rc.Create(ctx, ipamConfigConverter.ToV3(kvp))
	if err != nil {
		return nil, err
	}
	kvp, err = ipamConfigConverter.ToV1(nkvp)
	if err != nil {
		return nil, err
	}
	return kvp, nil
}

func (c *ipamConfigClient) Update(ctx context.Context, kvp *model.KVPair) (*model.KVPair, error) {
	log.Debug("Received Update request on IPAMConfig type")
	nkvp, err := c.rc.Update(ctx, ipamConfigConverter.ToV3(kvp))
	if err != nil {
		return nil, err
	}
	kvp, err = ipamConfigConverter.ToV1(nkvp)
	if err != nil {
		return nil, err
	}
	return kvp, nil
}

func (c *ipamConfigClient) DeleteKVP(ctx context.Context, kvp *model.KVPair) (*model.KVPair, error) {
	return c.Delete(ctx, kvp.Key, kvp.Revision, kvp.UID)
}

func (c *ipamConfigClient) Delete(ctx context.Context, key model.Key, revision string, uid *types.UID) (*model.KVPair, error) {
	k := model.ResourceKey{
		Name: model.IPAMConfigGlobalName,
		Kind: apiv3.KindIPAMConfig,
	}
	kvp, err := c.rc.Delete(ctx, k, revision, uid)
	if err != nil {
		return nil, err
	}
	v1nkvp, err := ipamConfigConverter.ToV1(kvp)
	if err != nil {
		return nil, err
	}
	return v1nkvp, nil
}

func (c *ipamConfigClient) Get(ctx context.Context, key model.Key, revision string) (*model.KVPair, error) {
	log.Debug("Received Get request on IPAMConfig type")
	k := model.ResourceKey{
		Name: model.IPAMConfigGlobalName,
		Kind: apiv3.KindIPAMConfig,
	}
	kvp, err := c.rc.Get(ctx, k, revision)
	if err != nil {
		log.WithError(err).Debug("get request on IPAMConfig type had error")
		return nil, err
	}
	v1kvp, err := ipamConfigConverter.ToV1(kvp)
	if err != nil {
		return nil, err
	}
	return v1kvp, nil

}

func (c *ipamConfigClient) List(ctx context.Context, list model.ListInterface, revision string) (*model.KVPairList, error) {
	log.Warn("Operation List is not supported on IPAMConfig type")
	return nil, cerrors.ErrorOperationNotSupported{
		Identifier: list,
		Operation:  "List",
	}
}

func (c *ipamConfigClient) Watch(ctx context.Context, list model.ListInterface, revision string) (api.WatchInterface, error) {
	log.Warn("Operation Watch is not supported on IPAMConfig type")
	return nil, cerrors.ErrorOperationNotSupported{
		Identifier: list,
		Operation:  "Watch",
	}
}

// EnsureInitialized is a no-op since the CRD should be
// initialized in advance.
func (c *ipamConfigClient) EnsureInitialized() error {
	return nil
}
