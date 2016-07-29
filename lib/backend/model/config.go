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
	matchGlobalConfig = regexp.MustCompile("^/?calico/v1/config/(.+)$")
	matchHostConfig   = regexp.MustCompile("^/?calico/v1/host/([^/]+)/config/(.+)$")
	matchReadyFlag    = regexp.MustCompile("^/calico/v1/Ready$")
	typeGlobalConfig  = rawStringType
	typeHostConfig    = rawStringType
	typeReadyFlag     = rawBoolType
)

type ReadyFlagKey struct {
}

func (key ReadyFlagKey) DefaultPath() (string, error) {
	return "/calico/v1/Ready", nil
}

func (key ReadyFlagKey) DefaultDeletePath() (string, error) {
	return key.DefaultPath()
}

func (key ReadyFlagKey) valueType() reflect.Type {
	return typeReadyFlag
}

type GlobalConfigKey struct {
	Name string `json:"-" validate:"required,name"`
}

func (key GlobalConfigKey) DefaultPath() (string, error) {
	k, err := key.DefaultDeletePath()
	return k + "/metadata", err
}

func (key GlobalConfigKey) DefaultDeletePath() (string, error) {
	if key.Name == "" {
		return "", errors.ErrorInsufficientIdentifiers{Name: "name"}
	}
	e := fmt.Sprintf("/calico/v1/config/%s", key.Name)
	return e, nil
}

func (key GlobalConfigKey) valueType() reflect.Type {
	return typeGlobalConfig
}

func (key GlobalConfigKey) String() string {
	return fmt.Sprintf("GlobalConfig(name=%s)", key.Name)
}

type GlobalConfigListOptions struct {
	Name string
}

func (options GlobalConfigListOptions) DefaultPathRoot() string {
	k := "/calico/v1/config"
	if options.Name == "" {
		return k
	}
	k = k + fmt.Sprintf("/%s", options.Name)
	return k
}

func (options GlobalConfigListOptions) ParseDefaultKey(ekey string) Key {
	glog.V(2).Infof("Get GlobalConfig key from %s", ekey)
	r := matchGlobalConfig.FindAllStringSubmatch(ekey, -1)
	if len(r) != 1 {
		glog.V(2).Infof("Didn't match regex")
		return nil
	}
	name := r[0][1]
	if options.Name != "" && name != options.Name {
		glog.V(2).Infof("Didn't match name %s != %s", options.Name, name)
		return nil
	}
	return GlobalConfigKey{Name: name}
}

type HostConfigKey struct {
	Hostname string `json:"-" validate:"required,name"`
	Name     string `json:"-" validate:"required,name"`
}

func (key HostConfigKey) DefaultPath() (string, error) {
	k, err := key.DefaultDeletePath()
	return k + "/metadata", err
}

func (key HostConfigKey) DefaultDeletePath() (string, error) {
	if key.Name == "" {
		return "", errors.ErrorInsufficientIdentifiers{Name: "name"}
	}
	if key.Hostname == "" {
		return "", errors.ErrorInsufficientIdentifiers{Name: "hostname"}
	}
	e := fmt.Sprintf("/calico/v1/host/%s/config/%s", key.Hostname, key.Name)
	return e, nil
}

func (key HostConfigKey) valueType() reflect.Type {
	return typeHostConfig
}

func (key HostConfigKey) String() string {
	return fmt.Sprintf("HostConfig(hostame=%s,name=%s)", key.Hostname, key.Name)
}

type HostConfigListOptions struct {
	Hostname string
	Name     string
}

func (options HostConfigListOptions) DefaultPathRoot() string {
	k := "/calico/v1/host"
	if options.Hostname == "" {
		return k
	}
	k = k + fmt.Sprintf("/%s/config", options.Hostname)
	if options.Name == "" {
		return k
	}
	k = k + fmt.Sprintf("/%s", options.Name)
	return k
}

func (options HostConfigListOptions) ParseDefaultKey(ekey string) Key {
	glog.V(2).Infof("Get HostConfig key from %s", ekey)
	r := matchHostConfig.FindAllStringSubmatch(ekey, -1)
	if len(r) != 1 {
		glog.V(2).Infof("Didn't match regex")
		return nil
	}
	hostname := r[0][1]
	name := r[0][2]
	if options.Hostname != "" && hostname != options.Name {
		glog.V(2).Infof("Didn't match hostname %s != %s", options.Hostname, hostname)
		return nil
	}
	if options.Name != "" && name != options.Name {
		glog.V(2).Infof("Didn't match name %s != %s", options.Name, name)
		return nil
	}
	return HostConfigKey{Name: name}
}
