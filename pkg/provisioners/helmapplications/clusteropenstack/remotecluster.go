/*
Copyright 2022-2024 EscherCloud.

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
	"crypto/sha256"
	"fmt"

	unikornv1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/constants"

	"github.com/eschercloudai/unikorn-core/pkg/cd"
	coreclient "github.com/eschercloudai/unikorn-core/pkg/client"
	"github.com/eschercloudai/unikorn-core/pkg/provisioners"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// releaseName generates a unique helm-compliant release name from the cluster's
// name.
func releaseName(cluster *unikornv1.KubernetesCluster) string {
	// This must be no longer than 53 characters and unique across all control
	// planes to avoid OpenStack network aliasing.
	sum := sha256.Sum256([]byte(cluster.Labels[constants.ControlPlaneLabel] + ":" + cluster.Name))

	hash := fmt.Sprintf("%x", sum)

	return "cluster-" + hash[:8]
}

// CAPIClusterName generates the cluster name that will be generated by helm for
// the given cluster's name.  This is referred to by the cluster autoscaler for
// example.
func CAPIClusterName(cluster *unikornv1.KubernetesCluster) string {
	return releaseName(cluster)
}

// KubeconfigSecretName generates the kubeconfig secret name that will be generated
// by CAPI for the given cluster's name.
func KubeconfigSecretName(cluster *unikornv1.KubernetesCluster) string {
	return releaseName(cluster) + "-kubeconfig"
}

type RemoteCluster struct {
	// cluster is the cluster we are referring to.
	cluster *unikornv1.KubernetesCluster
}

// Ensure this implements the remotecluster.Generator interface.
var _ provisioners.RemoteCluster = &RemoteCluster{}

// NewRemoteCluster return a new instance of a remote cluster generator.
func NewRemoteCluster(cluster *unikornv1.KubernetesCluster) *RemoteCluster {
	return &RemoteCluster{
		cluster: cluster,
	}
}

// ID implements the remotecluster.Generator interface.
func (g *RemoteCluster) ID() *cd.ResourceIdentifier {
	// You must call ResourceLabels() rather than access them directly
	// as this will add in the cluster name label from the resource name.
	// TODO: error checking.
	resourceLabels, _ := g.cluster.ResourceLabels()

	var labels []cd.ResourceIdentifierLabel

	for _, label := range constants.LabelPriorities() {
		if value, ok := resourceLabels[label]; ok {
			labels = append(labels, cd.ResourceIdentifierLabel{
				Name:  label,
				Value: value,
			})
		}
	}

	return &cd.ResourceIdentifier{
		Name:   "kubernetes",
		Labels: labels,
	}
}

// Config implements the remotecluster.Generator interface.
func (g *RemoteCluster) Config(ctx context.Context) (*clientcmdapi.Config, error) {
	log := log.FromContext(ctx)

	secret := &corev1.Secret{}

	secretKey := client.ObjectKey{
		Namespace: g.cluster.Name,
		Name:      KubeconfigSecretName(g.cluster),
	}

	// Retry getting the secret until it exists.
	if err := coreclient.DynamicClientFromContext(ctx).Get(ctx, secretKey, secret); err != nil {
		if errors.IsNotFound(err) {
			log.Info("kubernetes cluster kubeconfig does not exist, yielding")

			return nil, provisioners.ErrYield
		}

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
