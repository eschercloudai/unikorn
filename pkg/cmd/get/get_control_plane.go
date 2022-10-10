package get

import (
	"github.com/spf13/cobra"

	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// newGetControlPlaneCommand returns a command that is able to get or list Cluster API
// control planes found on the management cluster.
func newGetControlPlaneCommand(cf *genericclioptions.ConfigFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "control-plane",
		Short: "Get or list Cluster API control planes",
		Long:  "Get or list Cluster API control planes",
		Run: func(cmd *cobra.Command, args []string) {
		},
	}

	return cmd
}
