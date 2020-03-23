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
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apiv3 "github.com/projectcalico/libcalico-go/lib/apis/v3"
	"github.com/projectcalico/libcalico-go/lib/backend/model"
)

type IPAMHandleConverter struct{}

// ToV1 converts the given v3 KVPair into a v1 model representation
func (c IPAMHandleConverter) ToV1(kvpv3 *model.KVPair) (*model.KVPair, error) {
	handle := kvpv3.Value.(*apiv3.IPAMHandle).Spec.HandleID
	block := kvpv3.Value.(*apiv3.IPAMHandle).Spec.Block
	return &model.KVPair{
		Key: model.IPAMHandleKey{
			HandleID: handle,
		},
		Value: &model.IPAMHandle{
			HandleID: handle,
			Block:    block,
		},
		Revision: kvpv3.Revision,
	}, nil
}

// ParseKey parses the given model.Key, returning a suitable name, and CIDR
// for use in the v3 API.
func (c IPAMHandleConverter) ParseKey(k model.Key) string {
	return strings.ToLower(k.(model.IPAMHandleKey).HandleID)
}

// ToV3 takes the given v1 KVPair and converts it into a v3 representation.
func (c IPAMHandleConverter) ToV3(kvpv1 *model.KVPair) *model.KVPair {
	name := c.ParseKey(kvpv1.Key)
	handle := kvpv1.Key.(model.IPAMHandleKey).HandleID
	block := kvpv1.Value.(*model.IPAMHandle).Block
	return &model.KVPair{
		Key: model.ResourceKey{
			Name: name,
			Kind: apiv3.KindIPAMHandle,
		},
		Value: &apiv3.IPAMHandle{
			TypeMeta: metav1.TypeMeta{
				Kind:       apiv3.KindIPAMHandle,
				APIVersion: apiv3.GroupVersionCurrent,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:            name,
				ResourceVersion: kvpv1.Revision,
			},
			Spec: apiv3.IPAMHandleSpec{
				HandleID: handle,
				Block:    block,
			},
		},
		Revision: kvpv1.Revision,
	}
}

// ToV1ListInterface takes a v3 list interface and converts it to v1 representation
func (c IPAMHandleConverter) ToV1ListInterface(l model.ResourceListOptions) (model.ListInterface, error) {
	if l.Kind != apiv3.KindIPAMHandle {
		return nil, errors.New("passed wrong kind to IPAMHandleConverter")
	}
	if l.Name != "" {
		return nil, errors.New("cannot list v3 IPAMHandle by name")
	}
	return model.IPAMHandleListOptions{}, nil
}

// ToV1Key takes a v3 key and converts it to v1 representation
func (c IPAMHandleConverter) ToV1Key(k model.Key) (model.Key, error) {
	return nil, errors.New("cannot convert v3 IPAMHandle key to v1 key")
}
