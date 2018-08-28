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

package updateprocessors

import (
	"errors"
	"fmt"

	apiv3 "github.com/projectcalico/libcalico-go/lib/apis/v3"
	"github.com/projectcalico/libcalico-go/lib/backend/model"
	"github.com/projectcalico/libcalico-go/lib/backend/watchersyncer"
	cnet "github.com/projectcalico/libcalico-go/lib/net"

	log "github.com/sirupsen/logrus"
)

// Create a new SyncerUpdateProcessor to sync IPPool data in v1 format for
// consumption by both Felix and the BGP daemon.
func NewBGPPeerUpdateProcessor() watchersyncer.SyncerUpdateProcessor {
	return NewConflictResolvingCacheUpdateProcessor(apiv3.KindBGPPeer, convertBGPPeerV2ToV1)
}

// Convert v3 KVPair to the equivalent v1 KVPair.
func convertBGPPeerV2ToV1(kvp *model.KVPair) (*model.KVPair, error) {
	// Validate against incorrect key/value kinds.  This indicates a code bug rather
	// than a user error.
	v3key, ok := kvp.Key.(model.ResourceKey)
	if !ok || v3key.Kind != apiv3.KindBGPPeer {
		return nil, errors.New("Key is not a valid BGPPeer resource key")
	}
	v3res, ok := kvp.Value.(*apiv3.BGPPeer)
	if !ok {
		return nil, errors.New("Value is not a valid BGPPeer resource value")
	}

	// Correct data types. Handle the conversion.
	ip := cnet.ParseIP(v3res.Spec.PeerIP)
	if ip == nil && v3res.Spec.PeerSelector == "" {
		return nil, errors.New("Peer IP or selector is not assigned or is malformed")
	}

	// Create the Key.
	v1key := model.SelectorBGPPeerKey{
		Name: v3res.Name,
	}

	// Determine the node selector to use. We convert old-style "Global" and
	// per node peers into selector peers for simplicity.
	selector := v3res.Spec.NodeSelector
	if v3res.Spec.Node == "" && selector == "" {
		// This is an old-style "Global" peer, which means
		// select all nodes.
		selector = "all()"
		log.Infof("Converted global BGPPeer to selector - %s", selector)
	} else if v3res.Spec.Node != "" {
		// This peer selects a specific node. Use an exact selector
		// to represent this.
		selector = fmt.Sprintf("projectcalico.org/node == '%s'", v3res.Spec.Node)
		log.Infof("Converted node-specific BGPPeer to selector - %s", selector)
	}

	val := &model.BGPPeer{
		Name:         v3res.Name,
		ASNum:        v3res.Spec.ASNumber,
		NodeSelector: selector,
		PeerSelector: v3res.Spec.PeerSelector,
	}
	if ip != nil {
		val.PeerIP = *ip
	}

	return &model.KVPair{
		Key:      v1key,
		Value:    val,
		Revision: kvp.Revision,
	}, nil
}
