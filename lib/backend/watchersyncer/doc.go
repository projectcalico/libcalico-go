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

/*
Rawsyncer implements a generic syncer interface that can be used to sync from an
arbitrary set of Watchers.

The syncer returns data in the version as stored in the backend.

The implementation could easily be ported to work on the main client Watcher which
would be the preferred approach once all of the entries that we need to watch are
defined as resource types.

We implement this as a syncer rather than a "multi-watcher", because the syncer
does not require old state when sending a delete message, and since we have to maintain
a cache a syncer would have significantly reduced occupancy.
*/
package watchersyncer
