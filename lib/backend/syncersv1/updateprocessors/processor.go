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

package updateprocessors

import (
	"fmt"

	"github.com/projectcalico/libcalico-go/lib/backend/model"
	"github.com/projectcalico/libcalico-go/lib/backend/watchersyncer"
)

// NewGeneralUpdateProcessor implements an update processor that only needs to take in
// a conversion function. This is only meant to be used in cases where the Process and
// OnSyncerStarting methods do not have any special handling.
//
// One of the main differences between the general update processor and the conflict
// resolving name cache processor is that the conflict resolving name cache processor
// handles deletes by inspecting the value in the key-value pair. The general update processor
// will strictly convert the information it is given, whether the value is nil or not.

func NewGeneralUpdateProcessor(v2Kind string, converter ConvertV2ToV1) watchersyncer.SyncerUpdateProcessor {
	return &generalUpdateProcessor{
		v2Kind:    v2Kind,
		converter: converter,
	}
}

type generalUpdateProcessor struct {
	v2Kind    string
	converter ConvertV2ToV1
}

func (gup *generalUpdateProcessor) Process(kvp *model.KVPair) ([]*model.KVPair, error) {
	// Extract the name of the resource.
	rk, ok := kvp.Key.(model.ResourceKey)
	if !ok || rk.Kind != gup.v2Kind {
		return nil, fmt.Errorf("Incorrect key type - expecting resource of kind %s", gup.v2Kind)
	}

	// Convert the v2 resource to the equivalent v1 resource type.
	var response []*model.KVPair
	kvp, err := gup.converter(kvp)
	if err != nil {
		return response, err
	}

	response = append(response, kvp)
	return response, nil
}

func (gup *generalUpdateProcessor) OnSyncerStarting() {
	// Do nothing
}
