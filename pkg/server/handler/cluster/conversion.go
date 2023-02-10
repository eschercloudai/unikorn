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
	"fmt"
	"net"

	"github.com/gophercloud/utils/openstack/clientconfig"

	unikornv1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/constants"
	"github.com/eschercloudai/unikorn/pkg/server/errors"
	"github.com/eschercloudai/unikorn/pkg/server/generated"
	"github.com/eschercloudai/unikorn/pkg/server/handler/controlplane"
	"github.com/eschercloudai/unikorn/pkg/util"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/yaml"
)

// convertOpenstack converts from a custom resource into the API definition.
func convertOpenstack(in *unikornv1.KubernetesCluster) generated.KubernetesClusterOpenstack {
	openstack := generated.KubernetesClusterOpenstack{
		ComputeAvailabilityZone: *in.Spec.Openstack.FailureDomain,
		VolumeAvailabilityZone:  *in.Spec.Openstack.VolumeFailureDomain,
		ExternalNetworkID:       *in.Spec.Openstack.ExternalNetworkID,
		SshKeyName:              in.Spec.Openstack.SSHKeyName,
	}

	return openstack
}

// convertNetwork converts from a custom resource into the API definition.
func convertNetwork(in *unikornv1.KubernetesCluster) generated.KubernetesClusterNetwork {
	dnsNameservers := make([]string, len(in.Spec.Network.DNSNameservers))

	for i, address := range in.Spec.Network.DNSNameservers {
		dnsNameservers[i] = address.IP.String()
	}

	network := generated.KubernetesClusterNetwork{
		NodePrefix:     in.Spec.Network.NodeNetwork.IPNet.String(),
		ServicePrefix:  in.Spec.Network.ServiceNetwork.IPNet.String(),
		PodPrefix:      in.Spec.Network.PodNetwork.IPNet.String(),
		DnsNameservers: dnsNameservers,
	}

	return network
}

// convertAPI converts from a custom resource into the API definition.
func convertAPI(in *unikornv1.KubernetesCluster) *generated.KubernetesClusterAPI {
	if in.Spec.API == nil {
		return nil
	}

	api := &generated.KubernetesClusterAPI{}

	if len(in.Spec.API.SubjectAlternativeNames) > 0 {
		api.SubjectAlternativeNames = &in.Spec.API.SubjectAlternativeNames
	}

	if len(in.Spec.API.AllowedPrefixes) > 0 {
		allowedPrefixes := make([]string, len(in.Spec.API.AllowedPrefixes))

		for i, prefix := range in.Spec.API.AllowedPrefixes {
			allowedPrefixes[i] = prefix.IPNet.String()
		}

		api.AllowedPrefixes = &allowedPrefixes
	}

	return api
}

// convertMachine converts from a custom resource into the API definition.
func convertMachine(in *unikornv1.MachineGeneric) generated.OpenstackMachinePool {
	machine := generated.OpenstackMachinePool{
		Replicas:   *in.Replicas,
		Version:    string(*in.Version),
		ImageName:  *in.Image,
		FlavorName: *in.Flavor,
	}

	if in.DiskSize != nil {
		machine.Disk = &generated.OpenstackVolume{
			Size:             int(in.DiskSize.Value()) >> 30,
			AvailabilityZone: in.VolumeFailureDomain,
		}
	}

	return machine
}

// convertWorkloadPool converts from a custom resource into the API definition.
func convertWorkloadPool(in *unikornv1.KubernetesClusterWorkloadPoolsPoolSpec) generated.KubernetesClusterWorkloadPool {
	workloadPool := generated.KubernetesClusterWorkloadPool{
		Name:    in.Name,
		Machine: convertMachine(&in.KubernetesWorkloadPoolSpec.MachineGeneric),
	}

	if in.KubernetesWorkloadPoolSpec.Labels != nil {
		workloadPool.Labels = &in.KubernetesWorkloadPoolSpec.Labels
	}

	if in.KubernetesWorkloadPoolSpec.Autoscaling != nil {
		workloadPool.Autoscaling = &generated.KubernetesClusterAutoscaling{
			MinimumReplicas: *in.KubernetesWorkloadPoolSpec.Autoscaling.MinimumReplicas,
			MaximumReplicas: *in.KubernetesWorkloadPoolSpec.Autoscaling.MaximumReplicas,
		}

		if in.KubernetesWorkloadPoolSpec.Autoscaling.Scheduler != nil {
			workloadPool.Autoscaling.Scheduler = &generated.KubernetesClusterAutoscalingScheduler{
				Cpus:   *in.KubernetesWorkloadPoolSpec.Autoscaling.Scheduler.CPU,
				Memory: int(in.KubernetesWorkloadPoolSpec.Autoscaling.Scheduler.Memory.Value()) >> 30,
			}

			if in.KubernetesWorkloadPoolSpec.Autoscaling.Scheduler.GPU != nil {
				workloadPool.Autoscaling.Scheduler.Gpu = &generated.Gpu{
					Type:  *in.KubernetesWorkloadPoolSpec.Autoscaling.Scheduler.GPU.Type,
					Count: *in.KubernetesWorkloadPoolSpec.Autoscaling.Scheduler.GPU.Count,
				}
			}
		}
	}

	return workloadPool
}

