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
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apiv3 "github.com/projectcalico/libcalico-go/lib/apis/v3"
	"github.com/projectcalico/libcalico-go/lib/backend/model"
	"github.com/projectcalico/libcalico-go/lib/names"
	"github.com/projectcalico/libcalico-go/lib/net"
)

type BlockAffinityConverter struct{}

// ToV1 converts the given v3 KVPair into a v1 model representation
func (c BlockAffinityConverter) ToV1(kvpv3 *model.KVPair) (*model.KVPair, error) {
	// Parse the CIDR into a struct.
	_, cidr, err := net.ParseCIDR(kvpv3.Value.(*apiv3.BlockAffinity).Spec.CIDR)
	if err != nil {
		log.WithField("cidr", cidr).WithError(err).Error("failed to parse cidr")
		return nil, err
	}
	state := model.BlockAffinityState(kvpv3.Value.(*apiv3.BlockAffinity).Spec.State)
	return &model.KVPair{
		Key: model.BlockAffinityKey{
			CIDR: *cidr,
			Host: kvpv3.Value.(*apiv3.BlockAffinity).Spec.Node,
		},
		Value: &model.BlockAffinity{
			State: state,
		},
		Revision: kvpv3.Revision,
		UID:      &kvpv3.Value.(*apiv3.BlockAffinity).UID,
	}, nil
}

// ParseKey parses the given model.Key, returning a suitable name, and CIDR
// for use in the v3 API.
func (c BlockAffinityConverter) ParseKey(k model.Key) (name, cidr, host string) {
	host = k.(model.BlockAffinityKey).Host
	cidr = fmt.Sprintf("%s", k.(model.BlockAffinityKey).CIDR)
	cidrname := names.CIDRToName(k.(model.BlockAffinityKey).CIDR)

	// Include the hostname as well.
	host = k.(model.BlockAffinityKey).Host
	name = fmt.Sprintf("%s-%s", host, cidrname)

	if len(name) >= 253 {
		// If the name is too long, we need to shorten it.
		// Remove enough characters to get it below the 253 character limit,
		// as well as 11 characters to add a hash which helps with uniqueness,
		// and two characters for the `-` separators between clauses.
		name = fmt.Sprintf("%s-%s", host[:252-len(cidrname)-13], cidrname)

		// Add a hash to help with uniqueness.
		h := sha256.New()
		h.Write([]byte(fmt.Sprintf("%s+%s", host, cidrname)))
		name = fmt.Sprintf("%s-%s", name, hex.EncodeToString(h.Sum(nil))[:11])
	}
	return
}

// ToV3 takes the given v1 KVPair and converts it into a v3 representation.
func (c BlockAffinityConverter) ToV3(kvpv1 *model.KVPair) *model.KVPair {
	name, cidr, host := c.ParseKey(kvpv1.Key)
	state := kvpv1.Value.(*model.BlockAffinity).State
	return &model.KVPair{
		Key: model.ResourceKey{
			Name: name,
			Kind: apiv3.KindBlockAffinity,
		},
		Value: &apiv3.BlockAffinity{
			TypeMeta: metav1.TypeMeta{
				Kind:       apiv3.KindBlockAffinity,
				APIVersion: apiv3.GroupVersionCurrent,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:            name,
				ResourceVersion: kvpv1.Revision,
			},
			Spec: apiv3.BlockAffinitySpec{
				State: string(state),
				Node:  host,
				CIDR:  cidr,
			},
		},
		Revision: kvpv1.Revision,
	}
}

// ToV1Key takes a v3 key and converts it to v1 representation
func (c BlockAffinityConverter) ToV1Key(k model.Key) (model.Key, error) {
	return nil, errors.New("cannot convert v3 BlockAffinity key to v1 key")
}

// ToV1ListInterface takes a v3 list interface and converts it to v1 representation
func (c BlockAffinityConverter) ToV1ListInterface(l model.ResourceListOptions) (model.ListInterface, error) {
	if l.Kind != apiv3.KindBlockAffinity {
		return nil, errors.New("passed wrong kind to BlockAffinityConverter")
	}
	if l.Name != "" {
		return nil, errors.New("cannot list v3 BlockAffinity by name")
	}
	return model.BlockAffinityListOptions{
		Host:      "",
		IPVersion: 0,
	}, nil
}
