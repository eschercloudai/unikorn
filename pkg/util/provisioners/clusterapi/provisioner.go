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

package clusterapi

import (
	"context"

	provisioners "github.com/eschercloudai/unikorn/pkg/util/provisioners/generic"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Provisioner wraps up a whole load of horror code required to
// get vcluster into a deployed and usable state.
type Provisioner struct {
	// client provides access to Kubernetes.
	client client.Client
}

// New returns a new initialized provisioner object.
func New(client client.Client) *Provisioner {
	return &Provisioner{
		client: client,
	}
}

// Ensure the Provisioner interface is implemented.
var _ provisioners.Provisioner = &Provisioner{}

// Provision implements the Provision interface.
func (p *Provisioner) Provision(ctx context.Context) error {
	log := log.FromContext(ctx)

	log.V(1).Info("provisioning cluster API")

	log.V(1).Info("provisioning Cert Manager")

	certManagerProvisioner := provisioners.NewManifestProvisioner(p.client, provisioners.ManifestCertManager)

	if err := certManagerProvisioner.Provision(ctx); err != nil {
		return err
	}

	log.V(1).Info("waiting for Cert Manager webhook to be active")

	certManagerReady := provisioners.NewDeploymentReady(p.client, "cert-manager", "cert-manager-webhook")

	if err := provisioners.NewReadinessCheckWithRetry(certManagerReady).Check(ctx); err != nil {
		return err
	}

	log.V(1).Info("waiting for Cert Manager webhook to be functional")

	certificate := &unstructured.Unstructured{}
	certificate.SetAPIVersion("cert-manager.io/v1")
	certificate.SetKind("Issuer")
	certificate.SetName("test")
	certificate.SetNamespace("default")

	if err := unstructured.SetNestedField(certificate.Object, "foo", "spec", "ca", "secretName"); err != nil {
		return err
	}

	certmanagerFunctional := provisioners.NewWebhookReady(p.client, certificate)

	if err := provisioners.NewReadinessCheckWithRetry(certmanagerFunctional).Check(ctx); err != nil {
		return err
	}

	log.V(1).Info("provisioning Cluster API core")

	clusterAPICoreProvisioner := provisioners.NewManifestProvisioner(p.client, provisioners.ManifestClusterAPICore)

	if err := clusterAPICoreProvisioner.Provision(ctx); err != nil {
		return err
	}

	log.V(1).Info("provisioning Cluster API control plane")

	clusterAPIControlPlaneProvisioner := provisioners.NewManifestProvisioner(p.client, provisioners.ManifestClusterAPIControlPlane)

	if err := clusterAPIControlPlaneProvisioner.Provision(ctx); err != nil {
		return err
	}

	log.V(1).Info("provisioning Cluster API bootstrap")

	clusterAPIBootstrapProvisioner := provisioners.NewManifestProvisioner(p.client, provisioners.ManifestClusterAPIBootstrap)

	if err := clusterAPIBootstrapProvisioner.Provision(ctx); err != nil {
		return err
	}

	log.V(1).Info("provisioning Cluster API Openstack provider")

	clusterAPIProviderOpenstackProvisioner := provisioners.NewManifestProvisioner(p.client, provisioners.ManifestClusterAPIProviderOpenstack)

	if err := clusterAPIProviderOpenstackProvisioner.Provision(ctx); err != nil {
		return err
	}

	return nil
}
