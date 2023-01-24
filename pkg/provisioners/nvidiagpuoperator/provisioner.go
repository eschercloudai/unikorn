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

package nvidiagpuoperator

import (
	"context"

	"github.com/eschercloudai/unikorn/pkg/provisioners"
	"github.com/eschercloudai/unikorn/pkg/provisioners/application"
	"github.com/eschercloudai/unikorn/pkg/provisioners/remotecluster"
	"github.com/eschercloudai/unikorn/pkg/provisioners/serial"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// applicationName is the unique name of the application.
	applicationName = "nvidia-gpu-operator"

	// namespace is where to install the component.
	// NOTE: this requires the namespace to exist first, so pick an existing one.
	namespace = "kube-system"

	// licenseConfigMapName is the name of the config map we will create.
	licenseConfigMapName = "gridd-license"
)

// Provisioner encapsulates control plane provisioning.
type Provisioner struct {
	// client provides access to Kubernetes.
	client client.Client

	// resource defines the unique resource this provisioner belongs to.
	resource application.MutuallyExclusiveResource

	// remote is the remote cluster to deploy to.
	remote remotecluster.Generator
}

// New returns a new initialized provisioner object.
func New(client client.Client, resource application.MutuallyExclusiveResource, remote remotecluster.Generator) *Provisioner {
	return &Provisioner{
		client:   client,
		resource: resource,
		remote:   remote,
	}
}

// Ensure the Provisioner interface is implemented.
var _ provisioners.Provisioner = &Provisioner{}
var _ application.Generator = &Provisioner{}

// Resource implements the application.Generator interface.
func (p *Provisioner) Resource() application.MutuallyExclusiveResource {
	return p.resource
}

// Name implements the application.Generator interface.
func (p *Provisioner) Name() string {
	return applicationName
}

// Generate implements the application.Generator interface.
func (p *Provisioner) Generate() (client.Object, error) {
	object := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"spec": map[string]interface{}{
				"project": "default",
				"source": map[string]interface{}{
					//TODO:  programmable
					"repoURL":        "https://helm.ngc.nvidia.com/nvidia",
					"chart":          "gpu-operator",
					"targetRevision": "v22.9.1",
					"helm": map[string]interface{}{
						"parameters": []interface{}{
							map[string]interface{}{
								"name":  "driver.enabled",
								"value": "false",
							},
							map[string]interface{}{
								"name":  "driver.licensingConfig.configMapName",
								"value": licenseConfigMapName,
							},
						},
					},
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

	return object, nil
}

// generateLicenseConfigMap creates config data for the operator, because it's incapable
// of doing it itself, because it's obviously way too hard.
func (p *Provisioner) generateLicenseConfigMapProvisioner(ctx context.Context) (provisioners.Provisioner, error) {
	object := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      licenseConfigMapName,
			Namespace: namespace,
		},
		Data: map[string]string{
			// TODO: make configurable.
			"gridd.conf": "ServerAddress=gridlicense.nl1.eschercloud.com",
		},
	}

	client := p.client

	if p.remote != nil {
		remoteClient, err := remotecluster.GetClient(ctx, p.remote)
		if err != nil {
			return nil, err
		}

		client = remoteClient
	}

	return provisioners.NewResourceProvisioner(client, object), nil
}

// getProvisioner returns a generic provisioner for this component.
func (p *Provisioner) getProvisioner(ctx context.Context) (provisioners.Provisioner, error) {
	licenceConfigMapProvisioner, err := p.generateLicenseConfigMapProvisioner(ctx)
	if err != nil {
		return nil, err
	}

	provisioner := &serial.Provisioner{
		Provisioners: []provisioners.Provisioner{
			licenceConfigMapProvisioner,
			application.New(p.client, p).OnRemote(p.remote).InNamespace(namespace),
		},
	}

	return provisioner, nil
}

// Provision implements the Provision interface.
func (p *Provisioner) Provision(ctx context.Context) error {
	provisioner, err := p.getProvisioner(ctx)
	if err != nil {
		return err
	}

	if err := provisioner.Provision(ctx); err != nil {
		return err
	}

	return nil
}

// Deprovision implements the Provision interface.
func (p *Provisioner) Deprovision(ctx context.Context) error {
	provisioner, err := p.getProvisioner(ctx)
	if err != nil {
		return err
	}

	if err := provisioner.Deprovision(ctx); err != nil {
		return err
	}

	return nil
}
