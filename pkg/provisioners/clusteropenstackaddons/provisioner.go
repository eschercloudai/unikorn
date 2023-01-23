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
	"github.com/eschercloudai/unikorn/pkg/constants"
	"github.com/eschercloudai/unikorn/pkg/provisioners"
	"github.com/eschercloudai/unikorn/pkg/provisioners/cilium"
	"github.com/eschercloudai/unikorn/pkg/provisioners/clusteropenstack"
	"github.com/eschercloudai/unikorn/pkg/provisioners/concurrent"
	"github.com/eschercloudai/unikorn/pkg/provisioners/openstackcloudprovider"
	"github.com/eschercloudai/unikorn/pkg/provisioners/remotecluster"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Provisioner encapsulates control plane provisioning.
type Provisioner struct {
	// client provides access to Kubernetes.
	client client.Client

	// vclusterClient provides client access inside the control plane.
	vclusterClient client.Client

	// cluster is the Kubernetes cluster we're provisioning.
	cluster *unikornv1alpha1.KubernetesCluster
}

// New returns a new initialized provisioner object.
func New(ctx context.Context, client, vclusterClient client.Client, cluster *unikornv1alpha1.KubernetesCluster) (*Provisioner, error) {
	provisioner := &Provisioner{
		client:         client,
		vclusterClient: vclusterClient,
		cluster:        cluster,
	}

	return provisioner, nil
}

// Ensure the Provisioner interface is implemented.
var _ provisioners.Provisioner = &Provisioner{}

// newOpenstackCloudProviderProvisioner wraps up OCP provisioner configuration.
func (p *Provisioner) newOpenstackCloudProviderProvisioner(remote string) *openstackcloudprovider.Provisioner {
	return openstackcloudprovider.New(p.client, p.cluster, remote)
}

// newCiliumProvisioner wraps up Cilium provisioner configuration.
func (p *Provisioner) newCiliumProvisioner(remote string) *cilium.Provisioner {
	return cilium.New(p.client, p.cluster, remote)
}

func (p *Provisioner) getRemoteClusterGenerator() *clusteropenstack.RemoteClusterGenerator {
	clusterLabels := []string{
		p.cluster.Labels[constants.ControlPlaneLabel],
		p.cluster.Labels[constants.ProjectLabel],
	}

	return clusteropenstack.NewRemoteClusterGenerator(p.vclusterClient, p.cluster.Namespace, p.cluster.Name, clusterLabels)
}

// Provision implements the Provision interface.
func (p *Provisioner) Provision(ctx context.Context) error {
	rcg := p.getRemoteClusterGenerator()

	if err := remotecluster.New(p.client, rcg).Provision(ctx); err != nil {
		return err
	}

	group := concurrent.Provisioner{
		Group: "cluster add-ons",
		Provisioners: []provisioners.Provisioner{
			p.newCiliumProvisioner(rcg.Name()),
			p.newOpenstackCloudProviderProvisioner(rcg.Name()),
		},
	}

	if err := group.Provision(ctx); err != nil {
		return err
	}

	return nil
}

// Deprovision implements the Provision interface.
func (p *Provisioner) Deprovision(ctx context.Context) error {
	rcg := p.getRemoteClusterGenerator()

	group := concurrent.Provisioner{
		Group: "cluster add-ons",
		Provisioners: []provisioners.Provisioner{
			p.newCiliumProvisioner(rcg.Name()),
			p.newOpenstackCloudProviderProvisioner(rcg.Name()),
		},
	}

	if err := group.Deprovision(ctx); err != nil {
		return err
	}

	if err := remotecluster.New(p.client, rcg).Deprovision(ctx); err != nil {
		return err
	}

	return nil
}
