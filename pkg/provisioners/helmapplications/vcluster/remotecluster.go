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

package vcluster

import (
	"context"

	"github.com/eschercloudai/unikorn/pkg/cd"
	"github.com/eschercloudai/unikorn/pkg/constants"
	"github.com/eschercloudai/unikorn/pkg/provisioners"

	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type RemoteCluster struct {
	// client provides access to the Kubernetes instance where
	// the vcluster resources live.
	client client.Client

	// namespace tells us where the vcluster lives.
	namespace string

	// labels are used to form a unique and context specific name for
	// the remote cluster instance.
	labels []string
}

// Ensure this implements the remotecluster.Generator interface.
var _ provisioners.RemoteCluster = &RemoteCluster{}

// NewRemoteCluster return a new instance of a remote cluster generator.
func NewRemoteCluster(client client.Client, namespace string, labels []string) *RemoteCluster {
	return &RemoteCluster{
		client:    client,
		namespace: namespace,
		labels:    labels,
	}
}

// ID implements the remotecluster.Generator interface.
func (g *RemoteCluster) ID() *cd.ResourceIdentifier {
	// TODO: the labels handling is a bit smelly,
	return &cd.ResourceIdentifier{
		Name: "vcluster",
		Labels: []cd.ResourceIdentifierLabel{
			{
				Name:  constants.ControlPlaneLabel,
				Value: g.labels[0],
			},
			{
				Name:  constants.ProjectLabel,
				Value: g.labels[1],
			},
		},
	}
}

// Config implements the remotecluster.Generator interface.
func (g *RemoteCluster) Config(ctx context.Context) (*clientcmdapi.Config, error) {
	return NewControllerRuntimeClient(g.client).ClientConfig(ctx, g.namespace, false)
}
