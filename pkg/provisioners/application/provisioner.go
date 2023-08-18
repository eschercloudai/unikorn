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

package application

import (
	"context"
	"errors"
	"reflect"

	argoprojv1 "github.com/eschercloudai/unikorn/pkg/apis/argoproj/v1alpha1"
	unikornv1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/constants"
	"github.com/eschercloudai/unikorn/pkg/provisioners"
	"github.com/eschercloudai/unikorn/pkg/provisioners/remotecluster"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"
)

const (
	// namespace is where all the applications live.  By necessity at
	// present.
	// TODO: Make this dynamic.
	namespace = "argocd"
)

var (
	// ErrItemLengthMismatch is returned when items are listed but the
	// wrong number are returned.  Given we are dealing with unique applications
	// one or zero are expected.
	ErrItemLengthMismatch = errors.New("item count not as expected")

	// ErrItemNotFound is raised when we cannot find the droid we are looking for.
	ErrItemNotFound = errors.New("item not found")
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

// ReleaseNamer is an interface that allows generators to supply an implicit release
// name to Helm.
type ReleaseNamer interface {
	ReleaseName() string
}

// Paramterizer is an interface that allows generators to supply a list of parameters
// to Helm.  These are in addition to those defined by the application template.  At
// present, there is nothing special about overriding, it just appends, so ensure the
// explicit and implicit sets don't overlap.
type Paramterizer interface {
	Parameters(version *string) (map[string]string, error)
}

// ValuesGenerator is an interface that allows generators to supply a raw values.yaml
// file to Helm.  This accepts an object that can be marshaled to YAML.
type ValuesGenerator interface {
	Values(version *string) (interface{}, error)
}

// Customizer is a generic generator interface that implemnets raw customizations to
// the application template.  Try to avoid using this.
type Customizer interface {
	Customize(version *string, application *argoprojv1.Application) error
}

// Provisioner deploys an application that is keyed to a specific resource.
// For example, ArgoCD dictates that applications be installed in the same
// namespace, so we use the resource to define a unique set of labels that
// identifies that resource out of all others, and add in the application
// name to uniquely identify the application within that resource.
type Provisioner struct {
	provisioners.ProvisionerMeta

	// client provides access to Kubernetes.
	client client.Client

	// remote is the remote cluster to deploy to.
	remote provisioners.RemoteCluster

	// remoteNamespace explicitly sets the namespace for the application.
	namespace string

	// generator provides application generation functionality.
	generator interface{}

	// resource is the top level resource an application belongs to, this
	// is used to derive a unique label set to identify the resource.
	resource MutuallyExclusiveResource

	// application is the generic Helm application descriptor.
	application *unikornv1.HelmApplication

	// backgroundDelete means we don't care about whether it's deprovisioned
	// successfully or not, especially useful for flaky apps living in a
	// remote cluster that going to get terminated anyway.
	backgroundDelete bool
}

// New returns a new initialized provisioner object.
func New(client client.Client, name string, resource MutuallyExclusiveResource, application *unikornv1.HelmApplication) *Provisioner {
	return &Provisioner{
		ProvisionerMeta: provisioners.ProvisionerMeta{
			Name: name,
		},
		client:      client,
		resource:    resource,
		application: application,
	}
}

// Ensure the Provisioner interface is implemented.
var _ provisioners.Provisioner = &Provisioner{}

// OnRemote implements the Provision interface.
func (p *Provisioner) OnRemote(remote provisioners.RemoteCluster) *Provisioner {
	p.remote = remote

	return p
}

// InNamespace deploys the application into an explicit namespace.
func (p *Provisioner) InNamespace(namespace string) *Provisioner {
	p.namespace = namespace

	return p
}

// WithGenerator registers an object that can generate implicit configuration where
// you cannot do it all from the default set of arguments.
func (p *Provisioner) WithGenerator(generator interface{}) *Provisioner {
	p.generator = generator

	return p
}

func (p *Provisioner) BackgroundDelete() *Provisioner {
	p.backgroundDelete = true

	return p
}

// getLabels returns a unique set of labels for the application.
func (p *Provisioner) getLabels() (labels.Set, error) {
	l, err := p.resource.ResourceLabels()
	if err != nil {
		return nil, err
	}

	return labels.Merge(l, labels.Set{constants.ApplicationLabel: p.Name}), nil
}

// getReleaseName uses the release name in the application spec by default
// but allows the generator to override it.
func (p *Provisioner) getReleaseName() string {
	var name string

	if p.application.Spec.Release != nil {
		name = *p.application.Spec.Release
	}

	if p.generator != nil {
		if releaseNamer, ok := p.generator.(ReleaseNamer); ok {
			override := releaseNamer.ReleaseName()

			if override != "" {
				name = override
			}
		}
	}

	return name
}

// getParameters constructs a full list of Helm parameters by taking those provided
// in the application spec, and appending any that the generator yields.
func (p *Provisioner) getParameters() ([]argoprojv1.HelmParameter, error) {
	parameters := make([]argoprojv1.HelmParameter, 0, len(p.application.Spec.Parameters))

	for _, parameter := range p.application.Spec.Parameters {
		parameters = append(parameters, argoprojv1.HelmParameter{
			Name:  *parameter.Name,
			Value: *parameter.Value,
		})
	}

	if p.generator != nil {
		if parameterizer, ok := p.generator.(Paramterizer); ok {
			p, err := parameterizer.Parameters(p.application.Spec.Interface)
			if err != nil {
				return nil, err
			}

			for name, value := range p {
				parameters = append(parameters, argoprojv1.HelmParameter{
					Name:  name,
					Value: value,
				})
			}
		}
	}

	// Tempting as it is to delete this, this ensures we can get a zero value
	// out of this function.
	if len(parameters) == 0 {
		return nil, nil
	}

	return parameters, nil
}

// getValues delegates to the generator to get an option values.yaml file to
// pass to Helm.
func (p *Provisioner) getValues() (string, error) {
	if p.generator == nil {
		return "", nil
	}

	valuesGenerator, ok := p.generator.(ValuesGenerator)
	if !ok {
		return "", nil
	}

	values, err := valuesGenerator.Values(p.application.Spec.Interface)
	if err != nil {
		return "", err
	}

	marshaled, err := yaml.Marshal(values)
	if err != nil {
		return "", err
	}

	return string(marshaled), nil
}

// getDestinationName returns the destination cluster name.
func (p *Provisioner) getDestinationName() string {
	name := "in-cluster"

	if p.remote != nil {
		name = remotecluster.GenerateName(p.remote)
	}

	return name
}

// getDestinationNamespace returns an explicit namespace if one is set.
func (p *Provisioner) getDestinationNamespace() string {
	namespace := ""

	if p.namespace != "" {
		namespace = p.namespace
	}

	return namespace
}

// getSyncOptions accumulates any synchronization options.
func (p *Provisioner) getSyncOptions() []argoprojv1.ApplicationSyncOption {
	var options []argoprojv1.ApplicationSyncOption

	if p.application.Spec.CreateNamespace != nil && *p.application.Spec.CreateNamespace {
		options = append(options, argoprojv1.CreateNamespace)
	}

	if p.application.Spec.ServerSideApply != nil && *p.application.Spec.ServerSideApply {
		options = append(options, argoprojv1.ServerSideApply)
	}

	return options
}

// generateResource converts the provided object to a canonical unstructured form.
//
//nolint:cyclop
func (p *Provisioner) generateResource() (*argoprojv1.Application, error) {
	labels, err := p.getLabels()
	if err != nil {
		return nil, err
	}

	parameters, err := p.getParameters()
	if err != nil {
		return nil, err
	}

	values, err := p.getValues()
	if err != nil {
		return nil, err
	}

	helm := &argoprojv1.ApplicationSourceHelm{
		ReleaseName: p.getReleaseName(),
		Parameters:  parameters,
		Values:      values,
	}

	application := &argoprojv1.Application{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: p.Name + "-",
			Namespace:    namespace,
			Labels:       labels,
		},
		Spec: argoprojv1.ApplicationSpec{
			Project: "default",
			Source: argoprojv1.ApplicationSource{
				RepoURL:        *p.application.Spec.Repo,
				TargetRevision: *p.application.Spec.Version,
			},
			Destination: argoprojv1.ApplicationDestination{
				Name:      p.getDestinationName(),
				Namespace: p.getDestinationNamespace(),
			},
			SyncPolicy: argoprojv1.ApplicationSyncPolicy{
				Automated: &argoprojv1.ApplicationSyncAutomation{
					SelfHeal: true,
					Prune:    true,
				},
			},
		},
	}

	if !reflect.ValueOf(*helm).IsZero() {
		application.Spec.Source.Helm = helm
	}

	if p.application.Spec.Chart != nil {
		application.Spec.Source.Chart = *p.application.Spec.Chart
	}

	if p.application.Spec.Path != nil {
		application.Spec.Source.Path = *p.application.Spec.Path
	}

	syncOptions := p.getSyncOptions()

	if len(syncOptions) != 0 {
		application.Spec.SyncPolicy.SyncOptions = syncOptions
	}

	if p.generator != nil {
		if customization, ok := p.generator.(Customizer); ok {
			if err := customization.Customize(p.application.Spec.Interface, application); err != nil {
				return nil, err
			}
		}
	}

	return application, nil
}

