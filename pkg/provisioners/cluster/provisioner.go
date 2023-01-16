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

package cluster

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/gophercloud/utils/openstack/clientconfig"
	"golang.org/x/sync/errgroup"

	unikornv1alpha1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	argocdclient "github.com/eschercloudai/unikorn/pkg/argocd/client"
	argocdcluster "github.com/eschercloudai/unikorn/pkg/argocd/cluster"
	"github.com/eschercloudai/unikorn/pkg/constants"
	"github.com/eschercloudai/unikorn/pkg/provisioners"
	"github.com/eschercloudai/unikorn/pkg/provisioners/application"
	"github.com/eschercloudai/unikorn/pkg/provisioners/vcluster"
	"github.com/eschercloudai/unikorn/pkg/util/retry"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"
)

var (
	// ErrLabelMissing is returned when a required label is not present on
	// the cluster resource.
	ErrLabelMissing = errors.New("expected label missing")

	// ErrCloudConfiguration is returned when the cloud configuration is not
	// correctly formatted.
	ErrCloudConfiguration = errors.New("invalid cloud configuration")
)

// Provisioner encapsulates control plane provisioning.
type Provisioner struct {
	// client provides access to Kubernetes.
	client client.Client

	// cluster is the Kubernetes cluster we're provisioning.
	cluster *unikornv1alpha1.KubernetesCluster

	server string

	labels map[string]string

	workloadPools *unikornv1alpha1.KubernetesWorkloadPoolList
}

// New returns a new initialized provisioner object.
func New(client client.Client, cluster *unikornv1alpha1.KubernetesCluster, server string) *Provisioner {
	return &Provisioner{
		client:  client,
		cluster: cluster,
		server:  server,
	}
}

// Ensure the Provisioner interface is implemented.
var _ provisioners.Provisioner = &Provisioner{}

// WithLabels associates set of labels with the provisioner to uniquely identify
// the provisioner instance.
// TODO: this is fairly common, share it using interface aggregation.
func (p *Provisioner) WithLabels(l map[string]string) *Provisioner {
	p.labels = l

	return p
}

// getLabels returns an application specific set of labels to uniquely identify the
// application.
func (p *Provisioner) getLabels(app string) map[string]interface{} {
	l := map[string]interface{}{
		constants.ApplicationLabel: app,
	}

	for k, v := range p.labels {
		l[k] = v
	}

	return l
}

// generateMachineHelmValues translates the API's idea of a machine into what's
// expected by the underlying Helm chart.
func (p *Provisioner) generateMachineHelmValues(machine *unikornv1alpha1.MachineGeneric) map[string]interface{} {
	object := map[string]interface{}{
		"image":  *machine.Image,
		"flavor": *machine.Flavor,
	}

	if machine.DiskSize != nil {
		object["diskSize"] = machine.DiskSize.Value() >> 30
	}

	return object
}

// getWorkloadPools athers all workload pools that belong to this cluster.
// By default that's all the things, in reality most sane people will add label
// selectors.
func (p *Provisioner) getWorkloadPools(ctx context.Context) (*unikornv1alpha1.KubernetesWorkloadPoolList, error) {
	selector := labels.Everything()

	if p.cluster.Spec.WorkloadPools != nil && p.cluster.Spec.WorkloadPools.Selector != nil {
		s, err := metav1.LabelSelectorAsSelector(p.cluster.Spec.WorkloadPools.Selector)
		if err != nil {
			return nil, err
		}

		selector = s
	}

	workloadPools := &unikornv1alpha1.KubernetesWorkloadPoolList{}

	if err := p.client.List(ctx, workloadPools, &client.ListOptions{LabelSelector: selector}); err != nil {
		return nil, err
	}

	// The inherent problem here is a race condition, with us picking something up
	// even though it's marked for deletion, so it stays around.
	filtered := &unikornv1alpha1.KubernetesWorkloadPoolList{}

	for _, pool := range workloadPools.Items {
		if pool.DeletionTimestamp == nil {
			filtered.Items = append(filtered.Items, pool)
		}
	}

	return filtered, nil
}

