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

package consul

import (
	"github.com/projectcalico/libcalico-go/lib/backend/model"
	"strings"
)

// Consul keys can't start from `/` because it is a separator. We'll need to remove it and
// restore later on read operations
func calicoPathToConsulPath(path string) string {
	return strings.TrimLeft(path, "/")
}

func keyToDefaultPath(key model.Key) (string, error) {
	path, err := model.KeyToDefaultPath(key)
	return calicoPathToConsulPath(path), err
}

// Trailing slashes are important in the recursive delete operation, since Consul performs
// a greedy match on the provided prefix. If you were to use "foo" as the key, this would
// recursively delete any key starting with those letters such as "foo", "food", and "football"
// not just "foo". To ensure you are deleting a 'folder', always use a trailing slash.
func consulPathToDeletePath(s string) string {
	if len(s) == 0 {
		return s
	}

	if s[len(s)-1:] != "/" {
		s = s + "/"
	}

	return s
}

func keyToDefaultDeleteTreePath(key model.Key) (string, error) {
	path, err := model.KeyToDefaultDeletePath(key)
	consulPath := calicoPathToConsulPath(path)

	return consulPathToDeletePath(consulPath), err
}

func keyToDefaultDeleteParentPaths(key model.Key) ([]string, error) {
	paths, err := model.KeyToDefaultDeleteParentPaths(key)
	for i := 0; i < len(paths); i++ {
		paths[i] = consulPathToDeletePath(calicoPathToConsulPath(paths[i]))
	}

	return paths, err
}

func listOptionsToDefaultPathRoot(listOptions model.ListInterface) string {
	return calicoPathToConsulPath(model.ListOptionsToDefaultPathRoot(listOptions))
}

func consulPathToCalicoPath(path string) string {
	return "/" + path
}

func keyFromDefaultPath(path string) model.Key {
	return model.KeyFromDefaultPath(consulPathToCalicoPath(path))
}

func keyFromDefaultListPath(path string, l model.ListInterface) model.Key {
	return l.KeyFromDefaultPath(consulPathToCalicoPath(path))
}