// FindApplication looks up any existing resource using a label selector, you must use
// generated names here as it's a multi-tenant platform, argo enforces the use of a single
// namespace, and we want users to be able to define their own names irrespective
// of other users.
func (p *Provisioner) FindApplication(ctx context.Context) (*argoprojv1.Application, error) {
	var resources argoprojv1.ApplicationList

	l, err := p.getLabels()
	if err != nil {
		return nil, err
	}

	if err := p.client.List(ctx, &resources, &client.ListOptions{Namespace: namespace, LabelSelector: labels.SelectorFromSet(l)}); err != nil {
		return nil, err
	}

	if len(resources.Items) == 0 {
		return nil, ErrItemNotFound
	}

	if len(resources.Items) > 1 {
		return nil, ErrItemLengthMismatch
	}

	return &resources.Items[0], nil
}

// Provision implements the Provision interface.
func (p *Provisioner) Provision(ctx context.Context) error {
	log := log.FromContext(ctx)

	log.Info("provisioning application", "application", p.Name)

	// Convert the generic object type into unstructured for the next bit...
	required, err := p.generateResource()
	if err != nil {
		return err
	}

	// Resource, after provisioning, should be set to either the existing resource
	// or the newly created one.  The point here is the API will have filled in
	// the name so we can perform readiness checks.
	resource, err := p.FindApplication(ctx)
	if err != nil {
		// Something bad has happened.
		if !errors.Is(err, ErrItemNotFound) {
			return err
		}
	}

	// TODO: can probably just use controllerutil.CreateOrPatch.
	if resource == nil {
		log.Info("creating new application", "application", p.Name)

		if err := p.client.Create(ctx, required); err != nil {
			return err
		}

		resource = required
	} else {
		log.Info("updating existing application", "application", p.Name)

		// Replace the specification with what we expect.
		temp := resource.DeepCopy()
		temp.Labels = required.Labels
		temp.Spec = required.Spec

		if err := p.client.Patch(ctx, temp, client.MergeFrom(resource)); err != nil {
			return err
		}

		resource = temp
	}

	log.Info("checking application health", "application", p.Name)

	// NOTE: This isn't necessarily accurate, take CAPI clusters for instance,
	// that's just a bunch of CRs, and they are instantly healthy until
	// CAPI/CAPO take note and start making status updates...
	if resource.Status.Health == nil || resource.Status.Health.Status != argoprojv1.Healthy {
		log.Info("application not healthy, yielding", "application", p.Name)

		return provisioners.ErrYield
	}

	log.Info("application provisioned", "application", p.Name)

	return nil
}

