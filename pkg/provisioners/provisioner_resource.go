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

package provisioners

import (
	"context"

	"github.com/eschercloudai/unikorn/pkg/util"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

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
}

// Ensure the Provisioner interface is implemented.
var _ Provisioner = &ResourceProvisioner{}

// NewResourceProvisioner returns a new provisioner that is capable of applying
// a manifest with kubectl.  The path argument may be a path on the local file
// system or a URL.
func NewResourceProvisioner(client client.Client, resource client.Object) *ResourceProvisioner {
	return &ResourceProvisioner{
		client:   client,
		resource: resource,
	}
}

// Provision implements the Provision interface.
func (p *ResourceProvisioner) Provision(ctx context.Context) error {
	log := log.FromContext(ctx)

	var object *unstructured.Unstructured

	switch t := p.resource.(type) {
	case *unstructured.Unstructured:
		object = t
	default:
		gvk, err := util.ObjectGroupVersionKind(p.client.Scheme(), p.resource)
		if err != nil {
			return err
		}

		o, err := runtime.DefaultUnstructuredConverter.ToUnstructured(p.resource)
		if err != nil {
			return err
		}

		u := &unstructured.Unstructured{
			Object: o,
		}

		u.SetGroupVersionKind(*gvk)

		object = u
	}

	// Create the object if it doesn't exist.
	// TODO: the fallthrough case here should be update and upgrade,
	// but that's a ways off!
	objectKey := client.ObjectKeyFromObject(object)

	// NOTE: while we don't strictly need the existing resource yet, it'll
	// moan if you don't provide something to store into.
	existing, ok := object.NewEmptyInstance().(*unstructured.Unstructured)
	if !ok {
		panic("unstructured empty instance fail")
	}

	if err := p.client.Get(ctx, objectKey, existing); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}

		log.Info("creating object", "key", objectKey, "gvk", object.GroupVersionKind())

		if err := p.client.Create(ctx, object); err != nil {
			return err
		}
	}

	return nil
}
