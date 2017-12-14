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
Package upgrade implements a set of helper functionality for assisting with the
upgrade from the v1 data model to the v3 datamodel.  This package contains two
sub-packages.

The converters sub-package consists of the converter interfaces for each v1
resource type that allow the following mapping:
   v1 API Resource -> v1 backend model -> v3 API Resource

The migrator sub-package consists of a migration helper that can be used to
validate the data conversion will be successful, perform the actual data
migration (as part of an upgrade), and abort or complete the upgrade.
*/
package upgrade