// Deprovision implements the Provision interface.
func (p *Provisioner) Deprovision(ctx context.Context) error {
	log := log.FromContext(ctx)

	log.Info("deprovisioning application", "application", p.Name)

	resource, err := p.FindApplication(ctx)
	if err != nil {
		if errors.Is(err, ErrItemNotFound) {
			log.Info("application deleted", "application", p.Name)

			return nil
		}

		return err
	}

	if resource.GetDeletionTimestamp() != nil {
		if p.backgroundDelete {
			return nil
		}

		log.Info("waiting for application deletion", "application", p.Name)

		return provisioners.ErrYield
	}

	log.Info("adding application finalizer", "application", p.Name)

	// Apply a finalizer to ensure synchronous deletion. See
	// https://argo-cd.readthedocs.io/en/stable/user-guide/app_deletion/
	temp := resource.DeepCopy()
	temp.SetFinalizers([]string{"resources-finalizer.argocd.argoproj.io"})

	// Try to work around a race during deletion as per
	// https://github.com/argoproj/argo-cd/issues/12943
	temp.Spec.SyncPolicy.Automated = nil

	if err := p.client.Patch(ctx, temp, client.MergeFrom(resource)); err != nil {
		return err
	}

	log.Info("deleting application", "application", p.Name)

	if err := p.client.Delete(ctx, resource); err != nil {
		return err
	}

	if !p.backgroundDelete {
		return provisioners.ErrYield
	}

	return nil
}
