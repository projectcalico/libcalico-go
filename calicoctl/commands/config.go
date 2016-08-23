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

package commands

import (
	"github.com/docopt/docopt-go"
	"github.com/tigera/libcalico-go/lib/api"
	"github.com/tigera/libcalico-go/lib/client"

	"fmt"

	"github.com/ghodss/yaml"
	"github.com/golang/glog"
	"github.com/tigera/libcalico-go/lib/api/unversioned"
)

func Config(args []string) error {
	doc := EtcdIntro + `Manage system configuration parameters.

Usage:
  calicoctl config set <NAME> <VALUE> [--scope=<SCOPE>] [--component=<COMPONENT] [--hostname=<HOSTNAME>] [--raw]
  calicoctl config unset <NAME> <VALUE> [--scope=<SCOPE>] [--hostname=<HOSTNAME>] [--raw]
  calicoctl config show [<NAME>] [--scope=<SCOPE>] [--hostname=<HOSTNAME>] [--raw]

These commands can be used to manage system level configuration.  The table below details the
valid config names and values, and for each specifies the valid scope and component.  The scope
indicates whether the config applies at a global or node-specific scope.  The component indicates
a specific component in the Calico architecture.  If a config option is only valid for a single
scope then the scope need not be explicitly specified in the command; similarly for the component
option.  If the scope is set to 'node' then the hostname must be specified for the set and unset
commands.

The unset command reverts configuration back to its initial value.  Depending on the configuration
option, this either deletes the configuration completely from the datastore, or resets it to the
original system default value.

The '--raw' option allows users to set arbitrary configuration options for a particular scope and
component.  The component and scope are required when using the '--raw' option on the set and unset
commands.  In general we do not recommend use of the '--raw' option - it is there primarily to
assist with certain debug or low level operations, and should only be used when instructed.

 Name                | Component | Scope       | Value                                  | Unset value
---------------------+-----------+-------------+----------------------------------------+-------------
 logLevel            | bgp       | global,node | none,debug,info                        | -
                     | felix     | global,node | none,debug,info,warning,error,critical | -
 nodeToNodeMesh      | bgp       | global      | on,off                                 | off
 defaultNodeASNumber | bgp       | global      | 0-4294967295                           | 64511

Examples:
  # Turn off the full BGP node-to-node mesh
  calicoctl config set nodeToNodeMesh off

  # List a specific policy in YAML format
  calicoctl get -o yaml policy my-policy-1

Options:
  -n --hostname=<HOSTNAME>     The hostname.
  --scope=<SCOPE>              The scope of the resource type.  One of global, node.
  --component=<COMPONENT>      The component.  One of bgp, felix.
  --raw                        Operate on raw key and values - no consistency checks or data
                               validation are performed.
`
	parsedArgs, err := docopt.Parse(doc, args, true, "calicoctl", false, false)
	if err != nil {
		return err
	}
	if len(parsedArgs) == 0 {
		return nil
	}

	cmd := get{}
	results := executeConfigCommand(parsedArgs, cmd)
	glog.V(2).Infof("results: %+v", results)

	if results.err != nil {
		fmt.Printf("Error getting resources: %v\n", results.err)
		return err
	}

	// TODO Handle better - results should be groups as per input file
	// For simplicity convert the returned list of resources to expand any lists
	resources := convertToSliceOfResources(results.resources)

	if output, err := yaml.Marshal(resources); err != nil {
		fmt.Printf("Error outputing data: %v", err)
	} else {
		fmt.Printf("%s", string(output))
	}

	return nil
}

// commandInterface for replace command.
// Maps the generic resource types to the typed client interface.
type get struct {
}

func (g get) execute(client *client.Client, resource unversioned.Resource) (unversioned.Resource, error) {
	var err error
	switch r := resource.(type) {
	case api.HostEndpoint:
		resource, err = client.HostEndpoints().List(r.Metadata)
	case api.Policy:
		resource, err = client.Policies().List(r.Metadata)
	case api.Pool:
		resource, err = client.Pools().List(r.Metadata)
	case api.Profile:
		resource, err = client.Profiles().List(r.Metadata)
	case api.Tier:
		resource, err = client.Tiers().List(r.Metadata)
	case api.WorkloadEndpoint:
		resource, err = client.WorkloadEndpoints().List(r.Metadata)
	case api.BGPPeer:
		resource, err = client.BGPPeers().List(r.Metadata)
	default:
		panic(fmt.Errorf("Unhandled resource type: %v", resource))
	}

	return resource, err
}
