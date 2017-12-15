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

package resources

import (
	"fmt"
	"reflect"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	apiv1 "github.com/projectcalico/libcalico-go/lib/backend/k8s/v1"
	"github.com/projectcalico/libcalico-go/lib/backend/model"
)

const (
	GlobalBGPConfigResourceName = "GlobalBGPConfigs"
	GlobalBGPConfigCRDName      = "globalbgpconfigs.projectcalico.org"
)

func NewGlobalBGPConfigClientV1(c *kubernetes.Clientset, r *rest.RESTClient) K8sResourceClient {
	return &customK8sResourceClientV1{
		clientSet:       c,
		restClient:      r,
		name:            GlobalBGPConfigCRDName,
		resource:        GlobalBGPConfigResourceName,
		description:     "Calico Global BGP Configuration",
		k8sResourceType: reflect.TypeOf(apiv1.GlobalBGPConfig{}),
		k8sListType:     reflect.TypeOf(apiv1.GlobalBGPConfigList{}),
		converter:       GlobalBGPConfigConverterV1{},
	}
}

// GlobalBGPConfigConverterV1 implements the K8sResourceConverter interface.
type GlobalBGPConfigConverterV1 struct {
}

func (_ GlobalBGPConfigConverterV1) ListInterfaceToKey(l model.ListInterface) model.Key {
	if name := l.(model.GlobalBGPConfigListOptions).Name; name != "" {
		return model.GlobalBGPConfigKey{Name: name}
	}
	return nil
}

func (_ GlobalBGPConfigConverterV1) KeyToName(k model.Key) (string, error) {
	return strings.ToLower(k.(model.GlobalBGPConfigKey).Name), nil
}

func (_ GlobalBGPConfigConverterV1) NameToKey(name string) (model.Key, error) {
	return nil, fmt.Errorf("Mapping of Name to Key is not possible for global BGP config")
}

func (c GlobalBGPConfigConverterV1) ToKVPair(r Resource) (*model.KVPair, error) {
	t := r.(*apiv1.GlobalBGPConfig)
	return &model.KVPair{
		Key: model.GlobalBGPConfigKey{
			Name: t.Spec.Name,
		},
		Value:    t.Spec.Value,
		Revision: t.ResourceVersion,
	}, nil
}

func (c GlobalBGPConfigConverterV1) FromKVPair(kvp *model.KVPair) (Resource, error) {
	name, err := c.KeyToName(kvp.Key)
	if err != nil {
		return nil, err
	}
	crd := apiv1.GlobalBGPConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       "GlobalBGPConfig",
			APIVersion: "crd.projectcalico.org/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: apiv1.GlobalBGPConfigSpec{
			Name:  kvp.Key.(model.GlobalBGPConfigKey).Name,
			Value: kvp.Value.(string),
		},
	}
	crd.ResourceVersion = kvp.Revision
	return &crd, nil
}
