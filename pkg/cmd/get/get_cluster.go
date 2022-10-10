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
	"github.com/eschercloudai/unikorn/pkg/cmd/util"

	"github.com/spf13/cobra"

	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// getClusterOptions defines a set of options that are required to get
// a cluster.
type getClusterOptions struct {
	// controlPlane defines the control plane name that the cluster will
	// be searched for in.
	controlPlane string
}

// addFlags registers get cluster options flags with the specified cobra command.
func (o *getClusterOptions) addFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&o.controlPlane, "control-plane", "", "Control plane to deprovision the cluster from.")

	if err := cmd.MarkFlagRequired("control-plane"); err != nil {
		panic(err)
	}
}

var (
	getClusterExamples = util.TemplatedExample(`
        # List Kubernetes clusters in control plane foo
        {{.Application}} get cluster --control-plane foo`)
)

// newGetClusterCommand returns a command that is able to get or list Kubernetes clusters
// found in the provided Cluster API control plane.
func newGetClusterCommand(cf *genericclioptions.ConfigFlags) *cobra.Command {
	o := &getClusterOptions{}

	cmd := &cobra.Command{
		Use:     "cluster",
		Short:   "Get or list Kubernetes clusters",
		Long:    "Get or list Kubernetes clusters",
		Example: getClusterExamples,
		Run: func(cmd *cobra.Command, args []string) {
		},
	}

	o.addFlags(cmd)

	return cmd
}
