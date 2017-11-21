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

package clientv2

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/projectcalico/libcalico-go/lib/apiconfig"
	apiv2 "github.com/projectcalico/libcalico-go/lib/apis/v2"
	"github.com/projectcalico/libcalico-go/lib/options"
	"github.com/projectcalico/libcalico-go/lib/watch"
)

// GlobalNetworkPolicyInterface has methods to work with GlobalNetworkPolicy resources.
type GlobalNetworkPolicyInterface interface {
	Create(ctx context.Context, res *apiv2.GlobalNetworkPolicy, opts options.SetOptions) (*apiv2.GlobalNetworkPolicy, error)
	Update(ctx context.Context, res *apiv2.GlobalNetworkPolicy, opts options.SetOptions) (*apiv2.GlobalNetworkPolicy, error)
	Delete(ctx context.Context, name string, opts options.DeleteOptions) (*apiv2.GlobalNetworkPolicy, error)
	Get(ctx context.Context, name string, opts options.GetOptions) (*apiv2.GlobalNetworkPolicy, error)
	List(ctx context.Context, opts options.ListOptions) (*apiv2.GlobalNetworkPolicyList, error)
	Watch(ctx context.Context, opts options.ListOptions) (watch.Interface, error)
}

// globalnetworkpolicies implements GlobalNetworkPolicyInterface
type globalnetworkpolicies struct {
	client client
}

// Create takes the representation of a GlobalNetworkPolicy and creates it.  Returns the stored
// representation of the GlobalNetworkPolicy, and an error, if there is any.
func (r globalnetworkpolicies) Create(ctx context.Context, res *apiv2.GlobalNetworkPolicy, opts options.SetOptions) (*apiv2.GlobalNetworkPolicy, error) {
	defaultPolicyTypesField(res.Spec.Ingress, res.Spec.Egress, &res.Spec.Types)

	// Check if it is permitted to create the network policy with the specific alpha feature support.
	if err := r.checkAlphaFeatures(res); err != nil {
		return nil, err
	}

	// Properly prefix the name
	res.GetObjectMeta().SetName(convertPolicyNameForStorage(res.GetObjectMeta().GetName()))
	out, err := r.client.resources.Create(ctx, opts, apiv2.KindGlobalNetworkPolicy, res)
	if out != nil {
		// Remove the prefix out of the returned policy name.
		out.GetObjectMeta().SetName(convertPolicyNameFromStorage(out.GetObjectMeta().GetName()))
		return out.(*apiv2.GlobalNetworkPolicy), err
	}

	// Remove the prefix out of the returned policy name.
	res.GetObjectMeta().SetName(convertPolicyNameFromStorage(res.GetObjectMeta().GetName()))
	return nil, err
}

// Update takes the representation of a GlobalNetworkPolicy and updates it. Returns the stored
// representation of the GlobalNetworkPolicy, and an error, if there is any.
func (r globalnetworkpolicies) Update(ctx context.Context, res *apiv2.GlobalNetworkPolicy, opts options.SetOptions) (*apiv2.GlobalNetworkPolicy, error) {
	defaultPolicyTypesField(res.Spec.Ingress, res.Spec.Egress, &res.Spec.Types)

	// Check if it is permitted to create the network policy with the specific alpha feature support.
	if err := r.checkAlphaFeatures(res); err != nil {
		return nil, err
	}

	// Properly prefix the name
	res.GetObjectMeta().SetName(convertPolicyNameForStorage(res.GetObjectMeta().GetName()))
	out, err := r.client.resources.Update(ctx, opts, apiv2.KindGlobalNetworkPolicy, res)
	if out != nil {
		// Remove the prefix out of the returned policy name.
		out.GetObjectMeta().SetName(convertPolicyNameFromStorage(out.GetObjectMeta().GetName()))
		return out.(*apiv2.GlobalNetworkPolicy), err
	}

	// Remove the prefix out of the returned policy name.
	res.GetObjectMeta().SetName(convertPolicyNameFromStorage(res.GetObjectMeta().GetName()))
	return nil, err
}

// Delete takes name of the GlobalNetworkPolicy and deletes it. Returns an error if one occurs.
func (r globalnetworkpolicies) Delete(ctx context.Context, name string, opts options.DeleteOptions) (*apiv2.GlobalNetworkPolicy, error) {
	out, err := r.client.resources.Delete(ctx, opts, apiv2.KindGlobalNetworkPolicy, noNamespace, convertPolicyNameForStorage(name))
	if out != nil {
		// Remove the prefix out of the returned policy name.
		out.GetObjectMeta().SetName(convertPolicyNameFromStorage(out.GetObjectMeta().GetName()))
		return out.(*apiv2.GlobalNetworkPolicy), err
	}
	return nil, err
}

