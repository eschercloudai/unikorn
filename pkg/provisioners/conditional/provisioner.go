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

package conditional

import (
	"context"

	"github.com/eschercloudai/unikorn/pkg/provisioners"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Provisioner struct {
	provisioners.ProvisionerMeta

	// condition will execute the provisioner if true.
	condition func() bool

	// provisioner is the provisioner to provision.
	provisioner provisioners.Provisioner
}

func New(name string, condition func() bool, provisioner provisioners.Provisioner) *Provisioner {
	return &Provisioner{
		ProvisionerMeta: provisioners.ProvisionerMeta{
			Name: name,
		},
		condition:   condition,
		provisioner: provisioner,
	}
}

// Ensure the Provisioner interface is implemented.
var _ provisioners.Provisioner = &Provisioner{}

// Provision implements the Provision interface.
func (p *Provisioner) Provision(ctx context.Context) error {
	log := log.FromContext(ctx)

	if !p.condition() {
		log.Info("conditional deprovision", "provisioner", p.Name)

		return p.provisioner.Deprovision(ctx)
	}

	return p.provisioner.Provision(ctx)
}

// Deprovision implements the Provision interface.
func (p *Provisioner) Deprovision(ctx context.Context) error {
	log := log.FromContext(ctx)

	if !p.condition() {
		log.Info("skipping conditional deprovision", "provisioner", p.Name)

		return nil
	}

	return p.provisioner.Deprovision(ctx)
}
