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
	"errors"
	"net"

	"github.com/projectcalico/libcalico-go/lib/apiv2"
	"github.com/projectcalico/libcalico-go/lib/backend/model"
	"github.com/projectcalico/libcalico-go/lib/backend/watchersyncer"
	"github.com/projectcalico/libcalico-go/lib/names"
	cnet "github.com/projectcalico/libcalico-go/lib/net"
)

// Create a new SyncerUpdateProcessor to sync WorkloadEndpoint data in v1 format for
// consumption by Felix.
func NewWorkloadEndpointUpdateProcessor() watchersyncer.SyncerUpdateProcessor {
	return NewGeneralUpdateProcessor(apiv2.KindWorkloadEndpoint, convertWorkloadEndpointV2ToV1)
}

// Convert v2 KVPair to the equivalent v1 KVPair.
func convertWorkloadEndpointV2ToV1(kvp *model.KVPair) (*model.KVPair, error) {
	// Validate against incorrect key/value kinds.  This indicates a code bug rather
	// than a user error.
	v2key, ok := kvp.Key.(model.ResourceKey)
	if !ok || v2key.Kind != apiv2.KindWorkloadEndpoint {
		return nil, errors.New("Key is not a valid WorkloadEndpoint resource key")
	}

	if kvp.Value == nil {
		return nil, errors.New("Deletion attempted without enough information to form a v1 WorkloadEndpoint key")
	}

	v2res, ok := kvp.Value.(*apiv2.WorkloadEndpoint)
	if !ok {
		return nil, errors.New("Value is not a valid WorkloadEndpoint resource value")
	}

	v1key, err := convertWorkloadEndpointV2ToV1Key(v2res)
	if err != nil {
		return nil, err
	}

	v1value, err := convertWorkloadEndpointV2ToV1Value(v2res)
	if err != nil {
		// Currently ignore the error so that incorrect values get skipped instead of erroring out
		return &model.KVPair{
			Key: v1key,
		}, nil
	}

	return &model.KVPair{
		Key:      v1key,
		Value:    v1value,
		Revision: kvp.Revision,
	}, nil
}

func convertWorkloadEndpointV2ToV1Key(v2res *apiv2.WorkloadEndpoint) (model.WorkloadEndpointKey, error) {
	parts := names.ExtractDashSeparatedParms(v2res.GetName(), 4)
	if len(parts) != 4 {
		return model.WorkloadEndpointKey{}, errors.New("Not enough information provided to create v1 Workload Endpoint Key")
	}
	return model.WorkloadEndpointKey{
		Hostname:       parts[0],
		OrchestratorID: parts[1],
		WorkloadID:     v2res.GetNamespace() + "/" + parts[2],
		EndpointID:     parts[3],
	}, nil

}

func convertWorkloadEndpointV2ToV1Value(v2res *apiv2.WorkloadEndpoint) (*model.WorkloadEndpoint, error) {
	var v1value *model.WorkloadEndpoint
	// Deletion operations will have empty values so skip if empty
	if !v1FieldsEmpty(v2res) {
		var ipv4Nets []cnet.IPNet
		var ipv6Nets []cnet.IPNet
		for _, ipnString := range v2res.Spec.IPNetworks {
			_, ipn, err := cnet.ParseCIDROrIP(ipnString)
			if err != nil {
				return nil, err
			}
			ipnet := *(ipn.Network())
			if ipnet.Version() == 4 {
				ipv4Nets = append(ipv4Nets, ipnet)
			} else {
				ipv6Nets = append(ipv6Nets, ipnet)
			}
		}

		var ipv4NAT []model.IPNAT
		var ipv6NAT []model.IPNAT
		for _, ipnat := range v2res.Spec.IPNATs {
			nat := ConvertV2ToV1IPNAT(ipnat)
			if nat != nil {
				if nat.IntIP.Version() == 4 {
					ipv4NAT = append(ipv4NAT, *nat)
				} else {
					ipv6NAT = append(ipv6NAT, *nat)
				}
			}
		}

		var ipv4Gateway *cnet.IP
		var err error
		if v2res.Spec.IPv4Gateway != "" {
			ipv4Gateway, _, err = cnet.ParseCIDROrIP(v2res.Spec.IPv4Gateway)
			if err != nil {
				return nil, err
			}
		}

		var ipv6Gateway *cnet.IP
		if v2res.Spec.IPv6Gateway != "" {
			ipv6Gateway, _, err = cnet.ParseCIDROrIP(v2res.Spec.IPv6Gateway)
			if err != nil {
				return nil, err
			}
		}

		var cmac *cnet.MAC
		if v2res.Spec.MAC != "" {
			mac, err := net.ParseMAC(v2res.Spec.MAC)
			if err != nil {
				return nil, err
			}
			cmac = &cnet.MAC{mac}
		}

		// Convert the EndpointPort type from the API pkg to the v1 model equivalent type
		ports := []model.EndpointPort{}
		for _, port := range v2res.Spec.Ports {
			ports = append(ports, model.EndpointPort{
				Name:     port.Name,
				Protocol: port.Protocol,
				Port:     port.Port,
			})
		}

		v1value = &model.WorkloadEndpoint{
			State:            "active",
			Name:             v2res.Spec.InterfaceName,
			ActiveInstanceID: v2res.Spec.ContainerID,
			Mac:              cmac,
			ProfileIDs:       v2res.Spec.Profiles,
			IPv4Nets:         ipv4Nets,
			IPv6Nets:         ipv6Nets,
			IPv4NAT:          ipv4NAT,
			IPv6NAT:          ipv6NAT,
			Labels:           v2res.GetLabels(),
			IPv4Gateway:      ipv4Gateway,
			IPv6Gateway:      ipv6Gateway,
			Ports:            ports,
		}
	}

	return v1value, nil
}

func v1FieldsEmpty(v2res *apiv2.WorkloadEndpoint) bool {
	empty := true
	empty = empty && v2res.Spec.InterfaceName == ""
	empty = empty && len(v2res.Spec.IPNetworks) == 0
	empty = empty && len(v2res.Spec.IPNATs) == 0
	empty = empty && v2res.Spec.IPv4Gateway == ""
	empty = empty && v2res.Spec.IPv6Gateway == ""
	empty = empty && v2res.Spec.MAC == ""
	empty = empty && v2res.Spec.InterfaceName == ""
	empty = empty && v2res.Spec.ContainerID == ""
	empty = empty && len(v2res.Spec.Profiles) == 0
	empty = empty && len(v2res.GetLabels()) == 0
	empty = empty && len(v2res.Spec.Ports) == 0
	return empty
}

func ConvertV2ToV1IPNAT(ipnat apiv2.IPNAT) *model.IPNAT {
	internalip := cnet.ParseIP(ipnat.InternalIP)
	externalip := cnet.ParseIP(ipnat.ExternalIP)
	if internalip != nil && externalip != nil {
		return &model.IPNAT{
			IntIP: *internalip,
			ExtIP: *externalip,
		}
	}
	return nil
}
