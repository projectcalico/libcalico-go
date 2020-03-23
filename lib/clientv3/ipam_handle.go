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

	apiv3 "github.com/projectcalico/libcalico-go/lib/apis/v3"
	"github.com/projectcalico/libcalico-go/lib/options"
	validator "github.com/projectcalico/libcalico-go/lib/validator/v3"
)

// IPAMHandleInterface has methods to work with IPAMHandle resources.
type IPAMHandleInterface interface {
	Create(ctx context.Context, res *apiv3.IPAMHandle, opts options.SetOptions) (*apiv3.IPAMHandle, error)
	List(ctx context.Context, opts options.ListOptions) (*apiv3.IPAMHandleList, error)

	// Update is not supported on the etcdv3 backend, because it doesn't store the CreationTimestamp or UID.
	//Update(ctx context.Context, res *apiv3.IPAMHandle, opts options.SetOptions) (*apiv3.IPAMHandle, error)

	// Delete, Get, and Watch are not implemented because only Create and List are needed for
	// etcd -> KDD migration
	//Delete(ctx context.Context, name string, opts options.DeleteOptions) (*apiv3.IPAMHandle, error)
	//Get(ctx context.Context, name string, opts options.GetOptions) (*apiv3.IPAMHandle, error)
	//Watch(ctx context.Context, opts options.ListOptions) (watch.Interface, error)
}

// ipamHandles implements the IPAMHandleInterface
type ipamHandles struct {
	client client
}

// Create takes the representation of a IPAMHandle and creates it.
// Returns the stored representation of the IPAMHandle, and an error
// if there is any.
func (r ipamHandles) Create(ctx context.Context, res *apiv3.IPAMHandle, opts options.SetOptions) (*apiv3.IPAMHandle, error) {
	if err := validator.Validate(res); err != nil {
		return nil, err
	}

	out, err := r.client.resources.Create(ctx, opts, apiv3.KindIPAMHandle, res)
	if out != nil {
		return out.(*apiv3.IPAMHandle), err
	}
	return nil, err
}

// List returns the list of IPAMHandle objects that match the supplied options.
func (r ipamHandles) List(ctx context.Context, opts options.ListOptions) (*apiv3.IPAMHandleList, error) {
	res := &apiv3.IPAMHandleList{}
	if err := r.client.resources.List(ctx, opts, apiv3.KindIPAMHandle, apiv3.KindIPAMHandleList, res); err != nil {
		return nil, err
	}
	return res, nil
}
