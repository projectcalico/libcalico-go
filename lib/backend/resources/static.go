// Copyright (c) 2020 Tigera, Inc. All rights reserved.

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

package resources

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v3 "github.com/projectcalico/libcalico-go/lib/apis/v3"
	"github.com/projectcalico/libcalico-go/lib/backend/model"
)

// This file contains helpers for creating hard-coded resources used in the Kubernetes backend.
const (
	// AllowProfileName is the name of an "allow-all" profile.
	AllowProfileName = "projectcalico-allow-all"
)

// AllowProfile returns a single profile kvp with default allow rules.
// Since KDD doesn't support creation of arbitrary profiles, this profile can be used
// for non-kubernetes endpoints (e.g. host endpoints) to allow traffic.
func AllowProfile() *model.KVPair {
	// Create the profile
	profile := v3.NewProfile()
	profile.ObjectMeta = metav1.ObjectMeta{
		Name: AllowProfileName,
	}
	profile.Spec = v3.ProfileSpec{
		Ingress: []v3.Rule{{Action: v3.Allow}},
		Egress:  []v3.Rule{{Action: v3.Allow}},
	}

	// Embed the profile in a KVPair.
	return &model.KVPair{
		Key: model.ResourceKey{
			Name: "allow",
			Kind: v3.KindProfile,
		},
		Value:    profile,
		Revision: "",
	}
}
