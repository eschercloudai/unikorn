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
	"github.com/eschercloudai/unikorn/pkg/cmd/aliases"
	"github.com/eschercloudai/unikorn/pkg/cmd/errors"
	"github.com/eschercloudai/unikorn/pkg/cmd/util"
	"github.com/eschercloudai/unikorn/pkg/cmd/util/flags"
	"github.com/eschercloudai/unikorn/pkg/constants"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

// deleteWorkloadPoolOptions defines a set of options that are required to delete
// a workload pool.
type deleteWorkloadPoolOptions struct {
	// clusterFlags define cluster scoping.
	clusterFlags flags.ClusterFlags

	// deleteFlags define common deletion options.
	deleteFlags flags.DeleteFlags

	// names are the name of the workload pools to delete.
	names []string

	// client gives access to our custom resources.
	client unikorn.Interface
}

// addFlags registers delete workload pool options flags with the specified cobra command.
func (o *deleteWorkloadPoolOptions) addFlags(f cmdutil.Factory, cmd *cobra.Command) {
	o.clusterFlags.AddFlags(f, cmd)
	o.deleteFlags.AddFlags(f, cmd)
}

// completeNames either sets the names explcitly via the CLI or implicitly if --all
// is specified.
func (o *deleteWorkloadPoolOptions) completeNames(args []string) error {
	if !o.deleteFlags.All {
		o.names = util.UniqueString(args)

		return nil
	}

	namespace, err := o.clusterFlags.GetControlPlaneNamespace(context.TODO(), o.client)
	if err != nil {
		return err
	}

	selector := labels.Everything()

	if o.clusterFlags.Cluster != "" {
		clusterLabel, err := labels.NewRequirement(constants.KubernetesClusterLabel, selection.Equals, []string{o.clusterFlags.Cluster})
		if err != nil {
			return err
		}

		selector = selector.Add(*clusterLabel)
	}

	resources, err := o.client.UnikornV1alpha1().KubernetesWorkloadPools(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: selector.String()})
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

	if err := o.completeNames(args); err != nil {
		return err
	}

	return nil
}

// validate validates any tainted input not handled by complete() or flags
// processing.
func (o *deleteWorkloadPoolOptions) validate() error {
	if !o.deleteFlags.All && len(o.names) == 0 {
		return fmt.Errorf(`%w: resource names or --all must be specified`, errors.ErrInvalidName)
	}

	return nil
}

// run executes the command.
func (o *deleteWorkloadPoolOptions) run() error {
	namespace, err := o.clusterFlags.GetControlPlaneNamespace(context.TODO(), o.client)
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
        {{.Application}} delete workload-pool --control-plane foo`)
)

// newDeleteWorkloadPoolCommand creates a command that deletes a Kubenretes workload pool in the
// specified WorkloadPool API control plane.
func newDeleteWorkloadPoolCommand(f cmdutil.Factory) *cobra.Command {
	o := &deleteWorkloadPoolOptions{}

	cmd := &cobra.Command{
		Use:               "workload-pool",
		Short:             "Delete a Kubernetes workload pool",
		Long:              "Delete a Kubernetes workload pool",
		Example:           deleteWorkloadPoolExamples,
		ValidArgsFunction: o.clusterFlags.CompleteWorkloadPool(f),
		Aliases:           aliases.WorkloadPool,
		Run: func(cmd *cobra.Command, args []string) {
			util.AssertNilError(o.complete(f, args))
			util.AssertNilError(o.validate())
			util.AssertNilError(o.run())
		},
	}

	o.addFlags(f, cmd)

	return cmd
}
