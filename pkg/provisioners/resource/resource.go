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

package resource

import (
	"context"
	"fmt"

	"github.com/eschercloudai/unikorn/pkg/provisioners"

	"k8s.io/apimachinery/pkg/api/errors"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Provisioner is a provisioner that is able to parse and manage resources
// sourced from a yaml manifest.
type Provisioner struct {
	provisioners.ProvisionerMeta

	// client is a client to allow Kubernetes access.
	client client.Client

	// resource is the resource to provision.
	resource client.Object
}

// Ensure the Provisioner interface is implemented.
var _ provisioners.Provisioner = &Provisioner{}

// New returns a new provisioner that is capable of applying
// a manifest with kubectl.  The path argument may be a path on the local file
// system or a URL.
func New(client client.Client, resource client.Object) *Provisioner {
	return &Provisioner{
		client:   client,
		resource: resource,
	}
}

func mutate() error {
	return nil
}

// Provision implements the Provision interface.
func (p *Provisioner) Provision(ctx context.Context) error {
	log := log.FromContext(ctx)

	objectKey := client.ObjectKeyFromObject(p.resource)

	log.Info("creating object", "key", objectKey)

	result, err := controllerutil.CreateOrUpdate(ctx, p.client, p.resource, mutate)
	if err != nil {
		return err
	}

	log.Info(fmt.Sprintf("object %v", result), "name", p.resource.GetName(), "generateName", p.resource.GetGenerateName())

	return nil
}

// Deprovision implements the Provision interface.
func (p *Provisioner) Deprovision(ctx context.Context) error {
	log := log.FromContext(ctx)

	objectKey := client.ObjectKeyFromObject(p.resource)

	log.Info("deleting object", "key", objectKey)

	if err := p.client.Delete(ctx, p.resource); err != nil {
		if errors.IsNotFound(err) {
			log.Info("object deleted", "key", objectKey)

			return nil
		}

		return err
	}

	log.Info("awaiting object deletion", "key", objectKey)

	return provisioners.ErrYield
}
