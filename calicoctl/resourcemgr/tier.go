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

package resourcemgr

import (
	"github.com/tigera/libcalico-go/lib/api"
	"github.com/tigera/libcalico-go/lib/api/unversioned"
	"github.com/tigera/libcalico-go/lib/client"
)

func init() {
	registerResource(
		api.NewTier(),
		api.NewTierList(),
		[]string{"NAME", "ORDER"},
		[]string{"NAME", "ORDER"},
		map[string]string{
			"NAME":  "{{.Metadata.Name}}",
			"ORDER": "{{.Spec.Order}}",
		},
		func(client *client.Client, resource unversioned.Resource) (unversioned.Resource, error) {
			r := resource.(api.Tier)
			return client.Tiers().Apply(&r)
		},
		func(client *client.Client, resource unversioned.Resource) (unversioned.Resource, error) {
			r := resource.(api.Tier)
			return client.Tiers().Create(&r)
		},
		func(client *client.Client, resource unversioned.Resource) (unversioned.Resource, error) {
			r := resource.(api.Tier)
			return client.Tiers().Update(&r)
		},
		func(client *client.Client, resource unversioned.Resource) (unversioned.Resource, error) {
			r := resource.(api.Tier)
			return nil, client.Tiers().Delete(r.Metadata)
		},
		func(client *client.Client, resource unversioned.Resource) (unversioned.Resource, error) {
			r := resource.(api.Tier)
			return client.Tiers().List(r.Metadata)
		},
	)
}
