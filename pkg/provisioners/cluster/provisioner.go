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

package cluster

import (
	"context"

	unikornv1alpha1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/provisioners"
	"github.com/eschercloudai/unikorn/pkg/provisioners/clusterautoscaler"
	"github.com/eschercloudai/unikorn/pkg/provisioners/clusteropenstack"
	"github.com/eschercloudai/unikorn/pkg/provisioners/clusteropenstackaddons"
	"github.com/eschercloudai/unikorn/pkg/provisioners/concurrent"
	"github.com/eschercloudai/unikorn/pkg/provisioners/remotecluster"
	"github.com/eschercloudai/unikorn/pkg/provisioners/vcluster"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Provisioner encapsulates control plane provisioning.
type Provisioner struct {
	// client provides access to Kubernetes.
	client client.Client

	// remote is the remote cluster to deploy to.
	remote remotecluster.Generator

	// cluster is the Kubernetes cluster we're provisioning.
	cluster *unikornv1alpha1.KubernetesCluster
}

// New returns a new initialized provisioner object.
func New(ctx context.Context, client client.Client, cluster *unikornv1alpha1.KubernetesCluster) (*Provisioner, error) {
	provisioner := &Provisioner{
		client:  client,
		remote:  vcluster.NewRemoteClusterGenerator(client, cluster.Namespace, provisioners.VclusterRemoteLabelsFromCluster(cluster)),
		cluster: cluster,
	}

	return provisioner, nil
}

// Ensure the Provisioner interface is implemented.
var _ provisioners.Provisioner = &Provisioner{}

func (p *Provisioner) newClusterAutoscalerProvisioner() *clusterautoscaler.Provisioner {
	return clusterautoscaler.New(p.client, p.cluster, p.remote, p.cluster.Name, p.cluster.Name, p.cluster.Name+"-kubeconfig")
}

// Provision implements the Provision interface.
func (p *Provisioner) Provision(ctx context.Context) error {
	clusterProvisioner, err := clusteropenstack.New(ctx, p.client, p.cluster, p.remote)
	if err != nil {
		return err
	}

	addonsProvisioner, err := clusteropenstackaddons.New(ctx, p.client, p.cluster)
	if err != nil {
		return err
	}

	group := concurrent.Provisioner{
		Group: "kubernetes cluster",
		Provisioners: []provisioners.Provisioner{
			clusterProvisioner,
			addonsProvisioner,
		},
	}

	if err := group.Provision(ctx); err != nil {
		return err
	}

	// TODO: you can create with it on, turn it on, but not remove it...
	if p.cluster.AutoscalingEnabled() {
		if err := p.newClusterAutoscalerProvisioner().Provision(ctx); err != nil {
			return err
		}
	}

	return nil
}

// Deprovision implements the Provision interface.
func (p *Provisioner) Deprovision(ctx context.Context) error {
	if p.cluster.AutoscalingEnabled() {
		if err := p.newClusterAutoscalerProvisioner().Deprovision(ctx); err != nil {
			return err
		}
	}

	// Remove the addons first, Argo will probably have a fit if the cluster vanishes
	// before it has a chance to delete the contained add-on applications.
	addonsProvisioner, err := clusteropenstackaddons.New(ctx, p.client, p.cluster)
	if err != nil {
		return err
	}

	if err := addonsProvisioner.Deprovision(ctx); err != nil {
		return err
	}

	clusterProvisioner, err := clusteropenstack.New(ctx, p.client, p.cluster, p.remote)
	if err != nil {
		return err
	}

	if err := clusterProvisioner.Deprovision(ctx); err != nil {
		return err
	}

	return nil
}
