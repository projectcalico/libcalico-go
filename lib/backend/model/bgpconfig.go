// Copyright (c) 2016 Tigera, Inc. All rights reserved.
//
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
	matchGlobalBGPConfig = regexp.MustCompile("^/?calico/bgp/v1/global/(.+)$")
	typeGlobalBGPConfig  = rawStringType
)

type GlobalBGPConfigKey struct {
	Name string `json:"-" validate:"required,name"`
}

func (key GlobalBGPConfigKey) DefaultPath() (string, error) {
	return key.DefaultDeletePath()
}

func (key GlobalBGPConfigKey) DefaultDeletePath() (string, error) {
	if key.Name == "" {
		return "", errors.ErrorInsufficientIdentifiers{Name: "name"}
	}
	e := fmt.Sprintf("/calico/bgp/v1/global/%s", key.Name)
	return e, nil
}

func (key GlobalBGPConfigKey) valueType() reflect.Type {
	return typeGlobalBGPConfig
}

func (key GlobalBGPConfigKey) String() string {
	return fmt.Sprintf("GlobalBGPConfig(name=%s)", key.Name)
}

type GlobalBGPConfigListOptions struct {
	Name string
}

func (options GlobalBGPConfigListOptions) DefaultPathRoot() string {
	k := "/calico/bgp/v1/global"
	if options.Name == "" {
		return k
	}
	k = k + fmt.Sprintf("/%s", options.Name)
	return k
}

func (options GlobalBGPConfigListOptions) ParseDefaultKey(ekey string) Key {
	glog.V(2).Infof("Get GlobalBGPConfig key from %s", ekey)
	r := matchGlobalBGPConfig.FindAllStringSubmatch(ekey, -1)
	if len(r) != 1 {
		glog.V(2).Infof("Didn't match regex")
		return nil
	}
	name := r[0][1]
	if options.Name != "" && name != options.Name {
		glog.V(2).Infof("Didn't match name %s != %s", options.Name, name)
		return nil
	}
	return GlobalBGPConfigKey{Name: name}
}
