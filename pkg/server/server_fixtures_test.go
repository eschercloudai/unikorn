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

package server_test

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"

	unikornv1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/constants"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// mustCreateProjectFixture creates a project, and its randomly named namespace
// just as if unikorn-project-manager had picked up the create request, preformed it
// and also updated the status.
//
//nolint:unparam
func mustCreateProjectFixture(t *testing.T, tc *TestContext, projectID string) *unikornv1.Project {
	t.Helper()

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "project-",
			Labels: map[string]string{
				constants.ProjectLabel: projectNameFromID(projectID),
			},
		},
	}

	assert.NoError(t, tc.KubernetesClient().Create(context.TODO(), namespace))

	project := &unikornv1.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name: projectNameFromID(projectID),
		},
		Status: unikornv1.ProjectStatus{
			Namespace: namespace.Name,
			Conditions: []unikornv1.Condition{
				{
					Type:   unikornv1.ConditionAvailable,
					Status: corev1.ConditionTrue,
					Reason: unikornv1.ConditionReasonProvisioned,
				},
			},
		},
	}

	assert.NoError(t, tc.KubernetesClient().Create(context.TODO(), project))

	return project
}

// mustCreateControlPlaneFixture creates a control plane , and its randomly named namespace
// just as if unikorn-controlplane-manager had picked up the create request, preformed it
// and also updated the status.
//
//nolint:unparam
func mustCreateControlPlaneFixture(t *testing.T, tc *TestContext, namespace, name string) *unikornv1.ControlPlane {
	t.Helper()

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "controlplane-",
			Labels: map[string]string{
				constants.ProjectLabel:      projectNameFromID(projectID),
				constants.ControlPlaneLabel: name,
			},
		},
	}

	assert.NoError(t, tc.KubernetesClient().Create(context.TODO(), ns))

	bundleVersion := controlPlaneApplicationBundleName

	controlPlane := &unikornv1.ControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: unikornv1.ControlPlaneSpec{
			ApplicationBundle: &bundleVersion,
		},
		Status: unikornv1.ControlPlaneStatus{
			Namespace: ns.Name,
			Conditions: []unikornv1.Condition{
				{
					Type:   unikornv1.ConditionAvailable,
					Status: corev1.ConditionTrue,
					Reason: unikornv1.ConditionReasonProvisioned,
				},
			},
		},
	}

	assert.NoError(t, tc.KubernetesClient().Create(context.TODO(), controlPlane))

	return controlPlane
}

const (
	clusterComputeFailureDomain = "danger_nova"
	clusterStorageFailureDomain = "ceph"
	clusterSSHKeyName           = "chubb"
	clusterExternalNetworkID    = "e0c16797-e5db-4ee4-8305-16a72ded2b7e"
	clusterNodeNetwork          = "1.0.0.0/24"
	clusterServiceNetwork       = "2.0.0.0/24"
	clusterPodNetwork           = "3.0.0.0/24"
	clusterDNSNameserver        = "8.8.8.8"
	clusterControlPlaneReplicas = 3
	clusterWorkloadPoolName     = "you-got-to-work-hard"
	clusterWorkloadPoolReplicas = 10
)

