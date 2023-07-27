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

package concurrent

import (
	"context"

	"golang.org/x/sync/errgroup"

	"github.com/eschercloudai/unikorn/pkg/provisioners"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Provisioner struct {
	provisioners.ProvisionerMeta

	// provisioners is the set of provisions to provision
	// concurrently.
	provisioners []provisioners.Provisioner
}

func New(name string, p ...provisioners.Provisioner) *Provisioner {
	return &Provisioner{
		ProvisionerMeta: provisioners.ProvisionerMeta{
			Name: name,
		},
		provisioners: p,
	}
}

// Ensure the Provisioner interface is implemented.
var _ provisioners.Provisioner = &Provisioner{}

// Provision implements the Provision interface.
func (p *Provisioner) Provision(ctx context.Context) error {
	log := log.FromContext(ctx)

	log.Info("provisioning concurrency group", "group", p.Name)

	group := &errgroup.Group{}

	for i := range p.provisioners {
		provisioner := p.provisioners[i]

		callback := func() error {
			// As errgroup only saves the first error, we may lose some
			// logging information, so do this here when waiting on child
			// tasks.
			if err := provisioner.Provision(ctx); err != nil {
				log.Info("concurrency group member exited with error", "error", err, "group", p.Name, "provisioner", provisioner.ProvisionerName())

				return err
			}

			return nil
		}

		group.Go(callback)
	}

	if err := group.Wait(); err != nil {
		log.Info("concurrency group provision failed", "group", p.Name)

		return err
	}

	log.Info("concurrency group provisioned", "group", p.Name)

	return nil
}

// Deprovision implements the Provision interface.
func (p *Provisioner) Deprovision(ctx context.Context) error {
	log := log.FromContext(ctx)

	log.Info("deprovisioning concurrency group", "group", p.Name)

	group := &errgroup.Group{}

	for i := range p.provisioners {
		provisioner := p.provisioners[i]

		group.Go(func() error { return provisioner.Deprovision(ctx) })
	}

	if err := group.Wait(); err != nil {
		log.Info("concurrency group deprovision failed", "group", p.Name)

		return err
	}

	log.Info("concurrency group deprovisioned", "group", p.Name)

	return nil
}
