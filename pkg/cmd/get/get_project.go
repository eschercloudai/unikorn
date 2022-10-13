/*
Copyright 2022 EscherCloud.

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
	"os"
	"strings"

	unikornv1alpha1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/cmd/errors"
	"github.com/eschercloudai/unikorn/pkg/cmd/util"

	"github.com/spf13/cobra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/cmd/get"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/completion"
)

type getProjectOptions struct {
	// name allows explict filtering of control plane namespaces.
	names []string

	// printFlags gives a rich set of functionality shamelessly stolen from
	// kubectl e.g. -o yaml etc.
	printFlags *get.PrintFlags

	// f is the factory used to create clients.
	f cmdutil.Factory

	// client is the Kubernetes v1 client.
	client kubernetes.Interface
}

// newGetProjectOptions returns a correctly initialized set of options.
func newGetProjectOptions() *getProjectOptions {
	return &getProjectOptions{
		printFlags: get.NewGetPrintFlags(),
	}
}

// complete fills in any options not does automatically by flag parsing.
func (o *getProjectOptions) complete(f cmdutil.Factory, args []string) error {
	o.f = f

	var err error

	if o.client, err = f.KubernetesClientSet(); err != nil {
		return err
	}

	if len(args) != 0 {
		o.names = args
	}

	return nil
}

// validate validates any tainted input not handled by complete() or flags
// processing.
func (o *getProjectOptions) validate() error {
	if o.names != nil {
		for _, name := range o.names {
			if len(name) == 0 {
				return fmt.Errorf(`%w: "%s"`, errors.ErrInvalidName, name)
			}
		}
	}

	return nil
}

// humanReadableOutput indicates whether the output is human readable (server formatted
// as a table using additional printer columns), or machine readable (e.g. JSON, YAML).
func (o *getProjectOptions) humanReadableOutput() bool {
	return len(*o.printFlags.OutputFormat) == 0
}

// transformRequests requests the Kubernetes API return a formatted table when
// we are requesting human readable output.  This does server side expansion of
// additional printer columns from the CRDs.
func (o *getProjectOptions) transformRequests(req *rest.Request) {
	if !o.humanReadableOutput() {
		return
	}

	req.SetHeader("Accept", strings.Join([]string{
		fmt.Sprintf("application/json;as=Table;v=%s;g=%s", metav1.SchemeGroupVersion.Version, metav1.GroupName),
		"application/json",
	}, ","))
}

// run executes the command.
func (o *getProjectOptions) run() error {
	// We are using the "kubectl get" library to retrieve resources.  That command
	// is generic, it accepts a kind and name(s), or a list of type/name tuples.
	// In our case, the type is implicit, so we need to prepend it to keep things
	// working as they should.
	args := []string{unikornv1alpha1.ProjectResource}
	args = append(args, o.names...)

	r := o.f.NewBuilder().
		Unstructured().
		ResourceTypeOrNameArgs(true, args...).
		ContinueOnError().
		Latest().
		Flatten().
		TransformRequests(o.transformRequests).
		Do()

	infos, err := r.Infos()
	if err != nil {
		return err
	}

	for _, info := range infos {
		printer, err := o.printFlags.ToPrinter()
		if err != nil {
			return err
		}

		if o.humanReadableOutput() {
			printer = &get.TablePrinter{Delegate: printer}
		}

		if err := printer.PrintObj(info.Object, os.Stdout); err != nil {
			return err
		}
	}

	return nil
}

var (
	getProjectExample = util.TemplatedExample(`
	# Get a single project named my-project-name.
	{{.Application}} get project my-project-name

	# Get multiple projects.
	{{.Application}} get project my-project-name my-other-project-name`)
)

// newGetProjectCommand returns a command that is able to get or list Cluster API
// control planes found on the management cluster.
func newGetProjectCommand(f cmdutil.Factory) *cobra.Command {
	o := newGetProjectOptions()

	cmd := &cobra.Command{
		Use:     "project",
		Short:   "Get or list Cluster API control planes",
		Long:    "Get or list Cluster API control planes",
		Example: getProjectExample,
		Aliases: []string{
			"projects",
		},
		ValidArgsFunction: completion.ResourceNameCompletionFunc(f, unikornv1alpha1.ProjectResource),
		Run: func(cmd *cobra.Command, args []string) {
			util.AssertNilError(o.complete(f, args))
			util.AssertNilError(o.validate())
			util.AssertNilError(o.run())
		},
	}

	o.printFlags.AddFlags(cmd)

	return cmd
}
