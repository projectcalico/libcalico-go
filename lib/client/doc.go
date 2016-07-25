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

/*
Package client implements the northbound client used to manage Calico configuration.
This client is the main entry point for applications that are managing or querying
Calico configuration.

This client provides a typed interface for managing different resource types.  The
definitions for each resource type is defined in the following package:

	github.com/tigera/libcalico-go/lib/api

The client has a number of methods that return interfaces for managing:
	-  Tier resources
	-  Policy resources
	-  IP Pool resources
	-  Host endpoint resources
	-  Workload endpoint resources
	-  Profile resources
	-  IP Address Management

See interface definitions for details about the set of management commands for each
resource type.

Many of the management interfaces have a common set of commands to create, delete,
update and retrieve resource instances.  For example, an application using this
client to manage tier resources would create an instance of this client, create a
new Tiers interface and call the appropriate methods on that interface.  For example:

	// client config defaults to access an etcd backend datastore at
	// localhost:2379.  For alternative access details, set the appropriate
	// fields in the ClientConfig structure.
	config := api.ClientConfig{}
	client := New(&config)

	// Obtain the interface for managing tier resources.
	tiers := client.Tiers()

	// Create a new tier.  All Create() methods return an error of type
	// common.ErrorResourceAlreadyExists if the resource specified by its
	// unique identifiers already exists.
	tier, err := tiers.Create(&api.Tier{
		Metadata: api.TierMetadata{
			Name: "tier-1",
		},
		Spec: api.TierSpec{
			Order: 100
		},
	}

	// Update am existing tier.  All Update() methods return an error of type
	// common.ErrorResourceDoesNotExist if the resource specified by its
	// unique identifiers does not exist.
	tier, err = tiers.Update(&api.Tier{
		Metadata: api.TierMetadata{
			Name: "tier-1",
		},
		Spec: api.TierSpec{
			Order: 200
		},
	}

	// Apply (update or create) a tier.  All Apply() methods will update a resource
	// if it already exists, and will create a new resource if it does not.
	tier, err = tiers.Apply(&api.Tier{
		Metadata: api.TierMetadata{
			Name: "tier-2",
		},
		Spec: api.TierSpec{
			Order: 150
		},
	}

	// Delete a tier.  All Delete() methods return an error of type
	// common.ErrorResourceDoesNotExist if the resource specified by its
	// unique identifiers does not exist.
	tier, err = tiers.Delete(api.TierMetadata{
		Name: "tier-2",
	})

	// Get a tier.  All Get() methods return an error of type
	// common.ErrorResourceDoesNotExist if the resource specified by its
	// unique identifiers does not exist.
	tier, err = tiers.Get(api.TierMetadata{
		Name: "tier-2",
	})

	// List all tiers.  All List() methods take a (sub-)set of the resource
	// identifiers and return the corresponding list resource type that has an
	// Items field containing a list of resources that match the supplied
	// identifiers.
	tierList, err := tiers.List(api.TierMetadata{})

TODO:  The following is not yet implemented ... I need to add revision data to the
       front end.  It is included in the backend API though.

All Create(), Update(), Apply(), Get() and List() methods on each resource management
interface return instances of the appropriate resource type containing revision
metadata that can be used to provide atomic update operations on those resources.  For
example, the following will get a tier and perform an atomic update on that tier.

	// Get a tier - this contains revision information.
	tier, err = tiers.Get(api.TierMetadata{
		Name: "tier-2",
	})

	// Update the tier configuration.  If another application updates this tier
	// before we perform the update, the Update() method will return an error of
	// type common.ErrorResourceUpdateConflict.
	tier.Spec.Order = 222
	tier, err = tiers.Update(tier)
*/
package client
