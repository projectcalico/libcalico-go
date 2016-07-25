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
TODO:  WIP

Package backend implements the backend data store client and associated backend data types.

Backend client interface

The client interface provides a number of methods to access, modify and list objects in the
datastore.  The raw format of the data in the datastore is not exposed over this interface.
Instead backend data structures are returned from these methods and the client handles all
of the necessary marshaling and unmarshaling of the data.

The client defines a number of key structures and interfaces:
	-  DatastoreObject encapsulates a datastore resource, its key (i.e. its fully qualified
	   identification and its revision.  The object stored in the DatastoreObject is of
	   type interface{}, but should match the equivalent type for the key (more on this below).
	-  KeyInterface is used by the client to construct a key (appropriate for the specific
	   datastore) for a particular resource.  This is used to both lookup or modify an
	   existing resource, delete an existing resource, or create a new resource.  Each specific
	   KeyInterface is tied to a specific resource type.
	-  ListInterface is used by the client to enumerate a set of resources.  As with the
	   KeyInterface, a specific ListInterface is tied to a specific resource type.
	-  Backend resource types are defined for each type of resource that can be stored in the
	   datastore (e.g. HostEndpoint, Policy, Tier).  For each specific resource type, there
	   is a corresponding KeyInterface and ListInterface defined.  For example, a Tier resource
	   type has a corresponding TierKey defined (which implements the KeyInterface).  If you
	   use a TierKey to get a resource from the client, the DatastoreObject returned by the
	   client will contain a Tier object, and the Object field may be safely cast to a Tier type.

To create a new instance of the backend client using the NewClient constructor.

	config = api.ClientConfig{}  // Set appropriate access details
	client = NewClient(config *api.ClientConfig)

Create a new resource

To create a new resource in the datastore, use the client.Create() method.  This errors if
a resource with the same identifiers (key) already exists.

Datastructures

The backend client defines a DatastoreObject type that encapsulates details about an
object key (this is a KeyInterface  and is used to uniquely identify an object in the store),
the object itself (an interface{}, which should be JSON serializable) and a revision
(an interface{} used internally to handle atomic updates).

The client interface does not expose underlying details of the actual backend datastore
(e.g. etcd) other than in the configuration parameters used to connect the datastore.



*/
package backend
