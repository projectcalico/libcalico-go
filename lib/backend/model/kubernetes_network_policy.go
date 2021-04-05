// Copyright (c) 2021 Tigera, Inc. All rights reserved.

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
	"strings"

	"reflect"
)

const (
	KindKubernetesNetworkPolicy = "KubernetesNetworkPolicy"
)

// KubernetesNetworkPolicyKey represents a Kubernetes NetworkPolicy. Note that this is a pseudo-resource,
// exposed solely for use by the felixsyncer in KDD mode. As such, the functions and structures in this
// file are largely unimplemented.
type KubernetesNetworkPolicyKey struct {
	Name      string
	Namespace string
}

func (key KubernetesNetworkPolicyKey) defaultPath() (string, error) {
	return fmt.Sprintf("%s/%s", key.Name, key.Namespace), nil
}

func (key KubernetesNetworkPolicyKey) defaultDeletePath() (string, error) {
	return key.defaultPath()
}

func (key KubernetesNetworkPolicyKey) defaultDeleteParentPaths() ([]string, error) {
	return nil, nil
}

func (key KubernetesNetworkPolicyKey) valueType() (reflect.Type, error) {
	return typeIPPool, nil
}

func (key KubernetesNetworkPolicyKey) String() string {
	return fmt.Sprintf("KubernetesNetworkPolicy(%s/%s)", key.Namespace, key.Name)
}

type KubernetesNetworkPolicyListOptions struct {
}

func (options KubernetesNetworkPolicyListOptions) defaultPathRoot() string {
	return "/networking.k8s.io/v1/networkpolicy/"
}

func (options KubernetesNetworkPolicyListOptions) KeyFromDefaultPath(path string) Key {
	splits := strings.Split(path, "/")
	return KubernetesNetworkPolicyKey{Namespace: splits[0], Name: splits[1]}
}
