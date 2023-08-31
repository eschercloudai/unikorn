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

	// driver is the CD driver that implements applications.
	driver cd.Driver

	// generator provides a method to derive cluster names and configuration.
	generator provisioners.RemoteCluster
}

// New returns a new initialized provisioner object.
func New(driver cd.Driver, generator provisioners.RemoteCluster) *Provisioner {
	return &Provisioner{
		ProvisionerMeta: provisioners.ProvisionerMeta{
			Name: "remote-cluster",
		},
		driver:    driver,
		generator: generator,
	}
}

// Ensure the Provisioner interface is implemented.
var _ provisioners.Provisioner = &Provisioner{}

func (p *Provisioner) ID() *cd.ResourceIdentifier {
	return p.generator.ID()
}

// Provision implements the Provision interface.
func (p *Provisioner) Provision(ctx context.Context) error {
	log := log.FromContext(ctx)

	log.Info("provisioning remote cluster", "remotecluster", p.Name)

	config, err := p.generator.Config(ctx)
	if err != nil {
		return err
	}

	cluster := &cd.Cluster{
		Config: config,
	}

	if err := p.driver.CreateOrUpdateCluster(ctx, p.generator.ID(), cluster); err != nil {
		log.Info("remote cluster not ready, yielding", "remotecluster", p.Name)

		return provisioners.ErrYield
	}

	log.Info("remote cluster provisioned", "remotecluster", p.Name)

	return nil
}

// Deprovision implements the Provision interface.
func (p *Provisioner) Deprovision(ctx context.Context) error {
	log := log.FromContext(ctx)

	log.Info("deprovisioning remote cluster", "remotecluster", p.Name)

	if err := p.driver.DeleteCluster(ctx, p.generator.ID()); err != nil {
		return err
	}

	log.Info("remote cluster deprovisioned", "remotecluster", p.Name)

	return nil
}
