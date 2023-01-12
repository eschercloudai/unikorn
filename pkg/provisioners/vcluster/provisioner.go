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

package vcluster

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

const (
	// vclusterName is a static name used for all resources.  Each CP has its own
	// namespace, so this is safe for now.
	vclusterName = "vcluster"
)

var (
	// On home broadband it'll take about 90s to pull down images, plus any
	// readniness gates we put in the way.  If images are cached then 20s.
	//nolint:gochecknoglobals
	durationMetric = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "unikorn_vcluster_provision_duration",
		Help: "Time taken for vcluster to provision",
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

	// namespace is the namespace to provision in.
	namespace string

	// labels is a set of labels to identify the vcluster.
	labels map[string]string
}

// New returns a new initialized provisioner object.
func New(client client.Client, namespace string) *Provisioner {
	return &Provisioner{
		client:    client,
		namespace: namespace,
	}
}

// Ensure the Provisioner interface is implemented.
var _ provisioners.Provisioner = &Provisioner{}

func (p *Provisioner) WithLabels(l map[string]string) *Provisioner {
	p.labels = l

	return p
}

func (p *Provisioner) getLabels() map[string]interface{} {
	l := map[string]interface{}{
		constants.ApplicationLabel: "vcluster",
	}

	for k, v := range p.labels {
		l[k] = v
	}

	return l
}

func (p *Provisioner) generateApplication() *unstructured.Unstructured {
	// Okay, from this point on, things get a bit "meta" because whoever
	// wrote ArgoCD for some reason imported kubernetes, not client-go to
	// get access to the schema information...
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata": map[string]interface{}{
				"generateName": "vcluster-",
				"namespace":    "argocd",
				"labels":       p.getLabels(),
			},
			"spec": map[string]interface{}{
				"project": "default",
				"source": map[string]interface{}{
					//TODO:  programmable
					"repoURL":        "https://charts.loft.sh",
					"chart":          "vcluster",
					"targetRevision": "0.12.1",
					"helm": map[string]interface{}{
						"releaseName": "vcluster",
						// TODO: this is only required by unikornctl to get
						// the kubeconfig (e.g. set a reachable address).  It
						// wastes an IP and embiggens the attack vector.  If
						// we change this to reqire port-forwarding it'll do
						// the trick!
						"parameters": []map[string]interface{}{
							{
								"name":  "service.type",
								"value": "LoadBalancer",
							},
						},
					},
				},
				"destination": map[string]interface{}{
					"name":      "in-cluster",
					"namespace": p.namespace,
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
	timer := prometheus.NewTimer(durationMetric)
	defer timer.ObserveDuration()

	log := log.FromContext(ctx)

	log.Info("provisioning vcluster")

	if err := application.New(p.client, p.generateApplication()).Provision(ctx); err != nil {
		return err
	}

	log.Info("vcluster provisioned")

	return nil
}

// Deprovision implements the Provision interface.
func (p *Provisioner) Deprovision(ctx context.Context) error {
	if err := application.New(p.client, p.generateApplication()).Deprovision(ctx); err != nil {
		return err
	}

	return nil
}
