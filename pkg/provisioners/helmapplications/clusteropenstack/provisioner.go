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
	"encoding/base64"
	"fmt"

	unikornv1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/cd"
	"github.com/eschercloudai/unikorn/pkg/constants"
	"github.com/eschercloudai/unikorn/pkg/provisioners/application"
)

const (
	// applicationName is the unique name of the application.
	applicationName = "cluster-openstack"

	// legacyApplicationName is what the application needs to be called so it's
	// not deleted and recreated, which would be quite catastrophic for an entire
	// Kubernetes cluster!
	legacyApplicationName = "kubernetes-cluster"
)

// Provisioner encapsulates control plane provisioning.
type Provisioner struct {
	// driver is the CD driver that implments applications.
	driver cd.Driver

	// cluster is the Kubernetes cluster we're provisioning.
	cluster *unikornv1.KubernetesCluster

	// controlPlanePrefix contains the IP address prefix to add
	// to the cluster firewall if, required.
	controlPlanePrefix string
}

// New returns a new initialized provisioner object.
func New(ctx context.Context, driver cd.Driver, cluster *unikornv1.KubernetesCluster, controlPlanePrefix string) *application.Provisioner {
	provisioner := &Provisioner{
		driver:             driver,
		cluster:            cluster,
		controlPlanePrefix: controlPlanePrefix,
	}

	return application.New(driver, applicationName, cluster).WithApplicationName(legacyApplicationName).WithGenerator(provisioner)
}

// Ensure the Provisioner interface is implemented.
var _ application.ReleaseNamer = &Provisioner{}
var _ application.ValuesGenerator = &Provisioner{}
var _ application.PostProvisionHook = &Provisioner{}

// generateMachineHelmValues translates the API's idea of a machine into what's
// expected by the underlying Helm chart.
func (p *Provisioner) generateMachineHelmValues(machine *unikornv1.MachineGeneric, failureDomain *string) map[string]interface{} {
	object := map[string]interface{}{
		"image":  *machine.Image,
		"flavor": *machine.Flavor,
	}

	if failureDomain != nil {
		object["failureDomain"] = *failureDomain
	}

	if machine.ServerGroupID != nil {
		object["serverGroupID"] = *machine.ServerGroupID
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

// generateWorkloadPoolHelmValues translates the API's idea of a workload pool into
// what's expected by the underlying Helm chart.
func (p *Provisioner) generateWorkloadPoolHelmValues() map[string]interface{} {
	workloadPools := map[string]interface{}{}

	for i := range p.cluster.Spec.WorkloadPools.Pools {
		workloadPool := &p.cluster.Spec.WorkloadPools.Pools[i]

		object := map[string]interface{}{
			"version":  string(*workloadPool.Version),
			"replicas": *workloadPool.Replicas,
			"machine":  p.generateMachineHelmValues(&workloadPool.MachineGeneric, workloadPool.FailureDomain),
		}

		if p.cluster.AutoscalingEnabled() && workloadPool.Autoscaling != nil {
			object["autoscaling"] = generateWorkloadPoolSchedulerHelmValues(workloadPool)
		}

		if len(workloadPool.Labels) != 0 {
			labels := map[string]interface{}{}

			for key, value := range workloadPool.Labels {
				labels[key] = value
			}

			object["labels"] = labels
		}

		if len(workloadPool.Files) != 0 {
			files := make([]interface{}, len(workloadPool.Files))

			for i, file := range workloadPool.Files {
				files[i] = map[string]interface{}{
					"path":    *file.Path,
					"content": base64.StdEncoding.EncodeToString(file.Content),
				}
			}

			object["files"] = files
		}

		workloadPools[workloadPool.Name] = object
	}

	return workloadPools
}

// generateWorkloadPoolSchedulerHelmValues translates from Kubernetes API scheduling
// parameters into ones acceptable by Helm.
func generateWorkloadPoolSchedulerHelmValues(p *unikornv1.KubernetesClusterWorkloadPoolsPoolSpec) map[string]interface{} {
	// When enabled, scaling limits are required.
	values := map[string]interface{}{
		"limits": map[string]interface{}{
			"minReplicas": *p.Autoscaling.MinimumReplicas,
			"maxReplicas": *p.Autoscaling.MaximumReplicas,
		},
	}

	// When scaler from zero is enabled, you need to provide CPU and memory hints,
	// the autoscaler cannot guess the flavor attributes.
	if p.Autoscaling.Scheduler != nil {
		scheduling := map[string]interface{}{
			"cpu":    *p.Autoscaling.Scheduler.CPU,
			"memory": fmt.Sprintf("%dG", p.Autoscaling.Scheduler.Memory.Value()>>30),
		}

		// If the flavor has a GPU, then we also need to inform the autoscaler
		// about the GPU scheduling information.
		if p.Autoscaling.Scheduler.GPU != nil {
			gpu := map[string]interface{}{
				"type":  *p.Autoscaling.Scheduler.GPU.Type,
				"count": *p.Autoscaling.Scheduler.GPU.Count,
			}

			scheduling["gpu"] = gpu
		}

		values["scheduler"] = scheduling
	}

	return values
}

// Generate implements the application.Generator interface.
func (p *Provisioner) Values(version *string) (interface{}, error) {
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
		"computeFailureDomain": *p.cluster.Spec.Openstack.FailureDomain,
		"volumeFailureDomain":  *volumeFailureDomain,
		"externalNetworkID":    *p.cluster.Spec.Openstack.ExternalNetworkID,
	}

	if p.cluster.Spec.Openstack.CACert != nil {
		openstackValues["ca"] = base64.StdEncoding.EncodeToString(*p.cluster.Spec.Openstack.CACert)
	}

	if p.cluster.Spec.Openstack.SSHKeyName != nil {
		openstackValues["sshKeyName"] = *p.cluster.Spec.Openstack.SSHKeyName
	}

	labels, err := p.cluster.ResourceLabels()
	if err != nil {
		return nil, err
	}

	serverMetadata := map[string]interface{}{
		"cluster":      p.cluster.Name,
		"controlPlane": labels[constants.ControlPlaneLabel],
		"project":      labels[constants.ProjectLabel],
	}

	// TODO: generate types from the Helm values schema.
	values := map[string]interface{}{
		"openstack": openstackValues,
		"cluster": map[string]interface{}{
			"taints": []interface{}{
				// TODO: This is deprecated moving forward as the cilium operator provides these taints.
				//   We can't remove it yet though as it'd break any deployments pre kubernetes-cluster-1.4.0

				// This prevents things like coreDNS from coming up until
				// the CNI is installed.
				map[string]interface{}{
					"key":    "node.cilium.io/agent-not-ready",
					"effect": "NoSchedule",
					"value":  "true",
				},
			},
			"serverMetadata": serverMetadata,
		},
		"controlPlane": map[string]interface{}{
			"version":  string(*p.cluster.Spec.ControlPlane.Version),
			"replicas": *p.cluster.Spec.ControlPlane.Replicas,
			"machine":  p.generateMachineHelmValues(&p.cluster.Spec.ControlPlane.MachineGeneric, nil),
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

		values["api"] = apiValues
	}

	return values, nil
}

// ReleaseName implements the application.ReleaseNamer interface.
func (p *Provisioner) ReleaseName() string {
	return releaseName(p.cluster)
}

// PostHook implements the apllication PostProvisionHook interface.
func (p *Provisioner) PostProvision(ctx context.Context) error {
	return p.deleteOrphanedMachineDeployments(ctx)
}
