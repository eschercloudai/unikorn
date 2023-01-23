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

package serial

import (
	"context"

	"github.com/eschercloudai/unikorn/pkg/provisioners"
)

type Provisioner struct {
	// Name is the conditional name.
	Name string

	// Provisioners are the provisioner to provision in order.
	Provisioners []provisioners.Provisioner
}

// Ensure the Provisioner interface is implemented.
var _ provisioners.Provisioner = &Provisioner{}

// Provision implements the Provision interface.
func (p *Provisioner) Provision(ctx context.Context) error {
	for _, provisioner := range p.Provisioners {
		if err := provisioner.Provision(ctx); err != nil {
			return err
		}
	}

	return nil
}

// Deprovision implements the Provision interface.
// Note: things happen in the reverse order to provisioning, this assumes
// that the same code that generates the provisioner, generates the deprovisioner
// and ordering constraints matter.
func (p *Provisioner) Deprovision(ctx context.Context) error {
	for i := range p.Provisioners {
		provisioner := p.Provisioners[len(p.Provisioners)-1-i]

		if err := provisioner.Deprovision(ctx); err != nil {
			return err
		}
	}

	return nil
}
