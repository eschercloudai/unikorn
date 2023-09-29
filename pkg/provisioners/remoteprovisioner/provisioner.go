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

	// lock provides synchronization around concurrrency.
	lock sync.Mutex

	// refCount tells us how many remote provisioners have been registered.
	refCount int

	// currentCount tells us how many times remote provisioners have been called.
	currentCount int
}

func New(controller bool) *RemoteProvisioner {
	return &RemoteProvisioner{
		controller: controller,
	}
}

func (r *RemoteProvisioner) ProvisionOn(child provisioners.Provisioner) provisioners.Provisioner {
	r.refCount++

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
	p.provisioner.lock.Lock()
	defer p.provisioner.lock.Unlock()

	p.provisioner.currentCount++

	if p.provisioner.controller && p.provisioner.currentCount == 1 {
		provisioner := argocd.New()
		provisioner.OnRemote(p.Remote)

		if err := provisioner.Provision(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (p *remoteProvisionerProvisioner) Provision(ctx context.Context) error {
	if err := p.provisionRemoteProvisioner(ctx); err != nil {
		return err
	}

	childCtx := cd.NewContext(ctx, argodriver.New(clientlib.DynamicClientFromContext(ctx)))

	if err := p.child.Provision(childCtx); err != nil {
		return err
	}

	return nil
}

func (p *remoteProvisionerProvisioner) Deprovision(ctx context.Context) error {
	childCtx := cd.NewContext(ctx, argodriver.New(clientlib.DynamicClientFromContext(ctx)))

	if err := p.child.Deprovision(childCtx); err != nil {
		return err
	}

	// Once all concurrent remote provisioners have done their stuff
	// they will wait on the lock...
	p.provisioner.lock.Lock()
	defer p.provisioner.lock.Unlock()

	// ... adding themselves to the total...
	p.provisioner.currentCount++

	// ... and if all have completed without an error, then deprovision the
	// remote cluster itself.
	if p.provisioner.controller && p.provisioner.currentCount == p.provisioner.refCount {
		provisioner := argocd.New()
		provisioner.OnRemote(p.Remote)

		if err := provisioner.Deprovision(ctx); err != nil {
			return err
		}
	}

	return nil
}