// convertWorkloadPools converts from a custom resource into the API definition.
func convertWorkloadPools(in *unikornv1.KubernetesCluster) []generated.KubernetesClusterWorkloadPool {
	workloadPools := make([]generated.KubernetesClusterWorkloadPool, len(in.Spec.WorkloadPools.Pools))

	for i := range in.Spec.WorkloadPools.Pools {
		workloadPools[i] = convertWorkloadPool(&in.Spec.WorkloadPools.Pools[i])
	}

	return workloadPools
}

// convertFeatures converts from a custom resource into the API definition.
func convertFeatures(in *unikornv1.KubernetesCluster) *generated.KubernetesClusterFeatures {
	if in.Spec.Features == nil {
		return nil
	}

	features := &generated.KubernetesClusterFeatures{
		Autoscaling: in.Spec.Features.Autoscaling,
	}

	return features
}

// convertStatus converts from a custom resource into the API definition.
func convertStatus(in *unikornv1.KubernetesCluster) *generated.KubernetesResourceStatus {
	out := &generated.KubernetesResourceStatus{
		Name:         in.Name,
		CreationTime: in.CreationTimestamp.Time,
		Status:       "Unknown",
	}

	if in.DeletionTimestamp != nil {
		out.DeletionTime = &in.DeletionTimestamp.Time
	}

	if condition, err := in.LookupCondition(unikornv1.KubernetesClusterConditionAvailable); err == nil {
		out.Status = string(condition.Reason)
	}

	return out
}

// convert converts from a custom resource into the API definition.
func convert(in *unikornv1.KubernetesCluster) *generated.KubernetesCluster {
	out := &generated.KubernetesCluster{
		Name:          in.Name,
		Openstack:     convertOpenstack(in),
		Network:       convertNetwork(in),
		Api:           convertAPI(in),
		ControlPlane:  convertMachine(&in.Spec.ControlPlane.MachineGeneric),
		WorkloadPools: convertWorkloadPools(in),
		Features:      convertFeatures(in),
		Status:        convertStatus(in),
	}

	return out
}

// uconvertList converts from a custom resource list into the API definition.
func convertList(in *unikornv1.KubernetesClusterList) []*generated.KubernetesCluster {
	out := make([]*generated.KubernetesCluster, len(in.Items))

	for i := range in.Items {
		out[i] = convert(&in.Items[i])
	}

	return out
}

// createClientConfig creates an Openstack client configuration from the API.
func (c *Client) createClientConfig(options *generated.KubernetesCluster) ([]byte, string, error) {
	cloud := "cloud"

	clientConfig := &clientconfig.Clouds{
		Clouds: map[string]clientconfig.Cloud{
			cloud: {
				AuthType: clientconfig.AuthV3ApplicationCredential,
				AuthInfo: &clientconfig.AuthInfo{
					AuthURL:                     c.endpoint,
					ApplicationCredentialID:     *options.Openstack.ApplicationCredentialID,
					ApplicationCredentialSecret: *options.Openstack.ApplicationCredentialSecret,
				},
			},
		},
	}

	clientConfigYAML, err := yaml.Marshal(clientConfig)
	if err != nil {
		return nil, "", errors.OAuth2ServerError("unable to create cloud config").WithError(err)
	}

	return clientConfigYAML, cloud, nil
}

