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

package store

type DriverStatus uint8

const (
	WaitForDatastore DriverStatus = iota
	ResyncInProgress
	InSync
)

type DriverConfiguration struct {
}

type Driver interface {
	Start()
	// ForceResync()
}

type Update struct {
	Key        string
	ValueOrNil *string
}

type Callbacks interface {
	OnConfigLoaded(globalConfig map[string]string, hostConfig map[string]string)
	OnStatusUpdated(status DriverStatus)
	OnKeysUpdated(updates []Update)
}

type DriverConstructor func(callbacks Callbacks, config *DriverConfiguration) (Driver, error)

func Register(name string, constructor DriverConstructor) {
	// TODO Implement driver registration
}
