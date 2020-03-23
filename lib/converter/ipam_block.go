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
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apiv3 "github.com/projectcalico/libcalico-go/lib/apis/v3"
	"github.com/projectcalico/libcalico-go/lib/backend/model"
	"github.com/projectcalico/libcalico-go/lib/names"
	"github.com/projectcalico/libcalico-go/lib/net"
)

type IPAMBlockConverter struct{}

// ToV1 converts the given v3 KVPair into a v1 model representation
func (c IPAMBlockConverter) ToV1(kvpv3 *model.KVPair) (*model.KVPair, error) {
	cidrStr := kvpv3.Value.(*apiv3.IPAMBlock).Spec.CIDR
	_, cidr, err := net.ParseCIDR(cidrStr)
	if err != nil {
		return nil, err
	}

	ab := kvpv3.Value.(*apiv3.IPAMBlock)

	// Convert attributes.
	attrs := []model.AllocationAttribute{}
	for _, a := range ab.Spec.Attributes {
		attrs = append(attrs, model.AllocationAttribute{
			AttrPrimary:   a.AttrPrimary,
			AttrSecondary: a.AttrSecondary,
		})
	}

	return &model.KVPair{
		Key: model.BlockKey{
			CIDR: *cidr,
		},
		Value: &model.AllocationBlock{
			CIDR:           *cidr,
			Affinity:       ab.Spec.Affinity,
			StrictAffinity: ab.Spec.StrictAffinity,
			Allocations:    ab.Spec.Allocations,
			Unallocated:    ab.Spec.Unallocated,
			Attributes:     attrs,
			Deleted:        ab.Spec.Deleted,
		},
		Revision: kvpv3.Revision,
		UID:      &ab.UID,
	}, nil
}

// ParseKey parses the given model.Key, returning a suitable name, and CIDR
// for use in the v3 API.
func (c IPAMBlockConverter) ParseKey(k model.Key) (name, cidr string) {
	cidr = fmt.Sprintf("%s", k.(model.BlockKey).CIDR)
	name = names.CIDRToName(k.(model.BlockKey).CIDR)
	return
}

// ToV3 takes the given v1 KVPair and converts it into a v3 representation.
func (c IPAMBlockConverter) ToV3(kvpv1 *model.KVPair) *model.KVPair {
	name, cidr := c.ParseKey(kvpv1.Key)

	ab := kvpv1.Value.(*model.AllocationBlock)

	// Convert attributes.
	attrs := []apiv3.AllocationAttribute{}
	for _, a := range ab.Attributes {
		attrs = append(attrs, apiv3.AllocationAttribute{
			AttrPrimary:   a.AttrPrimary,
			AttrSecondary: a.AttrSecondary,
		})
	}

	return &model.KVPair{
		Key: model.ResourceKey{
			Name: name,
			Kind: apiv3.KindIPAMBlock,
		},
		Value: &apiv3.IPAMBlock{
			TypeMeta: metav1.TypeMeta{
				Kind:       apiv3.KindIPAMBlock,
				APIVersion: apiv3.GroupVersionCurrent,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:            name,
				ResourceVersion: kvpv1.Revision,
			},
			Spec: apiv3.IPAMBlockSpec{
				CIDR:           cidr,
				Allocations:    ab.Allocations,
				Unallocated:    ab.Unallocated,
				Affinity:       ab.Affinity,
				StrictAffinity: ab.StrictAffinity,
				Attributes:     attrs,
				Deleted:        ab.Deleted,
			},
		},
		Revision: kvpv1.Revision,
	}
}

// ToV1ListInterface takes a v3 list interface and converts it to v1 representation
func (c IPAMBlockConverter) ToV1ListInterface(l model.ResourceListOptions) (model.ListInterface, error) {
	if l.Kind != apiv3.KindIPAMBlock {
		return nil, errors.New("passed wrong kind to IPAMBlockConverter")
	}
	if l.Name != "" {
		return nil, errors.New("cannot list v3 IPAMBlock by name")
	}
	return model.BlockListOptions{
		IPVersion: 0,
	}, nil
}

// ToV1Key takes a v3 key and converts it to v1 representation
func (c IPAMBlockConverter) ToV1Key(k model.Key) (model.Key, error) {
	return nil, errors.New("cannot convert v3 IPAMBlock key to v1 key")
}
