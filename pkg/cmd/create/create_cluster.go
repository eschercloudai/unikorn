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

package create

import (
	"github.com/eschercloudai/unikorn/pkg/cmd/util"

	"github.com/spf13/cobra"

	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

// createClusterOptions defines a set of options that are required to create
// a cluster.
type createClusterOptions struct {
	// controlPlane defines the control plane name that the cluster will
	// be provisioned with.
	controlPlane string
}

// addFlags registers create cluster options flags with the specified cobra command.
func (o *createClusterOptions) addFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&o.controlPlane, "control-plane", "", "Control plane to provision the cluster in.")

	if err := cmd.MarkFlagRequired("control-plane"); err != nil {
		panic(err)
	}
}

var (
	createClusterExamples = util.TemplatedExample(`
        # Create a Kubernetes cluster
        {{.Application}} create cluster --control-plane foo`)
)

// newCreateClusterCommand creates a command that is able to provison a new Kubernetes
// cluster with a Cluster API control plane.
func newCreateClusterCommand(f cmdutil.Factory) *cobra.Command {
	o := &createClusterOptions{}

	cmd := &cobra.Command{
		Use:     "cluster",
		Short:   "Create a Kubernetes cluster",
		Long:    "Create a Kubernetes cluster",
		Example: createClusterExamples,
		Run: func(cmd *cobra.Command, args []string) {
		},
	}

	o.addFlags(cmd)

	return cmd
}
