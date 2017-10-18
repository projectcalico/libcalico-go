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

package clientv2

import (
	"context"
	"errors"

	apiv2 "github.com/projectcalico/libcalico-go/lib/apis/v2"
	"github.com/projectcalico/libcalico-go/lib/options"
	"github.com/projectcalico/libcalico-go/lib/watch"
)

// ClusterInformationInterface has methods to work with ClusterInformation resources.
type ClusterInformationInterface interface {
	Create(ctx context.Context, res *apiv2.ClusterInformation, opts options.SetOptions) (*apiv2.ClusterInformation, error)
	Update(ctx context.Context, res *apiv2.ClusterInformation, opts options.SetOptions) (*apiv2.ClusterInformation, error)
	Delete(ctx context.Context, name string, opts options.DeleteOptions) (*apiv2.ClusterInformation, error)
	Get(ctx context.Context, name string, opts options.GetOptions) (*apiv2.ClusterInformation, error)
	List(ctx context.Context, opts options.ListOptions) (*apiv2.ClusterInformationList, error)
	Watch(ctx context.Context, opts options.ListOptions) (watch.Interface, error)
}

// clusterInformation implements ClusterInformationInterface
type clusterInformation struct {
	client client
}

// Create takes the representation of a ClusterInformation and creates it.
// Returns the stored representation of the ClusterInformation, and an error
// if there is any.
func (r clusterInformation) Create(ctx context.Context, res *apiv2.ClusterInformation, opts options.SetOptions) (*apiv2.ClusterInformation, error) {
	if res.ObjectMeta.GetName() != "default" {
		return nil, errors.New("Cannot create a Cluster Information resource with a name other than \"default\"")
	}
	out, err := r.client.resources.Create(ctx, opts, apiv2.KindClusterInformation, res)
	if out != nil {
		return out.(*apiv2.ClusterInformation), err
	}
	return nil, err
}

// Update takes the representation of a ClusterInformation and updates it.
// Returns the stored representation of the ClusterInformation, and an error
// if there is any.
func (r clusterInformation) Update(ctx context.Context, res *apiv2.ClusterInformation, opts options.SetOptions) (*apiv2.ClusterInformation, error) {
	out, err := r.client.resources.Update(ctx, opts, apiv2.KindClusterInformation, res)
	if out != nil {
		return out.(*apiv2.ClusterInformation), err
	}
	return nil, err
}

// Delete takes name of the ClusterInformation and deletes it. Returns an
// error if one occurs.
func (r clusterInformation) Delete(ctx context.Context, name string, opts options.DeleteOptions) (*apiv2.ClusterInformation, error) {
	out, err := r.client.resources.Delete(ctx, opts, apiv2.KindClusterInformation, noNamespace, name)
	if out != nil {
		return out.(*apiv2.ClusterInformation), err
	}
	return nil, err
}

// Get takes name of the ClusterInformation, and returns the corresponding
// ClusterInformation object, and an error if there is any.
func (r clusterInformation) Get(ctx context.Context, name string, opts options.GetOptions) (*apiv2.ClusterInformation, error) {
	out, err := r.client.resources.Get(ctx, opts, apiv2.KindClusterInformation, noNamespace, name)
	if out != nil {
		return out.(*apiv2.ClusterInformation), err
	}
	return nil, err
}

// List returns the list of ClusterInformation objects that match the supplied options.
func (r clusterInformation) List(ctx context.Context, opts options.ListOptions) (*apiv2.ClusterInformationList, error) {
	res := &apiv2.ClusterInformationList{}
	if err := r.client.resources.List(ctx, opts, apiv2.KindClusterInformation, apiv2.KindClusterInformationList, res); err != nil {
		return nil, err
	}
	return res, nil
}

// Watch returns a watch.Interface that watches the ClusterInformation that
// match the supplied options.
func (r clusterInformation) Watch(ctx context.Context, opts options.ListOptions) (watch.Interface, error) {
	return r.client.resources.Watch(ctx, opts, apiv2.KindClusterInformation)
}
