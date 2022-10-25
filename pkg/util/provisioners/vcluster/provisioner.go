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

	unikornv1alpha1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/util"
	"github.com/eschercloudai/unikorn/pkg/util/provisioners/generic"
	"github.com/eschercloudai/unikorn/pkg/util/retry"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

const (
	// vclusterHelmRepo defines loft.sh's repo, sourced from vclusterctl.
	vclusterHelmRepo = "https://charts.loft.sh"

	// vclusterHelmChart defines the vclustre hlm chart, sourced from vclusterctl.
	vclusterHelmChart = "vcluster"
)

// Provisioner wraps up a whole load of horror code required to
// get vcluster into a deployed and usable state.
type Provisioner struct {
	// clients provides access to Kubernetes.
	clients cmdutil.Factory

	// controlPlane is the control plane resource this belongs to.
	// Resource names and namespaces are derived from this object.
	controlPlane *unikornv1alpha1.ControlPlane
}

// NewProvisioner returns a new initialized provisioner object.
func NewProvisioner(clients cmdutil.Factory, controlPlane *unikornv1alpha1.ControlPlane) *Provisioner {
	return &Provisioner{
		clients:      clients,
		controlPlane: controlPlane,
	}
}

// Ensure the Provisioner interface is implemented.
var _ generic.Provisioner = &Provisioner{}

// Provision implements the Provision interface.
func (p *Provisioner) Provision(ctx context.Context) error {
	name := p.controlPlane.Name
	namespace := p.controlPlane.Namespace

	client, err := p.clients.KubernetesClientSet()
	if err != nil {
		return err
	}

	// Setup the provisioned with a reference to the control plane so it
	// is torn down when the control plane is.
	gvk, err := util.ObjectGroupVersionKind(p.controlPlane)
	if err != nil {
		return err
	}

	ownerReferences := []metav1.OwnerReference{
		*metav1.NewControllerRef(p.controlPlane, *gvk),
	}

	// Expose the Kubernetes API as a LoadBalancer service, this will give
	// end users the ability to provision their own CAPI clusters short term.
	args := []string{
		"--set=service.type=LoadBalancer",
	}

	provisioner := generic.NewHelmProvisioner(p.clients, vclusterHelmRepo, vclusterHelmChart, namespace, name, args, ownerReferences)

	if err := provisioner.Provision(ctx); err != nil {
		return err
	}

	// TODO: this is inconsistent, perhaps we just want to use Factory everywhere??
	statefulsetReadiness := generic.NewStatefulSetReady(client, namespace, name)

	if err := generic.NewReadinessCheckWithRetry(statefulsetReadiness).Check(ctx); err != nil {
		return err
	}

	// The stateful set will provision a PVC to contain the Kubernetes "etcd"
	// database, and these don't get cleaned up, so reusing the same control
	// plane name will then go off and provision a load of stuff due to persistence.
	// There is an extension where you can cascade deletion, but as of writing (v1.25)
	// it's still in alpha.  For now, we manually link the PVC to the control plane.
	// TODO: we should inspect the stateful set for size, and also the volume name.
	pvc, err := client.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, "data-"+name+"-0", metav1.GetOptions{})
	if err != nil {
		return err
	}

	pvc.SetOwnerReferences(ownerReferences)

	if _, err := client.CoreV1().PersistentVolumeClaims(namespace).Update(ctx, pvc, metav1.UpdateOptions{}); err != nil {
		return err
	}

	// So regardless of whether the stateful set is up and running we need to do another
	// connectivity check because Neutron takes an age to actually provide it.
	if err := p.waitForNetwork(ctx, client, namespace, name); err != nil {
		return err
	}

	return err
}

// waitForNetwork does that, polls the VCluster API endpoint and waits for it to
// start working.
func (p *Provisioner) waitForNetwork(ctx context.Context, client kubernetes.Interface, namespace, name string) error {
	vclusterRESTClient, err := RESTClient(ctx, client, namespace, name)
	if err != nil {
		return err
	}

	vclusterClient, err := kubernetes.NewForConfig(vclusterRESTClient)
	if err != nil {
		return err
	}

	wait := func() error {
		if _, err := vclusterClient.Discovery().ServerVersion(); err != nil {
			return err
		}

		return nil
	}

	if err := retry.Forever().DoWithContext(ctx, wait); err != nil {
		return err
	}

	return err
}
