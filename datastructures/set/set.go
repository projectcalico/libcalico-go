// Copyright (c) 2016 Tigera, Inc. All rights reserved.
//
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

package set

import "errors"

type Set interface {
	Len() int
	Add(interface{})
	Discard(interface{})
	Contains(interface{}) bool
	Iter(func(item interface{}) error)
}

var (
	StopIteration = errors.New("Stop iteration")
)

func New() Set {
	return make(mapSet)
}

type mapSet map[interface{}]struct{}

func (set mapSet) Len() int {
	return len(set)
}

func (set mapSet) Add(item interface{}) {
	set[item] = struct{}{}
}

func (set mapSet) Discard(item interface{}) {
	delete(set, item)
}

func (set mapSet) Contains(item interface{}) bool {
	_, present := set[item]
	return present
}

func (set mapSet) Iter(visitor func(item interface{}) error) {
	for item, _ := range set {
		err := visitor(item)
		if err == StopIteration {
			break
		}
	}
}
