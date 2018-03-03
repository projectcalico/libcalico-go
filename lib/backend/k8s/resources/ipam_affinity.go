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
	"fmt"
	"reflect"
	"strings"

	apiv3 "github.com/projectcalico/libcalico-go/lib/apis/v3"
	"github.com/projectcalico/libcalico-go/lib/backend/api"
	"github.com/projectcalico/libcalico-go/lib/backend/model"
	cerrors "github.com/projectcalico/libcalico-go/lib/errors"
	"github.com/projectcalico/libcalico-go/lib/net"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	BlockAffinityResourceName = "BlockAffinities"
	BlockAffinityCRDName      = "blockaffinities.crd.projectcalico.org"
)

func NewAffinityBlockClient(c *kubernetes.Clientset, r *rest.RESTClient) K8sResourceClient {
	// Create a resource client which manages k8s CRDs.
	rc := customK8sResourceClient{
		clientSet:       c,
		restClient:      r,
		name:            BlockAffinityCRDName,
		resource:        BlockAffinityResourceName,
		description:     "Calico IPAM block affinities",
		k8sResourceType: reflect.TypeOf(apiv3.BlockAffinity{}),
		k8sResourceTypeMeta: metav1.TypeMeta{
			Kind:       apiv3.KindBlockAffinity,
			APIVersion: apiv3.GroupVersionCurrent,
		},
		k8sListType:  reflect.TypeOf(apiv3.BlockAffinityList{}),
		resourceKind: apiv3.KindBlockAffinity,
	}

	return &affinityBlockClient{rc: rc}
}

// Implements the api.Client interface for AffinityBlocks.
type affinityBlockClient struct {
	rc customK8sResourceClient
}

func toV1(kvpv3 *model.KVPair) *model.KVPair {
	splits := strings.Split(kvpv3.Key.(model.ResourceKey).Name, "---")
	host := splits[0]
	cidrStr := strings.Replace(splits[1], "--", "/", -1)
	cidrStr = strings.Replace(cidrStr, "-", ".", -1)
	_, cidr, err := net.ParseCIDR(cidrStr)
	if err != nil {
		panic(err)
	}
	state := model.BlockAffinityState(kvpv3.Value.(*apiv3.BlockAffinity).Spec.State)
	return &model.KVPair{
		Key: model.BlockAffinityKey{
			Host: host,
			CIDR: *cidr,
		},
		Value: &model.BlockAffinity{
			State: state,
		},
		Revision: kvpv3.Revision,
	}
}

func toV3(kvpv1 *model.KVPair) *model.KVPair {
	host := kvpv1.Key.(model.BlockAffinityKey).Host
	cidr := fmt.Sprintf("%s", kvpv1.Key.(mode.BlockAffinityKey).CIDR)
	cidr = strings.Replace(cidr, ".", "-", -1)
	cidr = strings.Replace(cidr, "/", "--", -1)
	name := fmt.Sprintf("%s---%s", host, cidr)
	state := kvpv1.Value.(*model.BlockAffinity).State
	return &model.KVPair{
		Key: model.ResourceKey{
			Name: name,
			Kind: apiv3.KindBlockAffinity,
		},
		Value: &apiv3.BlockAffinity{
			TypeMeta: metav1.TypeMeta{
				Kind:       apiv3.KindBlockAffinity,
				APIVersion: "crd.projectcalico.org/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:            name,
				ResourceVersion: kvpv1.Revision,
			},
			Spec: apiv3.BlockAffinitySpec{
				State: string(state),
			},
		},
		Revision: kvpv1.Revision,
	}
}

func (c *affinityBlockClient) Create(ctx context.Context, kvp *model.KVPair) (*model.KVPair, error) {
	nkvp := toV3(kvp)
	kvp, err := c.rc.Create(ctx, nkvp)
	if err != nil {
		return nil, err
	}
	return toV1(kvp), nil
}

func (c *affinityBlockClient) Update(ctx context.Context, kvp *model.KVPair) (*model.KVPair, error) {
	nkvp := toV3(kvp)
	kvp, err := c.rc.Update(ctx, nkvp)
	if err != nil {
		return nil, err
	}
	return toV1(kvp), nil
}

func (c *affinityBlockClient) Delete(ctx context.Context, key model.Key, revision string) (*model.KVPair, error) {
	host := key.(model.BlockAffinityKey).Host
	name := fmt.Sprintf("%s", host)
	k := model.ResourceKey{
		Name: name,
		Kind: apiv3.KindBlockAffinity,
	}
	kvp, err := c.rc.Delete(ctx, k, revision)
	if err != nil {
		return nil, err
	}
	return toV1(kvp), nil
}

func (c *affinityBlockClient) Get(ctx context.Context, key model.Key, revision string) (*model.KVPair, error) {
	host := key.(model.BlockAffinityKey).Host
	name := fmt.Sprintf("%s", host)
	k := model.ResourceKey{
		Name: name,
		Kind: apiv3.KindBlockAffinity,
	}
	kvp, err := c.rc.Get(ctx, k, revision)
	if err != nil {
		return nil, err
	}
	return toV1(kvp), nil
}

func (c *affinityBlockClient) List(ctx context.Context, list model.ListInterface, revision string) (*model.KVPairList, error) {
	l := model.ResourceListOptions{Kind: apiv3.KindBlockAffinity}
	v3list, err := c.rc.List(ctx, l, revision)
	if err != nil {
		return nil, err
	}

	kvpl := &model.KVPairList{KVPairs: []*model.KVPair{}}
	for _, i := range v3list.KVPairs {
		kvpl.KVPairs = append(kvpl.KVPairs, toV1(i))
	}
	return kvpl, nil
}

func (c *affinityBlockClient) Watch(ctx context.Context, list model.ListInterface, revision string) (api.WatchInterface, error) {
	log.Warn("Operation Watch is not supported on AffinityBlock type")
	return nil, cerrors.ErrorOperationNotSupported{
		Identifier: list,
		Operation:  "Watch",
	}
}

func (c *affinityBlockClient) EnsureInitialized() error {
	return nil
}
