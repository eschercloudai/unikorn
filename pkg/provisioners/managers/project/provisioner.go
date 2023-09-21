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

package project

import (
	"context"
	"errors"

	unikornv1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/provisioners"
	"github.com/eschercloudai/unikorn/pkg/provisioners/resource"
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
	provisioners.ProvisionerMeta

	// client provides access to Kubernetes.
	client client.Client

	// project is the Kubernetes project we're provisioning.
	project unikornv1.Project
}

// New returns a new initialized provisioner object.
func New(client client.Client) provisioners.ManagerProvisioner {
	return &Provisioner{
		client: client,
	}
}

// Ensure the ManagerProvisioner interface is implemented.
var _ provisioners.ManagerProvisioner = &Provisioner{}

func (p *Provisioner) Object() client.Object {
	return &p.project
}

// Provision implements the Provision interface.
func (p *Provisioner) Provision(ctx context.Context) error {
	labels, err := p.project.ResourceLabels()
	if err != nil {
		return err
	}

	// Namespace exists, leave it alone.
	namespace, err := util.GetResourceNamespace(ctx, p.client, labels)
	if err != nil {
		// Some other error, propagate it back up the stack.
		if !errors.Is(err, util.ErrNamespaceLookup) {
			return err
		}
	}

	if namespace == nil {
		// Create a new project namespace.
		namespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "project-",
				Labels:       labels,
			},
		}

		if err := resource.New(p.client, namespace).Provision(ctx); err != nil {
			return err
		}
	}

	p.project.Status.Namespace = namespace.Name

	return nil
}

// Deprovision implements the Provision interface.
func (p *Provisioner) Deprovision(ctx context.Context) error {
	labels, err := p.project.ResourceLabels()
	if err != nil {
		return err
	}

	// Get the project's namespace.
	namespace, err := util.GetResourceNamespace(ctx, p.client, labels)
	if err != nil {
		// Already dead.
		if errors.Is(err, util.ErrNamespaceLookup) {
			return nil
		}

		return err
	}

	// Deprovision the namespace and await deletion.
	if err := resource.New(p.client, namespace).Deprovision(ctx); err != nil {
		return err
	}

	return nil
}
