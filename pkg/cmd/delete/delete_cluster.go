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
	"github.com/spf13/cobra"

	"github.com/eschercloudai/unikorn/pkg/cmd/util"

	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

// deleteClusterOptions defines a set of options that are required to delete
// a cluster.
type deleteClusterOptions struct {
	// controlPlane defines the control plane name that the cluster will
	// be deprovisioned from.
	controlPlane string
}

// addFlags registers delete cluster options flags with the specified cobra command.
func (o *deleteClusterOptions) addFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&o.controlPlane, "control-plane", "", "Control plane to deprovision the cluster from.")

	if err := cmd.MarkFlagRequired("control-plane"); err != nil {
		panic(err)
	}
}

var (
	//nolint:gochecknoglobals
	deleteClusterExamples = util.TemplatedExample(`
        # Delete a Kubernetes cluster
        {{.Application}} delete cluster --control-plane foo`)
)

// newDeleteClusterCommand creates a command that deletes a Kubenretes cluster in the
// specified Cluster API control plane.
func newDeleteClusterCommand(_ cmdutil.Factory) *cobra.Command {
	o := &deleteClusterOptions{}

	cmd := &cobra.Command{
		Use:     "cluster",
		Short:   "Delete a Kubernetes cluster",
		Long:    "Delete a Kubernetes cluster",
		Example: deleteClusterExamples,
		Run: func(cmd *cobra.Command, args []string) {
		},
	}

	o.addFlags(cmd)

	return cmd
}