// generateWorkloadPoolHelmValues translates the API's idea of a workload pool into
// what's expected by the underlying Helm chart.
func (p *Provisioner) generateWorkloadPoolHelmValues() map[string]interface{} {
	workloadPools := map[string]interface{}{}

	// Not necessary for the delete case.
	// TODO: we should perhaps just set this in the New function to prevent
	// future problems.
	if p.workloadPools == nil {
		return nil
	}

	for i := range p.workloadPools.Items {
		workloadPool := &p.workloadPools.Items[i]

		object := map[string]interface{}{
			"version":  string(*workloadPool.Spec.Version),
			"replicas": *workloadPool.Spec.Replicas,
			"machine":  p.generateMachineHelmValues(&workloadPool.Spec.MachineGeneric),
		}

		if p.cluster.AutoscalingEnabled() && workloadPool.Spec.Autoscaling != nil {
			object["autoscaling"] = generateWorkloadPoolSchedulerHelmValues(workloadPool)
		}

		if len(workloadPool.Spec.Labels) != 0 {
			labels := map[string]interface{}{}

			for key, value := range workloadPool.Spec.Labels {
				labels[key] = value
			}

			object["labels"] = labels
		}

		if len(workloadPool.Spec.Files) != 0 {
			files := make([]interface{}, len(workloadPool.Spec.Files))

			for i, file := range workloadPool.Spec.Files {
				files[i] = map[string]interface{}{
					"path":    *file.Path,
					"content": base64.StdEncoding.EncodeToString(file.Content),
				}
			}

			object["files"] = files
		}

		workloadPools[workloadPool.GetName()] = object
	}

	return workloadPools
}

// generateWorkloadPoolSchedulerHelmValues translates from Kubernetes API scheduling
// parameters into ones acceptable by Helm.
func generateWorkloadPoolSchedulerHelmValues(p *unikornv1alpha1.KubernetesWorkloadPool) map[string]interface{} {
	// When enabled, scaling limits are required.
	values := map[string]interface{}{
		"limits": map[string]interface{}{
			"minReplicas": *p.Spec.Autoscaling.MinimumReplicas,
			"maxReplicas": *p.Spec.Autoscaling.MaximumReplicas,
		},
	}

	// When scaler from zero is enabled, you need to provide CPU and memory hints,
	// the autoscaler cannot guess the flavor attributes.
	if p.Spec.Autoscaling.Scheduler != nil {
		scheduling := map[string]interface{}{
			"cpu":    *p.Spec.Autoscaling.Scheduler.CPU,
			"memory": fmt.Sprintf("%dG", p.Spec.Autoscaling.Scheduler.Memory.Value()>>30),
		}

		// If the flavor has a GPU, then we also need to inform the autoscaler
		// about the GPU scheduling information.
		if p.Spec.Autoscaling.Scheduler.GPU != nil {
			gpu := map[string]interface{}{
				"type":  *p.Spec.Autoscaling.Scheduler.GPU.Type,
				"count": *p.Spec.Autoscaling.Scheduler.GPU.Count,
			}

			scheduling["gpu"] = gpu
		}

		values["scheduler"] = scheduling
	}

	return values
}

