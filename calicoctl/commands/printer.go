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
	"io/ioutil"
	"reflect"
	"strings"

	"bytes"
	"encoding/json"
	"os"
	"text/tabwriter"
	"text/template"

	"github.com/ghodss/yaml"
	"github.com/golang/glog"
	"github.com/tigera/libcalico-go/calicoctl/resourcemgr"
	"github.com/tigera/libcalico-go/lib/api/unversioned"
)

type resourcePrinter interface {
	print(resources []unversioned.Resource) error
}

// JSON output formatter
type resourcePrinterJSON struct{}

func (r resourcePrinterJSON) print(resources []unversioned.Resource) error {
	// TODO Handle better - results should be groups as per input file
	// For simplicity convert the returned list of resources to expand any lists
	resources = convertToSliceOfResources(resources)
	if output, err := json.MarshalIndent(resources, "", "  "); err != nil {
		return err
	} else {
		fmt.Printf("%s\n", string(output))
	}
	return nil
}

//YAML output formatter
type resourcePrinterYAML struct{}

func (r resourcePrinterYAML) print(resources []unversioned.Resource) error {
	// TODO Handle better - results should be groups as per input file
	// For simplicity convert the returned list of resources to expand any lists
	resources = convertToSliceOfResources(resources)
	if output, err := yaml.Marshal(resources); err != nil {
		return err
	} else {
		fmt.Printf("%s", string(output))
	}
	return nil
}

// Table output formatter
type resourcePrinterTable struct {
	// The headings to display in the table.  If this is nil, the default headings for the
	// resource are used instead (in which case the `wide` boolean below is used to specify
	// whether wide or narrow format is required.
	headings []string

	// Wide format.  When headings have not been explicitly specified, this is used to
	// determine whether to the resource-specific default wide or narrow headings.
	wide bool
}

func (r resourcePrinterTable) print(resources []unversioned.Resource) error {
	glog.V(2).Infof("Output in table format (wide=%v)", r.wide)
	for idx, resource := range resources {
		// Get the resource manager for the resource type.
		rm := resourcemgr.GetResourceManager(resource)

		// If there are multiple resources then make sure we leave a gap
		// between each table.
		if idx > 0 {
			fmt.Printf("\n")
		}

		// If no headings have been specified then we must be using the default
		// headings for that resource type.
		headings := r.headings
		if r.headings == nil {
			headings = rm.GetTableDefaultHeadings(r.wide)
		}

		// Look up the template string for the specific resource type.
		tpls, err := rm.GetTableTemplate(headings)
		if err != nil {
			return err
		}

		// Convert the template string into a template - we need to include the join
		// function.
		fns := template.FuncMap{
			"join": join,
		}
		tmpl, err := template.New("get").Funcs(fns).Parse(tpls)
		if err != nil {
			panic(err)
		}

		// Use a tabwriter to write out the teplate - this provides better formatting.
		writer := tabwriter.NewWriter(os.Stdout, 5, 1, 3, ' ', 0)
		err = tmpl.Execute(writer, resource)
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
		return err
	}

	err = tmpl.Execute(os.Stdout, resources)
	if err != nil {
		return err
	}
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
