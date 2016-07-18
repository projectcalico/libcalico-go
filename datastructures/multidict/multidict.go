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

package multidict

type StringToString map[string]map[string]bool

func NewStringToString() StringToString {
	sToS := make(StringToString)
	return sToS
}

func (s2s StringToString) Put(key, value string) {
	set, ok := s2s[key]
	if !ok {
		set = make(map[string]bool)
		s2s[key] = set
	}
	set[value] = true
}

func (s2s StringToString) Discard(key, value string) {
	set, ok := s2s[key]
	if !ok {
		return
	}
	delete(set, value)
	if len(set) == 0 {
		delete(s2s, key)
	}
}

func (s2s StringToString) Contains(key, value string) bool {
	set, ok := s2s[key]
	return ok && set[value]
}

func (s2s StringToString) ContainsKey(key string) bool {
	_, ok := s2s[key]
	return ok
}

func (s2s StringToString) Iter(key string, f func(value string)) {
	for value, _ := range s2s[key] {
		f(value)
	}
}

type IfaceToIface map[interface{}]map[interface{}]bool

func NewIfaceToIface() IfaceToIface {
	sToS := make(IfaceToIface)
	return sToS
}

func (i2i IfaceToIface) Put(key, value interface{}) {
	set, ok := i2i[key]
	if !ok {
		set = make(map[interface{}]bool)
		i2i[key] = set
	}
	set[value] = true
}

func (i2i IfaceToIface) Discard(key, value interface{}) {
	set, ok := i2i[key]
	if !ok {
		return
	}
	delete(set, value)
	if len(set) == 0 {
		delete(i2i, key)
	}
}

func (i2i IfaceToIface) Contains(key, value interface{}) bool {
	set, ok := i2i[key]
	return ok && set[value]
}

func (i2i IfaceToIface) ContainsKey(key interface{}) bool {
	_, ok := i2i[key]
	return ok
}

func (i2i IfaceToIface) Iter(key interface{}, f func(value interface{})) {
	for value, _ := range i2i[key] {
		f(value)
	}
}
