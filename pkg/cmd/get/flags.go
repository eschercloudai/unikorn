/*
Copyright 2022-2024 EscherCloud.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package get

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/cmd/get"
)

type getPrintFlags struct {
	// outputFormat selects formatting e.g. json, yaml, or human readable by default.
	outputFormat string

	// jsonYamlPrintFlags specifies any json/yaml formatting options.
	jsonYamlPrintFlags *genericclioptions.JSONYamlPrintFlags

	// humanReadableFlags allows the default table output format to be tweaked.
	humanReadableFlags *get.HumanPrintFlags
}

func newGetPrintFlags() *getPrintFlags {
	return &getPrintFlags{
		jsonYamlPrintFlags: genericclioptions.NewJSONYamlPrintFlags(),
		humanReadableFlags: get.NewHumanPrintFlags(),
	}
}

// allowedFormats specifies the possible formats for the output format flag.
func (o *getPrintFlags) allowedFormats() []string {
	var formats []string

	formats = append(formats, o.jsonYamlPrintFlags.AllowedFormats()...)
	formats = append(formats, o.humanReadableFlags.AllowedFormats()...)

	return formats
}

// outputCompletion is a shell completion function for the output format flag.
func (o *getPrintFlags) outputCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var matches []string

	for _, format := range o.allowedFormats() {
		if strings.HasPrefix(format, toComplete) {
			matches = append(matches, format)
		}
	}

	return matches, cobra.ShellCompDirectiveNoFileComp
}

// addFlags registers create cluster options flags with the specified cobra command.
func (o *getPrintFlags) addFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&o.outputFormat, "output", "o", "", fmt.Sprintf("Output format. One of (%s)", strings.Join(o.allowedFormats(), ", ")))

	o.jsonYamlPrintFlags.AddFlags(cmd)
	o.humanReadableFlags.AddFlags(cmd)

	if err := cmd.RegisterFlagCompletionFunc("output", o.outputCompletion); err != nil {
		panic(err)
	}
}

// toPrinter returns the correct printer for the given output format.
func (o *getPrintFlags) toPrinter() (printers.ResourcePrinter, error) {
	if printer, err := o.jsonYamlPrintFlags.ToPrinter(o.outputFormat); !genericclioptions.IsNoCompatiblePrinterError(err) {
		return printer, err
	}

	if printer, err := o.humanReadableFlags.ToPrinter(o.outputFormat); !genericclioptions.IsNoCompatiblePrinterError(err) {
		return &get.TablePrinter{Delegate: printer}, err
	}

	return nil, genericclioptions.NoCompatiblePrinterError{OutputFormat: &o.outputFormat, AllowedFormats: o.allowedFormats()}
}

// humanReadableOutput indicates whether the output is human readable (server formatted
// as a table using additional printer columns), or machine readable (e.g. JSON, YAML).
func (o *getPrintFlags) humanReadableOutput() bool {
	return len(o.outputFormat) == 0
}

// transformRequests requests the Kubernetes API return a formatted table when
// we are requesting human readable output.  This does server side expansion of
// additional printer columns from the CRDs.
func (o *getPrintFlags) transformRequests(req *rest.Request) {
	if !o.humanReadableOutput() {
		return
	}

	req.SetHeader("Accept", strings.Join([]string{
		fmt.Sprintf("application/json;as=Table;v=%s;g=%s", metav1.SchemeGroupVersion.Version, metav1.GroupName),
		"application/json",
	}, ","))
}
