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

	"github.com/docopt/docopt-go"
)

// Node function is a switch to node related sub-commands
func Node(args []string) error {
	var err error
	doc := `Usage: calicoctl node <command> [<args>...]

    ststus          Shows the status of the node. 

Description:
  Node specific commands for calicoctl
  
  See 'calicoctl node --help' to read about a specific subcommand.`

	arguments, _ := docopt.Parse(doc, args, true, "calicoctl", false, false)

	if arguments["<command>"] != nil {
		command := arguments["<command>"].(string)
		args := append([]string{command}, arguments["<args>"].([]string)...)

		switch command {
		case "status":
			err = Status(args)
		default:
			fmt.Println(doc)
		}
	}
	if err != nil {
		fmt.Printf("Error executing command: %s\n", err)
		os.Exit(1)
	}

	return nil
}