// createOpenstack creates the Openstack configuration part of a cluster.
func (c *Client) createOpenstack(options *generated.KubernetesCluster) (*unikornv1.KubernetesClusterOpenstackSpec, error) {
	ca, err := util.GetURLCACertificate(c.endpoint)
	if err != nil {
		return nil, errors.OAuth2ServerError("unable to get endpoint CA certificate").WithError(err)
	}

	clientConfig, cloud, err := c.createClientConfig(options)
	if err != nil {
		return nil, err
	}

	openstack := &unikornv1.KubernetesClusterOpenstackSpec{
		CACert:              &ca,
		CloudConfig:         &clientConfig,
		Cloud:               &cloud,
		FailureDomain:       &options.Openstack.ComputeAvailabilityZone,
		VolumeFailureDomain: &options.Openstack.VolumeAvailabilityZone,
		ExternalNetworkID:   &options.Openstack.ExternalNetworkID,
	}

	if options.Openstack.SshKeyName != nil {
		openstack.SSHKeyName = options.Openstack.SshKeyName
	}

	return openstack, nil
}

// createNetwork creates the network part of a cluster.
func createNetwork(options *generated.KubernetesCluster) (*unikornv1.KubernetesClusterNetworkSpec, error) {
	_, nodeNet, err := net.ParseCIDR(options.Network.NodePrefix)
	if err != nil {
		return nil, errors.OAuth2InvalidRequest("failed to parse node prefix").WithError(err)
	}

	_, serviceNet, err := net.ParseCIDR(options.Network.ServicePrefix)
	if err != nil {
		return nil, errors.OAuth2InvalidRequest("failed to parse service prefix").WithError(err)
	}

	_, podNet, err := net.ParseCIDR(options.Network.PodPrefix)
	if err != nil {
		return nil, errors.OAuth2InvalidRequest("failed to parse pod prefix").WithError(err)
	}

	dnsNameservers := make([]net.IP, len(options.Network.DnsNameservers))

	for i, server := range options.Network.DnsNameservers {
		ip := net.ParseIP(server)
		if ip == nil {
			return nil, errors.OAuth2InvalidRequest("failed to parse dns server IP")
		}

		dnsNameservers[i] = ip
	}

	network := &unikornv1.KubernetesClusterNetworkSpec{
		NodeNetwork:    &unikornv1.IPv4Prefix{IPNet: *nodeNet},
		ServiceNetwork: &unikornv1.IPv4Prefix{IPNet: *serviceNet},
		PodNetwork:     &unikornv1.IPv4Prefix{IPNet: *podNet},
		DNSNameservers: unikornv1.IPv4AddressSliceFromIPSlice(dnsNameservers),
	}

	return network, nil
}

// createAPI creates the Kuebernetes API part of the cluster.
func createAPI(options *generated.KubernetesCluster) (*unikornv1.KubernetesClusterAPISpec, error) {
	if options.Api == nil {
		//nolint:nilnil
		return nil, nil
	}

	api := &unikornv1.KubernetesClusterAPISpec{}

	if options.Api.SubjectAlternativeNames != nil {
		api.SubjectAlternativeNames = *options.Api.SubjectAlternativeNames
	}

	if options.Api.AllowedPrefixes != nil {
		prefixes := make([]unikornv1.IPv4Prefix, len(*options.Api.AllowedPrefixes))

		for i, prefix := range *options.Api.AllowedPrefixes {
			_, network, err := net.ParseCIDR(prefix)
			if err != nil {
				return nil, errors.OAuth2InvalidRequest("failed to parse api allowed prefix").WithError(err)
			}

			prefixes[i] = unikornv1.IPv4Prefix{IPNet: *network}
		}

		api.AllowedPrefixes = prefixes
	}

	return api, nil
}

// createMachineGeneric creates a generic machine part of the cluster.
func createMachineGeneric(m *generated.OpenstackMachinePool) (*unikornv1.MachineGeneric, error) {
	version := unikornv1.SemanticVersion(m.Version)

	machine := &unikornv1.MachineGeneric{
		Version:  &version,
		Replicas: &m.Replicas,
		Image:    &m.ImageName,
		Flavor:   &m.FlavorName,
	}

	if m.Disk != nil {
		size, err := resource.ParseQuantity(fmt.Sprintf("%dGi", m.Disk.Size))
		if err != nil {
			return nil, errors.OAuth2InvalidRequest("failed to parse disk size").WithError(err)
		}

		machine.DiskSize = &size

		if m.Disk.AvailabilityZone != nil {
			machine.VolumeFailureDomain = m.Disk.AvailabilityZone
		}
	}

	return machine, nil
}

