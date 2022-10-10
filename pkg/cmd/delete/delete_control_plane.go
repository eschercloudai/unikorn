package delete

import (
	"github.com/spf13/cobra"

	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// newDeleteControlPlaneCommand creates a command that deletes a Cluster API control plane.
func newDeleteControlPlaneCommand(cf *genericclioptions.ConfigFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "control-plane",
		Short: "Delete a Kubernetes cluster",
		Long:  "Delete a Kubernetes cluster",
		Run: func(cmd *cobra.Command, args []string) {
		},
	}

	return cmd
}
