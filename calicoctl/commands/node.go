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
	"fmt"
	"os"
	"strings"

	"github.com/docopt/docopt-go"
	"github.com/projectcalico/libcalico-go/calicoctl/commands/node"
)

// Node function is a switch to node related sub-commands
func Node(args []string) error {
	var err error
	doc := `Usage: 
	calicoctl node status 
	calicoctl node diags [--log-dir=<LOG_DIR>]
	calicoctl node checksystem

Options:
    --help                  Show this screen.
    status                  Shows the status of the node.
    diags                   Collects diagnostic information.
    --log-dir=<LOG_DIR>     The directory for logs [default: /var/log/calico]
    checksystem             Check for compatibility with the host system.
	
Description:
  Node specific commands for calicoctl
  
  See 'calicoctl node --help' to read about a specific subcommand.
  `

	arguments, err := docopt.Parse(doc, args, true, "calicoctl", false, false)
	if err != nil {
		return err
	}

	// If `--help` or `-h` is passed, then arguments map will be empty
	if len(arguments) > 0 {
		logDir := append([]string{"diags"}, arguments["--log-dir"].(string))

		// arguments["status"] is a bool and it's true when `calicoctl node status`
		// is passed, false when status is not present
		if arguments["status"].(bool) {
			err = node.Status()
		} else if arguments["diags"].(bool) {
			err = node.Diags(logDir)
		} else if arguments["checksystem"].(bool) {
			err = node.Checksystem()
		} else {
			fmt.Printf("Invalid option.\n")
			fmt.Println(doc)
		}

		if err != nil {
			fmt.Printf("Error executing command. Invalid option: 'calicoctl %s'. Use flag '--help' to read about a specific subcommand.\n", strings.Join(args, " "))
			os.Exit(1)
		}
	}

	return nil
}