// generateApplication creates an ArgoCD application for a cluster.
func (p *Provisioner) generateApplication() (*unstructured.Unstructured, error) {
	workloadPools := p.generateWorkloadPoolHelmValues()

	nameservers := make([]interface{}, len(p.cluster.Spec.Network.DNSNameservers))

	for i, nameserver := range p.cluster.Spec.Network.DNSNameservers {
		nameservers[i] = nameserver.IP.String()
	}

	// TODO: generate types from the Helm values schema.
	// TODO: add in API configuration.
	valuesRaw := map[string]interface{}{
		"openstack": map[string]interface{}{
			"cloud":             *p.cluster.Spec.Openstack.Cloud,
			"cloudsYAML":        base64.StdEncoding.EncodeToString(*p.cluster.Spec.Openstack.CloudConfig),
			"ca":                base64.StdEncoding.EncodeToString(*p.cluster.Spec.Openstack.CACert),
			"sshKeyName":        *p.cluster.Spec.Openstack.SSHKeyName,
			"region":            *p.cluster.Spec.Openstack.Region,
			"failureDomain":     *p.cluster.Spec.Openstack.FailureDomain,
			"externalNetworkID": *p.cluster.Spec.Openstack.ExternalNetworkID,
		},
		"cluster": map[string]interface{}{
			"taints": []interface{}{
				// This prevents things like coreDNS from coming up until
				// the CNI is installed.
				map[string]interface{}{
					"key":    "node.cilium.io/agent-not-ready",
					"effect": "NoSchedule",
					"value":  "true",
				},
			},
		},
		"controlPlane": map[string]interface{}{
			"version":  string(*p.cluster.Spec.ControlPlane.Version),
			"replicas": *p.cluster.Spec.ControlPlane.Replicas,
			"machine":  p.generateMachineHelmValues(&p.cluster.Spec.ControlPlane.MachineGeneric),
		},
		"workloadPools": workloadPools,
		"network": map[string]interface{}{
			"nodeCIDR": p.cluster.Spec.Network.NodeNetwork.IPNet.String(),
			"serviceCIDRs": []interface{}{
				p.cluster.Spec.Network.ServiceNetwork.IPNet.String(),
			},
			"podCIDRs": []interface{}{
				p.cluster.Spec.Network.PodNetwork.IPNet.String(),
			},
			"dnsNameservers": nameservers,
		},
	}

	values, err := yaml.Marshal(valuesRaw)
	if err != nil {
		return nil, err
	}

	// Okay, from this point on, things get a bit "meta" because whoever
	// wrote ArgoCD for some reason imported kubernetes, not client-go to
	// get access to the schema information...
	object := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata": map[string]interface{}{
				"generateName": "kubernetes-cluster-",
				"namespace":    "argocd",
				"labels":       p.getLabels("kubernetes-cluster"),
			},
			"spec": map[string]interface{}{
				"project": "default",
				"source": map[string]interface{}{
					//TODO:  programmable
					"repoURL":        "https://eschercloudai.github.io/helm-cluster-api",
					"chart":          "cluster-api-cluster-openstack",
					"targetRevision": "v0.3.2",
					"helm": map[string]interface{}{
						"releaseName": p.cluster.Name,
						"values":      string(values),
					},
				},
				"destination": map[string]interface{}{
					"name":      p.server,
					"namespace": p.cluster.Name,
				},
				"syncPolicy": map[string]interface{}{
					"automated": map[string]interface{}{
						"selfHeal": true,
						"prune":    true,
					},
					"syncOptions": []string{
						"CreateNamespace=true",
					},
				},
			},
		},
	}

	return object, nil
}

