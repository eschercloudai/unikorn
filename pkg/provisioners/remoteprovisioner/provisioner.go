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

package remoteprovisioner

import (
	"context"
	"sync"

	"github.com/eschercloudai/unikorn/pkg/cd"
	argodriver "github.com/eschercloudai/unikorn/pkg/cd/argocd"
	clientlib "github.com/eschercloudai/unikorn/pkg/client"
	"github.com/eschercloudai/unikorn/pkg/provisioners"
	"github.com/eschercloudai/unikorn/pkg/provisioners/helmapplications/argocd"
)

type RemoteProvisioner struct {
	// controller tells whether we "own" this resource or not.
	controller bool

	initializer sync.Once

	finalizer sync.WaitGroup
}

func New(controller bool) *RemoteProvisioner {
	return &RemoteProvisioner{
		controller: controller,
	}
}

func (r *RemoteProvisioner) ProvisionOn(child provisioners.Provisioner) provisioners.Provisioner {
	r.finalizer.Add(1)

	return &remoteProvisionerProvisioner{
		ProvisionerMeta: provisioners.ProvisionerMeta{
			Name: "remote-provisioner",
		},
		provisioner: r,
		child:       child,
	}
}

type remoteProvisionerProvisioner struct {
	provisioners.ProvisionerMeta

	provisioner *RemoteProvisioner

	child provisioners.Provisioner
}

func (p *remoteProvisionerProvisioner) provisionRemoteProvisioner(ctx context.Context) error {
	if !p.provisioner.controller {
		return nil
	}

	provisioner := argocd.New()
	provisioner.OnRemote(p.Remote)

	if err := provisioner.Provision(ctx); err != nil {
		return err
	}

	return nil
}

func (p *remoteProvisionerProvisioner) deprovisionRemoteProvisioner(ctx context.Context) error {
	if p.provisioner.controller {
		return nil
	}

	provisioner := argocd.New()
	provisioner.OnRemote(p.Remote)

	if err := provisioner.Deprovision(ctx); err != nil {
		return err
	}

	return nil
}

func (p *remoteProvisionerProvisioner) Provision(ctx context.Context) error {
	// Run the provisioner on the first time we encounter this, all other instances
	// will wait until the provisioner has executed successfully.
	var err error

	p.initializer.Do(func() { err = p.provisionRemoteProvisioner(ctx) })

	if err != nil {
		return err
	}

	// Run the child provisioner with a new driver context.
	childCtx := cd.NewContext(ctx, argodriver.New(clientlib.DynamicClientFromContext(ctx)))

	if err := p.child.Provision(childCtx); err != nil {
		return err
	}

	return nil
}

func (p *remoteProvisionerProvisioner) Deprovision(ctx context.Context) error {
	// Run the child provisioner with a new driver context.
	childCtx := cd.NewContext(ctx, argodriver.New(clientlib.DynamicClientFromContext(ctx)))

	if err := p.child.Deprovision(childCtx); err != nil {
		return err
	}

	// TODO: this logic is "elegant" but wrong, fix me.
	p.provisioner.finalizer.Done()
	p.provisioner.finalizer.Wait()

	var err error

	p.provisioner.initializer.Do(func() { err = deprovisionRemoteProvisioner(ctx) })
	if err != nil {
		return err
	}

	return nil
}
