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

package scope

import (
	"encoding/json"
	"fmt"
)

type Scope int

const (
	Undefined = iota
	Global
	Node
)

const (
	UndefinedStr = "undefined"
	GlobalStr    = "global"
	NodeStr      = "node"
)

// UnmarshalJSON implements the json.Unmarshaller interface.
func (f *Scope) UnmarshalJSON(b []byte) error {
	var value string
	err := json.Unmarshal(b, &value)
	if err != nil {
		return err
	}

	switch value {
	case UndefinedStr:
		*f = Undefined
	case GlobalStr:
		*f = Global
	case NodeStr:
		*f = Node
	default:
		return fmt.Errorf("Unrecognised scope value '%s'", value)
	}

	return nil
}

// String returns a friendly string value for this scope.
func (f *Scope) String() string {
	switch *f {
	case Undefined:
		return "scope:undefined"
	case Global:
		return "scope:global"
	case Node:
		return "scope:node"
	default:
		return "scope:unknown"
	}
}

// MarshalJSON implements the json.Marshaller interface.
func (f Scope) MarshalJSON() ([]byte, error) {
	switch f {
	case Undefined:
		return json.Marshal(UndefinedStr)
	case Global:
		return json.Marshal(GlobalStr)
	case Node:
		return json.Marshal(NodeStr)
	default:
		return nil, fmt.Errorf("unknown scope value `%d`", f)
	}
}
