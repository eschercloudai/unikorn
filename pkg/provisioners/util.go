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

package provisioners

import (
	unikornv1alpha1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/constants"
)

// VClusterRemoteLabelsFromControlPlane extracts a unique set of labels from the
// control plane for a remote vcluster.
func VClusterRemoteLabelsFromControlPlane(controlPlane *unikornv1alpha1.ControlPlane) []string {
	return []string{
		controlPlane.Name,
		controlPlane.Labels[constants.ProjectLabel],
	}
}

// VClusterRemoteLabelsFromCluster extracts a unique set of labels from the
// cluster for a remote vcluster.
func VclusterRemoteLabelsFromCluster(cluster *unikornv1alpha1.KubernetesCluster) []string {
	return []string{
		cluster.Labels[constants.ControlPlaneLabel],
		cluster.Labels[constants.ProjectLabel],
	}
}

// ClusterOpenstackLabelsFromCluster extracts a unique set of labels from the cluster
// for a remote kubernetes cluster.
func ClusterOpenstackLabelsFromCluster(cluster *unikornv1alpha1.KubernetesCluster) []string {
	return []string{
		cluster.Labels[constants.ControlPlaneLabel],
		cluster.Labels[constants.ProjectLabel],
	}
}
