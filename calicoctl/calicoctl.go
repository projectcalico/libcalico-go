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

package main

import (
	"fmt"
	"strings"

	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/docopt/docopt-go"
	"github.com/projectcalico/libcalico-go/calicoctl/commands"
)

func main() {
	usage := `Usage:
    calicoctl [options] <command> [<args>...]

    create         Create a resource by filename or stdin.
    replace        Replace a resource by filename or stdin.
    apply          Apply a resource by filename or stdin.  This creates a resource if
                   it does not exist, and replaces a resource if it does exists.
    delete         Delete a resource identified by file, stdin or resource type and name.
    get            Get a resource identified by file, stdin or resource type and name.
    version        Display the version of calicoctl.
    node           Node related commands.

Options:
  -h --help  Show this screen.
  -l --log-level=<level>  Set the log level (one of panic, fatal, error,
                         warn, info, debug) [default: panic]`
	var err error
	doc := commands.EtcdIntro + usage

	arguments, _ := docopt.Parse(doc, nil, true, "calicoctl", true, false)

	if logLevel := arguments["--log-level"]; logLevel != nil {
		parsedLogLevel, err := log.ParseLevel(logLevel.(string))
		if err != nil {
			fmt.Printf("Unknown log level: %s, expected one of: \n"+
				"panic, fatal, error, warn, info, debug.\n", logLevel)
			os.Exit(1)
		} else {
			log.SetLevel(parsedLogLevel)
			log.Infof("Log level set to %v", parsedLogLevel)
		}
	}

	if arguments["<command>"] != nil {
		command := arguments["<command>"].(string)
		args := append([]string{command}, arguments["<args>"].([]string)...)

		switch command {
		case "create":
			err = commands.Create(args)
		case "replace":
			err = commands.Replace(args)
		case "apply":
			err = commands.Apply(args)
		case "delete":
			err = commands.Delete(args)
		case "get":
			err = commands.Get(args)
		case "version":
			err = commands.Version(args)
		case "node":
			err = commands.Node(args)
		default:
			fmt.Println(usage)
		}

		if err != nil {
			fmt.Printf("Error executing command. Invalid option: 'calicoctl %s'. Use flag '--help' to read about a specific subcommand.\n", strings.Join(args, " "))
			os.Exit(1)
		}
	}

}
