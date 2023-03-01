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

package generic

import (
	"context"

	"github.com/eschercloudai/unikorn/pkg/provisioners"
	"github.com/eschercloudai/unikorn/pkg/provisioners/remotecluster"
	"github.com/eschercloudai/unikorn/pkg/readiness"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ResourceProvisioner is a provisioner that is able to parse and manage resources
// sourced from a yaml manifest.
type ResourceProvisioner struct {
	// client is a client to allow Kubernetes access.
	client client.Client

	// resource is the resource to provision.
	resource client.Object

	// remote is the remote cluster to deploy to.
	remote provisioners.RemoteCluster
}

// Ensure the Provisioner interface is implemented.
var _ provisioners.Provisioner = &ResourceProvisioner{}

// NewResourceProvisioner returns a new provisioner that is capable of applying
// a manifest with kubectl.  The path argument may be a path on the local file
// system or a URL.
func NewResourceProvisioner(client client.Client, resource client.Object) *ResourceProvisioner {
	return &ResourceProvisioner{
		client:   client,
		resource: resource,
	}
}

// OnRemote implements the Provision interface.
func (p *ResourceProvisioner) OnRemote(remote provisioners.RemoteCluster) provisioners.Provisioner {
	p.remote = remote

	return p
}

// InNamespace implements the Provision interface.
func (p *ResourceProvisioner) InNamespace(_ string) provisioners.Provisioner {
	return p
}

// getClient either uses the current client, or gets a new remote one.
func (p *ResourceProvisioner) getClient(ctx context.Context) (client.Client, error) {
	client := p.client

	if p.remote != nil {
		remoteClient, err := remotecluster.GetClient(ctx, p.remote)
		if err != nil {
			return nil, err
		}

		client = remoteClient
	}

	return client, nil
}

// Provision implements the Provision interface.
func (p *ResourceProvisioner) Provision(ctx context.Context) error {
	log := log.FromContext(ctx)

	c, err := p.getClient(ctx)
	if err != nil {
		return err
	}

	objectKey := client.ObjectKeyFromObject(p.resource)

	// The object may use a GenerateName, so only check for existence if
	// it's a named resource.  It's up to the caller to work out whether
	// to provision a resource with a generated name.
	if objectKey.Name != "" {
		// Provide somewhere for get to write into and extract the GVK.
		var existing unstructured.Unstructured

		if err := c.Scheme().Convert(p.resource, &existing, nil); err != nil {
			return err
		}

		// Object exists, leave it alone.
		// TODO: we could diff the current and existing versions and do
		// object updates here in future.
		err := c.Get(ctx, objectKey, &existing)
		if err == nil {
			return nil
		}

		// If it genuninely doesn't exist, fall through to creation...
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
	}

	log.Info("creating object", "key", objectKey)

	// This treats the resource as mutable, so updates will been seen by the caller.
	// Especially useful if Kubenretes fills some things in for you, but just be
	// aware the resource shouldn't be resued.
	if err := c.Create(ctx, p.resource); err != nil {
		return err
	}

	log.Info("object created", "key", objectKey)

	return nil
}

// Deprovision implements the Provision interface.
func (p *ResourceProvisioner) Deprovision(ctx context.Context) error {
	log := log.FromContext(ctx)

	c, err := p.getClient(ctx)
	if err != nil {
		return err
	}

	objectKey := client.ObjectKeyFromObject(p.resource)

	log.Info("deleting object", "key", objectKey)

	if err := c.Delete(ctx, p.resource); err != nil {
		if errors.IsNotFound(err) {
			log.Info("object deleted", "key", objectKey)

			return nil
		}

		return err
	}

	if err := readiness.NewRetry(readiness.NewResourceNotExists(c, p.resource)).Check(ctx); err != nil {
		return err
	}

	log.Info("object deleted", "key", objectKey)

	return nil
}
