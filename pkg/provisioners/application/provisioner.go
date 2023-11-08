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
	"slices"

	unikornv1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/cd"
	"github.com/eschercloudai/unikorn/pkg/provisioners"
	"github.com/eschercloudai/unikorn/pkg/provisioners/util"
	uutil "github.com/eschercloudai/unikorn/pkg/util"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Provisioner deploys an application that is keyed to a specific resource.
// For example, ArgoCD dictates that applications be installed in the same
// namespace, so we use the resource to define a unique set of labels that
// identifies that resource out of all others, and add in the application
// name to uniquely identify the application within that resource.
type Provisioner struct {
	// ProvisionerMeta defines the application name, this directly affects
	// the application what will be searched for in the application bundle
	// defined in the resource.  It will also be the default Application ID
	// name, unless overridden by applicationName.
	provisioners.ProvisionerMeta

	// namespace explicitly sets the namespace for the application.
	namespace string

	// applicationName allows the application name to be overridden.
	applicationName string

	// generator provides application generation functionality.
	generator interface{}
}

// New returns a new initialized provisioner object.
func New(name string) *Provisioner {
	return &Provisioner{
		ProvisionerMeta: provisioners.ProvisionerMeta{
			Name: name,
		},
	}
}

// Ensure the Provisioner interface is implemented.
var _ provisioners.Provisioner = &Provisioner{}

// InNamespace deploys the application into an explicit namespace.
func (p *Provisioner) InNamespace(namespace string) *Provisioner {
	p.namespace = namespace

	return p
}

// WithApplicationName allows the application name to be modified, rather than using
// application.Name.
func (p *Provisioner) WithApplicationName(name string) *Provisioner {
	p.applicationName = name

	return p
}

// WithGenerator registers an object that can generate implicit configuration where
// you cannot do it all from the default set of arguments.
func (p *Provisioner) WithGenerator(generator interface{}) *Provisioner {
	p.generator = generator

	return p
}

func (p *Provisioner) getResourceID(ctx context.Context) (*cd.ResourceIdentifier, error) {
	name := p.Name

	if p.applicationName != "" {
		name = p.applicationName
	}

	id := &cd.ResourceIdentifier{
		Name: name,
	}

	l, err := FromContext(ctx).ResourceLabels()
	if err != nil {
		return nil, err
	}

	if len(l) > 0 {
		id.Labels = make([]cd.ResourceIdentifierLabel, 0, len(l))

		// Make label ordering deterministic for the sake of testing...
		k := uutil.Keys(l)
		slices.Sort(k)

		for _, key := range k {
			id.Labels = append(id.Labels, cd.ResourceIdentifierLabel{
				Name:  key,
				Value: l[key],
			})
		}
	}

	return id, nil
}

// getReleaseName uses the release name in the application spec by default
// but allows the generator to override it.
func (p *Provisioner) getReleaseName(ctx context.Context, application *unikornv1.HelmApplication) string {
	var name string

	if application.Spec.Release != nil {
		name = *application.Spec.Release
	}

	if p.generator != nil {
		if releaseNamer, ok := p.generator.(ReleaseNamer); ok {
			override := releaseNamer.ReleaseName(ctx)

			if override != "" {
				name = override
			}
		}
	}

	return name
}

