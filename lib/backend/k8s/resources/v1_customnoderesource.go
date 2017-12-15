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
	"context"
	"sort"
	"strings"

	log "github.com/sirupsen/logrus"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/projectcalico/libcalico-go/lib/backend/api"
	"github.com/projectcalico/libcalico-go/lib/backend/model"
	"github.com/projectcalico/libcalico-go/lib/errors"
)

// Action strings - used for context logging.
type action string

const (
	actApply  action = "Apply"
	actCreate action = "Create"
	actUpdate action = "Update"
)

// customK8sNodeResourceConverterV1 defines an interface to map between the model and the
// annotation representation of a resource.
//
// This is maintained purely for migration purposes from the v1 data format.  It only
// requires the accessor (List/Get) methods to be implemented and all others can be
// mocked out.
type customK8sNodeResourceConverterV1 interface {
	// ListInterfaceToNodeAndName converts the ListInterface to the node name
	// and resource name.
	ListInterfaceToNodeAndName(model.ListInterface) (string, string, error)

	// KeyToNodeAndName converts the Key to the node name and resource name.
	KeyToNodeAndName(model.Key) (string, string, error)

	// NodeAndNameToKey converts the Node name and resource name to a Key.
	NodeAndNameToKey(string, string) (model.Key, error)
}

// customK8sNodeResourceClientConfigV1 is the config required for initializing a new
// per-node K8sResourceClient
type customK8sNodeResourceClientConfigV1 struct {
	ClientSet    *kubernetes.Clientset
	ResourceType string
	Converter    customK8sNodeResourceConverterV1
	Namespace    string
}

// NewCustomK8sNodeResourceClientV1 creates a new per-node K8sResourceClient.
func NewCustomK8sNodeResourceClientV1(config customK8sNodeResourceClientConfigV1) K8sResourceClient {
	return &customK8sNodeResourceClientV1{
		customK8sNodeResourceClientConfigV1: config,
		annotationKeyPrefix:                 config.Namespace + "/",
	}
}

// customK8sNodeResourceClientV1 implements the K8sResourceClient interface.  It
// should only be created using newCustomK8sNodeResourceClientConfig since that
// ensures it is wrapped with a retryWrapper.
type customK8sNodeResourceClientV1 struct {
	customK8sNodeResourceClientConfigV1
	annotationKeyPrefix string
}

// Methods not required for the v1 resources.
func (c *customK8sNodeResourceClientV1) Create(ctx context.Context, object *model.KVPair) (*model.KVPair, error) {
	panic("Not supported")
}
func (c *customK8sNodeResourceClientV1) Update(ctx context.Context, object *model.KVPair) (*model.KVPair, error) {
	panic("Not supported")
}
func (c *customK8sNodeResourceClientV1) Apply(object *model.KVPair) (*model.KVPair, error) {
	panic("not supported")
}
func (c *customK8sNodeResourceClientV1) Delete(ctx context.Context, key model.Key, revision string) (*model.KVPair, error) {
	panic("not supported")
}
func (c *customK8sNodeResourceClientV1) Watch(ctx context.Context, list model.ListInterface, revision string) (api.WatchInterface, error) {
	panic("Not supported")
}
func (c *customK8sNodeResourceClientV1) EnsureInitialized() error {
	panic("Not supported")
}

func (c *customK8sNodeResourceClientV1) Get(ctx context.Context, key model.Key, revision string) (*model.KVPair, error) {
	logContext := log.WithFields(log.Fields{
		"Key":       key,
		"Resource":  c.ResourceType,
		"Namespace": c.Namespace,
	})
	logContext.Debug("Get per-Node resource")

	// Get the names and the latest Node settings associated with the Key.
	name, node, err := c.getNameAndNodeFromKey(key)
	if err != nil {
		logContext.WithError(err).Info("Failed to get resource")
		return nil, err
	}

	// Extract the resource from the annotations.  It should exist.
	kvps, err := c.extractResourcesFromAnnotation(node, name)
	if err != nil {
		logContext.WithError(err).Error("Failed to get resource: error in data")
		return nil, err
	}
	if len(kvps) != 1 {
		logContext.Warning("Failed to get resource: resource does not exist")
		return nil, errors.ErrorResourceDoesNotExist{Identifier: key}
	}

	return kvps[0], nil
}

func (c *customK8sNodeResourceClientV1) List(ctx context.Context, list model.ListInterface, revision string) (*model.KVPairList, error) {
	logContext := log.WithFields(log.Fields{
		"ListInterface": list,
		"Resource":      c.ResourceType,
		"Namespace":     c.Namespace,
	})
	logContext.Debug("List per-Node Resources")
	kvps := []*model.KVPair{}

	// Extract the Node and Name from the ListInterface.
	nodeName, resName, err := c.Converter.ListInterfaceToNodeAndName(list)
	if err != nil {
		logContext.WithError(err).Info("Failed to list resources: error in list interface conversion")
		return nil, err
	}

	// Get a list of the required nodes - which will either be all of them
	// or a specific node.
	var nodes []apiv1.Node
	var rev string
	if nodeName != "" {
		newLogContext := logContext.WithField("NodeName", nodeName)
		node, err := c.ClientSet.CoreV1().Nodes().Get(nodeName, metav1.GetOptions{})
		if err != nil {
			err = K8sErrorToCalico(err, nodeName)
			if _, ok := err.(errors.ErrorResourceDoesNotExist); !ok {
				newLogContext.WithError(err).Error("Failed to list resources: unable to query node")
				return nil, err
			}
			newLogContext.WithError(err).Warning("Return no results for resource list: node does not exist")
			return &model.KVPairList{
				KVPairs: kvps,
			}, err
		}
		nodes = append(nodes, *node)
		rev = node.GetResourceVersion()
	} else {
		nodeList, err := c.ClientSet.CoreV1().Nodes().List(metav1.ListOptions{})
		if err != nil {
			logContext.WithError(err).Info("Failed to list resources: unable to list Nodes")
			return nil, K8sErrorToCalico(err, nodeName)
		}
		nodes = nodeList.Items
		rev = nodeList.GetResourceVersion()
	}

	// Loop through each of the nodes and extract the required data.
	for _, node := range nodes {
		nodeKVPs, err := c.extractResourcesFromAnnotation(&node, resName)
		if err != nil {
			logContext.WithField("NodeName", node.GetName()).WithError(err).Error("Error listing resources: error in data")
		}
		kvps = append(kvps, nodeKVPs...)
	}

	return &model.KVPairList{
		KVPairs:  kvps,
		Revision: rev,
	}, err
}

