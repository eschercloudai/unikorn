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

	"github.com/spf13/cobra"

	"github.com/eschercloudai/unikorn/generated/clientset/unikorn"
	unikornv1alpha1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/cmd/errors"
	"github.com/eschercloudai/unikorn/pkg/cmd/util"
	"github.com/eschercloudai/unikorn/pkg/cmd/util/completion"
	"github.com/eschercloudai/unikorn/pkg/constants"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	computil "k8s.io/kubectl/pkg/util/completion"
)

// deleteWorkloadPoolOptions defines a set of options that are required to delete
// a workload pool.
type deleteWorkloadPoolOptions struct {
	// project defines the project to delete the workload pool from.
	project string

	// controlPlane defines the control plane name that the workload pool will
	// be deleted from.
	controlPlane string

	// cluster defines the workload pool name that the resource is associated with.
	cluster string

	// names are the name of the workload pools to delete.
	names []string

	// all removes all resource that match the query.
	all bool

	// client gives access to our custom resources.
	client unikorn.Interface
}

// addFlags registers delete workload pool options flags with the specified cobra command.
func (o *deleteWorkloadPoolOptions) addFlags(f cmdutil.Factory, cmd *cobra.Command) {
	util.RequiredStringVarWithCompletion(cmd, &o.project, "project", "", "Kubernetes project name that contains the control plane.", computil.ResourceNameCompletionFunc(f, unikornv1alpha1.ProjectResource))
	util.RequiredStringVarWithCompletion(cmd, &o.controlPlane, "control-plane", "", "Control plane to deprovision the workload pool from.", completion.ControlPlanesCompletionFunc(f, &o.project))
	util.StringVarWithCompletion(cmd, &o.cluster, "cluster", "", "Control plane to the workload pool is in.", completion.ClustersCompletionFunc(f, &o.project, &o.controlPlane))
	cmd.Flags().BoolVar(&o.all, "all", false, "Select all workload pools that match the query.")
}

func (o *deleteWorkloadPoolOptions) completeNames(args []string) error {
	if !o.all {
		if len(args) == 0 {
			return errors.ErrIncorrectArgumentNum
		}

		o.names = args

		return nil
	}

	namespace, err := util.GetControlPlaneNamespace(context.TODO(), o.client, o.project, o.controlPlane)
	if err != nil {
		return err
	}

	selector := labels.Everything()

	if o.cluster != "" {
		clusterLabel, err := labels.NewRequirement(constants.KubernetesClusterLabel, selection.Equals, []string{o.cluster})
		if err != nil {
			return err
		}

		selector = selector.Add(*clusterLabel)
	}

	resources, err := o.client.UnikornV1alpha1().KubernetesClusters(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return err
	}

	o.names = make([]string, len(resources.Items))

	for i, resource := range resources.Items {
		o.names[i] = resource.Name
	}

	return nil
}

// complete fills in any options not does automatically by flag parsing.
func (o *deleteWorkloadPoolOptions) complete(f cmdutil.Factory, args []string) error {
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

	if err := o.completeNames(args); err != nil {
		return err
	}

	return nil
}

// validate validates any tainted input not handled by complete() or flags
// processing.
func (o *deleteWorkloadPoolOptions) validate() error {
	if !o.all && len(o.names) == 0 {
		return fmt.Errorf(`%w: resource names or --all must be specified`, errors.ErrInvalidName)
	}

	return nil
}

// run executes the command.
func (o *deleteWorkloadPoolOptions) run() error {
	namespace, err := util.GetControlPlaneNamespace(context.TODO(), o.client, o.project, o.controlPlane)
	if err != nil {
		return err
	}

	for _, name := range o.names {
		if err := o.client.UnikornV1alpha1().KubernetesWorkloadPools(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{}); err != nil {
			return err
		}

		fmt.Printf("%s.%s/%s deleted\n", unikornv1alpha1.KubernetesWorkloadPoolResource, unikornv1alpha1.GroupName, name)
	}

	return nil
}

var (
	//nolint:gochecknoglobals
	deleteWorkloadPoolExamples = util.TemplatedExample(`
        # Delete a Kubernetes workload pool
        {{.Application}} delete workload pool --control-plane foo`)
)

// newDeleteWorkloadPoolCommand creates a command that deletes a Kubenretes workload pool in the
// specified WorkloadPool API control plane.
func newDeleteWorkloadPoolCommand(f cmdutil.Factory) *cobra.Command {
	o := &deleteWorkloadPoolOptions{}

	cmd := &cobra.Command{
		Use:               "workload pool",
		Short:             "Delete a Kubernetes workload pool",
		Long:              "Delete a Kubernetes workload pool",
		Example:           deleteWorkloadPoolExamples,
		ValidArgsFunction: completion.WorkloadPoolsCompletionFunc(f, &o.project, &o.controlPlane, &o.cluster),
		Aliases: []string{
			"workload-pools",
			"wp",
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
