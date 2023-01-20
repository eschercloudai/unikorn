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

package application

import (
	"context"
	"errors"

	"github.com/eschercloudai/unikorn/pkg/constants"
	"github.com/eschercloudai/unikorn/pkg/provisioners"
	"github.com/eschercloudai/unikorn/pkg/readiness"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	ErrItemLengthMismatch = errors.New("item count not as expected")
)

// Provisioner wraps up a whole load of horror code required to
// get vcluster into a deployed and usable state.
type Provisioner struct {
	// client provides access to Kubernetes.
	client client.Client

	// name is the application name.
	name string

	// object is the object's required state.
	object client.Object

	// labels defines a unique application label selector.
	labels labels.Set
}

// New returns a new initialized provisioner object.
func New(client client.Client, name string, scope labels.Set, object client.Object) *Provisioner {
	return &Provisioner{
		client: client,
		name:   name,
		object: object,
		labels: labels.Merge(scope, labels.Set{constants.ApplicationLabel: name}),
	}
}

// Ensure the Provisioner interface is implemented.
var _ provisioners.Provisioner = &Provisioner{}

// toUnstructured converts the provided object to a canonical unstructured form.
func (p *Provisioner) toUnstructured() (*unstructured.Unstructured, error) {
	switch t := p.object.(type) {
	case *unstructured.Unstructured:
		return t, nil
	default:
		u := &unstructured.Unstructured{}

		if err := p.client.Scheme().Convert(t, u, nil); err != nil {
			return nil, err
		}

		return u, nil
	}
}

// findApplication looks up any existing resource using a label selector, you must use
// generated names here as it's a multi-tenant platform, argo enforces the use of a single
// namespace, and we want users to be able to define their own names irrespective
// of other users.
func (p *Provisioner) findApplication(ctx context.Context) (*unstructured.Unstructured, error) {
	resources := &unstructured.UnstructuredList{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
		},
	}

	selector := labels.SelectorFromSet(p.labels)

	if err := p.client.List(ctx, resources, &client.ListOptions{Namespace: "argocd", LabelSelector: selector}); err != nil {
		return nil, err
	}

	var resource *unstructured.Unstructured

	if len(resources.Items) > 1 {
		return nil, ErrItemLengthMismatch
	}

	if len(resources.Items) == 1 {
		resource = &resources.Items[0]
	}

	return resource, nil
}

// Provision implements the Provision interface.
func (p *Provisioner) Provision(ctx context.Context) error {
	log := log.FromContext(ctx)

	log.Info("provisioning application", "application", p.name)

	// Convert the generic object type into unstructured for the next bit...
	required, err := p.toUnstructured()
	if err != nil {
		return err
	}

	required.SetGenerateName(p.name + "-")
	required.SetLabels(p.labels)

	// Resource, after provisioning, should be set to either the existing resource
	// or the newly created one.  The point here is the API will have filled in
	// the name so we can perform readiness checks.
	resource, err := p.findApplication(ctx)
	if err != nil {
		return err
	}

	if resource == nil {
		log.Info("creating new application", "application", p.name)

		if err := p.client.Create(ctx, required); err != nil {
			return err
		}

		resource = required
	} else {
		log.Info("updating existing application", "application", p.name)

		// Replace the specification with what we expect.
		temp := resource.DeepCopy()
		temp.Object["spec"] = required.Object["spec"]

		if err := p.client.Patch(ctx, temp, client.MergeFrom(resource)); err != nil {
			return err
		}
	}

	log.Info("waiting for application to become healthy", "application", p.name)

	applicationHealthy := readiness.NewApplicationHealthy(p.client, resource)

	if err := readiness.NewRetry(applicationHealthy).Check(ctx); err != nil {
		return err
	}

	log.Info("application provisioned", "application", p.name)

	return nil
}

// Deprovision implements the Provision interface.
func (p *Provisioner) Deprovision(ctx context.Context) error {
	log := log.FromContext(ctx)

	log.Info("deprovisioning application", "application", p.name)

	resource, err := p.findApplication(ctx)
	if err != nil {
		return err
	}

	if resource == nil {
		log.Info("application does not exist", "application", p.name)

		return nil
	}

	log.Info("adding application finalizer", "application", p.name)

	// Apply a finalizer to ensure synchronous deletion. See
	// https://argo-cd.readthedocs.io/en/stable/user-guide/app_deletion/
	temp := resource.DeepCopy()
	temp.SetFinalizers([]string{"resources-finalizer.argocd.argoproj.io"})

	if err := p.client.Patch(ctx, temp, client.MergeFrom(resource)); err != nil {
		return err
	}

	log.Info("deleting application", "application", p.name)

	if err := p.client.Delete(ctx, resource); err != nil {
		return err
	}

	log.Info("waiting for application deletion", "application", p.name)

	if err := readiness.NewRetry(readiness.NewResourceNotExists(p.client, resource)).Check(ctx); err != nil {
		return err
	}

	log.Info("application deleted", "application", p.name)

	return nil
}
