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
	"fmt"

	unikornv1alpha1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	argocdclient "github.com/eschercloudai/unikorn/pkg/argocd/client"
	argocdcluster "github.com/eschercloudai/unikorn/pkg/argocd/cluster"
	"github.com/eschercloudai/unikorn/pkg/constants"
	"github.com/eschercloudai/unikorn/pkg/provisioners"
	"github.com/eschercloudai/unikorn/pkg/provisioners/cilium"
	"github.com/eschercloudai/unikorn/pkg/provisioners/concurrent"
	"github.com/eschercloudai/unikorn/pkg/provisioners/openstackcloudprovider"
	"github.com/eschercloudai/unikorn/pkg/provisioners/vcluster"
	"github.com/eschercloudai/unikorn/pkg/util/retry"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

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
func (p *Provisioner) newOpenstackCloudProviderProvisioner() *openstackcloudprovider.Provisioner {
	return openstackcloudprovider.New(p.client, p.cluster, p.argoCDClusterName())
}

// newCiliumProvisioner wraps up Cilium provisioner configuration.
func (p *Provisioner) newCiliumProvisioner() *cilium.Provisioner {
	return cilium.New(p.client, p.cluster, p.argoCDClusterName())
}

// argoCDClusterName returns a human readable server name.
func (p *Provisioner) argoCDClusterName() string {
	return fmt.Sprintf("kubernetes-%s-%s-%s", p.cluster.Labels[constants.ProjectLabel], p.cluster.Labels[constants.ControlPlaneLabel], p.cluster.Name)
}

// getKubernetesClusterConfig retrieves the Kubernetes configuration from
// a cluster API cluster.
func (p *Provisioner) getKubernetesClusterConfig(ctx context.Context) (*clientcmdapi.Config, error) {
	vc := vcluster.NewControllerRuntimeClient(p.client)

	vclusterClient, err := vc.Client(ctx, p.cluster.Namespace, false)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to get vcluster client", err)
	}

	secret := &corev1.Secret{}

	secretKey := client.ObjectKey{
		Namespace: p.cluster.Name,
		Name:      p.cluster.Name + "-kubeconfig",
	}

	// Retry getting the secret until it exists.
	getSecret := func() error {
		return vclusterClient.Get(ctx, secretKey, secret)
	}

	if err := retry.Forever().DoWithContext(ctx, getSecret); err != nil {
		return nil, err
	}

	config, err := clientcmd.NewClientConfigFromBytes(secret.Data["value"])
	if err != nil {
		return nil, err
	}

	rawConfig, err := config.RawConfig()
	if err != nil {
		return nil, err
	}

	return &rawConfig, nil
}

// Provision implements the Provision interface.
func (p *Provisioner) Provision(ctx context.Context) error {
	config, err := p.getKubernetesClusterConfig(ctx)
	if err != nil {
		return err
	}

	server := config.Clusters[config.Contexts[config.CurrentContext].Cluster].Server

	argocd, err := argocdclient.NewInCluster(ctx, p.client, "argocd")
	if err != nil {
		return err
	}

	// Retry adding the cluster until ArgoCD deems it's ready, it'll 500 until that
	// condition is met.
	upsertCluster := func() error {
		if err := argocdcluster.Upsert(ctx, argocd, p.argoCDClusterName(), server, config); err != nil {
			return err
		}

		return nil
	}

	if err := retry.Forever().DoWithContext(ctx, upsertCluster); err != nil {
		return err
	}

	group := concurrent.Provisioner{
		Group: "cluster add-ons",
		Provisioners: []provisioners.Provisioner{
			p.newCiliumProvisioner(),
			p.newOpenstackCloudProviderProvisioner(),
		},
	}

	if err := group.Provision(ctx); err != nil {
		return err
	}

	return nil
}

// Deprovision implements the Provision interface.
func (p *Provisioner) Deprovision(ctx context.Context) error {
	group := concurrent.Provisioner{
		Group: "cluster add-ons",
		Provisioners: []provisioners.Provisioner{
			p.newCiliumProvisioner(),
			p.newOpenstackCloudProviderProvisioner(),
		},
	}

	if err := group.Deprovision(ctx); err != nil {
		return err
	}

	// TODO: Can I delete the cluster and then have all the dependent applications
	// get magically cleaned up?
	argocd, err := argocdclient.NewInCluster(ctx, p.client, "argocd")
	if err != nil {
		return err
	}

	if err := argocdcluster.Delete(ctx, argocd, p.argoCDClusterName()); err != nil {
		return err
	}

	return nil
}
