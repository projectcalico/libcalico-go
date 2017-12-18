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
	"k8s.io/client-go/kubernetes"

	"github.com/projectcalico/libcalico-go/lib/backend/model"
)

const (
	perNodeBgpPeerAnnotationNamespace = "peer.bgp.projectcalico.org"
)

func NewNodeBGPPeerClientV1(c *kubernetes.Clientset) K8sResourceClient {
	return NewCustomK8sNodeResourceClientV1(customK8sNodeResourceClientConfigV1{
		ClientSet:    c,
		ResourceType: "NodeBGPPeer",
		Converter:    NodeBGPPeerConverterV1{},
		Namespace:    perNodeBgpPeerAnnotationNamespace,
	})
}

// NodeBGPPeerConverterV1 implements the customK8sNodeResourceConverterV1 interface.
type NodeBGPPeerConverterV1 struct{}

func (_ NodeBGPPeerConverterV1) ListInterfaceToNodeAndName(l model.ListInterface) (string, string, error) {
	pl := l.(model.NodeBGPPeerListOptions)
	if pl.PeerIP.IP == nil {
		return pl.Nodename, "", nil
	} else {
		return pl.Nodename, IPToResourceName(pl.PeerIP), nil
	}
}

func (_ NodeBGPPeerConverterV1) KeyToNodeAndName(k model.Key) (string, string, error) {
	pk := k.(model.NodeBGPPeerKey)
	return pk.Nodename, IPToResourceName(pk.PeerIP), nil
}

func (_ NodeBGPPeerConverterV1) NodeAndNameToKey(node, name string) (model.Key, error) {
	ip, err := ResourceNameToIP(name)
	if err != nil {
		return nil, err
	}

	return model.NodeBGPPeerKey{
		Nodename: node,
		PeerIP:   *ip,
	}, nil
}
