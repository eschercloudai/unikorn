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
	"github.com/eschercloudai/unikorn/pkg/cmd/aliases"
	"github.com/eschercloudai/unikorn/pkg/cmd/errors"
	"github.com/eschercloudai/unikorn/pkg/cmd/util"
	"github.com/eschercloudai/unikorn/pkg/cmd/util/completion"
	"github.com/eschercloudai/unikorn/pkg/cmd/util/flags"
	"github.com/eschercloudai/unikorn/pkg/constants"
	"github.com/eschercloudai/unikorn/pkg/providers/openstack"

	"k8s.io/apimachinery/pkg/api/resource"
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

	// computeAvailabilityZone defines in what Openstack failure domain the Kubernetes
	// cluster will be provisioned in.
	computeAvailabilityZone string

	// volumeAvailabilityZone defines in what Openstack failure domain volumes
	// will be provisoned in.
	volumeAvailabilityZone string

	// autoscaling allows the cluster to determine its own destiny, not the
	// user.
	autoscaling bool

	// minimumReplicas defines the minimum pool size for auto scaling.
	minimumReplicas int

	// maximumReplicas defines the maximum pool size for auto scaling.
	maximumReplicas int

	// labels defines labels on node creation.
	labels map[string]string

	// client gives access to our custom resources.
	client unikorn.Interface
}

