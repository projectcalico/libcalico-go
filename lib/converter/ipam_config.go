// Copyright (c) 2020 Tigera, Inc. All rights reserved.

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

package converter

import (
	"errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apiv3 "github.com/projectcalico/libcalico-go/lib/apis/v3"
	"github.com/projectcalico/libcalico-go/lib/backend/model"
)

type IPAMConfigConverter struct{}

// ToV1 converts the given v3 KVPair into a v1 model representation
func (c IPAMConfigConverter) ToV1(kvpv3 *model.KVPair) (*model.KVPair, error) {
	v3obj := kvpv3.Value.(*apiv3.IPAMConfig)
	return &model.KVPair{
		Key: model.IPAMConfigKey{},
		Value: &model.IPAMConfig{
			StrictAffinity:     v3obj.Spec.StrictAffinity,
			AutoAllocateBlocks: v3obj.Spec.AutoAllocateBlocks,
		},
		Revision: kvpv3.Revision,
		UID:      &kvpv3.Value.(*apiv3.IPAMConfig).UID,
	}, nil
}

// ToV3 takes the given v1 KVPair and converts it into a v3 representation.
func (c IPAMConfigConverter) ToV3(kvpv1 *model.KVPair) *model.KVPair {
	v1obj := kvpv1.Value.(*model.IPAMConfig)
	return &model.KVPair{
		Key: model.ResourceKey{
			Name: model.IPAMConfigGlobalName,
			Kind: apiv3.KindIPAMConfig,
		},
		Value: &apiv3.IPAMConfig{
			TypeMeta: metav1.TypeMeta{
				Kind:       apiv3.KindIPAMConfig,
				APIVersion: apiv3.GroupVersionCurrent,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:            model.IPAMConfigGlobalName,
				ResourceVersion: kvpv1.Revision,
			},
			Spec: apiv3.IPAMConfigSpec{
				StrictAffinity:     v1obj.StrictAffinity,
				AutoAllocateBlocks: v1obj.AutoAllocateBlocks,
			},
		},
		Revision: kvpv1.Revision,
	}
}

// ToV1ListInterface takes a v3 list interface and converts it to v1 representation
func (c IPAMConfigConverter) ToV1ListInterface(l model.ResourceListOptions) (model.ListInterface, error) {
	return nil, errors.New("cannot list v3 IPAMConfig")
}

// ToV1Key takes a v3 key and converts it to v1 representation
func (c IPAMConfigConverter) ToV1Key(k model.Key) (model.Key, error) {
	return model.IPAMConfigKey{}, nil
}
