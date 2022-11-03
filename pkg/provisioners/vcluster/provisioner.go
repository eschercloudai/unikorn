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

	unikornv1alpha1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/provisioners"
	"github.com/eschercloudai/unikorn/pkg/readiness"
	"github.com/eschercloudai/unikorn/pkg/util"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
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

	// controlPlane is the control plane resource this belongs to.
	// Resource names and namespaces are derived from this object.
	controlPlane *unikornv1alpha1.ControlPlane
}

// New returns a new initialized provisioner object.
func New(client client.Client, controlPlane *unikornv1alpha1.ControlPlane) *Provisioner {
	return &Provisioner{
		client:       client,
		controlPlane: controlPlane,
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

	name := p.controlPlane.Name
	namespace := p.controlPlane.Namespace

	// Setup the provisioned with a reference to the control plane so it
	// is torn down when the control plane is.
	gvk, err := util.ObjectGroupVersionKind(p.client.Scheme(), p.controlPlane)
	if err != nil {
		return err
	}

	ownerReferences := []metav1.OwnerReference{
		*metav1.NewControllerRef(p.controlPlane, *gvk),
	}

	provisioner := provisioners.NewManifestProvisioner(p.client, provisioners.ManifestVCluster).WithNamespace(namespace).WithName(name).WithOwnerReferences(ownerReferences)

	if err := provisioner.Provision(ctx); err != nil {
		return err
	}

	log.V(1).Info("waiting for stateful set to become ready")

	statefulsetReadiness := readiness.NewStatefulSet(p.client, namespace, name)

	if err := readiness.NewRetry(statefulsetReadiness).Check(ctx); err != nil {
		return err
	}

	log.V(1).Info("applying statefulset volume claim clean up")

	// The stateful set will provision a PVC to contain the Kubernetes "etcd"
	// database, and these don't get cleaned up, so reusing the same control
	// plane name will then go off and provision a load of stuff due to persistence.
	// There is an extension where you can cascade deletion, but as of writing (v1.25)
	// it's still in alpha.  For now, we manually link the PVC to the control plane.
	// TODO: we should inspect the stateful set for size, and also the volume name.
	pvc := &corev1.PersistentVolumeClaim{}
	if err := p.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "data-" + name + "-0"}, pvc); err != nil {
		return err
	}

	pvc.SetOwnerReferences(ownerReferences)

	if err := p.client.Update(ctx, pvc); err != nil {
		return err
	}

	return nil
}