// createControlPlane creates the control plane part of a cluster.
func createControlPlane(options *generated.KubernetesCluster) (*unikornv1.KubernetesClusterControlPlaneSpec, error) {
	machine, err := createMachineGeneric(&options.ControlPlane)
	if err != nil {
		return nil, err
	}

	controlPlane := &unikornv1.KubernetesClusterControlPlaneSpec{
		MachineGeneric: *machine,
	}

	return controlPlane, nil
}

// createWorkloadPools creates the workload pools part of a cluster.
func createWorkloadPools(options *generated.KubernetesCluster) (*unikornv1.KubernetesClusterWorkloadPoolsSpec, error) {
	workloadPools := &unikornv1.KubernetesClusterWorkloadPoolsSpec{}

	for i := range options.WorkloadPools {
		pool := &options.WorkloadPools[i]

		machine, err := createMachineGeneric(&pool.Machine)
		if err != nil {
			return nil, err
		}

		workloadPool := unikornv1.KubernetesClusterWorkloadPoolsPoolSpec{
			Name: pool.Name,
			KubernetesWorkloadPoolSpec: unikornv1.KubernetesWorkloadPoolSpec{
				MachineGeneric: *machine,
				FailureDomain:  pool.AvailabilityZone,
			},
		}

		if pool.Labels != nil {
			workloadPool.Labels = *pool.Labels
		}

		//nolint:nestif
		if pool.Autoscaling != nil {
			autoscaling := &unikornv1.MachineGenericAutoscaling{
				MinimumReplicas: &pool.Autoscaling.MinimumReplicas,
				MaximumReplicas: &pool.Autoscaling.MaximumReplicas,
			}

			if pool.Autoscaling.Scheduler != nil {
				memory, err := resource.ParseQuantity(fmt.Sprintf("%dGi", pool.Autoscaling.Scheduler.Memory))
				if err != nil {
					return nil, errors.OAuth2InvalidRequest("failed to parse workload pool memory hint").WithError(err)
				}

				autoscaling.Scheduler = &unikornv1.MachineGenericAutoscalingScheduler{
					CPU:    &pool.Autoscaling.Scheduler.Cpus,
					Memory: &memory,
				}

				if pool.Autoscaling.Scheduler.Gpu != nil {
					t := constants.NvidiaGPUType

					autoscaling.Scheduler.GPU = &unikornv1.MachineGenericAutoscalingSchedulerGPU{
						Type:  &t,
						Count: &pool.Autoscaling.Scheduler.Gpu.Count,
					}
				}
			}
		}

		workloadPools.Pools = append(workloadPools.Pools, workloadPool)
	}

	return workloadPools, nil
}

// createFeatures creates the features part of a cluster.
func createFeatures(options *generated.KubernetesCluster) *unikornv1.KubernetesClusterFeaturesSpec {
	if options.Features == nil {
		return nil
	}

	features := &unikornv1.KubernetesClusterFeaturesSpec{
		Autoscaling: options.Features.Autoscaling,
	}

	return features
}

// createCluster creates the full cluster custom resource.
func (c *Client) createCluster(controlPlane *controlplane.Meta, options *generated.KubernetesCluster) (*unikornv1.KubernetesCluster, error) {
	openstack, err := c.createOpenstack(options)
	if err != nil {
		return nil, err
	}

	network, err := createNetwork(options)
	if err != nil {
		return nil, err
	}

	api, err := createAPI(options)
	if err != nil {
		return nil, err
	}

	kubernetesCcontrolPlane, err := createControlPlane(options)
	if err != nil {
		return nil, err
	}

	kubernetesWorkloadPools, err := createWorkloadPools(options)
	if err != nil {
		return nil, err
	}

	cluster := &unikornv1.KubernetesCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      options.Name,
			Namespace: controlPlane.Namespace,
			Labels: map[string]string{
				constants.VersionLabel:      constants.Version,
				constants.ProjectLabel:      controlPlane.Project.Name,
				constants.ControlPlaneLabel: controlPlane.Name,
			},
		},
		Spec: unikornv1.KubernetesClusterSpec{
			Openstack:     openstack,
			Network:       network,
			API:           api,
			ControlPlane:  kubernetesCcontrolPlane,
			WorkloadPools: kubernetesWorkloadPools,
			Features:      createFeatures(options),
		},
	}

	return cluster, nil
}