// mustCreateKubernetesClusterFixture creates a basic cluster resource in Kubernetes.
//
//nolint:unparam
func mustCreateKubernetesClusterFixture(t *testing.T, tc *TestContext, namespace, name string) {
	t.Helper()

	bundleName := kubernetesClusterApplicationBundleName
	computeFailureDomain := clusterComputeFailureDomain
	storageFailureDomain := clusterStorageFailureDomain
	sshKeyName := clusterSSHKeyName
	externalNetworkID := clusterExternalNetworkID
	controlPlaneReplicas := clusterControlPlaneReplicas
	controlPlaneImageName := imageName
	controlPlaneKubernetesVersion := unikornv1.SemanticVersion("v" + imageK8sVersion)
	controlPlaneFlavor := flavorName
	workloadPoolReplicas := clusterWorkloadPoolReplicas
	workloadPoolImageName := imageName
	workloadPoolKubernetesVersion := unikornv1.SemanticVersion("v" + imageK8sVersion)
	workloadPoolFlavor := flavorName

	_, nodenetwork, err := net.ParseCIDR(clusterNodeNetwork)
	assert.NoError(t, err)
	_, serviceNetwork, err := net.ParseCIDR(clusterServiceNetwork)
	assert.NoError(t, err)
	_, podNetwork, err := net.ParseCIDR(clusterPodNetwork)
	assert.NoError(t, err)

	dnsNameserver := net.ParseIP(clusterDNSNameserver)

	cluster := &unikornv1.KubernetesCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: unikornv1.KubernetesClusterSpec{
			ApplicationBundle: &bundleName,
			Openstack: &unikornv1.KubernetesClusterOpenstackSpec{
				FailureDomain:       &computeFailureDomain,
				VolumeFailureDomain: &storageFailureDomain,
				SSHKeyName:          &sshKeyName,
				ExternalNetworkID:   &externalNetworkID,
			},
			Network: &unikornv1.KubernetesClusterNetworkSpec{
				NodeNetwork:    &unikornv1.IPv4Prefix{IPNet: *nodenetwork},
				ServiceNetwork: &unikornv1.IPv4Prefix{IPNet: *serviceNetwork},
				PodNetwork:     &unikornv1.IPv4Prefix{IPNet: *podNetwork},
				DNSNameservers: []unikornv1.IPv4Address{
					{IP: dnsNameserver},
				},
			},
			ControlPlane: &unikornv1.KubernetesClusterControlPlaneSpec{
				MachineGeneric: unikornv1.MachineGeneric{
					Replicas: &controlPlaneReplicas,
					Version:  &controlPlaneKubernetesVersion,
					Image:    &controlPlaneImageName,
					Flavor:   &controlPlaneFlavor,
				},
			},
			WorkloadPools: &unikornv1.KubernetesClusterWorkloadPoolsSpec{
				Pools: []unikornv1.KubernetesClusterWorkloadPoolsPoolSpec{
					{
						KubernetesWorkloadPoolSpec: unikornv1.KubernetesWorkloadPoolSpec{
							Name: clusterWorkloadPoolName,
							MachineGeneric: unikornv1.MachineGeneric{
								Replicas: &workloadPoolReplicas,
								Version:  &workloadPoolKubernetesVersion,
								Image:    &workloadPoolImageName,
								Flavor:   &workloadPoolFlavor,
							},
						},
					},
				},
			},
		},
		Status: unikornv1.KubernetesClusterStatus{
			Conditions: []unikornv1.Condition{
				{
					Type:   unikornv1.ConditionAvailable,
					Status: corev1.ConditionTrue,
					Reason: unikornv1.ConditionReasonProvisioned,
				},
			},
		},
	}

	assert.NoError(t, tc.KubernetesClient().Create(context.TODO(), cluster))
}

const (
	controlPlaneApplicationBundleName    = "control-plane-1.0.0"
	controlPlaneApplicationBundleVersion = "1.0.0"
)

// mustCreateControlPlaneApplicationBundleFixture creates a basic application bundle
// for a control plane.
func mustCreateControlPlaneApplicationBundleFixture(t *testing.T, tc *TestContext) {
	t.Helper()

	kind := unikornv1.ApplicationBundleResourceKindControlPlane
	version := controlPlaneApplicationBundleVersion

	bundle := &unikornv1.ApplicationBundle{
		ObjectMeta: metav1.ObjectMeta{
			Name: controlPlaneApplicationBundleName,
		},
		Spec: unikornv1.ApplicationBundleSpec{
			Kind:    &kind,
			Version: &version,
		},
	}

	assert.NoError(t, tc.KubernetesClient().Create(context.TODO(), bundle))
}

const (
	kubernetesClusterApplicationBundleName    = "kubernetes-cluster-1.0.0"
	kubernetesClusterApplicationBundleVersion = "2.0.0"
)

// mustKubernetesClusterApplicationBundleFixture creates a basic application bundle
// for a Kubernetes cluster.
func mustKubernetesClusterApplicationBundleFixture(t *testing.T, tc *TestContext) {
	t.Helper()

	kind := unikornv1.ApplicationBundleResourceKindKubernetesCluster
	version := kubernetesClusterApplicationBundleVersion

	bundle := &unikornv1.ApplicationBundle{
		ObjectMeta: metav1.ObjectMeta{
			Name: kubernetesClusterApplicationBundleName,
		},
		Spec: unikornv1.ApplicationBundleSpec{
			Kind:    &kind,
			Version: &version,
		},
	}

	assert.NoError(t, tc.KubernetesClient().Create(context.TODO(), bundle))
}
