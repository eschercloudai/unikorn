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

package remotecluster

import (
	"context"

	argocdclient "github.com/eschercloudai/unikorn/pkg/argocd/client"
	argocdcluster "github.com/eschercloudai/unikorn/pkg/argocd/cluster"
	"github.com/eschercloudai/unikorn/pkg/provisioners"
	"github.com/eschercloudai/unikorn/pkg/util/retry"

	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// namespace is where ArgoCD lives.
	// TODO: Make this dynamic.
	namespace = "argocd"
)

// Generator is an sbstraction around the sources of remote
// clusters e.g. a cluster API or vcluster Kubernetes instance.
type Generator interface {
	// Name is a unique name for the remote cluster, this must
	// be able to be procedurally generated in order to delete
	// the remote by name, when the Server() is unavailable e.g
	// derived from a deleted resource.
	Name() string

	// Server is the URL for the remote cluster endpoint.
	Server(ctx context.Context) (string, error)

	// Config returns the client configuration (aka parsed Kubeconfig.)
	Config(ctx context.Context) (*clientcmdapi.Config, error)
}

// Provisioner provides generic handling of remote cluster instances.
// Specialization is delegated to a provider specific interface.
type Provisioner struct {
	// client provides access to Kubernetes.
	client client.Client

	// generator provides a method to derive cluster names and configuration.
	generator Generator
}

// New returns a new initialized provisioner object.
func New(client client.Client, generator Generator) *Provisioner {
	return &Provisioner{
		client:    client,
		generator: generator,
	}
}

// Ensure the Provisioner interface is implemented.
var _ provisioners.Provisioner = &Provisioner{}

// Provision implements the Provision interface.
func (p *Provisioner) Provision(ctx context.Context) error {
	log := log.FromContext(ctx)

	log.Info("provisioning remote cluster", "remotecluster", p.generator.Name())

	argocd, err := argocdclient.NewInCluster(ctx, p.client, namespace)
	if err != nil {
		return err
	}

	// Retry adding the cluster until ArgoCD deems it's ready, it'll 500 until that
	// condition is met.  This allows the provisioner to be used to initialize remotes
	// in parallel with them coming up as some providers require add-ons to be installed
	// concurrently before a readiness check will succeed.
	upsertCluster := func() error {
		server, err := p.generator.Server(ctx)
		if err != nil {
			return err
		}

		config, err := p.generator.Config(ctx)
		if err != nil {
			return err
		}

		if err := argocdcluster.Upsert(ctx, argocd, p.generator.Name(), server, config); err != nil {
			return err
		}

		return nil
	}

	if err := retry.Forever().DoWithContext(ctx, upsertCluster); err != nil {
		return err
	}

	log.Info("remote cluster provisioned", "remotecluster", p.generator.Name())

	return nil
}

// Deprovision implements the Provision interface.
func (p *Provisioner) Deprovision(ctx context.Context) error {
	log := log.FromContext(ctx)

	log.Info("deprovisioning remote cluster", "remotecluster", p.generator.Name())

	argocd, err := argocdclient.NewInCluster(ctx, p.client, namespace)
	if err != nil {
		return err
	}

	if err := argocdcluster.Delete(ctx, argocd, p.generator.Name()); err != nil {
		return err
	}

	log.Info("remote cluster deprovisioned", "remotecluster", p.generator.Name())

	return nil
}
