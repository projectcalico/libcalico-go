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
parser contains three sub-packages which are based of the following three
repos:

	-  githib.com/go-yaml/yaml (yaml.v2)
	-  github.com/ghodss/yaml
	-  encoding/json

The yaml.v2 is modified to unmarshal a float as 64-bits rather than 32.  This
was a known limitation in the existing package.  The modification was somewhat
simplistic, but appropriate for calicoctl that does not need to support Float32
data types.

The json package is a modified version of the go-lang encoding/json package.  It
includes a patch that has been provided by michael.m.spiegel which can be found
at https://go-review.googlesource.com/#/c/27231/.  The patch has not yet been
approved, but this or a similar fix is likely to make it into the core
package (it has often been requested).  The fix allows for strict field matching
- rejecting a request when a field in the JSON does not match any fields in the
supplied structure.

The ghodss yaml package is modified, also to use 64-bit floats, and to reference
the modified yaml.v2 and json packages.
*/
package parser
