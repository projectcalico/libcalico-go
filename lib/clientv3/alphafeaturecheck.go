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

package clientv3

import (
	"errors"
	"fmt"

	"github.com/projectcalico/libcalico-go/lib/apiconfig"
	apiv3 "github.com/projectcalico/libcalico-go/lib/apis/v3"
)

// checkAlphaFeatures is used to check if GNP and NP rules are compatible with
// alphaFeatures flag 
func checkAlphaFeatures(alphaConfig string, ingress, egress []apiv3.Rule) error {
	if !apiconfig.IsAlphaFeatureSet(alphaConfig, apiconfig.AlphaFeatureSA) {
		errS := fmt.Sprintf("invalid alpha feature %s used", apiconfig.AlphaFeatureSA)
		for _, rule := range ingress {
			if rule.Source.ServiceAccounts != nil ||
				rule.Destination.ServiceAccounts != nil {
				return errors.New(errS)
			}
		}

		for _, rule := range egress {
			if rule.Source.ServiceAccounts != nil ||
				rule.Destination.ServiceAccounts != nil {
				return errors.New(errS)
			}
		}
	}
	return nil
}
