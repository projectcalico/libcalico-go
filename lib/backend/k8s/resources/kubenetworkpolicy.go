// Copyright (c) 2017-2021 Tigera, Inc. All rights reserved.

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
	"errors"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/projectcalico/libcalico-go/lib/backend/api"
	"github.com/projectcalico/libcalico-go/lib/backend/k8s/conversion"
	"github.com/projectcalico/libcalico-go/lib/backend/model"
	cerrors "github.com/projectcalico/libcalico-go/lib/errors"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
)

func NewKubernetesNetworkPolicyClient(c *kubernetes.Clientset) K8sResourceClient {
	return &networkPolicyClient{
		Converter: conversion.NewConverter(),
		clientSet: c,
	}
}

// Implements the api.Client interface for Kubernetes NetworkPolicy.
type networkPolicyClient struct {
	conversion.Converter
	clientSet *kubernetes.Clientset
}

func (c *networkPolicyClient) Create(ctx context.Context, kvp *model.KVPair) (*model.KVPair, error) {
	log.Debug("Received Create request on NetworkPolicy type")
	return nil, cerrors.ErrorOperationNotSupported{
		Identifier: kvp.Key,
		Operation:  "Create",
	}
}

func (c *networkPolicyClient) Update(ctx context.Context, kvp *model.KVPair) (*model.KVPair, error) {
	log.Debug("Received Update request on NetworkPolicy type")
	return nil, cerrors.ErrorOperationNotSupported{
		Identifier: kvp.Key,
		Operation:  "Update",
	}
}

func (c *networkPolicyClient) Apply(ctx context.Context, kvp *model.KVPair) (*model.KVPair, error) {
	return nil, cerrors.ErrorOperationNotSupported{
		Identifier: kvp.Key,
		Operation:  "Apply",
	}
}

func (c *networkPolicyClient) DeleteKVP(ctx context.Context, kvp *model.KVPair) (*model.KVPair, error) {
	return c.Delete(ctx, kvp.Key, kvp.Revision, kvp.UID)
}

func (c *networkPolicyClient) Delete(ctx context.Context, key model.Key, revision string, uid *types.UID) (*model.KVPair, error) {
	return nil, cerrors.ErrorOperationNotSupported{
		Identifier: key,
		Operation:  "Delete",
	}
}

func (c *networkPolicyClient) Get(ctx context.Context, key model.Key, revision string) (*model.KVPair, error) {
	log.Debug("Received Get request on Kubernetes NetworkPolicy type")
	k := key.(model.ResourceKey)
	if k.Name == "" {
		return nil, errors.New("Missing policy name")
	}
	if k.Namespace == "" {
		return nil, errors.New("Missing policy namespace")
	}

	// Assert that this is backed by a NetworkPolicy.
	if !strings.HasPrefix(k.Name, conversion.K8sNetworkPolicyNamePrefix) {
		// TODO
		return nil, fmt.Errorf("Bad name")
	}

	// Backed by a NetworkPolicy - extract the name.
	policyName := strings.TrimPrefix(k.Name, conversion.K8sNetworkPolicyNamePrefix)

	// Get the NetworkPolicy from the API and convert it.
	networkPolicy := networkingv1.NetworkPolicy{}
	err := c.clientSet.NetworkingV1().RESTClient().
		Get().
		Resource("networkpolicies").
		Namespace(k.Namespace).
		Name(policyName).
		VersionedParams(&metav1.GetOptions{ResourceVersion: revision}, scheme.ParameterCodec).
		Do(ctx).Into(&networkPolicy)
	if err != nil {
		return nil, K8sErrorToCalico(err, k)
	}
	return c.K8sNetworkPolicyToCalico(&networkPolicy)
}

