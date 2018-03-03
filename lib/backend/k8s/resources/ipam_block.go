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

package resources

import (
	"context"
	"reflect"

	apiv3 "github.com/projectcalico/libcalico-go/lib/apis/v3"
	"github.com/projectcalico/libcalico-go/lib/backend/api"
	"github.com/projectcalico/libcalico-go/lib/backend/model"
	cerrors "github.com/projectcalico/libcalico-go/lib/errors"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	IPAMBlockResourceName = "IPAMBlocks"
	IPAMBlockCRDName      = "ipamblocks.crd.projectcalico.org"
)

func NewIPAMBlockClient(c *kubernetes.Clientset, r *rest.RESTClient) K8sResourceClient {
	// Create a resource client which manages k8s CRDs.
	rc := customK8sResourceClient{
		clientSet:       c,
		restClient:      r,
		name:            IPAMBlockCRDName,
		resource:        IPAMBlockResourceName,
		description:     "Calico IPAM blocks",
		k8sResourceType: reflect.TypeOf(apiv3.IPAMBlock{}),
		k8sResourceTypeMeta: metav1.TypeMeta{
			Kind:       apiv3.KindIPAMBlock,
			APIVersion: apiv3.GroupVersionCurrent,
		},
		k8sListType:  reflect.TypeOf(apiv3.IPAMBlockList{}),
		resourceKind: apiv3.KindIPAMBlock,
	}

	return &ipamBlockClient{rc: rc}
}

// Implements the api.Client interface for IPAMBlocks.
type ipamBlockClient struct {
	rc customK8sResourceClient
}

func (c *ipamBlockClient) Create(ctx context.Context, kvp *model.KVPair) (*model.KVPair, error) {
	nkvp := &model.KVPair{
		Key: model.ResourceKey{
			Name:      "foo",
			Namespace: "bar",
			Kind:      apiv3.KindIPAMBlock,
		},
		Value: apiv3.IPAMBlock{
			Spec: apiv3.IPAMBlockSpec{},
		},
	}
	return c.rc.Create(ctx, nkvp)
}

func (c *ipamBlockClient) Update(ctx context.Context, kvp *model.KVPair) (*model.KVPair, error) {
	log.Warn("Operation Update is not supported on IPAMBlock type")
	return nil, cerrors.ErrorOperationNotSupported{
		Identifier: kvp.Key,
		Operation:  "Create",
	}
}

func (c *ipamBlockClient) Delete(ctx context.Context, key model.Key, revision string) (*model.KVPair, error) {
	log.Warn("Operation Delete is not supported on IPAMBlock type")
	return nil, cerrors.ErrorOperationNotSupported{
		Identifier: key,
		Operation:  "Delete",
	}
}

func (c *ipamBlockClient) Get(ctx context.Context, key model.Key, revision string) (*model.KVPair, error) {
	k := model.ResourceKey{
		Name:      "foo",
		Namespace: "bar",
		Kind:      apiv3.KindIPAMBlock,
	}
	return c.rc.Get(ctx, k, revision)
}

func (c *ipamBlockClient) List(ctx context.Context, list model.ListInterface, revision string) (*model.KVPairList, error) {
	l := model.ResourceListOptions{
		Name:      "foo",
		Namespace: "bar",
		Kind:      apiv3.KindIPAMBlock,
	}
	return c.rc.List(ctx, l, revision)
}

func (c *ipamBlockClient) Watch(ctx context.Context, list model.ListInterface, revision string) (api.WatchInterface, error) {
	log.Warn("Operation Watch is not supported on IPAMBlock type")
	return nil, cerrors.ErrorOperationNotSupported{
		Identifier: list,
		Operation:  "Watch",
	}
}

// EnsureInitialized is a no-op since the CRD should be
// initialized in advance.
func (c *ipamBlockClient) EnsureInitialized() error {
	return nil
}
