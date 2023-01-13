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
	"context"

	"github.com/spf13/cobra"

	"github.com/eschercloudai/unikorn/generated/clientset/unikorn"
	unikornv1alpha1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/cmd/util"
	"github.com/eschercloudai/unikorn/pkg/cmd/util/flags"
	"github.com/eschercloudai/unikorn/pkg/constants"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

// getWorkloadPoolOptions defines a set of options that are required to get
// a workload pool.
type getWorkloadPoolOptions struct {
	// clusterFlags define cluster scoping.
	clusterFlags flags.ClusterFlags

	// names is an explicit set of resource names to get.
	names []string

	// getPrintFlags is a generic and reduced set of printing options.
	getPrintFlags *getPrintFlags

	// f is the factory used to create clients.
	f cmdutil.Factory

	// client gives access to our custom resources.
	client unikorn.Interface
}

func newGetWorkloadPoolOptions() *getWorkloadPoolOptions {
	return &getWorkloadPoolOptions{
		getPrintFlags: newGetPrintFlags(),
	}
}

// addFlags registers get workload pool options flags with the specified cobra command.
func (o *getWorkloadPoolOptions) addFlags(cmd *cobra.Command, f cmdutil.Factory) {
	o.clusterFlags.AddFlags(f, cmd)
	o.getPrintFlags.addFlags(cmd)
}

// complete fills in any options not does automatically by flag parsing.
func (o *getWorkloadPoolOptions) complete(f cmdutil.Factory, args []string) error {
	o.f = f

	config, err := f.ToRESTConfig()
	if err != nil {
		return err
	}

	if o.client, err = unikorn.NewForConfig(config); err != nil {
		return err
	}

	if len(args) != 0 {
		o.names = args
	}

	return nil
}

// run executes the command.
func (o *getWorkloadPoolOptions) run() error {
	namespace, err := o.clusterFlags.GetControlPlaneNamespace(context.TODO(), o.client)
	if err != nil {
		return err
	}

	// We are using the "kubectl get" library to retrieve resources.  That command
	// is generic, it accepts a kind and name(s), or a list of type/name tuples.
	// In our case, the type is implicit, so we need to prepend it to keep things
	// working as they should.
	args := []string{unikornv1alpha1.KubernetesWorkloadPoolResource}

	selector := labels.Everything()

	if o.clusterFlags.Cluster != "" {
		clusterLabel, err := labels.NewRequirement(constants.KubernetesClusterLabel, selection.Equals, []string{o.clusterFlags.Cluster})
		if err != nil {
			return err
		}

		selector = selector.Add(*clusterLabel)
	} else {
		args = append(args, o.names...)
	}

	r := o.f.NewBuilder().
		Unstructured().
		NamespaceParam(namespace).
		ResourceTypeOrNameArgs(true, args...).
		LabelSelector(selector.String()).
		ContinueOnError().
		Latest().
		Flatten().
		TransformRequests(o.getPrintFlags.transformRequests).
		Do()

	if err := r.Err(); err != nil {
		return err
	}

	if err := o.getPrintFlags.printResult(r); err != nil {
		return err
	}

	return nil
}

var (
	//nolint:gochecknoglobals
	getWorkloadPoolExamples = util.TemplatedExample(`
        # List Kubernetes workload pools in control plane foo
        {{.Application}} get workload-pool --project foo --control-plane bar

        # List Kubernetes workload pools in control plane foo for a specific cluster baz
        {{.Application}} get workload-pool --project foo --control-plane bar --cluster baz`)
)

// newGetWorkloadPoolCommand returns a command that is able to get or list Kubernetes workload pools
// found in the provided WorkloadPool API control plane.
func newGetWorkloadPoolCommand(f cmdutil.Factory) *cobra.Command {
	o := newGetWorkloadPoolOptions()

	cmd := &cobra.Command{
		Use:               "workload-pool",
		Short:             "Get or list Kubernetes workload pools",
		Long:              "Get or list Kubernetes workload pools",
		Example:           getWorkloadPoolExamples,
		ValidArgsFunction: o.clusterFlags.CompleteWorkloadPool(f),
		Run: func(cmd *cobra.Command, args []string) {
			util.AssertNilError(o.complete(f, args))
			util.AssertNilError(o.run())
		},
	}

	o.addFlags(cmd, f)

	return cmd
}
