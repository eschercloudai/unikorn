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

package conditional

import (
	"context"

	"github.com/eschercloudai/unikorn/pkg/provisioners"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Provisioner struct {
	// Name is the conditional name.
	Name string

	// condition will execute the provisioner if true.
	Condition func() bool

	// Provisioner is the provisioner to provision.
	Provisioner provisioners.Provisioner
}

// Ensure the Provisioner interface is implemented.
var _ provisioners.Provisioner = &Provisioner{}

// Provision implements the Provision interface.
func (p *Provisioner) Provision(ctx context.Context) error {
	log := log.FromContext(ctx)

	if !p.Condition() {
		log.Info("skipping conditional provision", "provisioner", p.Name)

		return nil
	}

	return p.Provisioner.Provision(ctx)
}

// Deprovision implements the Provision interface.
func (p *Provisioner) Deprovision(ctx context.Context) error {
	log := log.FromContext(ctx)

	if !p.Condition() {
		log.Info("skipping conditional deprovision", "provisioner", p.Name)

		return nil
	}

	return p.Provisioner.Deprovision(ctx)
}