// getNameAndNodeFromKey extracts the resource name from the key
// and gets the Node resource from the Kubernetes API.
// Returns: the resource name, the Node resource.
func (c *customK8sNodeResourceClientV1) getNameAndNodeFromKey(key model.Key) (string, *apiv1.Node, error) {
	logContext := log.WithFields(log.Fields{
		"Key":       key,
		"Resource":  c.ResourceType,
		"Namespace": c.Namespace,
	})
	logContext.Debug("Extract node and resource info from Key")

	// Get the node and resource name.
	nodeName, resName, err := c.Converter.KeyToNodeAndName(key)
	if err != nil {
		logContext.WithError(err).Info("Error converting Key to Node and Resource name.")
		return "", nil, err
	}

	// Get the current node settings.
	node, err := c.ClientSet.CoreV1().Nodes().Get(nodeName, metav1.GetOptions{})
	if err != nil {
		logContext.WithError(err).Info("Error getting Node configuration")
		return "", nil, K8sErrorToCalico(err, key)
	}

	return resName, node, nil
}

// nameToAnnotationKey converts the resource name to the annotations key.
func (c *customK8sNodeResourceClientV1) nameToAnnotationKey(name string) string {
	return c.annotationKeyPrefix + name
}

// annotationKeyToName converts the annotations key to a resource name, or returns
// and empty string if the annotation key does not represent a resource.
func (c *customK8sNodeResourceClientV1) annotationKeyToName(key string) string {
	if strings.HasPrefix(key, c.annotationKeyPrefix) {
		return key[len(c.annotationKeyPrefix):]
	}
	return ""
}

// extractResourcesFromAnnotation queries the current Kubernetes Node resource
// and parses the per-node resource entries configured in the annotations.
// Returns the Node resource configuration and the slice of parsed resources.
func (c *customK8sNodeResourceClientV1) extractResourcesFromAnnotation(node *apiv1.Node, name string) ([]*model.KVPair, error) {
	logContext := log.WithFields(log.Fields{
		"ResourceType": name,
		"Resource":     c.ResourceType,
		"Namespace":    c.Namespace,
	})
	logContext.Debug("Extract node and resource info from Key")

	// Extract the resource entries from the annotation.  We do this either by
	// extracting the requested entry if it exists, or by iterating through each
	// annotation and checking if it corresponds to a resource.
	resStrings := make(map[string]string, 0)
	resNames := []string{}
	if name != "" {
		ak := c.nameToAnnotationKey(name)
		if v, ok := node.Annotations[ak]; ok {
			resStrings[name] = v
			resNames = append(resNames, name)
		}
	} else {
		for ak, v := range node.Annotations {
			if n := c.annotationKeyToName(ak); n != "" {
				resStrings[n] = v
				resNames = append(resNames, n)
			}
		}
	}

	// Sort the resource names to ensure the KVPairs are ordered.
	sort.Strings(resNames)

	// Process each entry in name order and add to the return set of KVPairs.
	// Use the node revision as the revision of each entry.  If we hit an error
	// unmarshalling then return the error if we are querying an exact entry, but
	// swallow the error if we are listing multiple (otherwise a single bad entry
	// would prevent all resources being listed).
	kvps := []*model.KVPair{}
	for _, resName := range resNames {
		key, err := c.Converter.NodeAndNameToKey(node.GetName(), resName)
		if err != nil {
			logContext.WithField("ResourceType", resName).WithError(err).Error("Error unmarshalling key")
			if name != "" {
				return nil, err
			}
			continue
		}

		value, err := model.ParseValue(key, []byte(resStrings[resName]))
		if err != nil {
			logContext.WithField("ResourceType", resName).WithError(err).Error("Error unmarshalling value")
			if name != "" {
				return nil, err
			}
			continue
		}
		kvp := &model.KVPair{
			Key:      key,
			Value:    value,
			Revision: node.GetResourceVersion(),
		}
		kvps = append(kvps, kvp)
	}

	return kvps, nil
}

// ExtractResourcesFromNode returns the resources stored in the Node configuration.
//
// This convenience method is expected to be removed in a future libcalico-go release.
func (c *customK8sNodeResourceClientV1) ExtractResourcesFromNode(node *apiv1.Node) ([]*model.KVPair, error) {
	return c.extractResourcesFromAnnotation(node, "")
}
