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
	"github.com/eschercloudai/unikorn/pkg/provisioners"
	"github.com/eschercloudai/unikorn/pkg/provisioners/application"
	"github.com/eschercloudai/unikorn/pkg/util"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
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
	cluster *unikornv1.KubernetesCluster

	// remote is the remote cluster to deploy to.
	remote provisioners.RemoteCluster

	// workloadPools is a snapshot of the workload pool members at
	// creation time.
	workloadPools *unikornv1.KubernetesWorkloadPoolList

	// controlPlanePrefix contains the IP address prefix to add
	// to the cluster firewall if, required.
	controlPlanePrefix string

	// application is the application used to identify the Helm chart to use.
	application *unikornv1.HelmApplication

	// namespace defines where to install the application.
	namespace string
}

// New returns a new initialized provisioner object.
func New(ctx context.Context, client client.Client, cluster *unikornv1.KubernetesCluster, application *unikornv1.HelmApplication) (provisioners.Provisioner, error) {
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
		workloadPools:      workloadPools,
		controlPlanePrefix: controlPlanePrefix,
		application:        application,
	}

	return provisioner, nil
}

// Ensure the Provisioner interface is implemented.
var _ provisioners.Provisioner = &Provisioner{}
var _ application.ReleaseNamer = &Provisioner{}
var _ application.ValuesGenerator = &Provisioner{}

// OnRemote implements the Provision interface.
func (p *Provisioner) OnRemote(remote provisioners.RemoteCluster) provisioners.Provisioner {
	p.remote = remote

	return p
}

// InNamespace implements the Provision interface.
func (p *Provisioner) InNamespace(namespace string) provisioners.Provisioner {
	p.namespace = namespace

	return p
}

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
func getWorkloadPools(ctx context.Context, c client.Client, cluster *unikornv1.KubernetesCluster) (*unikornv1.KubernetesWorkloadPoolList, error) {
	// No workload pools at all.
	if cluster.Spec.WorkloadPools == nil {
		//nolint:nilnil
		return nil, nil
	}

	// When using selector based pools, just list them.
	if cluster.Spec.WorkloadPools.Selector != nil {
		selector, err := metav1.LabelSelectorAsSelector(cluster.Spec.WorkloadPools.Selector)
		if err != nil {
			return nil, err
		}

		workloadPools := &unikornv1.KubernetesWorkloadPoolList{}

		if err := c.List(ctx, workloadPools, &client.ListOptions{LabelSelector: selector}); err != nil {
			return nil, err
		}

		// The inherent problem here is a race condition, with us picking something up
		// even though it's marked for deletion, so it stays around.
		filtered := &unikornv1.KubernetesWorkloadPoolList{}

		for _, pool := range workloadPools.Items {
			if pool.DeletionTimestamp == nil {
				filtered.Items = append(filtered.Items, pool)
			}
		}

		return filtered, nil
	}

	// Otherwise we are using inline pools.
	workloadPools := &unikornv1.KubernetesWorkloadPoolList{}

	for _, pool := range cluster.Spec.WorkloadPools.Pools {
		workloadPool := unikornv1.KubernetesWorkloadPool{
			ObjectMeta: metav1.ObjectMeta{
				Name: pool.Name,
			},
			Spec: pool.KubernetesWorkloadPoolSpec,
		}

		workloadPools.Items = append(workloadPools.Items, workloadPool)
	}

	return workloadPools, nil
}

// generateWorkloadPoolHelmValues translates the API's idea of a workload pool into
// what's expected by the underlying Helm chart.
func (p *Provisioner) generateWorkloadPoolHelmValues() map[string]interface{} {
	workloadPools := map[string]interface{}{}

	for i := range p.workloadPools.Items {
		workloadPool := &p.workloadPools.Items[i]

		object := map[string]interface{}{
			"version":  string(*workloadPool.Spec.Version),
			"replicas": *workloadPool.Spec.Replicas,
			"machine":  p.generateMachineHelmValues(&workloadPool.Spec.MachineGeneric, workloadPool.Spec.FailureDomain),
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
func generateWorkloadPoolSchedulerHelmValues(p *unikornv1.KubernetesWorkloadPool) map[string]interface{} {
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
		"ca":                   base64.StdEncoding.EncodeToString(*p.cluster.Spec.Openstack.CACert),
		"computeFailureDomain": *p.cluster.Spec.Openstack.FailureDomain,
		"volumeFailureDomain":  *volumeFailureDomain,
		"externalNetworkID":    *p.cluster.Spec.Openstack.ExternalNetworkID,
	}

	if p.cluster.Spec.Openstack.SSHKeyName != nil {
		openstackValues["sshKeyName"] = *p.cluster.Spec.Openstack.SSHKeyName
	}

	// TODO: generate types from the Helm values schema.
	values := map[string]interface{}{
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

func (p *Provisioner) ReleaseName() string {
	return p.cluster.Name
}

func (p *Provisioner) getProvisioner() provisioners.Provisioner {
	return application.New(p.client, applicationName, p.cluster, p.application).WithGenerator(p).OnRemote(p.remote).InNamespace(p.namespace)
}

// Provision implements the Provision interface.
func (p *Provisioner) Provision(ctx context.Context) error {
	// TODO: this application is special in that everything it creates is a
	// CRD, so as far as Argo is concerned, everything is healthy and the
	// check passes instantly, rather than waiting for the CAPI controllers
	// to do something.  We kinda fudge it due to the concurrent deployment
	// of the CNI and cloud controller add-ons blocking.
	if err := p.getProvisioner().Provision(ctx); err != nil {
		return err
	}

	if err := p.deleteOrphanedMachineDeployments(ctx); err != nil {
		return err
	}

	return nil
}

// Deprovision implements the Provision interface.
func (p *Provisioner) Deprovision(ctx context.Context) error {
	if err := p.getProvisioner().Deprovision(ctx); err != nil {
		return err
	}

	return nil
}
