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

package clusteropenstackaddons

import (
	"context"

	unikornv1alpha1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/provisioners"
	"github.com/eschercloudai/unikorn/pkg/provisioners/cilium"
	"github.com/eschercloudai/unikorn/pkg/provisioners/clusteropenstack"
	"github.com/eschercloudai/unikorn/pkg/provisioners/concurrent"
	"github.com/eschercloudai/unikorn/pkg/provisioners/openstackcloudprovider"
	"github.com/eschercloudai/unikorn/pkg/provisioners/remotecluster"
	"github.com/eschercloudai/unikorn/pkg/provisioners/vcluster"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Provisioner encapsulates control plane provisioning.
type Provisioner struct {
	// client provides access to Kubernetes.
	client client.Client

	// cluster is the Kubernetes cluster we're provisioning.
	cluster *unikornv1alpha1.KubernetesCluster
}

// New returns a new initialized provisioner object.
func New(ctx context.Context, client client.Client, cluster *unikornv1alpha1.KubernetesCluster) (*Provisioner, error) {
	provisioner := &Provisioner{
		client:  client,
		cluster: cluster,
	}

	return provisioner, nil
}

// Ensure the Provisioner interface is implemented.
var _ provisioners.Provisioner = &Provisioner{}

// newOpenstackCloudProviderProvisioner wraps up OCP provisioner configuration.
func (p *Provisioner) newOpenstackCloudProviderProvisioner(remote remotecluster.Generator) *openstackcloudprovider.Provisioner {
	return openstackcloudprovider.New(p.client, p.cluster, remote)
}

// newCiliumProvisioner wraps up Cilium provisioner configuration.
func (p *Provisioner) newCiliumProvisioner(remote remotecluster.Generator) *cilium.Provisioner {
	return cilium.New(p.client, p.cluster, remote)
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

// Provision implements the Provision interface.
func (p *Provisioner) Provision(ctx context.Context) error {
	remote, err := p.getRemoteClusterGenerator(ctx)
	if err != nil {
		return err
	}

	if err := remotecluster.New(p.client, remote).Provision(ctx); err != nil {
		return err
	}

	group := concurrent.Provisioner{
		Group: "cluster add-ons",
		Provisioners: []provisioners.Provisioner{
			p.newCiliumProvisioner(remote),
			p.newOpenstackCloudProviderProvisioner(remote),
		},
	}

	if err := group.Provision(ctx); err != nil {
		return err
	}

	return nil
}

// Deprovision implements the Provision interface.
func (p *Provisioner) Deprovision(ctx context.Context) error {
	remote, err := p.getRemoteClusterGenerator(ctx)
	if err != nil {
		return err
	}

	group := concurrent.Provisioner{
		Group: "cluster add-ons",
		Provisioners: []provisioners.Provisioner{
			p.newCiliumProvisioner(remote),
			p.newOpenstackCloudProviderProvisioner(remote),
		},
	}

	if err := group.Deprovision(ctx); err != nil {
		return err
	}

	if err := remotecluster.New(p.client, remote).Deprovision(ctx); err != nil {
		return err
	}

	return nil
}
