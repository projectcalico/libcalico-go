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
	"reflect"
	"regexp"

	"github.com/golang/glog"
	"github.com/tigera/libcalico-go/lib/errors"
)

var (
	matchHandle = regexp.MustCompile("^/?/calico/ipam/v2/handle/([^/]+)$")
	typeHandle  = reflect.TypeOf(IPAMHandle{})
)

type IPAMHandleKey struct {
	HandleID string `json:"id"`
}

func (key IPAMHandleKey) DefaultPath() (string, error) {
	if key.HandleID == "" {
		return "", errors.ErrorInsufficientIdentifiers{}
	}
	e := fmt.Sprintf("/calico/ipam/v2/handle/%s", key.HandleID)
	return e, nil
}

func (key IPAMHandleKey) DefaultDeletePath() (string, error) {
	return key.DefaultPath()
}

func (key IPAMHandleKey) valueType() reflect.Type {
	return typeHandle
}

type IPAMHandleListOptions struct {
	// TODO: Have some options here?
}

func (options IPAMHandleListOptions) DefaultPathRoot() string {
	k := "/calico/ipam/v2/handle/"
	// TODO: Allow filtering on individual host?
	return k
}

func (options IPAMHandleListOptions) ParseDefaultKey(ekey string) Key {
	glog.V(2).Infof("Get IPAM handle key from %s", ekey)
	r := matchBlock.FindAllStringSubmatch(ekey, -1)
	if len(r) != 1 {
		glog.V(2).Infof("%s didn't match regex", ekey)
		return nil
	}
	return IPAMHandleKey{HandleID: r[0][1]}
}

type IPAMHandle struct {
	HandleID string         `json:"-"`
	Block    map[string]int `json:"block"`
}
