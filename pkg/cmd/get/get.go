package get

import (
	"github.com/spf13/cobra"

	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// NewGetCommand returns a command that can list all resources, or get information
// about a single one.
func NewGetCommand(cf *genericclioptions.ConfigFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get Kubernetes clusters and resources",
		Long:  "Get Kubernetes clusters and resources",
	}

	commands := []*cobra.Command{
		newGetControlPlaneCommand(cf),
		newGetClusterCommand(cf),
	}

	cmd.AddCommand(commands...)

	return cmd
}
