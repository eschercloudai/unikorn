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
	"github.com/eschercloudai/unikorn/pkg/cmd/util/completion"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	computil "k8s.io/kubectl/pkg/util/completion"
)

// deleteClusterOptions defines a set of options that are required to delete
// a cluster.
type deleteClusterOptions struct {
	// project defines the project to delete the cluster from.
	project string

	// controlPlane defines the control plane name that the cluster will
	// be deleted from.
	controlPlane string

	// name is the name of the cluster.
	name string

	// client gives access to our custom resources.
	client unikorn.Interface
}

// addFlags registers delete cluster options flags with the specified cobra command.
func (o *deleteClusterOptions) addFlags(f cmdutil.Factory, cmd *cobra.Command) {
	util.RequiredStringVarWithCompletion(cmd, &o.project, "project", "", "Kubernetes project name that contains the control plane.", computil.ResourceNameCompletionFunc(f, unikornv1alpha1.ProjectResource))
	util.RequiredStringVarWithCompletion(cmd, &o.controlPlane, "control-plane", "", "Control plane to deprovision the cluster from.", completion.ControlPlanesCompletionFunc(f, &o.project))
}

// complete fills in any options not does automatically by flag parsing.
func (o *deleteClusterOptions) complete(f cmdutil.Factory, args []string) error {
	config, err := f.ToRESTConfig()
	if err != nil {
		return err
	}

	if o.client, err = unikorn.NewForConfig(config); err != nil {
		return err
	}

	if len(args) != 1 {
		return errors.ErrIncorrectArgumentNum
	}

	o.name = args[0]

	return nil
}

// validate validates any tainted input not handled by complete() or flags
// processing.
func (o *deleteClusterOptions) validate() error {
	return nil
}

// run executes the command.
func (o *deleteClusterOptions) run() error {
	namespace, err := util.GetControlPlaneNamespace(context.TODO(), o.client, o.project, o.controlPlane)
	if err != nil {
		return err
	}

	if err := o.client.UnikornV1alpha1().KubernetesClusters(namespace).Delete(context.TODO(), o.name, metav1.DeleteOptions{}); err != nil {
		return err
	}

	return nil
}

var (
	//nolint:gochecknoglobals
	deleteClusterExamples = util.TemplatedExample(`
        # Delete a Kubernetes cluster
        {{.Application}} delete cluster --control-plane foo`)
)

// newDeleteClusterCommand creates a command that deletes a Kubenretes cluster in the
// specified Cluster API control plane.
func newDeleteClusterCommand(f cmdutil.Factory) *cobra.Command {
	o := &deleteClusterOptions{}

	cmd := &cobra.Command{
		Use:               "cluster",
		Short:             "Delete a Kubernetes cluster",
		Long:              "Delete a Kubernetes cluster",
		Example:           deleteClusterExamples,
		ValidArgsFunction: completion.ClustersCompletionFunc(f, &o.project, &o.controlPlane),
		Run: func(cmd *cobra.Command, args []string) {
			util.AssertNilError(o.complete(f, args))
			util.AssertNilError(o.validate())
			util.AssertNilError(o.run())
		},
	}

	o.addFlags(f, cmd)

	return cmd
}