// Get takes name of the GlobalNetworkPolicy, and returns the corresponding GlobalNetworkPolicy object,
// and an error if there is any.
func (r globalnetworkpolicies) Get(ctx context.Context, name string, opts options.GetOptions) (*apiv2.GlobalNetworkPolicy, error) {
	out, err := r.client.resources.Get(ctx, opts, apiv2.KindGlobalNetworkPolicy, noNamespace, convertPolicyNameForStorage(name))
	if out != nil {
		// Remove the prefix out of the returned policy name.
		out.GetObjectMeta().SetName(convertPolicyNameFromStorage(out.GetObjectMeta().GetName()))
		return out.(*apiv2.GlobalNetworkPolicy), err
	}
	return nil, err
}

// List returns the list of GlobalNetworkPolicy objects that match the supplied options.
func (r globalnetworkpolicies) List(ctx context.Context, opts options.ListOptions) (*apiv2.GlobalNetworkPolicyList, error) {
	res := &apiv2.GlobalNetworkPolicyList{}
	if err := r.client.resources.List(ctx, opts, apiv2.KindGlobalNetworkPolicy, apiv2.KindGlobalNetworkPolicyList, res); err != nil {
		return nil, err
	}

	// Remove the prefix off of each policy name
	for i, _ := range res.Items {
		name := res.Items[i].GetObjectMeta().GetName()
		res.Items[i].GetObjectMeta().SetName(convertPolicyNameFromStorage(name))
	}

	return res, nil
}

// Watch returns a watch.Interface that watches the globalnetworkpolicies that match the
// supplied options.
func (r globalnetworkpolicies) Watch(ctx context.Context, opts options.ListOptions) (watch.Interface, error) {
	return r.client.resources.Watch(ctx, opts, apiv2.KindGlobalNetworkPolicy)
}

func (r globalnetworkpolicies) checkAlphaFeatures(res *apiv2.GlobalNetworkPolicy) error {
	if r.client.config.Spec.AlphaFeatures.Get(apiconfig.AlphaFeatureSA) == false {
		errS := fmt.Sprintf("Global NP %s invalid alpha feature %s used.", res.GetObjectMeta().GetName(), apiconfig.AlphaFeatureSA)
		for _, rule := range res.Spec.Ingress {
			if rule.Source.ServiceAccounts != nil ||
                           rule.Destination.ServiceAccounts != nil {
				return errors.New(errS)
			}
		}

		for _, rule := range res.Spec.Egress {
			if rule.Source.ServiceAccounts != nil ||
                           rule.Destination.ServiceAccounts != nil {
				return errors.New(errS)
			}
		}
	}

	return nil
}

func defaultPolicyTypesField(ingressRules, egressRules []apiv2.Rule, types *[]apiv2.PolicyType) {
	if len(*types) == 0 {
		// Default the Types field according to what inbound and outbound rules are present
		// in the policy.
		if len(egressRules) == 0 {
			// Policy has no egress rules, so apply this policy to ingress only.  (Note:
			// intentionally including the case where the policy also has no ingress
			// rules.)
			*types = []apiv2.PolicyType{apiv2.PolicyTypeIngress}
		} else if len(ingressRules) == 0 {
			// Policy has egress rules but no ingress rules, so apply this policy to
			// egress only.
			*types = []apiv2.PolicyType{apiv2.PolicyTypeEgress}
		} else {
			// Policy has both ingress and egress rules, so apply this policy to both
			// ingress and egress.
			*types = []apiv2.PolicyType{apiv2.PolicyTypeIngress, apiv2.PolicyTypeEgress}
		}
	}
}

func convertPolicyNameForStorage(name string) string {
	// Do nothing on names prefixed with "knp."
	if strings.HasPrefix(name, "knp.") {
		return name
	}
	return "default." + name
}

func convertPolicyNameFromStorage(name string) string {
	// Do nothing on names prefixed with "knp."
	if strings.HasPrefix(name, "knp.") {
		return name
	}
	parts := strings.SplitN(name, ".", 2)
	return parts[len(parts)-1]
}
