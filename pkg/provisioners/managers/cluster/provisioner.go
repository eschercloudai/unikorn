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
	"github.com/eschercloudai/unikorn/pkg/cd"
	"github.com/eschercloudai/unikorn/pkg/cd/argocd"
	"github.com/eschercloudai/unikorn/pkg/provisioners"
	"github.com/eschercloudai/unikorn/pkg/provisioners/concurrent"
	"github.com/eschercloudai/unikorn/pkg/provisioners/conditional"
	"github.com/eschercloudai/unikorn/pkg/provisioners/helmapplications/certmanager"
	"github.com/eschercloudai/unikorn/pkg/provisioners/helmapplications/certmanagerissuers"
	"github.com/eschercloudai/unikorn/pkg/provisioners/helmapplications/cilium"
	"github.com/eschercloudai/unikorn/pkg/provisioners/helmapplications/clusterautoscaler"
	"github.com/eschercloudai/unikorn/pkg/provisioners/helmapplications/clusterautoscaleropenstack"
	"github.com/eschercloudai/unikorn/pkg/provisioners/helmapplications/clusteropenstack"
	"github.com/eschercloudai/unikorn/pkg/provisioners/helmapplications/kubernetesdashboard"
	"github.com/eschercloudai/unikorn/pkg/provisioners/helmapplications/longhorn"
	"github.com/eschercloudai/unikorn/pkg/provisioners/helmapplications/metricsserver"
	"github.com/eschercloudai/unikorn/pkg/provisioners/helmapplications/nginxingress"
	"github.com/eschercloudai/unikorn/pkg/provisioners/helmapplications/nvidiagpuoperator"
	"github.com/eschercloudai/unikorn/pkg/provisioners/helmapplications/openstackcloudprovider"
	"github.com/eschercloudai/unikorn/pkg/provisioners/helmapplications/openstackplugincindercsi"
	"github.com/eschercloudai/unikorn/pkg/provisioners/helmapplications/prometheus"
	"github.com/eschercloudai/unikorn/pkg/provisioners/helmapplications/vcluster"
	"github.com/eschercloudai/unikorn/pkg/provisioners/remotecluster"
	"github.com/eschercloudai/unikorn/pkg/provisioners/serial"
	"github.com/eschercloudai/unikorn/pkg/provisioners/util"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Provisioner encapsulates control plane provisioning.
type Provisioner struct {
	provisioners.ProvisionerMeta

	// client provides access to Kubernetes.
	client client.Client

	// controlPlaneRemote is the remote cluster to deploy to.
	controlPlaneRemote provisioners.RemoteCluster

	// cluster is the Kubernetes cluster we're provisioning.
	cluster *unikornv1.KubernetesCluster

	clusterOpenstackApplication           *unikornv1.HelmApplication
	ciliumApplication                     *unikornv1.HelmApplication
	openstackCloudProviderApplication     *unikornv1.HelmApplication
	openstackPluginCinderCSIApplication   *unikornv1.HelmApplication
	metricsServerApplication              *unikornv1.HelmApplication
	nvidiaGPUOperatorApplication          *unikornv1.HelmApplication
	clusterAutoscalerApplication          *unikornv1.HelmApplication
	clusterAutoscalerOpenStackApplication *unikornv1.HelmApplication
	nginxIngressApplication               *unikornv1.HelmApplication
	certManagerApplication                *unikornv1.HelmApplication
	certManagerIssuersApplication         *unikornv1.HelmApplication
	kubernetesDashboardApplication        *unikornv1.HelmApplication
	longhornApplication                   *unikornv1.HelmApplication
	prometheusApplication                 *unikornv1.HelmApplication
}