// generateOpenstackCloudProviderApplication creates an ArgoCD application for
// the Openstack controller manager.
func (p *Provisioner) generateOpenstackCloudProviderApplication(server string) (*unstructured.Unstructured, error) {
	var clouds clientconfig.Clouds

	if err := yaml.Unmarshal(*p.cluster.Spec.Openstack.CloudConfig, &clouds); err != nil {
		return nil, err
	}

	cloud, ok := clouds.Clouds[*p.cluster.Spec.Openstack.Cloud]
	if !ok {
		return nil, fmt.Errorf("%w: cloud '%s' not found in clouds.yaml", ErrCloudConfiguration, *p.cluster.Spec.Openstack.Cloud)
	}

	valuesRaw := map[string]interface{}{
		"cloudConfig": map[string]interface{}{
			"global": map[string]interface{}{
				"auth-url":    cloud.AuthInfo.AuthURL,
				"username":    cloud.AuthInfo.Username,
				"password":    cloud.AuthInfo.Password,
				"domain-name": cloud.AuthInfo.DomainName,
				"tenant-name": cloud.AuthInfo.ProjectName,
			},
			"loadBalancer": map[string]interface{}{
				"floating-network-id": *p.cluster.Spec.Openstack.ExternalNetworkID,
			},
		},
		"tolerations": []interface{}{
			map[string]interface{}{
				"key":    "node-role.kubernetes.io/master",
				"effect": "NoSchedule",
			},
			map[string]interface{}{
				"key":    "node-role.kubernetes.io/control-plane",
				"effect": "NoSchedule",
			},
			map[string]interface{}{
				"key":    "node.cloudprovider.kubernetes.io/uninitialized",
				"effect": "NoSchedule",
				"value":  "true",
			},
			map[string]interface{}{
				"key":    "node.cilium.io/agent-not-ready",
				"effect": "NoSchedule",
				"value":  "true",
			},
		},
	}

	values, err := yaml.Marshal(valuesRaw)
	if err != nil {
		return nil, err
	}

	object := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata": map[string]interface{}{
				"generateName": "openstack-cloud-provider-",
				"namespace":    "argocd",
				"labels":       p.getLabels("openstack-cloud-provider"),
			},
			"spec": map[string]interface{}{
				"project": "default",
				"source": map[string]interface{}{
					//TODO: programmable
					//TODO: the revision does have a Kubernetes app version...
					"repoURL":        "https://kubernetes.github.io/cloud-provider-openstack",
					"chart":          "openstack-cloud-controller-manager",
					"targetRevision": "1.4.0",
					"helm": map[string]interface{}{
						"values": string(values),
					},
				},
				"destination": map[string]interface{}{
					"name":      server,
					"namespace": "ocp-system",
				},
				"syncPolicy": map[string]interface{}{
					"automated": map[string]interface{}{
						"selfHeal": true,
						"prune":    true,
					},
					"syncOptions": []string{
						"CreateNamespace=true",
					},
				},
			},
		},
	}

	return object, nil
}

