package create

import (
	"github.com/spf13/cobra"

	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// NewCreateCommand creates a command that allows creation of various resources.
func NewCreateCommand(cf *genericclioptions.ConfigFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create Kubernetes clusters and resources.",
		Long:  "Create Kubernetes clusters and resources.",
	}

	commands := []*cobra.Command{
		newCreateControlPlaneCommand(cf),
		newCreateClusterCommand(cf),
	}

	cmd.AddCommand(commands...)

	return cmd
}
