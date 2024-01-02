/*
Copyright 2022-2024 EscherCloud.

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
	clientlib "github.com/eschercloudai/unikorn/pkg/client"
	"github.com/eschercloudai/unikorn/pkg/constants"
	"github.com/eschercloudai/unikorn/pkg/provisioners"
	"github.com/eschercloudai/unikorn/pkg/provisioners/concurrent"
	"github.com/eschercloudai/unikorn/pkg/provisioners/conditional"
	"github.com/eschercloudai/unikorn/pkg/provisioners/helmapplications/certmanager"
	"github.com/eschercloudai/unikorn/pkg/provisioners/helmapplications/certmanagerissuers"
	"github.com/eschercloudai/unikorn/pkg/provisioners/helmapplications/cilium"
	"github.com/eschercloudai/unikorn/pkg/provisioners/helmapplications/clusterautoscaler"
	"github.com/eschercloudai/unikorn/pkg/provisioners/helmapplications/clusterautoscaleropenstack"
	"github.com/eschercloudai/unikorn/pkg/provisioners/helmapplications/clusteropenstack"
	"github.com/eschercloudai/unikorn/pkg/provisioners/helmapplications/ingressnginx"
	"github.com/eschercloudai/unikorn/pkg/provisioners/helmapplications/kubernetesdashboard"
	"github.com/eschercloudai/unikorn/pkg/provisioners/helmapplications/longhorn"
	"github.com/eschercloudai/unikorn/pkg/provisioners/helmapplications/metricsserver"
	"github.com/eschercloudai/unikorn/pkg/provisioners/helmapplications/nvidiagpuoperator"
	"github.com/eschercloudai/unikorn/pkg/provisioners/helmapplications/openstackcloudprovider"
	"github.com/eschercloudai/unikorn/pkg/provisioners/helmapplications/openstackplugincindercsi"
	"github.com/eschercloudai/unikorn/pkg/provisioners/helmapplications/prometheus"
	"github.com/eschercloudai/unikorn/pkg/provisioners/helmapplications/vcluster"
	"github.com/eschercloudai/unikorn/pkg/provisioners/remotecluster"
	"github.com/eschercloudai/unikorn/pkg/provisioners/serial"
	provisionersutil "github.com/eschercloudai/unikorn/pkg/provisioners/util"
	"github.com/eschercloudai/unikorn/pkg/util"

	"k8s.io/apimachinery/pkg/labels"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Provisioner encapsulates control plane provisioning.
type Provisioner struct {
	provisioners.ProvisionerMeta

	// cluster is the Kubernetes cluster we're provisioning.
	cluster unikornv1.KubernetesCluster
}

// New returns a new initialized provisioner object.
func New() provisioners.ManagerProvisioner {
	return &Provisioner{}
}

// Ensure the ManagerProvisioner interface is implemented.
var _ provisioners.ManagerProvisioner = &Provisioner{}

func (p *Provisioner) Object() unikornv1.ManagableResourceInterface {
	return &p.cluster
}

// getControlPlane gets the control plane object that owns this cluster.
func (p *Provisioner) getControlPlane(ctx context.Context) (*unikornv1.ControlPlane, error) {
	// TODO: error checking.
	projectLabels := labels.Set{
		constants.KindLabel:    constants.KindLabelValueProject,
		constants.ProjectLabel: p.cluster.Labels[constants.ProjectLabel],
	}

	projectNamespace, err := provisionersutil.GetResourceNamespace(ctx, projectLabels)
	if err != nil {
		return nil, err
	}

	var controlPlane unikornv1.ControlPlane

	key := client.ObjectKey{
		Namespace: projectNamespace.Name,
		Name:      p.cluster.Labels[constants.ControlPlaneLabel],
	}

	if err := clientlib.StaticClientFromContext(ctx).Get(ctx, key, &controlPlane); err != nil {
		return nil, err
	}

	return &controlPlane, nil
}

func (p *Provisioner) getProvisioner(ctx context.Context) (provisioners.Provisioner, error) {
	controlPlane, err := p.getControlPlane(ctx)
	if err != nil {
		return nil, err
	}

	remoteControlPlane := remotecluster.New(vcluster.NewRemoteCluster(p.cluster.Namespace, controlPlane), false)

	controlPlanePrefix, err := util.GetNATPrefix(ctx)
	if err != nil {
		return nil, err
	}

	remoteCluster := remotecluster.New(clusteropenstack.NewRemoteCluster(&p.cluster), true)

	clusterProvisioner := clusteropenstack.New(controlPlanePrefix).InNamespace(p.cluster.Name)

	// These applications are required to get the cluster up and running, they must
	// tolerate control plane taints, be scheduled onto control plane nodes and allow
	// scale from zero.
	bootstrapProvisioner := concurrent.New("cluster bootstrap",
		cilium.New(),
		openstackcloudprovider.New(),
	)

	clusterAutoscalerProvisioner := conditional.New("cluster-autoscaler",
		p.cluster.AutoscalingEnabled,
		concurrent.New("cluster-autoscaler",
			clusterautoscaler.New().InNamespace(p.cluster.Name),
			clusterautoscaleropenstack.New().InNamespace(p.cluster.Name),
		),
	)

	certManagerProvisioner := serial.New("cert-manager",
		certmanager.New(),
		certmanagerissuers.New(),
	)

	addonsProvisioner := serial.New("cluster add-ons",
		concurrent.New("cluster add-ons wave 1",
			openstackplugincindercsi.New(),
			metricsserver.New(),
			conditional.New("nvidia-gpu-operator", p.cluster.NvidiaOperatorEnabled, nvidiagpuoperator.New()),
			conditional.New("ingress-nginx", p.cluster.IngressEnabled, ingressnginx.New()),
			conditional.New("cert-manager", p.cluster.CertManagerEnabled, certManagerProvisioner),
			conditional.New("longhorn", p.cluster.FileStorageEnabled, longhorn.New()),
			conditional.New("prometheus", p.cluster.PrometheusEnabled, prometheus.New()),
		),
		concurrent.New("cluster add-ons wave 2",
			// TODO: this hack where it needs the remote is pretty ugly.
			conditional.New("kubernetes-dashboard", p.cluster.KubernetesDashboardEnabled, kubernetesdashboard.New()),
		),
	)

	// Create the cluster and the boostrap components in parallel, the cluster will
	// come up but never reach healthy until the CNI and cloud controller manager
	// are added.  Follow that up by the autoscaler as some addons may require worker
	// nodes to schedule onto.
	provisioner := remoteControlPlane.ProvisionOn(
		serial.New("kubernetes cluster",
			concurrent.New("kubernetes cluster",
				clusterProvisioner,
				remoteCluster.ProvisionOn(bootstrapProvisioner, remotecluster.BackgroundDeletion),
			),
			clusterAutoscalerProvisioner,
			remoteCluster.ProvisionOn(addonsProvisioner, remotecluster.BackgroundDeletion),
		),
	)

	return provisioner, nil
}

// Provision implements the Provision interface.
func (p *Provisioner) Provision(ctx context.Context) error {
	provisioner, err := p.getProvisioner(ctx)
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
	provisioner, err := p.getProvisioner(ctx)
	if err != nil {
		return err
	}

	if err := provisioner.Deprovision(ctx); err != nil {
		return err
	}

	return nil
}
