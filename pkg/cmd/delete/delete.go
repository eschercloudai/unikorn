package delete

import (
	"github.com/spf13/cobra"

	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// NewDeleteCommand creates a command that is responsible for deleting various resources.
func NewDeleteCommand(cf *genericclioptions.ConfigFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete Kubernetes clusters and resources",
		Long:  "Delete Kubernetes clusters and resources",
	}

	commands := []*cobra.Command{
		newDeleteControlPlaneCommand(cf),
		newDeleteClusterCommand(cf),
	}

	cmd.AddCommand(commands...)

	return cmd
}
