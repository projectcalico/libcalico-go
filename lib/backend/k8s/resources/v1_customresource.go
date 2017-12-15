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
	"reflect"

	log "github.com/sirupsen/logrus"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/projectcalico/libcalico-go/lib/backend/api"
	"github.com/projectcalico/libcalico-go/lib/backend/model"
	"github.com/projectcalico/libcalico-go/lib/errors"
)

// customK8sResourceConverterV1 defines an interface to map between KVPair representation
// and a custom Kubernetes resource.
type customK8sResourceConverterV1 interface {
	// ListInterfaceToKey converts a ListInterface to a Key if the
	// ListInterface specifies a specific instance, otherwise returns nil.
	ListInterfaceToKey(model.ListInterface) model.Key

	// Convert the Key to the Resource name.
	KeyToName(model.Key) (string, error)

	// Convert the Resource name to the Key.
	NameToKey(string) (model.Key, error)

	// Convert the Resource to a KVPair.
	ToKVPair(Resource) (*model.KVPair, error)

	// Convert a KVPair to a Resource.
	FromKVPair(*model.KVPair) (Resource, error)
}

// customK8sResourceClientV1 implements the K8sResourceClient interface and provides a generic
// mechanism for a 1:1 mapping between a Calico Resource and an equivalent Kubernetes
// custom resource type.
type customK8sResourceClientV1 struct {
	clientSet       *kubernetes.Clientset
	restClient      *rest.RESTClient
	name            string
	resource        string
	description     string
	k8sResourceType reflect.Type
	k8sListType     reflect.Type
	converter       customK8sResourceConverterV1
}

// Get gets an existing Custom K8s Resource instance in the k8s API using the supplied Key.
func (c *customK8sResourceClientV1) Get(ctx context.Context, key model.Key, rev string) (*model.KVPair, error) {
	logContext := log.WithFields(log.Fields{
		"Key":      key,
		"Resource": c.resource,
	})
	logContext.Debug("Get custom Kubernetes resource")
	name, err := c.converter.KeyToName(key)
	if err != nil {
		logContext.WithError(err).Info("Error getting resource")
		return nil, err
	}

	// Add the name to the log context now that we know it, and query
	// Kubernetes.
	logContext = logContext.WithField("Name", name)
	logContext.Debug("Get custom Kubernetes resource by name")
	resOut := reflect.New(c.k8sResourceType).Interface().(Resource)
	err = c.restClient.Get().
		Resource(c.resource).
		Name(name).
		Do().Into(resOut)
	if err != nil {
		logContext.WithError(err).Info("Error getting resource")
		return nil, K8sErrorToCalico(err, key)
	}

	return c.converter.ToKVPair(resOut)
}

// List lists configured Custom K8s Resource instances in the k8s API matching the
// supplied ListInterface.
func (c *customK8sResourceClientV1) List(ctx context.Context, list model.ListInterface, rev string) (*model.KVPairList, error) {
	logContext := log.WithFields(log.Fields{
		"ListInterface": list,
		"Resource":      c.resource,
	})
	logContext.Debug("List Custom K8s Resource")
	kvps := []*model.KVPair{}

	// Attempt to convert the ListInterface to a Key.  If possible, the parameters
	// indicate a fully qualified resource, and we'll need to use Get instead of
	// List.
	if key := c.converter.ListInterfaceToKey(list); key != nil {
		logContext.Debug("Performing List using Get")
		if kvp, err := c.Get(ctx, key, rev); err != nil {
			// The error will already be a Calico error type.  Ignore
			// error that it doesn't exist - we'll return an empty
			// list.
			if _, ok := err.(errors.ErrorResourceDoesNotExist); !ok {
				log.WithField("Resource", c.resource).WithError(err).Info("Error listing resource")
				return nil, err
			}
			return &model.KVPairList{
				KVPairs:  kvps,
				Revision: rev,
			}, nil
		} else {
			kvps = append(kvps, kvp)
			return &model.KVPairList{
				KVPairs:  kvps,
				Revision: rev,
			}, nil
		}
	}

	// Since we are not performing an exact Get, Kubernetes will return a
	// list of resources.
	reslOut := reflect.New(c.k8sListType).Interface().(ResourceList)

	// Perform the request.
	err := c.restClient.Get().
		Resource(c.resource).
		Do().Into(reslOut)
	if err != nil {
		// Don't return errors for "not found".  This just
		// means there are no matching Custom K8s Resources, and we should return
		// an empty list.
		if !kerrors.IsNotFound(err) {
			log.WithError(err).Info("Error listing resources")
			return nil, K8sErrorToCalico(err, list)
		}
		return &model.KVPairList{
			KVPairs:  kvps,
			Revision: reslOut.GetListMeta().GetResourceVersion(),
		}, nil
	}

	// We expect the list type to have an "Items" field that we can
	// iterate over.
	elem := reflect.ValueOf(reslOut).Elem()
	items := reflect.ValueOf(elem.FieldByName("Items").Interface())
	for idx := 0; idx < items.Len(); idx++ {
		res := items.Index(idx).Addr().Interface().(Resource)

		if kvp, err := c.converter.ToKVPair(res); err == nil {
			kvps = append(kvps, kvp)
		} else {
			logContext.WithError(err).WithField("Item", res).Warning("unable to process resource, skipping")
		}
	}
	return &model.KVPairList{
		KVPairs:  kvps,
		Revision: reslOut.GetListMeta().GetResourceVersion(),
	}, nil
}

// Methods not required for the v1 resources.
func (c *customK8sResourceClientV1) Create(ctx context.Context, object *model.KVPair) (*model.KVPair, error) {
	panic("Not supported")
}
func (c *customK8sResourceClientV1) Update(ctx context.Context, object *model.KVPair) (*model.KVPair, error) {
	panic("Not supported")
}
func (c *customK8sResourceClientV1) Apply(object *model.KVPair) (*model.KVPair, error) {
	panic("not supported")
}
func (c *customK8sResourceClientV1) Delete(ctx context.Context, key model.Key, revision string) (*model.KVPair, error) {
	panic("not supported")
}
func (c *customK8sResourceClientV1) Watch(ctx context.Context, list model.ListInterface, revision string) (api.WatchInterface, error) {
	panic("Not supported")
}
func (c *customK8sResourceClientV1) EnsureInitialized() error {
	panic("Not supported")
}
