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

package delete

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/eschercloudai/unikorn/generated/clientset/unikorn"
	unikornv1alpha1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/cmd/errors"
	"github.com/eschercloudai/unikorn/pkg/cmd/util"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/completion"
	"k8s.io/kubectl/pkg/util/templates"
)

type deleteProjectOptions struct {
	// name allows explicit filtering of control plane namespaces.
	names []string

	// unikornClient is a typed client for our custom resources.
	unikornClient unikorn.Interface
}

// complete fills in any options not does automatically by flag parsing.
func (o *deleteProjectOptions) complete(f cmdutil.Factory, args []string) error {
	config, err := f.ToRESTConfig()
	if err != nil {
		return err
	}

	if o.unikornClient, err = unikorn.NewForConfig(config); err != nil {
		return err
	}

	if len(args) < 1 {
		return errors.ErrIncorrectArgumentNum
	}

	o.names = args

	return nil
}

// validate validates any tainted input not handled by complete() or flags
// processing.
func (o *deleteProjectOptions) validate() error {
	for _, name := range o.names {
		if len(name) == 0 {
			return errors.ErrInvalidName
		}
	}

	return nil
}

// run executes the command.
func (o *deleteProjectOptions) run() error {
	for _, name := range o.names {
		if err := o.unikornClient.UnikornV1alpha1().Projects().Delete(context.TODO(), name, metav1.DeleteOptions{}); err != nil {
			return err
		}
	}

	return nil
}

var (
	//nolint:gochecknoglobals
	deleteProjectLong = templates.LongDesc(`
	Delete a project.

	Projects encapsulates multiple control planes as a means to provide
	isolation between users or organizations.  A project may contain multiple
	control planes, so deletion of a project will cascade down and delete
	all control planes in that project.`)

	//nolint:gochecknoglobals
	deleteProjectExample = util.TemplatedExample(`
        # Delete a single project named my-project-name.
        {{.Application}} delete project my-project-name

        # Delete multiple projects.
        {{.Application}} delete project my-project-name my-other-project-name`)
)

// newDeleteProjectCommand creates a command that deletes a Cluster API control plane.
func newDeleteProjectCommand(f cmdutil.Factory) *cobra.Command {
	o := &deleteProjectOptions{}

	cmd := &cobra.Command{
		Use:               "project",
		Short:             "Delete a project",
		Long:              deleteProjectLong,
		Example:           deleteProjectExample,
		ValidArgsFunction: completion.ResourceNameCompletionFunc(f, unikornv1alpha1.ProjectResource),
		Run: func(cmd *cobra.Command, args []string) {
			util.AssertNilError(o.complete(f, args))
			util.AssertNilError(o.validate())
			util.AssertNilError(o.run())
		},
	}

	return cmd
}