// New returns a new initialized provisioner object.
func New(ctx context.Context, client client.Client, cluster *unikornv1.KubernetesCluster) (*Provisioner, error) {
	provisioner := &Provisioner{
		client:             client,
		controlPlaneRemote: vcluster.NewRemoteClusterGenerator(client, cluster.Namespace, provisioners.VclusterRemoteLabelsFromCluster(cluster)),
		cluster:            cluster,
	}

	// TODO: need to remove optional falg once old cluster bundles are retired.
	unbundler := util.NewUnbundler(cluster, unikornv1.ApplicationBundleResourceKindKubernetesCluster)
	unbundler.AddApplication(&provisioner.clusterOpenstackApplication, "cluster-openstack")
	unbundler.AddApplication(&provisioner.ciliumApplication, "cilium")
	unbundler.AddApplication(&provisioner.openstackCloudProviderApplication, "openstack-cloud-provider")
	unbundler.AddApplication(&provisioner.openstackPluginCinderCSIApplication, "openstack-plugin-cinder-csi")
	unbundler.AddApplication(&provisioner.nvidiaGPUOperatorApplication, "nvidia-gpu-operator")
	unbundler.AddApplication(&provisioner.clusterAutoscalerApplication, "cluster-autoscaler")
	unbundler.AddApplication(&provisioner.clusterAutoscalerOpenStackApplication, "cluster-autoscaler-openstack", util.Optional)
	unbundler.AddApplication(&provisioner.metricsServerApplication, "metrics-server")
	unbundler.AddApplication(&provisioner.nginxIngressApplication, "nginx-ingress", util.Optional)
	unbundler.AddApplication(&provisioner.certManagerApplication, "cert-manager", util.Optional)
	unbundler.AddApplication(&provisioner.certManagerIssuersApplication, "cert-manager-issuers", util.Optional)
	unbundler.AddApplication(&provisioner.kubernetesDashboardApplication, "kubernetes-dashboard", util.Optional)
	unbundler.AddApplication(&provisioner.longhornApplication, "longhorn", util.Optional)
	unbundler.AddApplication(&provisioner.prometheusApplication, "prometheus", util.Optional)

	if err := unbundler.Unbundle(ctx, client); err != nil {
		return nil, err
	}

	return provisioner, nil
}

// Ensure the Provisioner interface is implemented.
var _ provisioners.Provisioner = &Provisioner{}

// getRemoteClusterGenerator returns a generator capable of reading the cluster
// kubeconfig from the underlying control plane.
func (p *Provisioner) getRemoteClusterGenerator(ctx context.Context) (*clusteropenstack.RemoteClusterGenerator, error) {
	client, err := vcluster.NewControllerRuntimeClient(p.client).Client(ctx, p.cluster.Namespace, false)
	if err != nil {
		return nil, err
	}

	return clusteropenstack.NewRemoteClusterGenerator(client, p.cluster), nil
}

func (p *Provisioner) newClusterAutoscalerProvisioner(driver cd.Driver) provisioners.Provisioner {
	return clusterautoscaler.New(driver, p.cluster, p.clusterAutoscalerApplication).OnRemote(p.controlPlaneRemote).InNamespace(p.cluster.Name)
}

func (p *Provisioner) newClusterAutoscalerOpenStackProvisioner(driver cd.Driver) provisioners.Provisioner {
	return clusterautoscaleropenstack.New(driver, p.cluster, p.clusterAutoscalerOpenStackApplication).OnRemote(p.controlPlaneRemote).InNamespace(p.cluster.Name)
}

// getBootstrapProvisioner installs the remote cluster, cloud controller manager
// and CNI in parallel with cluster creation.  NOTE: these applications MUST be
// installable without an worker nodes, thus all deployments must tolerate control
// plane taints, and select control plane nodes to support zero-sized workload pools
// correctly.
func (p *Provisioner) getBootstrapProvisioner(ctx context.Context, driver cd.Driver) (provisioners.Provisioner, error) {
	remote, err := p.getRemoteClusterGenerator(ctx)
	if err != nil {
		return nil, err
	}

	provisioner := serial.New("cluster add-ons",
		remotecluster.New(driver, remote),
		concurrent.New("cluster bootstrap",
			cilium.New(driver, p.cluster, p.ciliumApplication).OnRemote(remote).BackgroundDelete(),
			openstackcloudprovider.New(driver, p.cluster, p.openstackCloudProviderApplication).OnRemote(remote).BackgroundDelete(),
		),
	)

	return provisioner, nil
}

