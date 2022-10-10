package delete

import (
	"github.com/eschercloudai/unikorn/pkg/cmd/util"

	"github.com/spf13/cobra"

	"k8s.io/cli-runtime/pkg/genericclioptions"
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
	deleteClusterExamples = util.TemplatedExample(`
        # Delete a Kubernetes cluster
        {{.Application}} delete cluster --control-plane foo`)
)

// newDeleteClusterCommand creates a command that deletes a Kubenretes cluster in the
// specified Cluster API control plane.
func newDeleteClusterCommand(cf *genericclioptions.ConfigFlags) *cobra.Command {
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
