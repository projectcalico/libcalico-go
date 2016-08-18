// Copyright (c) 2016 Tigera, Inc. All rights reserved.

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

package model

import (
	"fmt"
	"regexp"

	"reflect"

	"github.com/golang/glog"
	"github.com/tigera/libcalico-go/lib/errors"
	"github.com/tigera/libcalico-go/lib/net"
)

var (
	matchGlobalBGPPeer = regexp.MustCompile("^/?calico/bgp/v1/global/peer_v./([^/]+)$")
	matchHostBGPPeer   = regexp.MustCompile("^/?calico/bgp/v1/host/([^/]+)/peer_v./([^/]+)$")
	typeBGPPeer        = reflect.TypeOf(BGPPeer{})
)

type BGPPeerKey struct {
	Hostname string `json:"-" validate:"omitempty"`
	PeerIP   net.IP `json:"-" validate:"required"`
}

func (key BGPPeerKey) DefaultPath() (string, error) {
	if key.PeerIP.IP == nil {
		return "", errors.ErrorInsufficientIdentifiers{Name: "peerIP"}
	}
	if key.Hostname == "" {
		e := fmt.Sprintf("/calico/bgp/v1/global/peer_v%d/%s",
			key.PeerIP.Version(), key.PeerIP)
		return e, nil
	} else {
		e := fmt.Sprintf("/calico/bgp/v1/host/%s/peer_v%d/%s",
			key.Hostname, key.PeerIP.Version(), key.PeerIP)
		return e, nil
	}
}

func (key BGPPeerKey) DefaultDeletePath() (string, error) {
	return key.DefaultPath()
}

func (key BGPPeerKey) valueType() reflect.Type {
	return typeBGPPeer
}

func (key BGPPeerKey) String() string {
	if key.Hostname == "" {
		return fmt.Sprintf("BGPPeer(global, ip=%s)", key.PeerIP)
	} else {
		return fmt.Sprintf("BGPPeer(hostname=%s, ip=%s)", key.Hostname, key.PeerIP)
	}
}

type BGPPeerListOptions struct {
	Hostname string
	PeerIP   net.IP
}

func (options BGPPeerListOptions) DefaultPathRoot() string {
	if options.Hostname == "" {
		return "/calico/bgp/v1"
	} else if options.PeerIP.IP == nil {
		return fmt.Sprintf("/calico/bgp/v1/host/%s",
			options.Hostname)
	} else {
		return fmt.Sprintf("/calico/bgp/v1/host/%s/peer_v%d/%s",
			options.Hostname, options.PeerIP.Version(), options.PeerIP)
	}
}

func (options BGPPeerListOptions) ParseDefaultKey(ekey string) Key {
	glog.V(2).Infof("Get BGPPeer key from %s", ekey)
	hostname := ""
	peerIP := net.IP{}
	ekeyb := []byte(ekey)

	if r := matchGlobalBGPPeer.FindAllSubmatch(ekeyb, -1); len(r) == 1 {
		_ = peerIP.UnmarshalText(r[0][1])
	} else if r := matchHostBGPPeer.FindAllSubmatch(ekeyb, -1); len(r) == 1 {
		hostname = string(r[0][1])
		_ = peerIP.UnmarshalText(r[0][2])
	} else {
		glog.V(2).Infof("%s didn't match regex", ekey)
		return nil
	}

	if options.PeerIP.IP != nil && !options.PeerIP.Equal(peerIP.IP) {
		glog.V(2).Infof("Didn't match peerIP %s != %s", options.PeerIP.String(), peerIP.String())
		return nil
	}
	if options.Hostname != "" && hostname != options.Hostname {
		glog.V(2).Infof("Didn't match hostname %s != %s", options.Hostname, hostname)
		return nil
	}
	return BGPPeerKey{PeerIP: peerIP, Hostname: hostname}
}

type BGPPeer struct {
	PeerIP net.IP `json:"ip"`
	ASNum  int    `json:"as_num"`
}
