/*
Copyright 2022-2023 EscherCloud.

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

package cluster

import (
	"context"

	unikornv1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/provisioners"
	"github.com/eschercloudai/unikorn/pkg/provisioners/concurrent"
	"github.com/eschercloudai/unikorn/pkg/provisioners/conditional"
	"github.com/eschercloudai/unikorn/pkg/provisioners/helmapplications/cilium"
	"github.com/eschercloudai/unikorn/pkg/provisioners/helmapplications/clusterautoscaler"
	"github.com/eschercloudai/unikorn/pkg/provisioners/helmapplications/clusteropenstack"
	"github.com/eschercloudai/unikorn/pkg/provisioners/helmapplications/nvidiagpuoperator"
	"github.com/eschercloudai/unikorn/pkg/provisioners/helmapplications/openstackcloudprovider"
	"github.com/eschercloudai/unikorn/pkg/provisioners/helmapplications/vcluster"
	"github.com/eschercloudai/unikorn/pkg/provisioners/remotecluster"
	"github.com/eschercloudai/unikorn/pkg/provisioners/serial"
	"github.com/eschercloudai/unikorn/pkg/provisioners/util"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Provisioner encapsulates control plane provisioning.
type Provisioner struct {
	// client provides access to Kubernetes.
	client client.Client

	// controlPlaneRemote is the remote cluster to deploy to.
	controlPlaneRemote provisioners.RemoteCluster

	// cluster is the Kubernetes cluster we're provisioning.
	cluster *unikornv1.KubernetesCluster

	clusterOpenstackApplication       *unikornv1.HelmApplication
	ciliumApplication                 *unikornv1.HelmApplication
	openstackCloudProviderApplication *unikornv1.HelmApplication
	nvidiaGPUOperatorApplication      *unikornv1.HelmApplication
	clusterAutoscalerApplication      *unikornv1.HelmApplication
}

// New returns a new initialized provisioner object.
func New(ctx context.Context, client client.Client, cluster *unikornv1.KubernetesCluster) (*Provisioner, error) {
	provisioner := &Provisioner{
		client:             client,
		controlPlaneRemote: vcluster.NewRemoteClusterGenerator(client, cluster.Namespace, provisioners.VclusterRemoteLabelsFromCluster(cluster)),
		cluster:            cluster,
	}

	unbundler := util.NewUnbundler(cluster, unikornv1.ApplicationBundleResourceKindKubernetesCluster)
	unbundler.AddApplication(&provisioner.clusterOpenstackApplication, "cluster-openstack")
	unbundler.AddApplication(&provisioner.ciliumApplication, "cilium")
	unbundler.AddApplication(&provisioner.openstackCloudProviderApplication, "openstack-cloud-provider")
	unbundler.AddApplication(&provisioner.nvidiaGPUOperatorApplication, "nvidia-gpu-operator")
	unbundler.AddApplication(&provisioner.clusterAutoscalerApplication, "cluster-autoscaler")

	if err := unbundler.Unbundle(ctx, client); err != nil {
		return nil, err
	}

	return provisioner, nil
}

// Ensure the Provisioner interface is implemented.
var _ provisioners.Provisioner = &Provisioner{}

// OnRemote implements the Provision interface.
func (p *Provisioner) OnRemote(_ provisioners.RemoteCluster) provisioners.Provisioner {
	return p
}

// InNamespace implements the Provision interface.
func (p *Provisioner) InNamespace(_ string) provisioners.Provisioner {
	return p
}

// getRemoteClusterGenerator returns a generator capable of reading the cluster
// kubeconfig from the underlying control plane.
func (p *Provisioner) getRemoteClusterGenerator(ctx context.Context) (*clusteropenstack.RemoteClusterGenerator, error) {
	client, err := vcluster.NewControllerRuntimeClient(p.client).Client(ctx, p.cluster.Namespace, false)
	if err != nil {
		return nil, err
	}

	return clusteropenstack.NewRemoteClusterGenerator(client, p.cluster.Namespace, p.cluster.Name, provisioners.ClusterOpenstackLabelsFromCluster(p.cluster)), nil
}

func (p *Provisioner) newClusterAutoscalerProvisioner() provisioners.Provisioner {
	return clusterautoscaler.New(p.client, p.cluster, p.clusterAutoscalerApplication, p.cluster.Name, p.cluster.Name+"-kubeconfig").OnRemote(p.controlPlaneRemote).InNamespace(p.cluster.Name)
}

// getAddonsProvisioner returns a generic provisioner for provisioning and deprovisioning.
func (p *Provisioner) getAddonsProvisioner(ctx context.Context) (provisioners.Provisioner, error) {
	remote, err := p.getRemoteClusterGenerator(ctx)
	if err != nil {
		return nil, err
	}

	// Provision the remote cluster, then once that's configured, install
	// the CNI and cloud provider in parallel.
	// NOTE: that nvidia is installed after the CNI and OCP controllers.
	// This application depends on the CNI to actually deploy, so while you
	// can do this in parallel and it'll work, when you deprovision you can
	// get stuck with the CNI gone, and the nvidia stuff needing the CNI to
	// uninstall properly.
	provisioner := serial.New("cluster add-ons",
		remotecluster.New(p.client, remote),
		concurrent.New("cluster add-ons",
			cilium.New(p.client, p.cluster, p.ciliumApplication).OnRemote(remote),
			openstackcloudprovider.New(p.client, p.cluster, p.openstackCloudProviderApplication).OnRemote(remote),
		),
		nvidiagpuoperator.New(p.client, p.cluster, p.nvidiaGPUOperatorApplication).OnRemote(remote),
	)

	return provisioner, nil
}

// Provision implements the Provision interface.
func (p *Provisioner) Provision(ctx context.Context) error {
	clusterProvisioner, err := clusteropenstack.New(ctx, p.client, p.cluster, p.clusterOpenstackApplication)
	if err != nil {
		return err
	}

	clusterProvisioner.OnRemote(p.controlPlaneRemote).InNamespace(p.cluster.Name)

	addonsProvisioner, err := p.getAddonsProvisioner(ctx)
	if err != nil {
		return err
	}

	// TODO: you can create with autoscaling on, turn it on, but not remove it.
	// That would require tracking of some variety.
	provisioner := serial.New("kubernetes cluster",
		concurrent.New("kubernetes cluster",
			clusterProvisioner,
			addonsProvisioner,
		),
		conditional.New("cluster-autoscaler",
			p.cluster.AutoscalingEnabled,
			p.newClusterAutoscalerProvisioner(),
		),
	)

	if err := provisioner.Provision(ctx); err != nil {
		return err
	}

	return nil
}

// Deprovision implements the Provision interface.
func (p *Provisioner) Deprovision(ctx context.Context) error {
	clusterProvisioner, err := clusteropenstack.New(ctx, p.client, p.cluster, p.clusterOpenstackApplication)
	if err != nil {
		return err
	}

	clusterProvisioner.OnRemote(p.controlPlaneRemote).InNamespace(p.cluster.Name)

	addonsProvisioner, err := p.getAddonsProvisioner(ctx)
	if err != nil {
		return err
	}

	// Remove the addons first, Argo will probably have a fit if the cluster vanishes
	// before it has a chance to delete the contained add-on applications.
	provisioner := serial.New("kubernetes cluster",
		clusterProvisioner,
		addonsProvisioner,
		conditional.New("cluster-autoscaler",
			p.cluster.AutoscalingEnabled,
			p.newClusterAutoscalerProvisioner(),
		),
	)

	if err := provisioner.Deprovision(ctx); err != nil {
		return err
	}

	return nil
}
