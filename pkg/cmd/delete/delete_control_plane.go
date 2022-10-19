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
	"fmt"

	"github.com/eschercloudai/unikorn/generated/clientset/unikorn"
	unikornv1alpha1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/cmd/errors"
	"github.com/eschercloudai/unikorn/pkg/cmd/util"
	"github.com/eschercloudai/unikorn/pkg/cmd/util/completion"

	"github.com/spf13/cobra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	computil "k8s.io/kubectl/pkg/util/completion"
)

type deleteControlPlaneOptions struct {
	project string

	name string

	// unikornClient gives access to our custom resources.
	unikornClient unikorn.Interface
}

// addFlags registers create cluster options flags with the specified cobra command.
func (o *deleteControlPlaneOptions) addFlags(f cmdutil.Factory, cmd *cobra.Command) {
	cmd.Flags().StringVar(&o.project, "project", "", "Kubernetes project name that contains the control plane.")

	if err := cmd.MarkFlagRequired("project"); err != nil {
		panic(err)
	}

	if err := cmd.RegisterFlagCompletionFunc("project", computil.ResourceNameCompletionFunc(f, unikornv1alpha1.ProjectResource)); err != nil {
		panic(err)
	}
}

// complete fills in any options not does automatically by flag parsing.
func (o *deleteControlPlaneOptions) complete(f cmdutil.Factory, args []string) error {
	var err error

	config, err := f.ToRESTConfig()
	if err != nil {
		return err
	}

	if o.unikornClient, err = unikorn.NewForConfig(config); err != nil {
		return err
	}

	o.name = args[0]

	return nil
}

// validate validates any tainted input not handled by complete() or flags
// processing.
func (o *deleteControlPlaneOptions) validate() error {
	if len(o.name) == 0 {
		return fmt.Errorf(`%w: "%s"`, errors.ErrInvalidName, o.name)
	}

	if len(o.project) == 0 {
		return fmt.Errorf(`%w: "%s"`, errors.ErrInvalidName, o.project)
	}

	return nil
}

// run executes the command.
func (o *deleteControlPlaneOptions) run() error {
	project, err := o.unikornClient.UnikornV1alpha1().Projects().Get(context.TODO(), o.project, metav1.GetOptions{})
	if err != nil {
		return err
	}

	namespace := project.Status.Namespace
	if len(namespace) == 0 {
		return fmt.Errorf("project namespace undefined")
	}

	if err := o.unikornClient.UnikornV1alpha1().ControlPlanes(namespace).Delete(context.TODO(), o.name, metav1.DeleteOptions{}); err != nil {
		return err
	}

	return nil
}

// newDeleteControlPlaneCommand creates a command that deletes a Cluster API control plane.
func newDeleteControlPlaneCommand(f cmdutil.Factory) *cobra.Command {
	o := deleteControlPlaneOptions{}

	cmd := &cobra.Command{
		Use:               "control-plane",
		Short:             "Delete a Kubernetes cluster",
		Long:              "Delete a Kubernetes cluster",
		ValidArgsFunction: completion.ControlPlanesCompletionFunc(f, &o.project),
		Aliases: []string{
			"control-planes",
			"cp",
		},
		Run: func(cmd *cobra.Command, args []string) {
			util.AssertNilError(o.complete(f, args))
			util.AssertNilError(o.validate())
			util.AssertNilError(o.run())
		},
	}

	o.addFlags(f, cmd)

	return cmd
}
