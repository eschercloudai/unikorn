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

package controlplane

import (
	"context"
	"errors"

	"github.com/prometheus/client_golang/prometheus"

	unikornv1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/cd"
	"github.com/eschercloudai/unikorn/pkg/cd/argocd"
	"github.com/eschercloudai/unikorn/pkg/provisioners"
	"github.com/eschercloudai/unikorn/pkg/provisioners/concurrent"
	"github.com/eschercloudai/unikorn/pkg/provisioners/generic"
	"github.com/eschercloudai/unikorn/pkg/provisioners/helmapplications/certmanager"
	"github.com/eschercloudai/unikorn/pkg/provisioners/helmapplications/clusterapi"
	"github.com/eschercloudai/unikorn/pkg/provisioners/helmapplications/vcluster"
	"github.com/eschercloudai/unikorn/pkg/provisioners/remotecluster"
	"github.com/eschercloudai/unikorn/pkg/provisioners/serial"
	"github.com/eschercloudai/unikorn/pkg/provisioners/util"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// On home broadband it'll take about 150s to pull down images, plus any
	// readniness gates we put in the way.  If images are cached then 45s.
	//nolint:gochecknoglobals
	durationMetric = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "unikorn_controlplane_provision_duration",
		Help: "Time taken for controlplane to provision",
		Buckets: []float64{
			1, 5, 10, 15, 20, 30, 45, 60, 90, 120, 180, 240, 300,
		},
	})
)

//nolint:gochecknoinits
func init() {
	metrics.Registry.MustRegister(durationMetric)
}

// Provisioner encapsulates control plane provisioning.
type Provisioner struct {
	provisioners.ProvisionerMeta

	// client provides access to Kubernetes.
	client client.Client

	// controlPlane is the control plane CR this deployment relates to
	controlPlane unikornv1.ControlPlane

	vclusterApplication    *unikornv1.HelmApplication
	certManagerApplication *unikornv1.HelmApplication
	clusterAPIApplication  *unikornv1.HelmApplication
}

// New returns a new initialized provisioner object.
func New(client client.Client) provisioners.ManagerProvisioner {
	provisioner := &Provisioner{
		client: client,
	}

	return provisioner
}

// Ensure the ManagerProvisioner interface is implemented.
var _ provisioners.ManagerProvisioner = &Provisioner{}

func (p *Provisioner) Object() client.Object {
	return &p.controlPlane
}

// unbundleApplications gets application data required for provisioning.
// TODO: We should really just put this in the application, then you don't
// need to worry about missing any out, or not calling this function.
func (p *Provisioner) unbundleApplications(ctx context.Context) error {
	unbundler := util.NewUnbundler(&p.controlPlane, unikornv1.ApplicationBundleResourceKindControlPlane)
	unbundler.AddApplication(&p.vclusterApplication, "vcluster")
	unbundler.AddApplication(&p.certManagerApplication, "cert-manager")
	unbundler.AddApplication(&p.clusterAPIApplication, "cluster-api")

	if err := unbundler.Unbundle(ctx, p.client); err != nil {
		return err
	}

	return nil
}

// provisionNamespace creates a namespace for the control plane so that clusters
// contained within have their own namespace and won't clash with others in the
// same project.
func (p *Provisioner) provisionNamespace(ctx context.Context) (*corev1.Namespace, error) {
	labels, err := p.controlPlane.ResourceLabels()
	if err != nil {
		return nil, err
	}

	namespace, err := util.GetResourceNamespace(ctx, p.client, labels)
	if err == nil {
		return namespace, nil
	}

	// Some other error, propagate it back up the stack.
	if !errors.Is(err, util.ErrNamespaceLookup) {
		return nil, err
	}

	// Create a new control plane namespace.
	namespace = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "controlplane-",
			Labels:       labels,
		},
	}

	if err := generic.NewResourceProvisioner(p.client, namespace).Provision(ctx); err != nil {
		return nil, err
	}

	p.controlPlane.Status.Namespace = namespace.Name

	if err := p.client.Status().Update(ctx, &p.controlPlane); err != nil {
		return nil, err
	}

	return namespace, nil
}

