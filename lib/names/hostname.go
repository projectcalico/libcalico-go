// Copyright (c) 2016-2019 Tigera, Inc. All rights reserved.

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

package names

import (
	"os"
	"strings"
)

// Hostname returns this hosts hostname, converting to lowercase so that
// it is valid for use in the Calico API.
func Hostname() (string, error) {
	if h, err := os.Hostname(); err != nil {
		return "", err
	} else {
		return strings.ToLower(strings.TrimSpace(h)), nil
	}
}
