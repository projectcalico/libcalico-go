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
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"regexp"
	"strings"

	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
	"github.com/docopt/docopt-go"

	"github.com/olekukonko/tablewriter"
)

var ipv4 = true

// Status prings status of the node and returns error (if any)
func Status(args []string) error {
	doc := `Usage:
calicoctl node status

Description:
  Display the status of the calico node`

	_, _ = docopt.Parse(doc, args, true, "calicoctl", false, false)

	ctx := context.Background()

	defaultHeaders := map[string]string{"User-Agent": "engine-api-cli-1.0"}
	cli, err := client.NewClient("unix:///var/run/docker.sock", "v1.22", nil, defaultHeaders)
	if err != nil {
		panic(err)
	}

	options := types.ContainerListOptions{All: true}
	containers, err := cli.ContainerList(ctx, options)
	if err != nil {
		panic(err)
	}

	for _, c := range containers {
		if c.Names[0] == "/calico-node" && c.State == "running" {

			fmt.Printf("calico-node container is running. Status: %s\n", c.Status)

			// config := types.ExecConfig{
			// 	User:         "root",
			// 	AttachStdin:  true,
			// 	AttachStderr: true,
			// 	AttachStdout: true,
			// 	//Detach:       true,
			// 	Cmd: []string{"sh", "-c", "cat libraries.txt"},
			// }

			// exec, err := cli.ContainerExecCreate(context.Background(), c.ID, config)
			// if err != nil {
			// 	fmt.Printf("failed to create exec: %v", err)
			// 	break
			// }

			// reply, err := cli.ContainerExecAttach(context.Background(), exec.ID, config)
			// if err != nil {
			// 	fmt.Printf("failed to attach to the container: %v", err)
			// 	break
			// }
			// defer reply.Close()

			// b := make([]byte, 255)

			// _, err = reply.Reader.Read(b)
			// if err != nil {
			// 	fmt.Printf("failed to exececute command inside the container: %v", err)
			// 	break
			// }

			//
			// re, err := regexp.Compile(`calico\s(.*)\n`)
			// if err != nil {
			// 	fmt.Printf("failed to compile regex: %v", err)
			// 	break
			// }

			// parts := strings.Fields(re.FindString(string(b)))
			// fmt.Println(parts)

			// if len(parts) == 2 {
			// 	felixVersion := parts[1][1 : len(parts[1])-1]

			// 	fmt.Printf("Running felix version %v\n", felixVersion)
			// }

			//

			//if calico-node container is running, then connect to the bird socket and get the data
			c, err := net.Dial("unix", "/var/run/calico/bird.ctl")
			if err != nil {
				panic(err)
			}
			defer c.Close()

			fmt.Println()

			_, _ = c.Write([]byte("show protocols\n"))
			if err != nil {
				log.Fatal("Error writing to BIRD socket:", err)
			}
			reader(c)

			return nil
		}
	}

	// return and print message if calio-node is not running
	fmt.Println("calico-node container not running")
	return nil

}

func reader(r io.Reader) {
	buf := make([]byte, 1024)

	n, err := r.Read(buf[:])
	if err != nil {
		return
	}

	resp := string(buf[:n])

	drawTable(resp)

}

func drawTable(in string) {
	data := [][]string{}

	ipSep := "."
	if !ipv4 {
		ipSep = ":"
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Peer address", "Peer type", "State", "Since", "Info"})

	r, _ := regexp.Compile(`\w+_\d+_\d+_\d+_\d+`)
	for _, line := range strings.Split(in, "\n") {

		s := r.FindString(line)

		if s != "" {
			col := []string{}
			// split columns
			fields := strings.Fields(line)[3:6]
			if strings.HasPrefix(s, "Mesh_") {
				s = s[5:]
				s = strings.Replace(s, "_", ipSep, -1)
				col = append(col, s)
				col = append(col, "node-to-node mesh")
				col = append(col, fields...)

			} else if strings.HasPrefix(s, "Node_") {
				s = s[5:]
				s = strings.Replace(s, "_", ipSep, -1)
				col = append(col, s)
				col = append(col, "node specific")
				col = append(col, fields...)
			} else if strings.HasPrefix(s, "Global_") {
				s = s[7:]
				s = strings.Replace(s, "_", ipSep, -1)
				col = append(col, s)
				col = append(col, "global")
				col = append(col, fields...)
			} else {
				// did not match any of the predefined options for BIRD
				fmt.Println(errors.New("Error: Did not match any of the predefined options for BIRD"))
				break
			}
			data = append(data, col)

		}
	}

	for _, v := range data {
		table.Append(v)
	}
	table.Render()
}
