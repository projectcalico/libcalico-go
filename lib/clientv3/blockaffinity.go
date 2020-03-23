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

// BlockAffinityInterface has methods to work with BlockAffinity resources.
type BlockAffinityInterface interface {
	Create(ctx context.Context, res *apiv3.BlockAffinity, opts options.SetOptions) (*apiv3.BlockAffinity, error)
	List(ctx context.Context, opts options.ListOptions) (*apiv3.BlockAffinityList, error)

	// Update is not supported on the etcdv3 backend, because it doesn't store the CreationTimestamp or UID.
	//Update(ctx context.Context, res *apiv3.BlockAffinity, opts options.SetOptions) (*apiv3.BlockAffinity, error)

	// Delete is not supported on the etcdv3 backend, because the resource name might be a hash of nodename + CIDR
	// which doesn't reliably allow us to infer the etcd key.
	//Delete(ctx context.Context, name string, opts options.DeleteOptions) (*apiv3.BlockAffinity, error)

	// Get is not supported on the etcdv3 backend, because the resource name might be a hash of nodename + CIDR
	// which doesn't reliably allow us to infer the etcd key.
	//Get(ctx context.Context, name string, opts options.GetOptions) (*apiv3.BlockAffinity, error)

	// Watch is not supported because we don't need it right now; this interface
	// is added to support etcd -> KDD migration
	//Watch(ctx context.Context, opts options.ListOptions) (watch.Interface, error)
}

// blockAffinities implements the BlockAffinityInterface
type blockAffinities struct {
	client client
}

// Create takes the representation of a BlockAffinity and creates it.
// Returns the stored representation of the BlockAffinity, and an error
// if there is any.
func (r blockAffinities) Create(ctx context.Context, res *apiv3.BlockAffinity, opts options.SetOptions) (*apiv3.BlockAffinity, error) {
	if err := validator.Validate(res); err != nil {
		return nil, err
	}

	out, err := r.client.resources.Create(ctx, opts, apiv3.KindBlockAffinity, res)
	if out != nil {
		return out.(*apiv3.BlockAffinity), err
	}
	return nil, err
}

// List returns the list of BlockAffinity objects that match the supplied options.
func (r blockAffinities) List(ctx context.Context, opts options.ListOptions) (*apiv3.BlockAffinityList, error) {
	res := &apiv3.BlockAffinityList{}
	if err := r.client.resources.List(ctx, opts, apiv3.KindBlockAffinity, apiv3.KindBlockAffinityList, res); err != nil {
		return nil, err
	}
	return res, nil
}
