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

package project

import (
	"context"
	"errors"

	unikornv1alpha1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/constants"
	"github.com/eschercloudai/unikorn/pkg/provisioners"
	"github.com/eschercloudai/unikorn/pkg/provisioners/util"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	ErrLabelMissing = errors.New("expected label missing")
)

// Provisioner encapsulates control plane provisioning.
type Provisioner struct {
	// client provides access to Kubernetes.
	client client.Client

	// project is the Kubernetes project we're provisioning.
	project *unikornv1alpha1.Project
}

// New returns a new initialized provisioner object.
func New(client client.Client, project *unikornv1alpha1.Project) *Provisioner {
	return &Provisioner{
		client:  client,
		project: project,
	}
}

// Ensure the Provisioner interface is implemented.
var _ provisioners.Provisioner = &Provisioner{}

// Provision implements the Provision interface.
func (p *Provisioner) Provision(ctx context.Context) error {
	// Namespace exists, leave it alone.
	_, err := util.GetResourceNamespace(ctx, p.client, constants.ProjectLabel, p.project.Name)
	if err == nil {
		return nil
	}

	// Some other error, propagate it back up the stack.
	if !errors.Is(err, util.ErrNamespaceLookup) {
		return err
	}

	// Create a new project namespace.
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "project-",
			Labels: map[string]string{
				constants.ProjectLabel: p.project.Name,
			},
		},
	}

	if err := provisioners.NewResourceProvisioner(p.client, namespace).Provision(ctx); err != nil {
		return err
	}

	p.project.Status.Namespace = namespace.Name

	if err := p.client.Status().Update(ctx, p.project); err != nil {
		return err
	}

	return nil
}

// Deprovision implements the Provision interface.
func (p *Provisioner) Deprovision(ctx context.Context) error {
	// Get the project's namespace.
	namespace, err := util.GetResourceNamespace(ctx, p.client, constants.ProjectLabel, p.project.Name)
	if err != nil {
		// Already dead.
		if errors.Is(err, util.ErrNamespaceLookup) {
			return nil
		}

		return err
	}

	// Find any control planes and delete them.  They in turn will delete clusters and
	// free any Openstack resources.
	controlPlanes := &unikornv1alpha1.ControlPlaneList{}
	if err := p.client.List(ctx, controlPlanes, &client.ListOptions{Namespace: namespace.Name}); err != nil {
		return err
	}

	for i := range controlPlanes.Items {
		if err := provisioners.NewResourceProvisioner(p.client, &controlPlanes.Items[i]).Deprovision(ctx); err != nil {
			return err
		}
	}

	// Deprovision the namespace and await deletion.
	if err := provisioners.NewResourceProvisioner(p.client, namespace).Deprovision(ctx); err != nil {
		return err
	}

	return nil
}
