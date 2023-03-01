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

	unikornv1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/provisioners"
	"github.com/eschercloudai/unikorn/pkg/provisioners/application"
	"github.com/eschercloudai/unikorn/pkg/provisioners/generic"
	"github.com/eschercloudai/unikorn/pkg/provisioners/serial"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// applicationName is the unique name of the application.
	applicationName = "nvidia-gpu-operator"

	// defaultNamespace is where to install the component.
	// NOTE: this requires the namespace to exist first, so pick an existing one.
	defaultNamespace = "kube-system"

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
	remote provisioners.RemoteCluster

	// application is the application used to identify the Helm chart to use.
	application *unikornv1.HelmApplication

	// namespace defines where to install the application.
	namespace string
}

// New returns a new initialized provisioner object.
func New(client client.Client, resource application.MutuallyExclusiveResource, application *unikornv1.HelmApplication) provisioners.Provisioner {
	return &Provisioner{
		client:      client,
		resource:    resource,
		application: application,
		namespace:   defaultNamespace,
	}
}

// Ensure the Provisioner interface is implemented.
var _ provisioners.Provisioner = &Provisioner{}
var _ application.ValuesGenerator = &Provisioner{}

// OnRemote implements the Provision interface.
func (p *Provisioner) OnRemote(remote provisioners.RemoteCluster) provisioners.Provisioner {
	p.remote = remote

	return p
}

// InNamespace implements the Provision interface.
func (p *Provisioner) InNamespace(namespace string) provisioners.Provisioner {
	p.namespace = namespace

	return p
}

// Generate implements the application.Generator interface.
func (p *Provisioner) Values(version *string) (interface{}, error) {
	// We limit images to those with the driver pre-installed as it's far quicker for UX.
	// Also the default affinity is broken and prevents scale to zero, also tolerations
	// don't allow execution using our default taints.
	// TODO: This includes the node-feature-discovery as a subchart, and doesn't expose
	// node selectors/tolerations, however, it does scale to zero.
	values := map[string]interface{}{
		"driver": map[string]interface{}{
			"enabled": false,
			"licensingConfig": map[string]interface{}{
				"configMapName": licenseConfigMapName,
			},
		},
		"operator": map[string]interface{}{
			"affinity": map[string]interface{}{
				"nodeAffinity": map[string]interface{}{
					"preferredDuringSchedulingIgnoredDuringExecution": nil,
					"requiredDuringSchedulingIgnoredDuringExecution": map[string]interface{}{
						"nodeSelectorTerms": []interface{}{
							map[string]interface{}{
								"matchExpressions": []interface{}{
									map[string]interface{}{
										"key":      "node-role.kubernetes.io/control-plane",
										"operator": "Exists",
									},
								},
							},
						},
					},
				},
			},
			"tolerations": []interface{}{
				map[string]interface{}{
					"key":      "node-role.kubernetes.io/master",
					"operator": "Equal",
					"effect":   "NoSchedule",
				},
				map[string]interface{}{
					"key":      "node-role.kubernetes.io/control-plane",
					"operator": "Equal",
					"effect":   "NoSchedule",
				},
			},
		},
	}

	return values, nil
}

// generateLicenseConfigMap creates config data for the operator, because it's incapable
// of doing it itself, because it's obviously way too hard.
func (p *Provisioner) generateLicenseConfigMapProvisioner() provisioners.Provisioner {
	object := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      licenseConfigMapName,
			Namespace: p.namespace,
		},
		Data: map[string]string{
			// TODO: make configurable.
			// TODO: make mutable.
			"gridd.conf": "ServerAddress=gridlicense.nl1.eschercloud.com",
		},
	}

	return generic.NewResourceProvisioner(p.client, object).OnRemote(p.remote)
}

func (p *Provisioner) getProvisioner() provisioners.Provisioner {
	return application.New(p.client, applicationName, p.resource, p.application).WithGenerator(p).OnRemote(p.remote).InNamespace(p.namespace)
}

// Provision implements the Provision interface.
func (p *Provisioner) Provision(ctx context.Context) error {
	provisioner := serial.New("nvidia GPU operator",
		// TODO: delete me when baked into the image.
		p.generateLicenseConfigMapProvisioner(),
		p.getProvisioner(),
	)

	if err := provisioner.Provision(ctx); err != nil {
		return err
	}

	return nil
}

// Deprovision implements the Provision interface.
func (p *Provisioner) Deprovision(ctx context.Context) error {
	// Ignore the config map, that will be deleted by the cluster.
	if err := p.getProvisioner().Deprovision(ctx); err != nil {
		return err
	}

	return nil
}
