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
	"github.com/eschercloudai/unikorn/pkg/provisioners/remotecluster"
	"github.com/eschercloudai/unikorn/pkg/readiness"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// namespace is where all the applications live.  BY necessity at
	// present.
	// TODO: Make this dynamic.
	namespace = "argocd"
)

var (
	// ErrItemLengthMismatch is returned when items are listed but the
	// wrong number are returned.  Given we are dealing with unique applications
	// one or zero are expected.
	ErrItemLengthMismatch = errors.New("item count not as expected")
)

// MutuallyExclusiveResource is a generic interface over all resource types,
// where the resource can be uniquely identified.  As these typically map to
// custom resource types, be extra careful you don't overload anything in
// metav1.Object or runtime.Object.
type MutuallyExclusiveResource interface {
	// ResourceLabels returns a set of labels from the resource that uniquely
	// identify it, if they all were to reside in the same namespace.
	// In database terms this would be a composite key.
	ResourceLabels() (labels.Set, error)
}

// Generator defines a common interface for clients to
// generate application templates.
type Generator interface {
	// Resouece returns the parent resource an application
	// belongs to.
	Resource() MutuallyExclusiveResource

	// Name returns the unique application name.
	Name() string

	// Generate creates an application resource template. It just needs
	// to return the spec as a top level key, all other type and object
	// meta are filled in by this.
	// TODO: This is still very Argo specific, we need to abstract away
	// this to be more Helm-centric.
	Generate() (client.Object, error)
}

// Provisioner deploys an application that is keyed to a specific resource.
// For example, ArgoCD dictates that applications be installed in the same
// namespace, so we use the resource to define a unique set of labels that
// identifies that resource out of all others, and add in the application
// name to uniquely identify the application within that resource.
// TODO: These can still alias e.g. {a: a, app: foo} and {a: a, b: b, app: foo}
// match when selected using the first set with the same application deployed
// in different scopes.
type Provisioner struct {
	// client provides access to Kubernetes.
	client client.Client

	// remote is the remote cluster to deploy to.
	remote remotecluster.Generator

	// remoteNamespace explicitly sets the namespace for the application.
	namespace string

	// generator provides application generation functionality.
	generator Generator
}

// New returns a new initialized provisioner object.
func New(client client.Client, generator Generator) *Provisioner {
	return &Provisioner{
		client:    client,
		generator: generator,
	}
}

// Ensure the Provisioner interface is implemented.
var _ provisioners.Provisioner = &Provisioner{}

// OnRemote deploys the application on a remote cluster.
func (p *Provisioner) OnRemote(remote remotecluster.Generator) *Provisioner {
	p.remote = remote

	return p
}

// InNamespace deploys the application into an explicit namespace.
func (p *Provisioner) InNamespace(namespace string) *Provisioner {
	p.namespace = namespace

	return p
}

// labels returns a unique set of labels for the application.
func (p *Provisioner) labels() (labels.Set, error) {
	l, err := p.generator.Resource().ResourceLabels()
	if err != nil {
		return nil, err
	}

	return labels.Merge(l, labels.Set{constants.ApplicationLabel: p.generator.Name()}), nil
}

// toUnstructured converts the provided object to a canonical unstructured form.
func (p *Provisioner) toUnstructured() (*unstructured.Unstructured, error) {
	object, err := p.generator.Generate()
	if err != nil {
		return nil, err
	}

	switch t := object.(type) {
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

	l, err := p.labels()
	if err != nil {
		return nil, err
	}

	if err := p.client.List(ctx, resources, &client.ListOptions{Namespace: namespace, LabelSelector: labels.SelectorFromSet(l)}); err != nil {
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

// applyResourceDefaults adds in things we explicitly control about the application.
func (p *Provisioner) applyResourceDefaults(object *unstructured.Unstructured) error {
	labels, err := p.labels()
	if err != nil {
		return err
	}

	object.SetAPIVersion("argoproj.io/v1alpha1")
	object.SetKind("Application")
	object.SetGenerateName(p.generator.Name() + "-")
	object.SetNamespace(namespace)
	object.SetLabels(labels)

	return nil
}

// applyResourceDestination adds in optional destination information.
func (p *Provisioner) applyResourceDestination(object *unstructured.Unstructured) error {
	destination := map[string]interface{}{}

	if p.remote != nil {
		destination["name"] = remotecluster.GenerateName(p.remote)
	}

	if p.namespace != "" {
		destination["namespace"] = p.namespace
	}

	if len(destination) != 0 {
		if err := unstructured.SetNestedField(object.Object, destination, "spec", "destination"); err != nil {
			return err
		}
	}

	return nil
}

// Provision implements the Provision interface.
func (p *Provisioner) Provision(ctx context.Context) error {
	log := log.FromContext(ctx)

	log.Info("provisioning application", "application", p.generator.Name())

	// Convert the generic object type into unstructured for the next bit...
	required, err := p.toUnstructured()
	if err != nil {
		return err
	}

	if err := p.applyResourceDefaults(required); err != nil {
		return err
	}

	if err := p.applyResourceDestination(required); err != nil {
		return err
	}

	// Resource, after provisioning, should be set to either the existing resource
	// or the newly created one.  The point here is the API will have filled in
	// the name so we can perform readiness checks.
	resource, err := p.findApplication(ctx)
	if err != nil {
		return err
	}

	if resource == nil {
		log.Info("creating new application", "application", p.generator.Name())

		if err := p.client.Create(ctx, required); err != nil {
			return err
		}

		resource = required
	} else {
		log.Info("updating existing application", "application", p.generator.Name())

		// Replace the specification with what we expect.
		temp := resource.DeepCopy()
		temp.Object["spec"] = required.Object["spec"]

		if err := p.client.Patch(ctx, temp, client.MergeFrom(resource)); err != nil {
			return err
		}
	}

	log.Info("waiting for application to become healthy", "application", p.generator.Name())

	applicationHealthy := readiness.NewApplicationHealthy(p.client, resource)

	if err := readiness.NewRetry(applicationHealthy).Check(ctx); err != nil {
		return err
	}

	log.Info("application provisioned", "application", p.generator.Name())

	return nil
}

// Deprovision implements the Provision interface.
func (p *Provisioner) Deprovision(ctx context.Context) error {
	log := log.FromContext(ctx)

	log.Info("deprovisioning application", "application", p.generator.Name())

	resource, err := p.findApplication(ctx)
	if err != nil {
		return err
	}

	if resource == nil {
		log.Info("application does not exist", "application", p.generator.Name())

		return nil
	}

	log.Info("adding application finalizer", "application", p.generator.Name())

	// Apply a finalizer to ensure synchronous deletion. See
	// https://argo-cd.readthedocs.io/en/stable/user-guide/app_deletion/
	temp := resource.DeepCopy()
	temp.SetFinalizers([]string{"resources-finalizer.argocd.argoproj.io"})

	if err := p.client.Patch(ctx, temp, client.MergeFrom(resource)); err != nil {
		return err
	}

	log.Info("deleting application", "application", p.generator.Name())

	if err := p.client.Delete(ctx, resource); err != nil {
		return err
	}

	log.Info("waiting for application deletion", "application", p.generator.Name())

	if err := readiness.NewRetry(readiness.NewResourceNotExists(p.client, resource)).Check(ctx); err != nil {
		return err
	}

	log.Info("application deleted", "application", p.generator.Name())

	return nil
}
