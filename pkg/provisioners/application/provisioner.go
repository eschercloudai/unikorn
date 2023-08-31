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

	unikornv1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/cd"
	"github.com/eschercloudai/unikorn/pkg/provisioners"

	"k8s.io/apimachinery/pkg/labels"

	"sigs.k8s.io/controller-runtime/pkg/log"
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
	Customize(version *string) ([]cd.HelmApplicationField, error)
}

// Provisioner deploys an application that is keyed to a specific resource.
// For example, ArgoCD dictates that applications be installed in the same
// namespace, so we use the resource to define a unique set of labels that
// identifies that resource out of all others, and add in the application
// name to uniquely identify the application within that resource.
type Provisioner struct {
	provisioners.ProvisionerMeta

	// driver is the CD driver that implements applications.
	driver cd.Driver

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
func New(driver cd.Driver, name string, resource MutuallyExclusiveResource, application *unikornv1.HelmApplication) *Provisioner {
	return &Provisioner{
		ProvisionerMeta: provisioners.ProvisionerMeta{
			Name: name,
		},
		driver:      driver,
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

func (p *Provisioner) getResourceID() (*cd.ResourceIdentifier, error) {
	id := &cd.ResourceIdentifier{
		Name: p.Name,
	}

	l, err := p.resource.ResourceLabels()
	if err != nil {
		return nil, err
	}

	if len(l) > 0 {
		id.Labels = make([]cd.ResourceIdentifierLabel, 0, len(l))

		for k, v := range l {
			id.Labels = append(id.Labels, cd.ResourceIdentifierLabel{
				Name:  k,
				Value: v,
			})
		}
	}

	return id, nil
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
func (p *Provisioner) getParameters() ([]cd.HelmApplicationParameter, error) {
	parameters := make([]cd.HelmApplicationParameter, 0, len(p.application.Spec.Parameters))

	for _, parameter := range p.application.Spec.Parameters {
		parameters = append(parameters, cd.HelmApplicationParameter{
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
				parameters = append(parameters, cd.HelmApplicationParameter{
					Name:  name,
					Value: value,
				})
			}
		}
	}

	// Makes gomock happy as "nil" != "[]foo{}".
	if len(parameters) == 0 {
		return nil, nil
	}

	return parameters, nil
}

// getValues delegates to the generator to get an option values.yaml file to
// pass to Helm.
func (p *Provisioner) getValues() (interface{}, error) {
	if p.generator == nil {
		//nolint:nilnil
		return nil, nil
	}

	valuesGenerator, ok := p.generator.(ValuesGenerator)
	if !ok {
		//nolint:nilnil
		return nil, nil
	}

	values, err := valuesGenerator.Values(p.application.Spec.Interface)
	if err != nil {
		return nil, err
	}

	return values, nil
}

// getClusterID returns the destination cluster name.
func (p *Provisioner) getClusterID() *cd.ResourceIdentifier {
	if p.remote != nil {
		return p.remote.ID()
	}

	return nil
}

// getNamespace returns an explicit namespace if one is set.
func (p *Provisioner) getNamespace() string {
	if p.namespace != "" {
		return p.namespace
	}

	return ""
}

// generateApplication converts the provided object to a canonical form for a driver.
func (p *Provisioner) generateApplication() (*cd.HelmApplication, error) {
	parameters, err := p.getParameters()
	if err != nil {
		return nil, err
	}

	values, err := p.getValues()
	if err != nil {
		return nil, err
	}

	application := &cd.HelmApplication{
		Repo:       *p.application.Spec.Repo,
		Version:    *p.application.Spec.Version,
		Release:    p.getReleaseName(),
		Parameters: parameters,
		Values:     values,
		Cluster:    p.getClusterID(),
		Namespace:  p.getNamespace(),
	}

	if p.application.Spec.Chart != nil {
		application.Chart = *p.application.Spec.Chart
	}

	if p.application.Spec.Path != nil {
		application.Path = *p.application.Spec.Path
	}

	if p.application.Spec.CreateNamespace != nil {
		application.CreateNamespace = *p.application.Spec.CreateNamespace
	}

	if p.application.Spec.ServerSideApply != nil {
		application.ServerSideApply = *p.application.Spec.ServerSideApply
	}

	if p.generator != nil {
		if customization, ok := p.generator.(Customizer); ok {
			ignoredDifferences, err := customization.Customize(p.application.Spec.Interface)
			if err != nil {
				return nil, err
			}

			application.IgnoreDifferences = ignoredDifferences
		}
	}

	return application, nil
}

// Provision implements the Provision interface.
func (p *Provisioner) Provision(ctx context.Context) error {
	log := log.FromContext(ctx)

	log.Info("provisioning application", "application", p.Name)

	// Convert the generic object type into what's expected by the driver interface.
	id, err := p.getResourceID()
	if err != nil {
		return err
	}

	application, err := p.generateApplication()
	if err != nil {
		return err
	}

	if err := p.driver.CreateOrUpdateHelmApplication(ctx, id, application); err != nil {
		return err
	}

	log.Info("application provisioned", "application", p.Name)

	return nil
}

// Deprovision implements the Provision interface.
func (p *Provisioner) Deprovision(ctx context.Context) error {
	log := log.FromContext(ctx)

	log.Info("deprovisioning application", "application", p.Name)

	id, err := p.getResourceID()
	if err != nil {
		return err
	}

	if err := p.driver.DeleteHelmApplication(ctx, id, p.backgroundDelete); err != nil {
		return err
	}

	return nil
}