// generateCiliumApplication creates an ArgoCD application for the
// Cilium CNI plugin.
func (p *Provisioner) generateCiliumApplication(server string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata": map[string]interface{}{
				"generateName": "cilium-",
				"namespace":    "argocd",
				"labels":       p.getLabels("cilium"),
			},
			"spec": map[string]interface{}{
				"project": "default",
				"source": map[string]interface{}{
					//TODO:  programmable
					"repoURL":        "https://helm.cilium.io/",
					"chart":          "cilium",
					"targetRevision": "1.12.4",
				},
				"destination": map[string]interface{}{
					"name":      server,
					"namespace": "kube-system",
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

// generateClusterAuotscalerApplication creates an in-cluster instance of the
// cluster autoscaler that is deployed in the same namespace as the cluster,
// with namespace scoped privilege.
func (p *Provisioner) generateClusterAuotscalerApplication() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata": map[string]interface{}{
				"generateName": "cluster-autoscaler-",
				"namespace":    "argocd",
				"labels":       p.getLabels("cluster-autoscaler"),
			},
			"spec": map[string]interface{}{
				"project": "default",
				"source": map[string]interface{}{
					//TODO:  programmable
					"repoURL":        "https://kubernetes.github.io/autoscaler",
					"chart":          "cluster-autoscaler",
					"targetRevision": "9.21.1",
					"helm": map[string]interface{}{
						"parameters": []interface{}{
							map[string]interface{}{
								"name":  "cloudProvider",
								"value": "clusterapi",
							},
							map[string]interface{}{
								"name":  "clusterAPIMode",
								"value": "kubeconfig-incluster",
							},
							map[string]interface{}{
								"name":  "clusterAPIKubeconfigSecret",
								"value": p.cluster.Name + "-kubeconfig",
							},
							map[string]interface{}{
								"name":  "autoDiscovery.clusterName",
								"value": p.cluster.Name,
							},
							map[string]interface{}{
								"name":  "extraArgs.scale-down-delay-after-add",
								"value": "5m",
							},
							map[string]interface{}{
								"name":  "extraArgs.scale-down-unneeded-time",
								"value": "5m",
							},
							map[string]interface{}{
								"name":  "rbac.clusterScoped",
								"value": "false",
							},
						},
					},
				},
				"destination": map[string]interface{}{
					"name":      p.server,
					"namespace": p.cluster.Name,
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

// getWorkloadPoolMachineDeploymentNames gets a list of machine deployments that should
// exist for this cluster.
// TODO: this is horrific and relies on knowing the internal workings of the Helm chart
// not just the public API!!!
func (p *Provisioner) getWorkloadPoolMachineDeploymentNames() []string {
	names := make([]string, len(p.workloadPools.Items))

	for i, pool := range p.workloadPools.Items {
		names[i] = fmt.Sprintf("%s-pool-%s", p.cluster.Name, pool.GetName())
	}

	return names
}

// filterOwnedResources removes any resources that aren't owned by the cluster.
func (p *Provisioner) filterOwnedResources(resources []unstructured.Unstructured) []unstructured.Unstructured {
	var filtered []unstructured.Unstructured

	for _, resource := range resources {
		ownerReferences := resource.GetOwnerReferences()

		for _, ownerReference := range ownerReferences {
			if ownerReference.Kind != "Cluster" || ownerReference.Name != p.cluster.Name {
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
	return p.getOwnedResource(ctx, c, "infrastructure.cluster.x-k8s.io/v1alpha5", "OpenStackMachineTemplate")
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

// provisionCluster creates a Kubernetes cluster application.
func (p *Provisioner) provisionCluster(ctx context.Context) error {
	log := log.FromContext(ctx)

	log.Info("provisioning kubernetes cluster")

	// Do this once so it's atomic, we don't want it changing in different
	// places.
	workloadPools, err := p.getWorkloadPools(ctx)
	if err != nil {
		return err
	}

	p.workloadPools = workloadPools

	object, err := p.generateApplication()
	if err != nil {
		return err
	}

	if err := application.New(p.client, object).Provision(ctx); err != nil {
		return err
	}

	if err := p.deleteOrphanedMachineDeployments(ctx); err != nil {
		return err
	}

	log.Info("kubernetes cluster provisioned")

	return nil
}

// getKubernetesClusterConfig retrieves the Kubernetes configuration from
// a cluster API cluster.
func (p *Provisioner) getKubernetesClusterConfig(ctx context.Context) (*clientcmdapi.Config, error) {
	vc := vcluster.NewControllerRuntimeClient(p.client)

	vclusterClient, err := vc.Client(ctx, p.cluster.Namespace, false)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to get vcluster client", err)
	}

	secret := &corev1.Secret{}

	secretKey := client.ObjectKey{
		Namespace: p.cluster.Name,
		Name:      p.cluster.Name + "-kubeconfig",
	}

	// Retry getting the secret until it exists.
	getSecret := func() error {
		return vclusterClient.Get(ctx, secretKey, secret)
	}

	if err := retry.Forever().DoWithContext(ctx, getSecret); err != nil {
		return nil, err
	}

	config, err := clientcmd.NewClientConfigFromBytes(secret.Data["value"])
	if err != nil {
		return nil, err
	}

	rawConfig, err := config.RawConfig()
	if err != nil {
		return nil, err
	}

	return &rawConfig, nil
}

// provisionAddOns runs in parallel with provisionCluster.  The cluster API machine
// deployment will not become healthy until the Kubernetes nodes report as healty
// and that requires a CNI to be installed, and the cloud provider.  Obviously this
// isn't made easy by CAPI, many have tried, many have failed.  We need to poll the
// CAPI deployment until the Kubernetes config is available, install it in ArgoCD, then
// deploy the Cilium and cloud provider applications on the remote.
func (p *Provisioner) provisionAddOns(ctx context.Context) error {
	log := log.FromContext(ctx)

	log.Info("provisioning addons")

	config, err := p.getKubernetesClusterConfig(ctx)
	if err != nil {
		return err
	}

	server := config.Clusters[config.Contexts[config.CurrentContext].Cluster].Server

	argocd, err := argocdclient.NewInCluster(ctx, p.client, "argocd")
	if err != nil {
		return err
	}

	// Retry adding the cluster until ArgoCD deems it's ready, it'll 500 until that
	// condition is met.
	upsertCluster := func() error {
		if err := argocdcluster.Upsert(ctx, argocd, server, config); err != nil {
			return err
		}

		return nil
	}

	if err := retry.Forever().DoWithContext(ctx, upsertCluster); err != nil {
		return err
	}

	group, gctx := errgroup.WithContext(ctx)

	group.Go(func() error { return p.provisionOpenstackCloudProvider(gctx, server) })
	group.Go(func() error { return p.provisionCilium(gctx, server) })

	if err := group.Wait(); err != nil {
		return err
	}

	log.Info("addons provisioned")

	return nil
}

// provisionCilium applies the Cilium CNI to the Kubnernetes cluster.
func (p *Provisioner) provisionCilium(ctx context.Context, server string) error {
	log := log.FromContext(ctx)

	log.Info("provisioning cilium")

	if err := application.New(p.client, p.generateCiliumApplication(server)).Provision(ctx); err != nil {
		return err
	}

	log.Info("cilium provisioned")

	return nil
}

// provisionOpenstackCloudProvider applies the openstack cloud controller
// to the Kubnernetes cluster.
func (p *Provisioner) provisionOpenstackCloudProvider(ctx context.Context, server string) error {
	log := log.FromContext(ctx)

	log.Info("provisioning openstack cloud provider")

	object, err := p.generateOpenstackCloudProviderApplication(server)
	if err != nil {
		return err
	}

	if err := application.New(p.client, object).Provision(ctx); err != nil {
		return err
	}

	log.Info("openstack cloud provider provisioned")

	return nil
}

// provisionClusterAutoscaler creates a cluster autoscaler in the control
// plane if autoscaling is enabled.
func (p *Provisioner) provisionClusterAutoscaler(ctx context.Context) error {
	log := log.FromContext(ctx)

	// TODO: you can create with it on, turn it on, but not remove it...
	if !p.cluster.AutoscalingEnabled() {
		return nil
	}

	log.Info("provisioning cluster autoscaler")

	if err := application.New(p.client, p.generateClusterAuotscalerApplication()).Provision(ctx); err != nil {
		return err
	}

	log.Info("cluster autoscaler provisioned")

	return nil
}

// Provision implements the Provision interface.
func (p *Provisioner) Provision(ctx context.Context) error {
	log := log.FromContext(ctx)

	log.Info("provisioning unikorn kubernetes cluster")

	group, gctx := errgroup.WithContext(ctx)

	group.Go(func() error { return p.provisionCluster(gctx) })
	group.Go(func() error { return p.provisionAddOns(gctx) })

	if err := group.Wait(); err != nil {
		return err
	}

	if err := p.provisionClusterAutoscaler(ctx); err != nil {
		return err
	}

	log.Info("unikorn kubernetes cluster provisioned")

	return nil
}

// Deprovision implements the Provision interface.
//
//nolint:cyclop
func (p *Provisioner) Deprovision(ctx context.Context) error {
	log := log.FromContext(ctx)

	config, err := p.getKubernetesClusterConfig(ctx)
	if err != nil {
		return err
	}

	server := config.Clusters[config.Contexts[config.CurrentContext].Cluster].Server

	if p.cluster.AutoscalingEnabled() {
		log.Info("deprovisioning cluster autoscaler")

		if err := application.New(p.client, p.generateClusterAuotscalerApplication()).Deprovision(ctx); err != nil {
			return err
		}

		log.Info("cluster autoscaler deprovisioned")
	}

	log.Info("deprovisioning cilium")

	if err := application.New(p.client, p.generateCiliumApplication(server)).Deprovision(ctx); err != nil {
		return err
	}

	log.Info("cilium deprovisioned")

	log.Info("deprovisioning openstack cloud provider")

	object, err := p.generateOpenstackCloudProviderApplication(server)
	if err != nil {
		return err
	}

	if err := application.New(p.client, object).Deprovision(ctx); err != nil {
		return err
	}

	log.Info("openstack cloud provider deprovisioned")

	log.Info("deprovisioning kubernetes cluster")

	argocd, err := argocdclient.NewInCluster(ctx, p.client, "argocd")
	if err != nil {
		return err
	}

	if err := argocdcluster.Delete(ctx, argocd, server); err != nil {
		return err
	}

	object, err = p.generateApplication()
	if err != nil {
		return err
	}

	if err := application.New(p.client, object).Deprovision(ctx); err != nil {
		return err
	}

	log.Info("kubernetes cluster deprovisioned")

	return nil
}
