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
	"fmt"
	"strings"

	"github.com/eschercloudai/unikorn/pkg/provisioners/remotecluster"
	"github.com/eschercloudai/unikorn/pkg/util/retry"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type RemoteClusterGenerator struct {
	// client provides access to the Kubernetes instance where
	// the cluster API resources live.
	client client.Client

	// namespace tells us where the cluster lives.
	namespace string

	// name is the name of the cluster.
	name string

	// labels are used to form a unique and context specific name for
	// the remote cluster instance.
	labels []string
}

// Ensure this implements the remotecluster.Generator interface.
var _ remotecluster.Generator = &RemoteClusterGenerator{}

// NewRemoteClusterGenerator return a new instance of a remote cluster generator.
func NewRemoteClusterGenerator(client client.Client, namespace, name string, labels []string) *RemoteClusterGenerator {
	return &RemoteClusterGenerator{
		client:    client,
		namespace: namespace,
		name:      name,
		labels:    labels,
	}
}

// Name implements the remotecluster.Generator interface.
func (g *RemoteClusterGenerator) Name() string {
	name := fmt.Sprintf("kubernetes-%s", g.name)

	if len(g.labels) != 0 {
		name += ":" + strings.Join(g.labels, ":")
	}

	return name
}

// Server implements the remotecluster.Generator interface.
func (g *RemoteClusterGenerator) Server(ctx context.Context) (string, error) {
	config, err := g.Config(ctx)
	if err != nil {
		return "", err
	}

	return config.Clusters[config.Contexts[config.CurrentContext].Cluster].Server, nil
}

// Config implements the remotecluster.Generator interface.
func (g *RemoteClusterGenerator) Config(ctx context.Context) (*clientcmdapi.Config, error) {
	secret := &corev1.Secret{}

	secretKey := client.ObjectKey{
		Namespace: g.name,
		Name:      g.name + "-kubeconfig",
	}

	// Retry getting the secret until it exists.
	getSecret := func() error {
		return g.client.Get(ctx, secretKey, secret)
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
