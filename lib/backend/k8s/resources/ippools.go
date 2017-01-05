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

package resources

import (
	goerrors "errors"
	"fmt"
	log "github.com/Sirupsen/logrus"

	"github.com/projectcalico/libcalico-go/lib/backend/api"
	"github.com/projectcalico/libcalico-go/lib/backend/k8s/thirdparty"
	"github.com/projectcalico/libcalico-go/lib/backend/k8s/utils"
	"github.com/projectcalico/libcalico-go/lib/backend/model"
	"github.com/projectcalico/libcalico-go/lib/errors"

	"k8s.io/client-go/kubernetes"
	kerrors "k8s.io/client-go/pkg/api/errors"
	kapiv1 "k8s.io/client-go/pkg/api/v1"
	extensions "k8s.io/client-go/pkg/apis/extensions/v1beta1"
	"k8s.io/client-go/rest"
)

func NewIPPools(c *kubernetes.Clientset, r *rest.RESTClient) api.Client {
	return &client{
		clientSet: c,
		tprClient: r,
	}
}

// Implements the api.Client interface for pools.
type client struct {
	clientSet *kubernetes.Clientset
	tprClient *rest.RESTClient
}

func (c *client) Create(kvp *model.KVPair) (*model.KVPair, error) {
	tpr := IPPoolToThirdParty(kvp)
	res := thirdparty.IpPool{}
	req := c.tprClient.Post().
		Resource("ippools").
		Namespace("kube-system").
		Body(tpr)
	err := req.Do().Into(&res)
	if err != nil {
		return nil, utils.K8sErrorToCalico(err, kvp.Key)
	}
	kvp.Revision = res.Metadata.ResourceVersion
	return kvp, nil
}

func (c *client) Update(kvp *model.KVPair) (*model.KVPair, error) {
	tpr := IPPoolToThirdParty(kvp)
	res := thirdparty.IpPool{}
	req := c.tprClient.Put().
		Resource("ippools").
		Namespace("kube-system").
		Body(tpr).
		Name(tpr.Metadata.Name)
	err := req.Do().Into(&res)
	if err != nil {
		return nil, utils.K8sErrorToCalico(err, kvp.Key)
	}
	kvp.Revision = tpr.Metadata.ResourceVersion
	return kvp, nil
}

func (c *client) Apply(kvp *model.KVPair) (*model.KVPair, error) {
	updated, err := c.Update(kvp)
	if err != nil {
		if _, ok := err.(errors.ErrorResourceDoesNotExist); !ok {
			return nil, err
		}

		// It doesn't exist - create it.
		updated, err = c.Create(kvp)
		if err != nil {
			return nil, err
		}
	}
	return updated, nil
}

func (c *client) Delete(kvp *model.KVPair) error {
	result := c.tprClient.Delete().
		Resource("ippools").
		Namespace("kube-system").
		Name(tprName(kvp.Key.(model.IPPoolKey))).
		Do()

		// TODO: Error conversion
	return utils.K8sErrorToCalico(result.Error(), kvp.Key)
}

func (c *client) Get(key model.Key) (*model.KVPair, error) {
	tpr := thirdparty.IpPool{}
	err := c.tprClient.Get().
		Resource("ippools").
		Namespace("kube-system").
		Name(tprName(key.(model.IPPoolKey))).
		Do().Into(&tpr)
	if err != nil {
		return nil, utils.K8sErrorToCalico(err, key)
	}

	return ThirdPartyToIPPool(&tpr), nil
}

func (c *client) List(list model.ListInterface) ([]*model.KVPair, error) {
	kvps := []*model.KVPair{}
	tprs := thirdparty.IpPoolList{}
	l := list.(model.IPPoolListOptions)

	// Build the request.
	req := c.tprClient.Get().Resource("ippools").Namespace("kube-system")
	if l.CIDR.IP != nil {
		k := model.IPPoolKey{CIDR: l.CIDR}
		req.Name(tprName(k))
	}

	// Perform the request.
	err := req.Do().Into(&tprs)
	if err != nil {
		// Don't return errors for "not found".  This just
		// means there are no IPPools, and we should return
		// an empty list.
		if !kerrors.IsNotFound(err) {
			return nil, utils.K8sErrorToCalico(err, l)
		}
	}

	// Convert them to KVPairs.
	for _, tpr := range tprs.Items {
		kvps = append(kvps, ThirdPartyToIPPool(&tpr))
	}
	return kvps, nil
}

func (c *client) EnsureInitialized() error {
	log.Info("Ensuring IP Pool ThirdPartyResource exists")
	tpr := extensions.ThirdPartyResource{
		ObjectMeta: kapiv1.ObjectMeta{
			Name:      "ip-pool.projectcalico.org",
			Namespace: "kube-system",
		},
		Description: "Calico IP Pools",
		Versions:    []extensions.APIVersion{{Name: "v1"}},
	}
	_, err := c.clientSet.Extensions().ThirdPartyResources().Create(&tpr)
	if err != nil {
		// Don't care if it already exists.
		if !kerrors.IsAlreadyExists(err) {
			return goerrors.New(fmt.Sprintf("failed to create ThirdPartyResource %s: %s", tpr.ObjectMeta.Name, err))
		}
	}
	return nil
}

// Syncer is not implemented for ip pools.
func (c *client) Syncer(callbacks api.SyncerCallbacks) api.Syncer {
	return fakeSyncer{}
}

type fakeSyncer struct{}

func (f fakeSyncer) Start() {}
