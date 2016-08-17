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
	"io/ioutil"
	"reflect"
	"strings"

	"bytes"
	"encoding/json"
	"github.com/ghodss/yaml"
	"github.com/golang/glog"
	"github.com/tigera/libcalico-go/calicoctl/resourcemgr"
	"github.com/tigera/libcalico-go/lib/api/unversioned"
	"os"
	"text/tabwriter"
	"text/template"
)

func Get(args []string) error {
	doc := EtcdIntro + `Display one or many resources identified by file, stdin or resource type and name.

Possible resource types include: policy

By specifying the output as 'template' and providing a Go template as the value
of the --template flag, you can filter the attributes of the fetched resource(s).

Usage:
  calicoctl get ([--tier=<TIER>] [--hostname=<HOSTNAME>] [--scope=<SCOPE>] (<KIND> [<NAME>]) |
                 --filename=<FILENAME>)
                [--output=<OUTPUT>] [--config=<CONFIG>]


Examples:
  # List all policy in default output format.
  calicoctl get policy

  # List a specific policy in YAML format
  calicoctl get -o yaml policy my-policy-1

Options:
  -f --filename=<FILENAME>     Filename to use to get the resource.  If set to "-" loads from stdin.
  -o --output=<OUTPUT FORMAT>  Output format.  One of: ps, wide, yaml, json, go-template=...,
                               go-template-file=...  [Default: ps]
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

	var rp resourcePrinter
	output := parsedArgs["--output"].(string)
	switch output {
	case "yaml":
		rp = resourcePrinterYAML{}
	case "json":
		rp = resourcePrinterJSON{}
	case "ps":
		rp = resourcePrinterTable{wide: false}
	case "wide":
		rp = resourcePrinterTable{wide: true}
	default:
		// Output format may be a key=value pair, so split on "=" to find out.
		outputParms := strings.SplitN(output, "=", 2)
		if len(outputParms) == 2 {
			switch outputParms[0] {
			case "go-template":
				rp = resourcePrinterTemplate{template: outputParms[1]}
			case "go-template-file":
				rp = resourcePrinterTemplateFile{templateFile: outputParms[1]}
			}
		}
	}

	if rp == nil {
		return fmt.Errorf("Unrecognized output format: %s", output)
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
	// resources := convertToSliceOfResources(results.resources)
	return rp.print(results.resources)
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

type resourcePrinter interface {
	print(resources []unversioned.Resource) error
}

// JSON output formatter
type resourcePrinterJSON struct {}
func (r resourcePrinterJSON) print(resources []unversioned.Resource) error {
	if output, err := json.MarshalIndent(resources, "", "  "); err != nil {
		return err
	} else {
		fmt.Printf("%s", string(output))
	}
	return nil
}

//YAML output formatter
type resourcePrinterYAML struct {}
func (r resourcePrinterYAML) print(resources []unversioned.Resource) error {
	if output, err := yaml.Marshal(resources); err != nil {
		return err
	} else {
		fmt.Printf("%s", string(output))
	}
	return nil
}

// Table output formatter
type resourcePrinterTable struct {
	// Wide format.  If false, prints minimum information to identify a resource.  If true,
	// table includes additional useful information dependent on resource type.
	wide bool
}
func (r resourcePrinterTable) print(resources []unversioned.Resource) error {
	glog.V(2).Infof("Output in table format (wide=%v)", r.wide)
	for idx, resource := range resources {
		// If there are multiple resources then make sure we leave a gap
		// between each table.
		if idx > 1 {
			fmt.Printf("\n")
		}

		// Look up the template string for the specific resource type.
		tpls := resourcemgr.GetPSTemplate(resource, r.wide)

		fns := template.FuncMap{
			"join": join,
		}
		tmpl, err := template.New("get").Funcs(fns).Parse(tpls)
		if err != nil {
			panic(err)
		}

		writer := tabwriter.NewWriter(os.Stdout, 5, 1, 3, ' ', 0)
		err = tmpl.Execute(writer, resources)
		if err != nil {
			panic(err)
		}
		writer.Flush()

		// Templates for ps format are internally defined and therefore we should not
		// hit errors writing the table formats.
		if err != nil {
			panic(err)
		}
	}
	return nil
}

// Go-template-file output formatter.
type resourcePrinterTemplateFile struct {
	templateFile string
}
func (r resourcePrinterTemplateFile) print(resources []unversioned.Resource) error {
	template, err := ioutil.ReadFile(r.templateFile)
	if err != nil {
		return err
	}
	rp := resourcePrinterTemplate{template: string(template)}
	return rp.print(resources)
}

// Go-template output formatter.
type resourcePrinterTemplate struct {
	template string
}
func (r resourcePrinterTemplate) print(resources []unversioned.Resource) error {

	fns := template.FuncMap{
		"join": join,
	}
	tmpl, err := template.New("get").Funcs(fns).Parse(r.template)
	if err != nil {
		panic(err)
	}

	writer := tabwriter.NewWriter(os.Stdout, 5, 1, 3, ' ', 0)
	err = tmpl.Execute(writer, resources)
	if err != nil {
		panic(err)
	}
	writer.Flush()
	return nil
}

func join(items interface{}, separator string) string {
	// If this is a slice of strings - just use the strings.Join function.
	switch s := items.(type) {
	case []string:
		return strings.Join(s, separator)
	case fmt.Stringer:
		return s.String()
	}

	// Otherwise, provided this is a slice, just convert each item to a string and
	// join together.
	switch reflect.TypeOf(items).Kind() {
	case reflect.Slice:
		slice := reflect.ValueOf(items)
		buf := new(bytes.Buffer)
		for i := 0; i < slice.Len(); i++ {
			if i > 0 {
				buf.WriteString(separator)
			}
			fmt.Fprint(buf, slice.Index(i).Interface())
		}
		return buf.String()
	}

	// The supplied items is not a slice - so just convert to a string.
	return fmt.Sprint(items)
}