func (c *networkPolicyClient) List(ctx context.Context, list model.ListInterface, revision string) (*model.KVPairList, error) {
	log.Debug("Received List request on Kubernetes NetworkPolicy type")
	l := list.(model.ResourceListOptions)
	if l.Name != "" {
		// Exact lookup on a NetworkPolicy.
		kvp, err := c.Get(ctx, model.ResourceKey{Name: l.Name, Namespace: l.Namespace, Kind: l.Kind}, revision)
		if err != nil {
			// Return empty slice of KVPair if the object doesn't exist, return the error otherwise.
			if _, ok := err.(cerrors.ErrorResourceDoesNotExist); ok {
				return &model.KVPairList{
					KVPairs:  []*model.KVPair{},
					Revision: revision,
				}, nil
			} else {
				return nil, err
			}
		}

		return &model.KVPairList{
			KVPairs:  []*model.KVPair{kvp},
			Revision: revision,
		}, nil
	}

	// List all of the k8s NetworkPolicy objects.
	networkPolicies := networkingv1.NetworkPolicyList{}
	req := c.clientSet.NetworkingV1().RESTClient().
		Get().
		Resource("networkpolicies")
	if l.Namespace != "" {
		// Add the namespace if requested.
		req = req.Namespace(l.Namespace)
	}
	err := req.Do(ctx).Into(&networkPolicies)
	if err != nil {
		log.WithError(err).Info("Unable to list K8s Network Policy resources")
		return nil, K8sErrorToCalico(err, l)
	}

	// For each policy, turn it into a Policy and generate the list.
	npKvps := model.KVPairList{KVPairs: []*model.KVPair{}}
	for _, p := range networkPolicies.Items {
		kvp, err := c.K8sNetworkPolicyToCalico(&p)
		if err != nil {
			log.WithError(err).Info("Failed to convert K8s Network Policy")
			return nil, err
		}
		npKvps.KVPairs = append(npKvps.KVPairs, kvp)
	}

	// Add in the Revision information.
	npKvps.Revision = networkPolicies.ResourceVersion
	log.WithFields(log.Fields{
		"num_kvps": len(npKvps.KVPairs),
		"revision": npKvps.Revision}).Debug("Returning Kubernetes NP KVPs")
	return &npKvps, nil
}

func (c *networkPolicyClient) EnsureInitialized() error {
	return nil
}

func (c *networkPolicyClient) Watch(ctx context.Context, list model.ListInterface, revision string) (api.WatchInterface, error) {
	// Build watch options to pass to k8s.
	opts := metav1.ListOptions{Watch: true}
	rlo, ok := list.(model.ResourceListOptions)
	if !ok {
		return nil, fmt.Errorf("ListInterface is not a ResourceListOptions: %s", list)
	}

	// Watch a specific networkPolicy
	if len(rlo.Name) != 0 {
		if len(rlo.Namespace) == 0 {
			return nil, errors.New("cannot watch a specific NetworkPolicy without a namespace")
		}
		// We've been asked to watch a specific networkpolicy.
		log.WithField("name", rlo.Name).Debug("Watching a single networkpolicy")
		// Backed by a NetworkPolicy - extract the name.
		policyName := rlo.Name
		if !strings.HasPrefix(rlo.Name, conversion.K8sNetworkPolicyNamePrefix) {
			// TODO: Error
		}
		policyName = strings.TrimPrefix(rlo.Name, conversion.K8sNetworkPolicyNamePrefix)

		// write back in rlo for custom resource watch below
		rlo.Name = policyName
		opts.FieldSelector = fields.OneTermEqualSelector("metadata.name", policyName).String()
	}

	opts.ResourceVersion = revision
	log.Debugf("Watching Kubernetes NetworkPolicy at revision %q", revision)
	k8sRawWatch, err := c.clientSet.NetworkingV1().NetworkPolicies(rlo.Namespace).Watch(ctx, opts)
	if err != nil {
		return nil, K8sErrorToCalico(err, list)
	}
	converter := func(r Resource) (*model.KVPair, error) {
		np, ok := r.(*networkingv1.NetworkPolicy)
		if !ok {
			return nil, errors.New("KubernetesNetworkPolicy conversion with incorrect k8s resource type")
		}

		return c.K8sNetworkPolicyToCalico(np)
	}
	return newK8sWatcherConverter(ctx, "KubernetesNetworkPolicy", converter, k8sRawWatch), nil
}
