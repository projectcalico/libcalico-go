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
	"github.com/tigera/libcalico-go/lib/scope"

	"fmt"

	"github.com/golang/glog"
	"github.com/tigera/libcalico-go/lib/component"
)

func Config(args []string) error {
	doc := EtcdIntro + `Manage system configuration parameters.

Usage:
  calicoctl config set <NAME> <VALUE>
      [--scope=<SCOPE>] [--component=<COMPONENT] [--hostname=<HOSTNAME>] [--config=<CONFIG>]
  calicoctl config unset <NAME> <VALUE>
      [--scope=<SCOPE>] [--component=<COMPONENT] [--hostname=<HOSTNAME>] [--config=<CONFIG>]
  calicoctl config show [<NAME>]
      [--scope=<SCOPE>] [--component=<COMPONENT] [--hostname=<HOSTNAME>] [--config=<CONFIG>]

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
assist with certain debug or low level operations and should only be used when instructed.

 Name                | Component | Scope       | Value                                  | Unset value
---------------------+-----------+-------------+----------------------------------------+-------------
 logLevel            | bgp       | global,node | none,debug,info                        | -
                     | felix     | global,node | none,debug,info,warning,error,critical | -
 nodeToNodeMesh      | bgp       | global      | on,off                                 | on
 defaultNodeASNumber | bgp       | global      | 0-4294967295                           | 64511
 ipip                | felix     | global      | on,off                                 | -

Examples:
  # Turn off the full BGP node-to-node mesh
  calicoctl config set nodeToNodeMesh off

  # Set BGP log level to info for node with hostname "host1"
  calicoctl config set logLevel info --scope=node --component=bgp --hostname=host1

  # Display the full set of config values
  calicoctl config show

Options:
  -n --hostname=<HOSTNAME>     The hostname.
  --scope=<SCOPE>              The scope of the resource type.  One of global, node.
  --component=<COMPONENT>      The component.  One of bgp, felix.
  -c --config=<CONFIG>         Filename containing connection configuration in YAML or JSON format.
                               [default: /etc/calico/calicoctl.cfg]
`
	parsedArgs, err := docopt.Parse(doc, args, true, "calicoctl", false, false)
	if err != nil {
		return err
	}
	if len(parsedArgs) == 0 {
		return nil
	}

	// Load the client config and connect.
	cf := parsedArgs["--config"].(string)
	client, err := newClient(cf)
	if err != nil {
		return err
	}
	glog.V(2).Infof("Client: %v\n", client)

	// From the command line arguments construct the Config object to send to the client.
	hostname := parsedArgs["--hostname"]
	scopeStr := parsedArgs["--scope"]
	componentStr := parsedArgs["--component"]
	name := parsedArgs["<NAME>"]
	value := parsedArgs["<VALUE>"]

	config := api.NewConfig()
	config.Metadata.Hostname = hostname
	config.Metadata.Scope = scope.Scope(scopeStr)
	config.Metadata.Component = component.Component(componentStr)
	config.Metadata.Name = name
	config.Spec.Value = value

	var configList *api.ConfigList
	if parsedArgs["set"] != nil {
		_, err = client.Config().Set(config)
	} else if parsedArgs["unset"] != nil {
		err = client.Config().Unset(config.Metadata)
	} else {
		configList, err = client.Config().List(config.Metadata)
		if err != nil {

		}
		fmt.Println(configList)
	}

	return err
}
