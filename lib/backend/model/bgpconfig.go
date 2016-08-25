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
	matchGlobalBGPConfig = regexp.MustCompile("^/?calico/bgp/v1/global/([^/]+)$")
	matchHostBGPConfig   = regexp.MustCompile("^/?calico/bgp/v1/host/([^/]+)/([^/]+)$")
	typeGlobalBGPConfig  = rawStringType
	typeHostBGPConfig    = rawStringType
)

type GlobalBGPConfigKey struct {
	// The name of the global BGP config key.
	Name string `json:"-" validate:"required,name"`
}

func (key GlobalBGPConfigKey) defaultPath() (string, error) {
	return key.defaultDeletePath()
}

func (key GlobalBGPConfigKey) defaultDeletePath() (string, error) {
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
	// The name of the global BGP config key.
	Name string
}

func (options GlobalBGPConfigListOptions) defaultPathRoot() string {
	k := "/calico/bgp/v1/global"
	if options.Name == "" {
		return k
	}
	k = k + fmt.Sprintf("/%s", options.Name)
	return k
}

func (options GlobalBGPConfigListOptions) KeyFromDefaultPath(ekey string) Key {
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

type HostBGPConfigKey struct {
	// The hostname for the host specific BGP config
	Hostname string `json:"-" validate:"required,name"`

	// The name of the host specific BGP config key.
	Name string `json:"-" validate:"required,name"`
}

func (key HostBGPConfigKey) defaultPath() (string, error) {
	return key.defaultDeletePath()
}

func (key HostBGPConfigKey) defaultDeletePath() (string, error) {
	if key.Hostname == "" {
		return "", errors.ErrorInsufficientIdentifiers{Name: "hostname"}
	}
	if key.Name == "" {
		return "", errors.ErrorInsufficientIdentifiers{Name: "name"}
	}
	e := fmt.Sprintf("/calico/bgp/v1/host/%s/%s", key.Hostname, key.Name)
	return e, nil
}

func (key HostBGPConfigKey) valueType() reflect.Type {
	return typeHostBGPConfig
}

func (key HostBGPConfigKey) String() string {
	return fmt.Sprintf("HostBGPConfig(hostname=%s; name=%s)", key.Hostname, key.Name)
}

type HostBGPConfigListOptions struct {
	Hostname string
	Name     string
}

func (options HostBGPConfigListOptions) defaultPathRoot() string {
	k := "/calico/bgp/v1/host"
	if options.Hostname == "" {
		return k
	}
	k = k + fmt.Sprintf("/%s", options.Hostname)
	if options.Name == "" {
		return k
	}
	k = k + fmt.Sprintf("/%s", options.Name)
	return k
}

func (options HostBGPConfigListOptions) KeyFromDefaultPath(ekey string) Key {
	glog.V(2).Infof("Get HostBGPConfig key from %s", ekey)
	r := matchHostBGPConfig.FindAllStringSubmatch(ekey, -1)
	if len(r) != 1 {
		glog.V(2).Infof("Didn't match regex")
		return nil
	}
	hostname := r[0][1]
	name := r[0][2]
	if options.Hostname != "" && hostname != options.Hostname {
		glog.V(2).Infof("Didn't match hostname %s != %s", options.Hostname, hostname)
		return nil
	}
	if options.Name != "" && name != options.Name {
		glog.V(2).Infof("Didn't match name %s != %s", options.Name, name)
		return nil
	}
	return HostBGPConfigKey{Name: name, Hostname: hostname}
}
