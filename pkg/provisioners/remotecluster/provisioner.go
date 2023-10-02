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
	"sync"

	"github.com/eschercloudai/unikorn/pkg/cd"
	"github.com/eschercloudai/unikorn/pkg/provisioners"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

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

	// generator provides a method to derive cluster names and configuration.
	generator provisioners.RemoteCluster

	// controller tells whether we "own" this resource or not.
	controller bool

	// lock provides synchronization around concurrrency.
	lock sync.Mutex

	// refCount tells us how many remote provisioners have been registered.
	refCount int

	// currentCount tells us how many times remote provisioners have been called.
	currentCount int
}

// New returns a new initialized provisioner object.
func New(generator provisioners.RemoteCluster, controller bool) *Provisioner {
	return &Provisioner{
		ProvisionerMeta: provisioners.ProvisionerMeta{
			Name: "remote-cluster",
		},
		generator:  generator,
		controller: controller,
	}
}

func (p *Provisioner) ID() *cd.ResourceIdentifier {
	return p.generator.ID()
}

// ProvisionOn returns a provisioner that will provision the remote,
// and provision the child provisioner on that remote.
func (p *Provisioner) ProvisionOn(child provisioners.Provisioner) provisioners.Provisioner {
	p.refCount++

	return &remoteProvisioner{
		ProvisionerMeta: provisioners.ProvisionerMeta{
			Name: p.Name + "-dynamic",
		},
		provisioner: p,
		child:       child,
	}
}

type remoteProvisioner struct {
	provisioners.ProvisionerMeta
	provisioner *Provisioner
	child       provisioners.Provisioner
}

// Ensure the Provisioner interface is implemented.
var _ provisioners.Provisioner = &remoteProvisioner{}

func (p *remoteProvisioner) provisionRemote(ctx context.Context) error {
	log := log.FromContext(ctx)

	p.provisioner.lock.Lock()
	defer p.provisioner.lock.Unlock()

	p.provisioner.currentCount++

	id := p.provisioner.generator.ID()

	// If this is the first remote cluster encountered, reconcile it.
	if p.provisioner.controller && p.provisioner.currentCount == 1 {
		log.Info("provisioning remote cluster", "remotecluster", id)

		config, err := p.provisioner.generator.Config(ctx)
		if err != nil {
			return err
		}

		cluster := &cd.Cluster{
			Config: config,
		}

		if err := cd.FromContext(ctx).CreateOrUpdateCluster(ctx, id, cluster); err != nil {
			log.Info("remote cluster not ready, yielding", "remotecluster", id)

			return provisioners.ErrYield
		}

		log.Info("remote cluster provisioned", "remotecluster", id)
	}

	return nil
}

// Provision implements the Provision interface.
func (p *remoteProvisioner) Provision(ctx context.Context) error {
	if err := p.provisionRemote(ctx); err != nil {
		return err
	}

	// TODO: make this a shallow clone!
	p.child.OnRemote(p.provisioner.generator)

	// Remote is registered, create the remote applications.
	if err := p.child.Provision(ctx); err != nil {
		return err
	}

	return nil
}

// Deprovision implements the Provision interface.
func (p *remoteProvisioner) Deprovision(ctx context.Context) error {
	log := log.FromContext(ctx)

	// TODO: make this a shallow clone!
	p.child.OnRemote(p.provisioner.generator)

	// Remove the applications.
	if err := p.child.Deprovision(ctx); err != nil {
		return err
	}

	// Once all concurrent remote provisioner have done there stuff
	// they will wait on the lock...
	p.provisioner.lock.Lock()
	defer p.provisioner.lock.Unlock()

	// ... adding themselves to the total...
	p.provisioner.currentCount++

	id := p.provisioner.generator.ID()

	// ... and if all have completed without an error, then deprovision the
	// remote cluster itself.
	if p.provisioner.controller && p.provisioner.currentCount == p.provisioner.refCount {
		log.Info("deprovisioning remote cluster", "remotecluster", id)

		if err := cd.FromContext(ctx).DeleteCluster(ctx, id); err != nil {
			return err
		}

		log.Info("remote cluster deprovisioned", "remotecluster", id)
	}

	return nil
}