// getAddonsProvisioner returns a generic provisioner for provisioning and deprovisioning.
// Unlike bootstrap components, these don't necessarily need to be foreced onto the control
// plane nodes, and we shouldn't be expected to foot the bill for everything.
func (p *Provisioner) getAddonsProvisioner(ctx context.Context, driver cd.Driver) (provisioners.Provisioner, error) {
	remote, err := p.getRemoteClusterGenerator(ctx)
	if err != nil {
		return nil, err
	}

	// Provision the remote cluster, then once that's configured, install
	// the CNI and cloud provider in parallel.
	provisioner := serial.New("cluster add-ons",
		concurrent.New("cluster add-ons wave 1",
			openstackplugincindercsi.New(driver, p.cluster, p.openstackPluginCinderCSIApplication).OnRemote(remote).BackgroundDelete(),
			metricsserver.New(driver, p.cluster, p.metricsServerApplication).OnRemote(remote).BackgroundDelete(),
			conditional.New("nvidia-gpu-operator", p.cluster.NvidiaOperatorEnabled, nvidiagpuoperator.New(driver, p.cluster, p.nvidiaGPUOperatorApplication).OnRemote(remote).BackgroundDelete()),
			conditional.New("nginx-ingress", p.cluster.IngressEnabled, nginxingress.New(driver, p.cluster, p.nginxIngressApplication).OnRemote(remote).BackgroundDelete()),
			conditional.New("cert-manager", p.cluster.CertManagerEnabled,
				serial.New("cert-manager",
					certmanager.New(driver, p.cluster, p.certManagerApplication).OnRemote(remote).BackgroundDelete(),
					certmanagerissuers.New(driver, p.cluster, p.certManagerIssuersApplication).OnRemote(remote).BackgroundDelete(),
				),
			),
			conditional.New("longhorn", p.cluster.FileStorageEnabled, longhorn.New(driver, p.cluster, p.longhornApplication).OnRemote(remote).BackgroundDelete()),
			conditional.New("prometheus", p.cluster.PrometheusEnabled, prometheus.New(driver, p.cluster, p.prometheusApplication).OnRemote(remote).BackgroundDelete()),
		),
		concurrent.New("cluster add-ons wave 2",
			conditional.New("kubernetes-dashboard", p.cluster.KubernetesDashboardEnabled, kubernetesdashboard.New(driver, p.cluster, p.kubernetesDashboardApplication, remote).OnRemote(remote).BackgroundDelete()),
		),
	)

	return provisioner, nil
}

func (p *Provisioner) getProvisioner(ctx context.Context, driver cd.Driver) (provisioners.Provisioner, error) {
	clusterProvisioner, err := clusteropenstack.New(ctx, driver, p.cluster, p.clusterOpenstackApplication)
	if err != nil {
		return nil, err
	}

	// TODO: this is ugly, consider options pattern?
	clusterProvisioner.OnRemote(p.controlPlaneRemote).InNamespace(p.cluster.Name)

	bootstrapProvisioner, err := p.getBootstrapProvisioner(ctx, driver)
	if err != nil {
		return nil, err
	}

	addonsProvisioner, err := p.getAddonsProvisioner(ctx, driver)
	if err != nil {
		return nil, err
	}

	// Create the cluster and the boostrap components in parallel, the cluster will
	// come up but never reach healthy until the CNI and cloud controller manager
	// are added.  Follow that up by the autoscaler as some addons may require worker
	// nodes to schedule onto.
	provisioner := serial.New("kubernetes cluster",
		concurrent.New("kubernetes cluster",
			clusterProvisioner,
			bootstrapProvisioner,
		),
		conditional.New("cluster-autoscaler",
			p.cluster.AutoscalingEnabled,
			concurrent.New("cluster-autoscaler",
				p.newClusterAutoscalerProvisioner(driver),
				// TODO: this came in 1.2.0, so is not present in 1.1.0 thus
				// needs to be optional temporarily otherwise everything will break
				// on older clusters.
				conditional.New("cluster-autoscaler-openstack",
					func() bool {
						return p.clusterAutoscalerOpenStackApplication != nil
					},
					p.newClusterAutoscalerOpenStackProvisioner(driver),
				),
			),
		),
		addonsProvisioner,
	)

	return provisioner, nil
}

// Provision implements the Provision interface.
func (p *Provisioner) Provision(ctx context.Context) error {
	client, err := argocd.NewInCluster(ctx, p.client)
	if err != nil {
		return err
	}

	driver := argocd.NewDriver(p.client, client)

	provisioner, err := p.getProvisioner(ctx, driver)
	if err != nil {
		return err
	}

	if err := provisioner.Provision(ctx); err != nil {
		return err
	}

	return nil
}

// Deprovision implements the Provision interface.
func (p *Provisioner) Deprovision(ctx context.Context) error {
	client, err := argocd.NewInCluster(ctx, p.client)
	if err != nil {
		return err
	}

	driver := argocd.NewDriver(p.client, client)

	provisioner, err := p.getProvisioner(ctx, driver)
	if err != nil {
		return err
	}

	if err := provisioner.Deprovision(ctx); err != nil {
		return err
	}

	return nil
}
