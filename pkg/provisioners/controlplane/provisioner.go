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

package controlplane

import (
	"context"
	"errors"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"

	unikornv1alpha1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	argocdclient "github.com/eschercloudai/unikorn/pkg/argocd/client"
	argocdcluster "github.com/eschercloudai/unikorn/pkg/argocd/cluster"
	"github.com/eschercloudai/unikorn/pkg/constants"
	"github.com/eschercloudai/unikorn/pkg/provisioners"
	"github.com/eschercloudai/unikorn/pkg/provisioners/clusterapi"
	"github.com/eschercloudai/unikorn/pkg/provisioners/util"
	"github.com/eschercloudai/unikorn/pkg/provisioners/vcluster"

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
	// client provides access to Kubernetes.
	client client.Client

	// controlPlane is the control plane CR this deployment relates to
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

var ErrMissingLabel = errors.New("expected label is missing")

func (p *Provisioner) controlPlaneLabels() (map[string]string, error) {
	project, ok := p.controlPlane.Labels[constants.ProjectLabel]
	if !ok {
		return nil, ErrMissingLabel
	}

	return map[string]string{
		constants.ProjectLabel:      project,
		constants.ControlPlaneLabel: p.controlPlane.Name,
	}, nil
}

// Provision implements the Provision interface.
func (p *Provisioner) Provision(ctx context.Context) error {
	log := log.FromContext(ctx)

	log.Info("provisioning control plane")

	timer := prometheus.NewTimer(durationMetric)
	defer timer.ObserveDuration()

	namespace, err := p.provisionNamespace(ctx)
	if err != nil {
		return err
	}

	labels, err := p.controlPlaneLabels()
	if err != nil {
		return err
	}

	// Provision a virtual cluster for CAPI to live in.
	vclusterProvisioner := vcluster.New(p.client, namespace.Name).WithLabels(labels)

	if err := vclusterProvisioner.Provision(ctx); err != nil {
		return err
	}

	// Create the cluster in ArgoCD.
	if err := p.provisionArgoCDCluster(ctx, namespace.Name); err != nil {
		return err
	}

	// Provision CAPI in the vcluster.
	clusterAPIProvisioner := clusterapi.New(p.client, argoCDClusterServer(namespace.Name)).WithLabels(labels)

	if err := clusterAPIProvisioner.Provision(ctx); err != nil {
		return err
	}

	log.Info("control plane provisioned")

	return nil
}

// provisionNamespace creates a namespace for the control plane so that clusters
// contained within have their own namespace and won't clash with others in the
// same project.
func (p *Provisioner) provisionNamespace(ctx context.Context) (*corev1.Namespace, error) {
	namespace, err := util.GetResourceNamespace(ctx, p.client, constants.ControlPlaneLabel, p.controlPlane.Name)
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
			Labels: map[string]string{
				constants.ControlPlaneLabel: p.controlPlane.Name,
			},
		},
	}

	if err := provisioners.NewResourceProvisioner(p.client, namespace).Provision(ctx); err != nil {
		return nil, err
	}

	p.controlPlane.Status.Namespace = namespace.Name

	if err := p.client.Status().Update(ctx, p.controlPlane); err != nil {
		return nil, err
	}

	return namespace, nil
}

// argoCDClusterServer returns the ArgoCD vcluster server name for a namespace.
func argoCDClusterServer(namespace string) string {
	return fmt.Sprintf("https://vcluster.%s", namespace)
}

// provisionArgoCDCluster creates an ArgoCD cluster for the control plane to
// be provisioned into.
func (p *Provisioner) provisionArgoCDCluster(ctx context.Context, namespace string) error {
	// Grab the client condiguration from the vcluster.
	vc := vcluster.NewControllerRuntimeClient(p.client)

	vclusterConfig, err := vc.ClientConfig(ctx, namespace, false)
	if err != nil {
		return err
	}

	// Create the cluster in ArgoCD.
	argocd, err := argocdclient.NewInCluster(ctx, p.client, "argocd")
	if err != nil {
		return err
	}

	if err := argocdcluster.Upsert(ctx, argocd, argoCDClusterServer(namespace), vclusterConfig); err != nil {
		return err
	}

	return nil
}

// Deprovision implements the Provision interface.
func (p *Provisioner) Deprovision(ctx context.Context) error {
	namespace, err := util.GetResourceNamespace(ctx, p.client, constants.ControlPlaneLabel, p.controlPlane.Name)
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

	labels, err := p.controlPlaneLabels()
	if err != nil {
		return err
	}

	// Deprovision the CAPI application.
	if err := clusterapi.New(p.client, "").WithLabels(labels).Deprovision(ctx); err != nil {
		return err
	}

	// Delete the cluster from ArgoCD
	if err := p.deprovisionArgoCDCluster(ctx, namespace.Name); err != nil {
		return err
	}

	// Deprovision the vcluster application.
	if err := vcluster.New(p.client, namespace.Name).WithLabels(labels).Deprovision(ctx); err != nil {
		return err
	}

	// Deprovision the namespace and await deletion.
	// This will clean up all the vcluster gubbins and the CAPI stuff contained within.
	if err := provisioners.NewResourceProvisioner(p.client, namespace).Deprovision(ctx); err != nil {
		return err
	}

	return nil
}

// deprovisionClusters removes any kubernetes clusters, and frees up OpenStack resources.
func (p *Provisioner) deprovisionClusters(ctx context.Context, namespace string) error {
	clusters := &unikornv1alpha1.KubernetesClusterList{}
	if err := p.client.List(ctx, clusters, &client.ListOptions{Namespace: namespace}); err != nil {
		return err
	}

	for i := range clusters.Items {
		if err := provisioners.NewResourceProvisioner(p.client, &clusters.Items[i]).Deprovision(ctx); err != nil {
			return err
		}
	}

	return nil
}

// deprovisionArgoCDCluster removes the vcluster from ArgoCD.
func (p *Provisioner) deprovisionArgoCDCluster(ctx context.Context, namespace string) error {
	argocd, err := argocdclient.NewInCluster(ctx, p.client, "argocd")
	if err != nil {
		return err
	}

	if err := argocdcluster.Delete(ctx, argocd, argoCDClusterServer(namespace)); err != nil {
		return err
	}

	return nil
}
