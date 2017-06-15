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

package model

import (
	"fmt"

	"regexp"

	"reflect"

	log "github.com/Sirupsen/logrus"
	"github.com/projectcalico/libcalico-go/lib/errors"
	"github.com/projectcalico/libcalico-go/lib/net"
)

var (
	matchNetworkSet = regexp.MustCompile("^/?calico/v1/netset/([^/]+)$")
	typeNetworkSet  = reflect.TypeOf(NetworkSet{})
)

type NetworkSetKey struct {
	Name string `json:"-" validate:"required,namespacedName"`
}

func (key NetworkSetKey) defaultPath() (string, error) {
	if key.Name == "" {
		return "", errors.ErrorInsufficientIdentifiers{Name: "name"}
	}
	e := fmt.Sprintf("/calico/v1/netset/%s", escapeName(key.Name))
	return e, nil
}

func (key NetworkSetKey) defaultDeletePath() (string, error) {
	return key.defaultPath()
}

func (key NetworkSetKey) defaultDeleteParentPaths() ([]string, error) {
	return nil, nil
}

func (key NetworkSetKey) valueType() reflect.Type {
	return typeNetworkSet
}

func (key NetworkSetKey) String() string {
	return fmt.Sprintf("NetworkSet(name=%s)", key.Name)
}

type NetworkSetListOptions struct {
	Name string
}

func (options NetworkSetListOptions) defaultPathRoot() string {
	k := "/calico/v1/netset"
	if options.Name == "" {
		return k
	}
	k = k + fmt.Sprintf("/%s", escapeName(options.Name))
	return k
}

func (options NetworkSetListOptions) KeyFromDefaultPath(path string) Key {
	log.Debugf("Get NetworkSet key from %s", path)
	r := matchNetworkSet.FindAllStringSubmatch(path, -1)
	if len(r) != 1 {
		log.Debugf("Didn't match regex")
		return nil
	}
	name := unescapeName(r[0][1])
	if options.Name != "" && name != options.Name {
		log.Debugf("Didn't match name %s != %s", options.Name, name)
		return nil
	}
	return NetworkSetKey{Name: name}
}

type NetworkSet struct {
	IPv4Nets []net.IPNet       `json:"ipv4_nets,omitempty" validate:"omitempty,dive,cidrv4"`
	IPv6Nets []net.IPNet       `json:"ipv6_nets,omitempty" validate:"omitempty,dive,cidrv6"`
	Labels   map[string]string `json:"labels,omitempty" validate:"omitempty,labels"`
}
