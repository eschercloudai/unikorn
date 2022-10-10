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
