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

package clusteropenstack

import (
	"context"
	"encoding/base64"
	"fmt"

	unikornv1alpha1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/provisioners"
	"github.com/eschercloudai/unikorn/pkg/provisioners/application"
	"github.com/eschercloudai/unikorn/pkg/provisioners/remotecluster"
	"github.com/eschercloudai/unikorn/pkg/util"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

const (
	// applicationName is the unique name of the application.
	applicationName = "kubernetes-cluster"
)

// Provisioner encapsulates control plane provisioning.
type Provisioner struct {
	// client provides access to Kubernetes.
	client client.Client

	// cluster is the Kubernetes cluster we're provisioning.
	cluster *unikornv1alpha1.KubernetesCluster

	// remote is the remote cluster to deploy to.
	remote remotecluster.Generator

	// workloadPools is a snapshot of the workload pool members at
	// creation time.
	workloadPools *unikornv1alpha1.KubernetesWorkloadPoolList

	// controlPlanePrefix contains the IP address prefix to add
	// to the cluster firewall if, required.
	controlPlanePrefix string
}

// New returns a new initialized provisioner object.
func New(ctx context.Context, client client.Client, cluster *unikornv1alpha1.KubernetesCluster, remote remotecluster.Generator) (*Provisioner, error) {
	// Do this once so it's atomic, we don't want it changing in different
	// places.
	workloadPools, err := getWorkloadPools(ctx, client, cluster)
	if err != nil {
		return nil, err
	}

	// Add the SNAT address of the control plane's default route.
	// Sadly, we are the only thing guaranteed to live behind the same
	// router, the CLI tools and UI are or can be used anywhere, so
	// we'll take on this hack.
	controlPlanePrefix, err := util.GetNATPrefix(ctx)
	if err != nil {
		return nil, err
	}

	provisioner := &Provisioner{
		client:             client,
		cluster:            cluster,
		remote:             remote,
		workloadPools:      workloadPools,
		controlPlanePrefix: controlPlanePrefix,
	}

	return provisioner, nil
}

// Ensure the Provisioner interface is implemented.
var _ provisioners.Provisioner = &Provisioner{}
var _ application.Generator = &Provisioner{}

// generateMachineHelmValues translates the API's idea of a machine into what's
// expected by the underlying Helm chart.
func (p *Provisioner) generateMachineHelmValues(machine *unikornv1alpha1.MachineGeneric) map[string]interface{} {
	object := map[string]interface{}{
		"image":  *machine.Image,
		"flavor": *machine.Flavor,
	}

	if machine.DiskSize != nil {
		disk := map[string]interface{}{
			"size": machine.DiskSize.Value() >> 30,
		}

		if machine.VolumeFailureDomain != nil {
			disk["failureDomain"] = *machine.VolumeFailureDomain
		}

		object["disk"] = disk
	}

	return object
}

// getWorkloadPools athers all workload pools that belong to this cluster.
// By default that's all the things, in reality most sane people will add label
// selectors.
func getWorkloadPools(ctx context.Context, c client.Client, cluster *unikornv1alpha1.KubernetesCluster) (*unikornv1alpha1.KubernetesWorkloadPoolList, error) {
	selector := labels.Everything()

	if cluster.Spec.WorkloadPools != nil && cluster.Spec.WorkloadPools.Selector != nil {
		s, err := metav1.LabelSelectorAsSelector(cluster.Spec.WorkloadPools.Selector)
		if err != nil {
			return nil, err
		}

		selector = s
	}

	workloadPools := &unikornv1alpha1.KubernetesWorkloadPoolList{}

	if err := c.List(ctx, workloadPools, &client.ListOptions{LabelSelector: selector}); err != nil {
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

// Resource implements the application.Generator interface.
func (p *Provisioner) Resource() application.MutuallyExclusiveResource {
	return p.cluster
}

// Name implements the application.Generator interface.
func (p *Provisioner) Name() string {
	return applicationName
}

// Generate implements the application.Generator interface.
func (p *Provisioner) Generate() (client.Object, error) {
	workloadPools := p.generateWorkloadPoolHelmValues()

	nameservers := make([]interface{}, len(p.cluster.Spec.Network.DNSNameservers))

	for i, nameserver := range p.cluster.Spec.Network.DNSNameservers {
		nameservers[i] = nameserver.IP.String()
	}

	// Support interim legacy behavior.
	volumeFailureDomain := p.cluster.Spec.Openstack.VolumeFailureDomain
	if volumeFailureDomain == nil {
		volumeFailureDomain = p.cluster.Spec.Openstack.FailureDomain
	}

	openstackValues := map[string]interface{}{
		"cloud":                *p.cluster.Spec.Openstack.Cloud,
		"cloudsYAML":           base64.StdEncoding.EncodeToString(*p.cluster.Spec.Openstack.CloudConfig),
		"ca":                   base64.StdEncoding.EncodeToString(*p.cluster.Spec.Openstack.CACert),
		"computeFailureDomain": *p.cluster.Spec.Openstack.FailureDomain,
		"volumeFailureDomain":  *volumeFailureDomain,
		"externalNetworkID":    *p.cluster.Spec.Openstack.ExternalNetworkID,
	}

	if p.cluster.Spec.Openstack.SSHKeyName != nil {
		openstackValues["sshKeyName"] = *p.cluster.Spec.Openstack.SSHKeyName
	}

	// TODO: generate types from the Helm values schema.
	valuesRaw := map[string]interface{}{
		"openstack": openstackValues,
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

	if p.cluster.Spec.API != nil {
		apiValues := map[string]interface{}{}

		if p.cluster.Spec.API.SubjectAlternativeNames != nil {
			apiValues["certificateSANs"] = p.cluster.Spec.API.SubjectAlternativeNames
		}

		if p.cluster.Spec.API.AllowedPrefixes != nil {
			// Add the SNAT IP so CAPI can manage the cluster.
			allowList := []interface{}{
				p.controlPlanePrefix,
			}

			for _, prefix := range p.cluster.Spec.API.AllowedPrefixes {
				allowList = append(allowList, prefix.IPNet.String())
			}

			apiValues["allowList"] = allowList
		}

		valuesRaw["api"] = apiValues
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
			"spec": map[string]interface{}{
				"project": "default",
				"source": map[string]interface{}{
					//TODO:  programmable
					"repoURL":        "https://eschercloudai.github.io/helm-cluster-api",
					"chart":          "cluster-api-cluster-openstack",
					"targetRevision": "v0.3.9",
					"helm": map[string]interface{}{
						"releaseName": p.cluster.Name,
						"values":      string(values),
					},
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

// Provision implements the Provision interface.
func (p *Provisioner) Provision(ctx context.Context) error {
	// TODO: this application is special in that everything it creates is a
	// CRD, so as far as Argo is concerned, everything is healthy and the
	// check passes instantly, rather than waiting for the CAPI controllers
	// to do something.  We kinda fudge it due to the concurrent deployment
	// of the CNI and cloud controller add-ons blocking.
	if err := application.New(p.client, p).OnRemote(p.remote).InNamespace(p.cluster.Name).Provision(ctx); err != nil {
		return err
	}

	if err := p.deleteOrphanedMachineDeployments(ctx); err != nil {
		return err
	}

	return nil
}

// Deprovision implements the Provision interface.
func (p *Provisioner) Deprovision(ctx context.Context) error {
	if err := application.New(p.client, p).OnRemote(p.remote).InNamespace(p.cluster.Name).Deprovision(ctx); err != nil {
		return err
	}

	return nil
}
