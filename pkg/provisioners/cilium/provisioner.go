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

package cilium

import (
	"context"

	"github.com/eschercloudai/unikorn/pkg/provisioners"
	"github.com/eschercloudai/unikorn/pkg/provisioners/application"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// applicationName is the unique name of the application.
	applicationName = "cilium"
)

// Provisioner encapsulates control plane provisioning.
type Provisioner struct {
	// client provides access to Kubernetes.
	client client.Client

	// server is the ArgoCD server to provision in.
	server string

	// scope defines a unique application scope.
	scope map[string]string
}

// New returns a new initialized provisioner object.
func New(client client.Client, server string, scope map[string]string) *Provisioner {
	return &Provisioner{
		client: client,
		server: server,
		scope:  scope,
	}
}

// Ensure the Provisioner interface is implemented.
var _ provisioners.Provisioner = &Provisioner{}

// generateApplication creates an ArgoCD application for the
// Cilium CNI plugin.
func (p *Provisioner) generateApplication() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata": map[string]interface{}{
				"namespace": "argocd",
			},
			"spec": map[string]interface{}{
				"project": "default",
				"source": map[string]interface{}{
					//TODO:  programmable
					"repoURL":        "https://helm.cilium.io/",
					"chart":          "cilium",
					"targetRevision": "1.12.4",
				},
				"destination": map[string]interface{}{
					"name":      p.server,
					"namespace": "kube-system",
				},
				"syncPolicy": map[string]interface{}{
					"automated": map[string]interface{}{
						"selfHeal": true,
						"prune":    true,
					},
				},
			},
		},
	}
}

// Provision implements the Provision interface.
func (p *Provisioner) Provision(ctx context.Context) error {
	if err := application.New(p.client, applicationName, p.scope, p.generateApplication()).Provision(ctx); err != nil {
		return err
	}

	return nil
}

// Deprovision implements the Provision interface.
func (p *Provisioner) Deprovision(ctx context.Context) error {
	if err := application.New(p.client, applicationName, p.scope, p.generateApplication()).Deprovision(ctx); err != nil {
		return err
	}

	return nil
}
