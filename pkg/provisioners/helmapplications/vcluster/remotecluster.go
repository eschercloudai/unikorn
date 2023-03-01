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
	"fmt"

	"github.com/eschercloudai/unikorn/pkg/provisioners"

	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type RemoteClusterGenerator struct {
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
var _ provisioners.RemoteCluster = &RemoteClusterGenerator{}

// NewRemoteClusterGenerator return a new instance of a remote cluster generator.
func NewRemoteClusterGenerator(client client.Client, namespace string, labels []string) *RemoteClusterGenerator {
	return &RemoteClusterGenerator{
		client:    client,
		namespace: namespace,
		labels:    labels,
	}
}

// Name implements the remotecluster.Generator interface.
func (g *RemoteClusterGenerator) Name() string {
	return "vcluster"
}

// Labels mplements the remotecluster.Generator interface.
func (g *RemoteClusterGenerator) Labels() []string {
	// The instance name is implicit, for now.
	labels := []string{"vcluster"}

	labels = append(labels, g.labels...)

	return labels
}

// Server implements the remotecluster.Generator interface.
func (g *RemoteClusterGenerator) Server(_ context.Context) (string, error) {
	return fmt.Sprintf("https://vcluster.%s", g.namespace), nil
}

// Config implements the remotecluster.Generator interface.
func (g *RemoteClusterGenerator) Config(ctx context.Context) (*clientcmdapi.Config, error) {
	return NewControllerRuntimeClient(g.client).ClientConfig(ctx, g.namespace, false)
}