// getParameters constructs a full list of Helm parameters by taking those provided
// in the application spec, and appending any that the generator yields.
func (p *Provisioner) getParameters(ctx context.Context, application *unikornv1.HelmApplication) ([]cd.HelmApplicationParameter, error) {
	parameters := make([]cd.HelmApplicationParameter, 0, len(application.Spec.Parameters))

	for _, parameter := range application.Spec.Parameters {
		parameters = append(parameters, cd.HelmApplicationParameter{
			Name:  *parameter.Name,
			Value: *parameter.Value,
		})
	}

	if p.generator != nil {
		if parameterizer, ok := p.generator.(Paramterizer); ok {
			p, err := parameterizer.Parameters(ctx, application.Spec.Interface)
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
func (p *Provisioner) getValues(ctx context.Context, application *unikornv1.HelmApplication) (interface{}, error) {
	if p.generator == nil {
		//nolint:nilnil
		return nil, nil
	}

	valuesGenerator, ok := p.generator.(ValuesGenerator)
	if !ok {
		//nolint:nilnil
		return nil, nil
	}

	values, err := valuesGenerator.Values(ctx, application.Spec.Interface)
	if err != nil {
		return nil, err
	}

	return values, nil
}

// getClusterID returns the destination cluster name.
func (p *Provisioner) getClusterID() *cd.ResourceIdentifier {
	if p.Remote != nil {
		return p.Remote.ID()
	}

	return nil
}

// getApplication looks up the application in the resource's application catalogue/bundle.
func (p *Provisioner) getApplication(ctx context.Context) (*unikornv1.HelmApplication, error) {
	var application *unikornv1.HelmApplication

	unbundler := util.NewUnbundler(FromContext(ctx))
	unbundler.AddApplication(&application, p.Name)

	if err := unbundler.Unbundle(ctx); err != nil {
		return nil, err
	}

	return application, nil
}

// generateApplication converts the provided object to a canonical form for a driver.
//
//nolint:cyclop
func (p *Provisioner) generateApplication(ctx context.Context) (*cd.HelmApplication, error) {
	application, err := p.getApplication(ctx)
	if err != nil {
		return nil, err
	}

	parameters, err := p.getParameters(ctx, application)
	if err != nil {
		return nil, err
	}

	values, err := p.getValues(ctx, application)
	if err != nil {
		return nil, err
	}

	cdApplication := &cd.HelmApplication{
		Repo:       *application.Spec.Repo,
		Version:    *application.Spec.Version,
		Release:    p.getReleaseName(ctx, application),
		Parameters: parameters,
		Values:     values,
		Cluster:    p.getClusterID(),
		Namespace:  p.namespace,
	}

	if application.Spec.Chart != nil {
		cdApplication.Chart = *application.Spec.Chart
	}

	if application.Spec.Path != nil {
		cdApplication.Path = *application.Spec.Path
	}

	if application.Spec.CreateNamespace != nil {
		cdApplication.CreateNamespace = *application.Spec.CreateNamespace
	}

	if application.Spec.ServerSideApply != nil {
		cdApplication.ServerSideApply = *application.Spec.ServerSideApply
	}

	if p.generator != nil {
		if customization, ok := p.generator.(Customizer); ok {
			ignoredDifferences, err := customization.Customize(application.Spec.Interface)
			if err != nil {
				return nil, err
			}

			cdApplication.IgnoreDifferences = ignoredDifferences
		}
	}

	return cdApplication, nil
}

// Provision implements the Provision interface.
func (p *Provisioner) Provision(ctx context.Context) error {
	log := log.FromContext(ctx)

	log.Info("provisioning application", "application", p.Name, "remote", p.Remote)

	// Convert the generic object type into what's expected by the driver interface.
	id, err := p.getResourceID(ctx)
	if err != nil {
		return err
	}

	application, err := p.generateApplication(ctx)
	if err != nil {
		return err
	}

	if err := cd.FromContext(ctx).CreateOrUpdateHelmApplication(ctx, id, application); err != nil {
		return err
	}

	log.Info("application provisioned", "application", p.Name)

	if p.generator != nil {
		if hook, ok := p.generator.(PostProvisionHook); ok {
			if err := hook.PostProvision(ctx); err != nil {
				return err
			}
		}
	}

	return nil
}

// Deprovision implements the Provision interface.
func (p *Provisioner) Deprovision(ctx context.Context) error {
	log := log.FromContext(ctx)

	log.Info("deprovisioning application", "application", p.Name)

	id, err := p.getResourceID(ctx)
	if err != nil {
		return err
	}

	if err := cd.FromContext(ctx).DeleteHelmApplication(ctx, id, p.BackgroundDelete); err != nil {
		return err
	}

	return nil
}