// deprovisionClusters removes any kubernetes clusters, and frees up OpenStack resources.
func (p *Provisioner) deprovisionClusters(ctx context.Context, namespace string) error {
	clusters := &unikornv1.KubernetesClusterList{}
	if err := p.client.List(ctx, clusters, &client.ListOptions{Namespace: namespace}); err != nil {
		return err
	}

	for i := range clusters.Items {
		if err := generic.NewResourceProvisioner(p.client, &clusters.Items[i]).Deprovision(ctx); err != nil {
			return err
		}
	}

	return nil
}

// getRemoteClusterGenerator returns a generator capable of reading the vcluster
// kubeconfig from the underlying control cluster.
func (p *Provisioner) getRemoteClusterGenerator(namespace string) *vcluster.RemoteClusterGenerator {
	return vcluster.NewRemoteClusterGenerator(p.client, namespace, provisioners.VClusterRemoteLabelsFromControlPlane(&p.controlPlane))
}

// getControlPlaneProvisioner returns a provisoner that encodes control plane
// provisioning steps.
func (p *Provisioner) getControlPlaneProvisioner(driver cd.Driver, namespace string) provisioners.Provisioner {
	remote := p.getRemoteClusterGenerator(namespace)

	// Provision the vitual cluster, setup the remote cluster then
	// install cert manager and cluster API into it.
	return serial.New("control plane",
		vcluster.New(driver, &p.controlPlane, p.vclusterApplication).InNamespace(namespace),
		remotecluster.New(driver, remote),
		concurrent.New("cluster-api",
			certmanager.New(driver, &p.controlPlane, p.certManagerApplication).OnRemote(remote).BackgroundDelete(),
			clusterapi.New(driver, &p.controlPlane, p.clusterAPIApplication).OnRemote(remote).BackgroundDelete(),
		),
	)
}

// Provision implements the Provision interface.
func (p *Provisioner) Provision(ctx context.Context) error {
	log := log.FromContext(ctx)

	log.Info("provisioning control plane")

	timer := prometheus.NewTimer(durationMetric)
	defer timer.ObserveDuration()

	if err := p.unbundleApplications(ctx); err != nil {
		return err
	}

	namespace, err := p.provisionNamespace(ctx)
	if err != nil {
		return err
	}

	client, err := argocd.NewInCluster(ctx, p.client)
	if err != nil {
		return err
	}

	driver := argocd.NewDriver(p.client, client)

	// Indicate the namespace is created before provisioning the rest.  This will inevitably
	// yield as the components take some time to become healthy.  It does however give
	// clusters an opportunity to be provisioned before the CP is fully up, reducing
	// latency at the front-end.
	p.controlPlane.Status.Namespace = namespace.Name

	if err := p.getControlPlaneProvisioner(driver, namespace.Name).Provision(ctx); err != nil {
		return err
	}

	log.Info("control plane provisioned")

	return nil
}

// Deprovision implements the Provision interface.
func (p *Provisioner) Deprovision(ctx context.Context) error {
	if err := p.unbundleApplications(ctx); err != nil {
		return err
	}

	labels, err := p.controlPlane.ResourceLabels()
	if err != nil {
		return err
	}

	namespace, err := util.GetResourceNamespace(ctx, p.client, labels)
	if err != nil {
		// Already dead.
		if errors.Is(err, util.ErrNamespaceLookup) {
			return nil
		}

		return err
	}

	// Find any clusters and delete them and free any Openstack resources.
	if err := p.deprovisionClusters(ctx, namespace.Name); err != nil {
		return err
	}

	client, err := argocd.NewInCluster(ctx, p.client)
	if err != nil {
		return err
	}

	driver := argocd.NewDriver(p.client, client)

	// Remove the control plane.
	if err := p.getControlPlaneProvisioner(driver, namespace.Name).Deprovision(ctx); err != nil {
		return err
	}

	// Deprovision the namespace and await deletion.
	// This will clean up all the vcluster gubbins and the CAPI stuff contained within.
	if err := generic.NewResourceProvisioner(p.client, namespace).Deprovision(ctx); err != nil {
		return err
	}

	return nil
}
