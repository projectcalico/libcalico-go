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

	"fmt"

	"github.com/golang/glog"
)

func Delete(args []string) error {
	doc := EtcdIntro + `Delete a resource identified by file, stdin or resource type and name.

Usage:
  calicoctl delete (([--tier=<TIER>] [--hostname=<HOSTNAME>] [--scope=<SCOPE>] <KIND> <NAME>) |
                    --filename=<FILE>)
                   [--skip-not-exists] [--config=<CONFIG>]

Examples:
  # Delete a policy using the type and name specified in policy.yaml.
  calicoctl delete -f ./policy.yaml

  # Delete a policy based on the type and name in the YAML passed into stdin.
  cat policy.yaml | calicoctl delete -f -

  # Delete policy in the default tier with name "foo"
  calicoctl delete policy foo

  # Delete policy in the tier "bar" with name "foo"
  calicoctl delete policy --tier=bar foo

Options:
  -s --skip-not-exists         Skip over and treat as successful, resources that don't exist.
  -f --filename=<FILENAME>     Filename to use to delete the resource.  If set to "-" loads from stdin.
  -t --tier=<TIER>             The policy tier.
  -n --hostname=<HOSTNAME>     The hostname.
  -c --config=<CONFIG>         Filename containing connection configuration in YAML or JSON format.
                               [default: /etc/calico/calicoctl.cfg]
  --scope=<SCOPE>              The scope of the resource type.  One of global, node.  This is only valid
                               for BGP peers and is used to indicate whether the peer is a global peer
                               or node-specific.
`
	parsedArgs, err := docopt.Parse(doc, args, true, "calicoctl", false, false)
	if err != nil {
		return err
	}
	if len(parsedArgs) == 0 {
		return nil
	}

	results := executeConfigCommand(parsedArgs, actionDelete)
	glog.V(2).Infof("results: %+v", results)

	if results.fileInvalid {
		fmt.Printf("Error processing input file: %v\n", results.err)
	} else if results.numHandled == 0 {
		if results.numResources == 0 {
			fmt.Printf("No resources specified in file\n")
		} else if results.numResources == 1 {
			fmt.Printf("Failed to delete '%s' resource: %v\n", results.singleKind, results.err)
		} else if results.singleKind != "" {
			fmt.Printf("Failed to delete any '%s' resources: %v\n", results.singleKind, results.err)
		} else {
			fmt.Printf("Failed to delete any resources: %v\n", results.err)
		}
	} else if results.err == nil {
		if results.singleKind != "" {
			fmt.Printf("Successfully deleted %d '%s' resource(s)\n", results.numHandled, results.singleKind)
		} else {
			fmt.Printf("Successfully deleted %d resource(s)\n", results.numHandled)
		}
	} else {
		fmt.Printf("Partial success: ")
		if results.singleKind != "" {
			fmt.Printf("deleted the first %d out of %d '%s' resources:\n",
				results.numHandled, results.numResources, results.singleKind)
		} else {
			fmt.Printf("deleted the first %d out of %d resources:\n",
				results.numHandled, results.numResources)
		}
		fmt.Printf("Hit error: %v\n", results.err)
	}

	return results.err
}
