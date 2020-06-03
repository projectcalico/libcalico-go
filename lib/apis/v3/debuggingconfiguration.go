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

package v3

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	KindDebuggingConfiguration     = "DebuggingConfiguration"
	KindDebuggingConfigurationList = "DebuggingConfigurationList"
)

type LogLevel string

const (
	LogLevelInfo  LogLevel = "Info"
	LogLevelDebug LogLevel = "Debug"
)

type Component string

const (
	ComponentCalicoNode      Component = "calico-node"
	ComponentKubeControllers Component = "calico-kube-controllers"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DebuggingConfiguration contains the debugging configuration for Tigera Enterprise components.
type DebuggingConfiguration struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the DebuggingConfiguration.
	Spec DebuggingConfigurationSpec `json:"spec,omitempty"`
}

// DebuggingConfigurationSpec contains the debugging configuration values for Tigera Enterprise components.
type DebuggingConfigurationSpec struct {
	// Configuration contains debugging configuration as granular as per container on a given node.
	Configuration []ComponentConfiguration `json:"co,omitempty" validate:"omitempty,dive"`
}

// ComponentConfiguration is the debugging configuration to be applied to the Tigera Enterprise.
// If multiple configurations apply to same process on a given node, the evaluation order is as follow:
// - component/node
// - component
type ComponentConfiguration struct {
	// Component indicates which Tigera Enterprise component the configuration applies to.
	Component Component `json:"component"`

	// LogSeverity is the log severity above which logs are sent to the stdout. [Default: Info]
	LogSeverity LogLevel `json:"logLevel,omitempty" validate:"omitempty,debuggingLogLevel"`

	// Node if present restricts the configuration to the Component running on the specified node
	// only. If not specified, configuration will apply to all replicas.
	Node string `json:"node,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DebuggingConfigurationList contains a list of DebuggingConfiguration resources.
type DebuggingConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []DebuggingConfiguration `json:"items"`
}

// NewDebuggingConfiguration creates a new (zeroed) DebuggingConfiguration struct with
// the TypeMetadata initialized to the current version.
func NewDebuggingConfiguration() *DebuggingConfiguration {
	return &DebuggingConfiguration{
		TypeMeta: metav1.TypeMeta{
			Kind:       KindDebuggingConfiguration,
			APIVersion: GroupVersionCurrent,
		},
	}
}

// NewDebuggingConfigurationList creates a new (zeroed) DebuggingConfigurationList struct with the TypeMetadata
// initialized to the current version.
func NewDebuggingConfigurationList() *DebuggingConfigurationList {
	return &DebuggingConfigurationList{
		TypeMeta: metav1.TypeMeta{
			Kind:       KindDebuggingConfigurationList,
			APIVersion: GroupVersionCurrent,
		},
	}
}

// IsValidDebuggingConfigurationComponent validates that provided component is valid.
// If invalid, returns also a message listing the accepted/valid components
func IsValidDebuggingConfigurationComponent(c Component) (bool, string) {
	switch c {
	case ComponentCalicoNode, ComponentKubeControllers:
		return true, ""
	}
	return false, fmt.Sprint("Valid components are:", ComponentCalicoNode, ComponentKubeControllers)
}

// IsValidDebuggingConfigurationNode validates that provided component/node pair is valid.
// Only daemonset components allow specifying node.
func IsValidDebuggingConfigurationNode(c Component) (bool, string) {
	if c != ComponentCalicoNode {
		return false, fmt.Sprintf("Node can be specified only for component %s", ComponentCalicoNode)
	}
	return true, ""
}
