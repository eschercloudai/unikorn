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

package serial

import (
	"context"

	"github.com/eschercloudai/unikorn/pkg/provisioners"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Provisioner struct {
	// name is the group name.
	name string

	// provisioners are the provisioner to provision in order.
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

	log.Info("provisioning serial group", "group", p.name)

	for _, provisioner := range p.provisioners {
		if err := provisioner.Provision(ctx); err != nil {
			return err
		}
	}

	log.Info("serial group provisioned", "group", p.name)

	return nil
}

// Deprovision implements the Provision interface.
// Note: things happen in the reverse order to provisioning, this assumes
// that the same code that generates the provisioner, generates the deprovisioner
// and ordering constraints matter.
func (p *Provisioner) Deprovision(ctx context.Context) error {
	log := log.FromContext(ctx)

	log.Info("deprovisioning serial group", "group", p.name)

	for i := range p.provisioners {
		provisioner := p.provisioners[len(p.provisioners)-(i+1)]

		if err := provisioner.Deprovision(ctx); err != nil {
			return err
		}
	}

	log.Info("serial group deprovisioned", "group", p.name)

	return nil
}