// addFlags registers create cluster options flags with the specified cobra command.
func (o *createWorkloadPoolOptions) addFlags(f cmdutil.Factory, cmd *cobra.Command) {
	o.clusterFlags.AddFlags(f, cmd)

	// Openstack configuration options.
	flags.StringVarWithCompletion(cmd, &o.cloud, "cloud", "", "Cloud config to use within clouds.yaml.", completion.CloudCompletionFunc)

	// Kubernetes workload pool options.
	flags.RequiredVar(cmd, &o.version, "version", "Kubernetes version to deploy.  Provisioning will be faster if this matches the version preloaded on images defined by the --image flag.")
	flags.RequiredStringVarWithCompletion(cmd, &o.flavor, "flavor", "", "Kubernetes workload Openstack flavor (see: 'openstack flavor list'.)", completion.OpenstackFlavorCompletionFunc(&o.cloud))
	cmd.Flags().IntVar(&o.replicas, "replicas", defaultWorkloadReplicas, "Number of workload replicas.")
	flags.RequiredStringVarWithCompletion(cmd, &o.image, "image", "", "Openstack image (see: 'openstack image list'.)", completion.OpenstackImageCompletionFunc(&o.cloud))
	flags.StringVarWithCompletion(cmd, &o.computeAvailabilityZone, "compute-availability-zone", "", "Openstack availability zone to provision into. Will default to that specified for the control plane. (see: 'openstack availability zone list --compute'.)", completion.OpenstackComputeAvailabilityZoneCompletionFunc(&o.cloud))
	flags.StringVarWithCompletion(cmd, &o.volumeAvailabilityZone, "volume-availability-zone", "", "Openstack availability zone to provision into.  Will default to that specified for the control plane. (see: 'openstack availability zone list --volume'.)", completion.OpenstackVolumeAvailabilityZoneCompletionFunc(&o.cloud))
	cmd.Flags().Var(&o.diskSize, "disk-size", "Disk size, defaults to that of the machine flavor.")
	cmd.Flags().StringToStringVar(&o.labels, "labels", nil, "Labels to add on node creation. (format: key1=value1,key2=value2)")

	// Feature enablement.
	cmd.Flags().BoolVar(&o.autoscaling, "enable-autoscaling", false, "Enables workload pool auto-scaling. To function, you must enable autoscaling on the cluster.")
	cmd.Flags().IntVar(&o.minimumReplicas, "minimum-replicas", 3, "Set the minimum number of auto-scaling replicas.")
	cmd.Flags().IntVar(&o.maximumReplicas, "maximum-replicas", 10, "Set the maximum number of auto-scaling replicas.")
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

// applyAutoscaling adds any autoscaling configuration to the cluster.
func (o *createWorkloadPoolOptions) applyAutoscaling(workloadPool *unikornv1alpha1.KubernetesWorkloadPool) error {
	if !o.autoscaling {
		return nil
	}

	compute, err := openstack.NewComputeClient(openstack.NewCloudsProvider(o.cloud))
	if err != nil {
		return err
	}

	flavor, err := compute.Flavor(context.Background(), o.flavor)
	if err != nil {
		return err
	}

	flavorExtraSpecs, err := compute.FlavorExtraSpecs(context.Background(), flavor)
	if err != nil {
		return err
	}

	memory, err := resource.ParseQuantity(fmt.Sprintf("%dMi", flavor.RAM))
	if err != nil {
		return err
	}

	workloadPool.Spec.Autoscaling = &unikornv1alpha1.MachineGenericAutoscaling{
		MinimumReplicas: &o.minimumReplicas,
		MaximumReplicas: &o.maximumReplicas,
		Scheduler: &unikornv1alpha1.MachineGenericAutoscalingScheduler{
			CPU:    &flavor.VCPUs,
			Memory: &memory,
		},
	}

	gpu, ok, err := openstack.FlavorGPUs(flavor, flavorExtraSpecs)
	if err != nil {
		return err
	}

	if ok {
		gpuType := constants.NvidiaGPUType

		workloadPool.Spec.Autoscaling.Scheduler.GPU = &unikornv1alpha1.MachineGenericAutoscalingSchedulerGPU{
			Type:  &gpuType,
			Count: &gpu.GPUs,
		}
	}

	return nil
}

// run executes the command.
func (o *createWorkloadPoolOptions) run() error {
	namespace, err := o.clusterFlags.GetControlPlaneNamespace(context.TODO(), o.client)
	if err != nil {
		return err
	}

	name := o.clusterFlags.Cluster + "-" + o.name

	version := unikornv1alpha1.SemanticVersion(o.version.Semver)

	workloadPool := &unikornv1alpha1.KubernetesWorkloadPool{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				constants.VersionLabel:           constants.Version,
				constants.ProjectLabel:           o.clusterFlags.Project,
				constants.ControlPlaneLabel:      o.clusterFlags.ControlPlane,
				constants.KubernetesClusterLabel: o.clusterFlags.Cluster,
			},
		},
		Spec: unikornv1alpha1.KubernetesWorkloadPoolSpec{
			Name:   &o.name,
			Labels: o.labels,
			MachineGeneric: unikornv1alpha1.MachineGeneric{
				Version:  &version,
				Image:    &o.image,
				Flavor:   &o.flavor,
				Replicas: &o.replicas,
				DiskSize: o.diskSize.Quantity,
			},
		},
	}

	if o.computeAvailabilityZone != "" {
		workloadPool.Spec.FailureDomain = &o.computeAvailabilityZone
	}

	if o.volumeAvailabilityZone != "" {
		workloadPool.Spec.VolumeFailureDomain = &o.volumeAvailabilityZone
	}

	if err := o.applyAutoscaling(workloadPool); err != nil {
		return err
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
	o := &createWorkloadPoolOptions{
		clusterFlags: flags.ClusterFlags{
			ClusterRequired: true,
		},
	}

	cmd := &cobra.Command{
		Use:     "workload-pool",
		Short:   "Create a Kubernetes cluster workload pool",
		Long:    createWorkloadPoolLong,
		Example: createWorkloadPoolExamples,
		Aliases: aliases.WorkloadPool,
		Run: func(cmd *cobra.Command, args []string) {
			util.AssertNilError(o.complete(f, args))
			util.AssertNilError(o.validate())
			util.AssertNilError(o.run())
		},
	}

	o.addFlags(f, cmd)

	return cmd
}
