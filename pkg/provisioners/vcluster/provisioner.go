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

	"github.com/eschercloudai/unikorn/pkg/provisioners"
	"github.com/eschercloudai/unikorn/pkg/readiness"

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

// Provision implements the Provision interface.
func (p *Provisioner) Provision(ctx context.Context) error {
	timer := prometheus.NewTimer(durationMetric)
	defer timer.ObserveDuration()

	log := log.FromContext(ctx)

	log.V(1).Info("provisioning vcluster")

	provisioner := provisioners.NewManifestProvisioner(p.client, provisioners.ManifestVCluster).WithNamespace(p.namespace).WithName(vclusterName)

	if err := provisioner.Provision(ctx); err != nil {
		return err
	}

	log.V(1).Info("waiting for stateful set to become ready")

	statefulsetReadiness := readiness.NewStatefulSet(p.client, p.namespace, vclusterName)

	if err := readiness.NewRetry(statefulsetReadiness).Check(ctx); err != nil {
		return err
	}

	return nil
}

// Deprovision implements the Provision interface.
func (p *Provisioner) Deprovision(context.Context) error {
	return nil
}
