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

package clusteropenstack

import (
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/eschercloudai/unikorn/pkg/provisioners/helmapplications/vcluster"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// In older versions we used the name verbatim, that could blow the 63 character limit
// easily.  In new versions, the chart will hash the pool name to keep it to 8 characters.
func (p *Provisioner) helmWorkloadPoolName(name string) string {
	sum := sha256.Sum256([]byte(name))

	hash := fmt.Sprintf("%x", sum)

	return hash[:8]
}

// getWorkloadPoolMachineDeploymentNames gets a list of machine deployments that should
// exist for this cluster.
// TODO: this is horrific and relies on knowing the internal workings of the Helm chart
// not just the public API!!!
// TODO: the new cluster chart in 1.2.o will contain a "pool.eschercloud.ai/name" annotaion
// that will give a verbatim pool name for use with this once 1.1.0 cluster have gone.
func (p *Provisioner) getWorkloadPoolMachineDeploymentNames() []string {
	names := make([]string, len(p.cluster.Spec.WorkloadPools.Pools))

	for i, pool := range p.cluster.Spec.WorkloadPools.Pools {
		names[i] = fmt.Sprintf("%s-pool-%s", releaseName(p.cluster), p.helmWorkloadPoolName(pool.Name))
	}

	return names
}

// filterOwnedResources removes any resources that aren't owned by the cluster.
func (p *Provisioner) filterOwnedResources(resources []unstructured.Unstructured) []unstructured.Unstructured {
	var filtered []unstructured.Unstructured

	for _, resource := range resources {
		ownerReferences := resource.GetOwnerReferences()

		for _, ownerReference := range ownerReferences {
			if ownerReference.Kind != "Cluster" || ownerReference.Name != releaseName(p.cluster) {
				continue
			}

			filtered = append(filtered, resource)
		}
	}

	return filtered
}

// getOwnedResource returns resources of the specified API version/kind that belong
// to the cluster.
func (p *Provisioner) getOwnedResource(ctx context.Context, c client.Client, apiVersion, kind string) ([]unstructured.Unstructured, error) {
	objects := &unstructured.UnstructuredList{
		Object: map[string]interface{}{
			"apiVersion": apiVersion,
			"kind":       kind,
		},
	}

	options := &client.ListOptions{
		Namespace: p.cluster.Name,
	}

	if err := c.List(ctx, objects, options); err != nil {
		return nil, err
	}

	return p.filterOwnedResources(objects.Items), nil
}

// getMachineDeployments gets all live machine deployments for the cluster.
func (p *Provisioner) getMachineDeployments(ctx context.Context, c client.Client) ([]unstructured.Unstructured, error) {
	// TODO: this is flaky as hell, due to hard coded versions, needs a fix upstream.
	return p.getOwnedResource(ctx, c, "cluster.x-k8s.io/v1beta1", "MachineDeployment")
}

// getKubeadmConfigTemplates gets all live config templates for the cluster.
func (p *Provisioner) getKubeadmConfigTemplates(ctx context.Context, c client.Client) ([]unstructured.Unstructured, error) {
	// TODO: this is flaky as hell, due to hard coded versions, needs a fix upstream.
	return p.getOwnedResource(ctx, c, "bootstrap.cluster.x-k8s.io/v1beta1", "KubeadmConfigTemplate")
}

// getKubeadmControlPlanes gets all live control planes for the cluster.
func (p *Provisioner) getKubeadmControlPlanes(ctx context.Context, c client.Client) ([]unstructured.Unstructured, error) {
	// TODO: this is flaky as hell, due to hard coded versions, needs a fix upstream.
	return p.getOwnedResource(ctx, c, "controlplane.cluster.x-k8s.io/v1beta1", "KubeadmControlPlane")
}

// getOpenstackMachineTemplates gets all live machine templates for the cluster.
func (p *Provisioner) getOpenstackMachineTemplates(ctx context.Context, c client.Client) ([]unstructured.Unstructured, error) {
	// TODO: this is flaky as hell, due to hard coded versions, needs a fix upstream.
	return p.getOwnedResource(ctx, c, "infrastructure.cluster.x-k8s.io/v1alpha6", "OpenStackMachineTemplate")
}

// resourceExistsUnstructured tells whether the resource exists in the
// expected list of names.
func resourceExistsUnstructured(o unstructured.Unstructured, names []string) bool {
	for _, name := range names {
		if name == o.GetName() {
			return true
		}
	}

	return false
}

// filterNamedResourcesUnstructured returns only the resources in the names list.
func filterNamedResourcesUnstructured(objects []unstructured.Unstructured, names []string) []unstructured.Unstructured {
	var filtered []unstructured.Unstructured

	for _, o := range objects {
		if resourceExistsUnstructured(o, names) {
			filtered = append(filtered, o)
		}
	}

	return filtered
}

// getExpectedKubeadmConfigTemplateNames extracts the expected config templates from the
// deployment references.
func getExpectedKubeadmConfigTemplateNames(deployments []unstructured.Unstructured) []string {
	names := make([]string, len(deployments))

	for i, deployment := range deployments {
		// TODO: may be not ok.
		names[i], _, _ = unstructured.NestedString(deployment.Object, "spec", "template", "spec", "bootstrap", "configRef", "name")
	}

	return names
}

// getExpectedOpenstackMachineTemplateNames extracts the expected machine templates from the
// deployment references.
func getExpectedOpenstackMachineTemplateNames(deployments []unstructured.Unstructured, controlPlanes []unstructured.Unstructured) []string {
	//nolint: prealloc
	var names []string

	for _, deployment := range deployments {
		// TODO: may be not ok.
		name, _, _ := unstructured.NestedString(deployment.Object, "spec", "template", "spec", "infrastructureRef", "name")

		names = append(names, name)
	}

	for _, controlPlane := range controlPlanes {
		name, _, _ := unstructured.NestedString(controlPlane.Object, "spec", "machineTemplate", "infrastructureRef", "name")

		names = append(names, name)
	}

	return names
}

// deleteForeignResources removes any resources in the given object set that
// don't have a corresponding name in the allowed list.
func deleteForeignResources(ctx context.Context, c client.Client, objects []unstructured.Unstructured, allowed []string) error {
	log := log.FromContext(ctx)

	for i, o := range objects {
		if resourceExistsUnstructured(o, allowed) {
			continue
		}

		log.Info("deleting orphaned resource", "kind", o.GetKind(), "name", o.GetName())

		if err := c.Delete(ctx, &objects[i]); err != nil {
			return err
		}
	}

	return nil
}

// deleteOrphanedMachineDeployments does just that. So what happens when you
// delete a workload pool is that the application notes it's no longer in the
// manifest, BUT, and I like big buts, cluster-api has added an owner reference,
// so Argo thinks it's an implicitly created resource now.  So, what we need to
// do is manually delete any orphaned MachineDeployments.
func (p *Provisioner) deleteOrphanedMachineDeployments(ctx context.Context) error {
	vc := vcluster.NewControllerRuntimeClient(p.client)

	vclusterClient, err := vc.Client(ctx, p.cluster.Namespace, false)
	if err != nil {
		return fmt.Errorf("%w: failed to create vcluster client", err)
	}

	deployments, err := p.getMachineDeployments(ctx, vclusterClient)
	if err != nil {
		return err
	}

	kubeadmConfigTemplates, err := p.getKubeadmConfigTemplates(ctx, vclusterClient)
	if err != nil {
		return err
	}

	kubeadmControlPlanes, err := p.getKubeadmControlPlanes(ctx, vclusterClient)
	if err != nil {
		return err
	}

	openstackMachineTemplates, err := p.getOpenstackMachineTemplates(ctx, vclusterClient)
	if err != nil {
		return err
	}

	// Work out the machine deployment names that should exist, grab all that
	// exist, and remember the intersection.
	deploymentNames := p.getWorkloadPoolMachineDeploymentNames()

	expectedDeployments := filterNamedResourcesUnstructured(deployments, deploymentNames)

	// Get the expected kubeadm config template and openstack machine template names from
	// the deployments.  These are generated by Helm, and unguessable.
	kubeadmConfigTemplatesNames := getExpectedKubeadmConfigTemplateNames(expectedDeployments)
	openstackMachineTemplatesNames := getExpectedOpenstackMachineTemplateNames(expectedDeployments, kubeadmControlPlanes)

	if err := deleteForeignResources(ctx, vclusterClient, deployments, deploymentNames); err != nil {
		return err
	}

	if err := deleteForeignResources(ctx, vclusterClient, kubeadmConfigTemplates, kubeadmConfigTemplatesNames); err != nil {
		return err
	}

	if err := deleteForeignResources(ctx, vclusterClient, openstackMachineTemplates, openstackMachineTemplatesNames); err != nil {
		return err
	}

	return nil
}
