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

package clientv3

import (
	"context"
	"errors"

	apiv3 "github.com/projectcalico/libcalico-go/lib/apis/v3"
	"github.com/projectcalico/libcalico-go/lib/options"
	validator "github.com/projectcalico/libcalico-go/lib/validator/v3"
)

// IPAMConfigInterface has methods to work with IPAMConfig resources.
type IPAMConfigInterface interface {
	Create(ctx context.Context, res *apiv3.IPAMConfig, opts options.SetOptions) (*apiv3.IPAMConfig, error)

	Get(ctx context.Context, name string, opts options.GetOptions) (*apiv3.IPAMConfig, error)

	// Update is not supported on the etcdv3 backend, because it doesn't store the CreationTimestamp or UID.
	//Update(ctx context.Context, res *apiv3.IPAMConfig, opts options.SetOptions) (*apiv3.IPAMConfig, error)

	// List is not supported on the etcdv3 backend, because the IPAM Config model doesn't have a
	// list interface.
	// List(ctx context.Context, opts options.ListOptions) (*apiv3.IPAMConfigList, error)

	// Delete, and Watch are not implemented because only Create and List are needed for
	// etcd -> KDD migration
	//Delete(ctx context.Context, name string, opts options.DeleteOptions) (*apiv3.IPAMConfig, error)
	//Watch(ctx context.Context, opts options.ListOptions) (watch.Interface, error)
}

// ipamConfigs implements the IPAMConfigInterface
type ipamConfigs struct {
	client client
}

// Create takes the representation of a IPAMConfig and creates it.
// Returns the stored representation of the IPAMConfig, and an error
// if there is any.
func (r ipamConfigs) Create(ctx context.Context, res *apiv3.IPAMConfig, opts options.SetOptions) (*apiv3.IPAMConfig, error) {
	if err := validator.Validate(res); err != nil {
		return nil, err
	}

	if res.ObjectMeta.GetName() != "default" {
		return nil, errors.New("Cannot create an IPAM Config resource with a name other than \"default\"")
	}
	out, err := r.client.resources.Create(ctx, opts, apiv3.KindIPAMConfig, res)
	if out != nil {
		return out.(*apiv3.IPAMConfig), err
	}
	return nil, err
}

// Get takes name of the IPAMConfig, and returns the corresponding
// IPAMConfig object, and an error if there is any.
func (r ipamConfigs) Get(ctx context.Context, name string, opts options.GetOptions) (*apiv3.IPAMConfig, error) {
	out, err := r.client.resources.Get(ctx, opts, apiv3.KindIPAMConfig, noNamespace, name)
	if out != nil {
		return out.(*apiv3.IPAMConfig), err
	}
	return nil, err
}
