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

package concurrent

import (
	"context"

	"golang.org/x/sync/errgroup"

	"github.com/eschercloudai/unikorn/pkg/provisioners"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Provisioner struct {
	// anem is the concurrency group name.
	name string

	// provisioners is the set of provisions to provision
	// concurrently.
	provisioners []provisioners.Provisioner
}

func New(name string, provisioners ...provisioners.Provisioner) *Provisioner {
	return &Provisioner{
		name:         name,
		provisioners: provisioners,
	}
}

// Ensure the Provisioner interface is implemented.
var _ provisioners.Provisioner = &Provisioner{}

// OnRemote implements the Provision interface.
func (p *Provisioner) OnRemote(_ provisioners.RemoteCluster) provisioners.Provisioner {
	return p
}

// InNamespace implements the Provision interface.
func (p *Provisioner) InNamespace(_ string) provisioners.Provisioner {
	return p
}

// Provision implements the Provision interface.
func (p *Provisioner) Provision(ctx context.Context) error {
	log := log.FromContext(ctx)

	log.Info("provisioning concurrency group", "group", p.name)

	group, gctx := errgroup.WithContext(ctx)

	for i := range p.provisioners {
		provisioner := p.provisioners[i]

		group.Go(func() error { return provisioner.Provision(gctx) })
	}

	if err := group.Wait(); err != nil {
		return err
	}

	log.Info("concurrency group provisioned", "group", p.name)

	return nil
}

// Deprovision implements the Provision interface.
func (p *Provisioner) Deprovision(ctx context.Context) error {
	log := log.FromContext(ctx)

	log.Info("deprovisioning concurrency group", "group", p.name)

	group, gctx := errgroup.WithContext(ctx)

	for i := range p.provisioners {
		provisioner := p.provisioners[i]

		group.Go(func() error { return provisioner.Deprovision(gctx) })
	}

	if err := group.Wait(); err != nil {
		return err
	}

	log.Info("concurrency group deprovisioned", "group", p.name)

	return nil
}
