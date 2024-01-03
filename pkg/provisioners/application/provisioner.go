/*
Copyright 2022-2024 EscherCloud.

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
	clientlib "github.com/eschercloudai/unikorn/pkg/client"
	"github.com/eschercloudai/unikorn/pkg/provisioners"
	uutil "github.com/eschercloudai/unikorn/pkg/util"

	"sigs.k8s.io/controller-runtime/pkg/client"
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
	// name, unless overridden by Name.
	provisioners.ProvisionerMeta

	// namespace explicitly sets the namespace for the application.
	namespace string

	// generator provides application generation functionality.
	generator interface{}

	// allowDegraded accepts a degraded status as a success for an application.
	allowDegraded bool

	// getApplicationReference provides an abstract way to get concrete application metadata
	// and provisioning information.
	getApplicationReference GetterFunc

	// applicationVersion is a reference to a versioned application.
	applicationVersion *unikornv1.HelmApplicationVersion
}

// New returns a new initialized provisioner object.
// Note as the application lookup is dynamic, we need to defer initialization
// until later in the provisioning to keep top-level interfaces clean.
func New(getApplicationReference GetterFunc) *Provisioner {
	return &Provisioner{
		getApplicationReference: getApplicationReference,
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
	p.Name = name

	return p
}

// WithGenerator registers an object that can generate implicit configuration where
// you cannot do it all from the default set of arguments.
func (p *Provisioner) WithGenerator(generator interface{}) *Provisioner {
	p.generator = generator

	return p
}

// AllowDegraded accepts a degraded status as a success for an application.
func (p *Provisioner) AllowDegraded() *Provisioner {
	p.allowDegraded = true

	return p
}

func (p *Provisioner) getResourceID(ctx context.Context) (*cd.ResourceIdentifier, error) {
	id := &cd.ResourceIdentifier{
		Name: p.Name,
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
func (p *Provisioner) getReleaseName(ctx context.Context) string {
	var name string

	if p.applicationVersion.Release != nil {
		name = *p.applicationVersion.Release
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
func (p *Provisioner) getParameters(ctx context.Context) ([]cd.HelmApplicationParameter, error) {
	parameters := make([]cd.HelmApplicationParameter, 0, len(p.applicationVersion.Parameters))

	for _, parameter := range p.applicationVersion.Parameters {
		parameters = append(parameters, cd.HelmApplicationParameter{
			Name:  *parameter.Name,
			Value: *parameter.Value,
		})
	}

	if p.generator != nil {
		if parameterizer, ok := p.generator.(Paramterizer); ok {
			p, err := parameterizer.Parameters(ctx, p.applicationVersion.Interface)
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
func (p *Provisioner) getValues(ctx context.Context) (interface{}, error) {
	if p.generator == nil {
		//nolint:nilnil
		return nil, nil
	}

	valuesGenerator, ok := p.generator.(ValuesGenerator)
	if !ok {
		//nolint:nilnil
		return nil, nil
	}

	values, err := valuesGenerator.Values(ctx, p.applicationVersion.Interface)
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

// generateApplication converts the provided object to a canonical form for a driver.
func (p *Provisioner) generateApplication(ctx context.Context) (*cd.HelmApplication, error) {
	parameters, err := p.getParameters(ctx)
	if err != nil {
		return nil, err
	}

	values, err := p.getValues(ctx)
	if err != nil {
		return nil, err
	}

	cdApplication := &cd.HelmApplication{
		Repo:          *p.applicationVersion.Repo,
		Version:       *p.applicationVersion.Version,
		Release:       p.getReleaseName(ctx),
		Parameters:    parameters,
		Values:        values,
		Cluster:       p.getClusterID(),
		Namespace:     p.namespace,
		AllowDegraded: p.allowDegraded,
	}

	if p.applicationVersion.Chart != nil {
		cdApplication.Chart = *p.applicationVersion.Chart
	}

	if p.applicationVersion.Path != nil {
		cdApplication.Path = *p.applicationVersion.Path
	}

	if p.applicationVersion.CreateNamespace != nil {
		cdApplication.CreateNamespace = *p.applicationVersion.CreateNamespace
	}

	if p.applicationVersion.ServerSideApply != nil {
		cdApplication.ServerSideApply = *p.applicationVersion.ServerSideApply
	}

	if p.generator != nil {
		if customization, ok := p.generator.(Customizer); ok {
			ignoredDifferences, err := customization.Customize(p.applicationVersion.Interface)
			if err != nil {
				return nil, err
			}

			cdApplication.IgnoreDifferences = ignoredDifferences
		}
	}

	return cdApplication, nil
}

// initialize must be called in Provision/Deprovision to do the application
// resolution in a path that has an error handler (as opposed to a constructor).
func (p *Provisioner) initialize(ctx context.Context) error {
	ref, err := p.getApplicationReference(ctx)
	if err != nil {
		return err
	}

	cli := clientlib.StaticClientFromContext(ctx)

	key := client.ObjectKey{
		Name: *ref.Name,
	}

	// TODO: Take the kind into consideration??
	application := &unikornv1.HelmApplication{}

	if err := cli.Get(ctx, key, application); err != nil {
		return err
	}

	// This may have been manually overridden.
	// TODO: this is only used by the clusteropenstack...
	if p.Name == "" {
		p.Name = application.Name
	}

	version, err := application.GetVersion(*ref.Version)
	if err != nil {
		return err
	}

	p.applicationVersion = version

	return nil
}

// Provision implements the Provision interface.
func (p *Provisioner) Provision(ctx context.Context) error {
	log := log.FromContext(ctx)

	if err := p.initialize(ctx); err != nil {
		return err
	}

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

	if err := p.initialize(ctx); err != nil {
		return err
	}

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
