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

package remotecluster

import (
	"context"
	"strings"

	argocdclient "github.com/eschercloudai/unikorn/pkg/argocd/client"
	argocdcluster "github.com/eschercloudai/unikorn/pkg/argocd/cluster"
	"github.com/eschercloudai/unikorn/pkg/provisioners"
	provisionererrors "github.com/eschercloudai/unikorn/pkg/provisioners/errors"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// namespace is where ArgoCD lives.
	// TODO: Make this dynamic.
	namespace = "argocd"
)

// GenerateName combines the generator's name and labels to form a unique
// remote cluster name.
func GenerateName(generator provisioners.RemoteCluster) string {
	name := generator.Name()

	if len(generator.Labels()) != 0 {
		name += "-" + strings.Join(generator.Labels(), ":")
	}

	return name
}

// GetClient gets a client from the remote generator.
func GetClient(ctx context.Context, generator provisioners.RemoteCluster) (client.Client, error) {
	getter := func() (*clientcmdapi.Config, error) {
		return generator.Config(ctx)
	}

	restConfig, err := clientcmd.BuildConfigFromKubeconfigGetter("", getter)
	if err != nil {
		return nil, err
	}

	return client.New(restConfig, client.Options{})
}

// Provisioner provides generic handling of remote cluster instances.
// Specialization is delegated to a provider specific interface.
type Provisioner struct {
	provisioners.ProvisionerMeta

	// client provides access to Kubernetes.
	client client.Client

	// generator provides a method to derive cluster names and configuration.
	generator provisioners.RemoteCluster
}

// New returns a new initialized provisioner object.
func New(client client.Client, generator provisioners.RemoteCluster) *Provisioner {
	return &Provisioner{
		ProvisionerMeta: provisioners.ProvisionerMeta{
			Name: GenerateName(generator),
		},
		client:    client,
		generator: generator,
	}
}

// Ensure the Provisioner interface is implemented.
var _ provisioners.Provisioner = &Provisioner{}

// Provision implements the Provision interface.
func (p *Provisioner) Provision(ctx context.Context) error {
	log := log.FromContext(ctx)

	log.Info("provisioning remote cluster", "remotecluster", p.Name)

	argocd, err := argocdclient.NewInCluster(ctx, p.client, namespace)
	if err != nil {
		return err
	}

	server, err := p.generator.Server(ctx)
	if err != nil {
		return err
	}

	config, err := p.generator.Config(ctx)
	if err != nil {
		return err
	}

	// Retry adding the cluster until ArgoCD deems it's ready, it'll 500 until that
	// condition is met.  This allows the provisioner to be used to initialize remotes
	// in parallel with them coming up as some providers require add-ons to be installed
	// concurrently before a readiness check will succeed.
	if err := argocdcluster.Upsert(ctx, argocd, p.Name, server, config); err != nil {
		log.Info("remote cluster not ready, yielding", "remotecluster", p.Name)

		return provisionererrors.ErrYield
	}

	log.Info("remote cluster provisioned", "remotecluster", p.Name)

	return nil
}

// Deprovision implements the Provision interface.
func (p *Provisioner) Deprovision(ctx context.Context) error {
	log := log.FromContext(ctx)

	log.Info("deprovisioning remote cluster", "remotecluster", p.Name)

	argocd, err := argocdclient.NewInCluster(ctx, p.client, namespace)
	if err != nil {
		return err
	}

	if err := argocdcluster.Delete(ctx, argocd, p.Name); err != nil {
		return err
	}

	log.Info("remote cluster deprovisioned", "remotecluster", p.Name)

	return nil
}
