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

	"github.com/prometheus/client_golang/prometheus"

	"github.com/eschercloudai/unikorn/pkg/constants"
	"github.com/eschercloudai/unikorn/pkg/provisioners"
	"github.com/eschercloudai/unikorn/pkg/provisioners/application"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// On home broadband it'll take about 60s to pull down images, plus any
	// readniness gates we put in the way.  If images are cached then 20s.
	//nolint:gochecknoglobals
	durationMetric = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "unikorn_clusterapi_provision_duration",
		Help: "Time taken for clusterapi to provision",
		Buckets: []float64{
			1, 5, 10, 15, 20, 30, 45, 60, 90, 120,
		},
	})
)

//nolint:gochecknoinits
func init() {
	metrics.Registry.MustRegister(durationMetric)
}

// Provisioner wraps up a whole load of horror code required to
// get vcluster into a deployed and usable state.
type Provisioner struct {
	// client provides access to Kubernetes.
	client client.Client

	server string

	// labels is a set of labels to identify the applications.
	labels map[string]string
}

// New returns a new initialized provisioner object.
func New(client client.Client, server string) *Provisioner {
	return &Provisioner{
		client: client,
		server: server,
	}
}

func (p *Provisioner) WithLabels(l map[string]string) *Provisioner {
	p.labels = l

	return p
}

func (p *Provisioner) getLabels(app string) map[string]interface{} {
	l := map[string]interface{}{
		constants.ApplicationLabel: app,
	}

	for k, v := range p.labels {
		l[k] = v
	}

	return l
}

// Ensure the Provisioner interface is implemented.
var _ provisioners.Provisioner = &Provisioner{}

func (p *Provisioner) generateCertManagerApplication() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata": map[string]interface{}{
				"generateName": "cert-manager-",
				"namespace":    "argocd",
				"labels":       p.getLabels("cert-manager"),
			},
			"spec": map[string]interface{}{
				"project": "default",
				"source": map[string]interface{}{
					//TODO:  programmable
					"repoURL":        "https://charts.jetstack.io",
					"chart":          "cert-manager",
					"targetRevision": "v1.10.1",
					"helm": map[string]interface{}{
						"releaseName": "cert-manager",
						"parameters": []map[string]interface{}{
							{
								"name":  "installCRDs",
								"value": "true",
							},
						},
					},
				},
				"destination": map[string]interface{}{
					"name":      p.server,
					"namespace": "cert-manager",
				},
				"syncPolicy": map[string]interface{}{
					"automated": map[string]interface{}{
						"selfHeal": true,
					},
					"syncOptions": []string{
						"CreateNamespace=true",
					},
				},
			},
		},
	}
}

func (p *Provisioner) generateClusterAPIApplication() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata": map[string]interface{}{
				"generateName": "cluster-api-",
				"namespace":    "argocd",
				"labels":       p.getLabels("cluster-api"),
			},
			"spec": map[string]interface{}{
				"project": "default",
				"source": map[string]interface{}{
					//TODO:  programmable
					"repoURL":        "https://eschercloudai.github.io/helm-cluster-api",
					"chart":          "cluster-api",
					"targetRevision": "v0.1.1",
				},
				"destination": map[string]interface{}{
					"name": p.server,
				},
				"ignoreDifferences": []map[string]interface{}{
					{
						"group": "rbac.authorization.k8s.io",
						"kind":  "ClusterRole",
						"name":  "capi-aggregated-manager-role",
						"jsonPointers": []interface{}{
							"/rules",
						},
					},
					{
						"group": "rbac.authorization.k8s.io",
						"kind":  "ClusterRole",
						"name":  "capi-kubeadm-control-plane-aggregated-manager-role",
						"jsonPointers": []interface{}{
							"/rules",
						},
					},
					{
						"group": "apiextensions.k8s.io",
						"kind":  "CustomResourceDefinition",
						"jsonPointers": []interface{}{
							"/spec/conversion/webhook/clientConfig/caBundle",
						},
					},
				},
				"syncPolicy": map[string]interface{}{
					"automated": map[string]interface{}{
						"selfHeal": true,
					},
				},
			},
		},
	}
}

// Provision implements the Provision interface.
func (p *Provisioner) Provision(ctx context.Context) error {
	timer := prometheus.NewTimer(durationMetric)
	defer timer.ObserveDuration()

	log := log.FromContext(ctx)

	// TODO: code repetition.
	log.Info("provisioning cert manager")

	if err := application.New(p.client, p.generateCertManagerApplication()).Provision(ctx); err != nil {
		return err
	}

	log.Info("cert manager provisioned")

	log.Info("provisioning cluster API")

	if err := application.New(p.client, p.generateClusterAPIApplication()).Provision(ctx); err != nil {
		return err
	}

	log.Info("cluster API provisioned")

	return nil
}

// Deprovision implements the Provision interface.
func (p *Provisioner) Deprovision(ctx context.Context) error {
	log := log.FromContext(ctx)

	log.Info("deprovisioning cluster API")

	if err := application.New(p.client, p.generateClusterAPIApplication()).Deprovision(ctx); err != nil {
		return err
	}

	log.Info("cluster API deprovisioned")

	log.Info("deprovisioning cert manager")

	if err := application.New(p.client, p.generateCertManagerApplication()).Deprovision(ctx); err != nil {
		return err
	}

	log.Info("cert manager deprovisioned")

	return nil
}
