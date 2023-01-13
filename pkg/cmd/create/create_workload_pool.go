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
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/eschercloudai/unikorn/generated/clientset/unikorn"
	unikornv1alpha1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/cmd/errors"
	"github.com/eschercloudai/unikorn/pkg/cmd/util"
	"github.com/eschercloudai/unikorn/pkg/cmd/util/completion"
	"github.com/eschercloudai/unikorn/pkg/cmd/util/flags"
	"github.com/eschercloudai/unikorn/pkg/constants"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"
)

const (
	defaultWorkloadReplicas = 3
)

// createWorkloadPoolOptions defines a set of options that are required to create
// a cluster.
type createWorkloadPoolOptions struct {
	// clusterFlags define control plane scoping.
	clusterFlags flags.ClusterFlags

	// cluster is the cluster name this belongs to.
	cluster string

	// cloud indicates the clouds.yaml key to use.  If only one exists it
	// will default to that, otherwise it's a required parameter.
	cloud string

	// name is the name of the workload pool.
	name string

	// version defines the Kubernetes version to install.
	version flags.SemverFlag

	// image defines the Openstack image for Kubernetes nodes.
	image string

	// flavor defines the Openstack VM flavor.
	flavor string

	// replicas defines the number of replicas (nodes).
	replicas int

	// diskSize defines the persistent volume size to provision with.
	diskSize flags.QuantityFlag

	// availabilityZone defines in what Openstack failure domain the Kubernetes
	// cluster will be provisioned in.
	availabilityZone string

	// client gives access to our custom resources.
	client unikorn.Interface
}

// addFlags registers create cluster options flags with the specified cobra command.
func (o *createWorkloadPoolOptions) addFlags(f cmdutil.Factory, cmd *cobra.Command) {
	// TODO: make required.
	o.clusterFlags.AddFlags(f, cmd)

	// Openstack configuration options.
	flags.StringVarWithCompletion(cmd, &o.cloud, "cloud", "", "Cloud config to use within clouds.yaml.", completion.CloudCompletionFunc)

	// Kubernetes workload pool options.
	flags.RequiredVar(cmd, &o.version, "version", "Kubernetes version to deploy.  Provisioning will be faster if this matches the version preloaded on images defined by the --image flag.")
	flags.RequiredStringVarWithCompletion(cmd, &o.flavor, "flavor", "", "Kubernetes workload Openstack flavor (see: 'openstack flavor list'.)", completion.OpenstackFlavorCompletionFunc(&o.cloud))
	cmd.Flags().IntVar(&o.replicas, "replicas", defaultWorkloadReplicas, "Number of workload replicas.")
	flags.RequiredStringVarWithCompletion(cmd, &o.image, "image", "", "Openstack image (see: 'openstack image list'.)", completion.OpenstackImageCompletionFunc(&o.cloud))
	flags.StringVarWithCompletion(cmd, &o.availabilityZone, "availability-zone", "", "Openstack availability zone to provision into. Will default to that specified for the control plane. (see: 'openstack availability zone list'.)", completion.OpenstackAvailabilityZoneCompletionFunc(&o.cloud))
	cmd.Flags().Var(&o.diskSize, "disk-size", "Disk size, defaults to that of the machine flavor.")
}

// complete fills in any options not does automatically by flag parsing.
func (o *createWorkloadPoolOptions) complete(f cmdutil.Factory, args []string) error {
	config, err := f.ToRESTConfig()
	if err != nil {
		return err
	}

	if o.client, err = unikorn.NewForConfig(config); err != nil {
		return err
	}

	if len(args) != 1 {
		return errors.ErrIncorrectArgumentNum
	}

	o.name = args[0]

	return nil
}

// validate validates any tainted input not handled by complete() or flags
// processing.
func (o *createWorkloadPoolOptions) validate() error {
	return nil
}

// run executes the command.
func (o *createWorkloadPoolOptions) run() error {
	namespace, err := o.clusterFlags.GetControlPlaneNamespace(context.TODO(), o.client)
	if err != nil {
		return err
	}

	name := o.cluster + "-" + o.name

	version := unikornv1alpha1.SemanticVersion(o.version.Semver)

	workloadPool := &unikornv1alpha1.KubernetesWorkloadPool{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				constants.VersionLabel:           constants.Version,
				constants.ProjectLabel:           o.clusterFlags.Project,
				constants.ControlPlaneLabel:      o.clusterFlags.ControlPlane,
				constants.KubernetesClusterLabel: o.cluster,
			},
		},
		Spec: unikornv1alpha1.KubernetesWorkloadPoolSpec{
			Name: &o.name,
			MachineGeneric: unikornv1alpha1.MachineGeneric{
				Version:  &version,
				Image:    &o.image,
				Flavor:   &o.flavor,
				Replicas: &o.replicas,
				DiskSize: o.diskSize.Quantity,
			},
		},
	}

	if _, err := o.client.UnikornV1alpha1().KubernetesWorkloadPools(namespace).Create(context.TODO(), workloadPool, metav1.CreateOptions{}); err != nil {
		return err
	}

	fmt.Printf("%s.%s/%s created\n", unikornv1alpha1.KubernetesWorkloadPoolResource, unikornv1alpha1.GroupName, name)

	return nil
}

var (
	//nolint:gochecknoglobals
	createWorkloadPoolLong = templates.LongDesc(`
	Create a Kubernetes cluster workload pool.

	This command defines workload pools for a Kubernetes cluster.  You can define
	pools before, or after creating a cluster.  The former will provision them
	during control plane scale up and result in faster start-up times.  The latter
	will provision the pools after the cluster is fully scaled and healthy.

	The --cloud flag is optional, but it's recommended to set it to allow tab
	completion on flavor and image parameters.`)

	//nolint:gochecknoglobals
	createWorkloadPoolExamples = util.TemplatedExample(`
        # Create a Kubernetes cluster workload pool
        {{.Application}} create workload-pool --project foo --control-plane bar --cluster foo --flavor g.medium_a100_MIG_2g.20gb --version v1.24.7 --image ubuntu-2004-kube-v1.24.7 baz`)
)

// newCreateWorkloadPoolCommand creates a command that is able to provison a new Kubernetes
// cluster with a Cluster API control plane.
func newCreateWorkloadPoolCommand(f cmdutil.Factory) *cobra.Command {
	o := createWorkloadPoolOptions{}

	cmd := &cobra.Command{
		Use:     "workload-pool",
		Short:   "Create a Kubernetes cluster workload pool",
		Long:    createWorkloadPoolLong,
		Example: createWorkloadPoolExamples,
		Run: func(cmd *cobra.Command, args []string) {
			util.AssertNilError(o.complete(f, args))
			util.AssertNilError(o.validate())
			util.AssertNilError(o.run())
		},
	}

	o.addFlags(f, cmd)

	return cmd
}
