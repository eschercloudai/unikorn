package command

import (
	"fmt"

	"github.com/eschercloudai/unikorn/pkg/command/util"
	"github.com/eschercloudai/unikorn/pkg/constants"

	"github.com/spf13/cobra"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/kubectl/pkg/util/templates"
)

var (
	rootLongDesc = templates.LongDesc(`
	EscherCloudAI Kubernetes Provisioning.

	This CLI toolset provides dynamic provisioning of Kubernetes clusters
	and Cluster API control planes.  It also provides various Kubernetes
	cluster life-cycle management functions.  For additional details on
	how the individual components operatate, see the individual 'create'
	help topics.

	This tool is designed with flexibility in mind, so doesn't force any
	sort of cardinality between control planes and clusters, however as
	the control plane management is somewhat new, there are bugs that
	suggest a 1:1 mapping is best intiial for teardown operations.  In the
	future, some upstream hardening will allow 1:N and better resource
	utilisation.`)
)

// newRootCommand returns the root command and all its subordinates.
// This sets global flags for standard Kubernetes configuration options
// such as --kubeconfig, --context, --namespace, etc.
func newRootCommand(cf *genericclioptions.ConfigFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   constants.Application,
		Short: "EscherCloudAI Kubernetes Provisioning.",
		Long:  rootLongDesc,
	}

	cf.AddFlags(cmd.PersistentFlags())

	commands := []*cobra.Command{
		newVersionCommand(),
		newCreateCommand(cf),
		newDeleteCommand(cf),
		newGetCommand(cf),
	}

	cmd.AddCommand(commands...)

	return cmd
}

// newVersionCommand returns a version command that prints out application
// and versioning information.
func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print this command's version.",
		Long:  "Print this command's version.",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(constants.VersionString())
		},
	}
}

// newCreateCommand creates a command that allows creation of various resources.
func newCreateCommand(cf *genericclioptions.ConfigFlags) *cobra.Command {
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

// DynamicTemplateOptions allows some parameters to be passed into help text
// and that text to be templated so it will update automatically when the
// options do.
type DynamicTemplateOptions struct {
	// Application is the application name as defined by argv[0].
	Application string
}

// newDynamicTemplateOptions returns am intialiized template options struct.
func newDynamicTemplateOptions() *DynamicTemplateOptions {
	return &DynamicTemplateOptions{
		Application: constants.Application,
	}
}

// templatedExample applies a templating function to the example string so
// we can make the text dyanamic.  It also applies standard Kubernetes
// formatting rules after the templating step.
func templatedExample(s string) string {
	s = util.TemplatedString(s, newDynamicTemplateOptions())
	return templates.Examples(s)
}

var (
	createControlPlaneLong = templates.LongDesc(`
	Create a Cluster API control plane.

	Control planes are modelled on Kubernetes namespaces, this gives
	us a primitive to label, and annotate, to aid in life-cycle management.

	Each control plane namespace will contain an instance of a loft.io
	vcluster.  The use of vclusters allows a level of isolation between
	users in a multi-tenancy environment.  It also allows trivial deletion
	of resources contained within that vcluster as that is not subject
	to finalizers and the like (Cluster API is poorly tested in failure
	scenarios.)`)

	createControlPlaneExample = templatedExample(`
	# Create a control plane with a generated name.
	{{.Application}} create control-plane

	# Create a control plane with an explcit name.
	{{.Application}} create control-plane foo`)
)

// newCreateControlPlaneCommand creates a command that can create control planes.
// The initial intention is to have a per-user/organization control plane that
// contains Cluster API in a virtual cluster
func newCreateControlPlaneCommand(cf *genericclioptions.ConfigFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "control-plane",
		Short:   "Create a Cluster API control plane.",
		Long:    createControlPlaneLong,
		Example: createControlPlaneExample,
		Run: func(cmd *cobra.Command, args []string) {
		},
	}

	return cmd
}

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
	createClusterExamples = templatedExample(`
	# Create a Kubernetes cluster
	{{.Application}} create cluster --control-plane foo`)
)

// newCreateClusterCommand creates a command that is able to provison a new Kubernetes
// cluster with a Cluster API control plane.
func newCreateClusterCommand(cf *genericclioptions.ConfigFlags) *cobra.Command {
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

// newDeleteCommand creates a command that is responsible for deleting various resources.
func newDeleteCommand(cf *genericclioptions.ConfigFlags) *cobra.Command {
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
	deleteClusterExamples = templatedExample(`
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

// newGetCommand returns a command that can list all resources, or get information
// about a single one.
func newGetCommand(cf *genericclioptions.ConfigFlags) *cobra.Command {
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
	getClusterExamples = templatedExample(`
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

// Generate creates a hierarchy of cobra commands for the application.  It can
// also be used to walk the structure and generate HTML documentation for example.
func Generate() *cobra.Command {
	cf := genericclioptions.NewConfigFlags(true)

	cmd := newRootCommand(cf)

	return cmd
}
