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
	"context"
	"encoding/base64"
	"errors"
	"net/url"

	argocdapi "github.com/eschercloudai/argocd-client-go/pkg/api"

	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

var (
	ErrContextUndefined = errors.New("kubeconfig context undefined")
)

// Upsert is an idempotent method to create a cluster in ArgoCD from a client config.
// This is limited to X.509 mutual authentication at present.
func Upsert(ctx context.Context, client *argocdapi.APIClient, name string, config *clientcmdapi.Config) error {
	clientContext, ok := config.Contexts[config.CurrentContext]
	if !ok {
		return ErrContextUndefined
	}

	tlsClientConfig := argocdapi.NewV1alpha1TLSClientConfig()
	tlsClientConfig.SetCaData(base64.StdEncoding.EncodeToString(config.Clusters[clientContext.Cluster].CertificateAuthorityData))
	tlsClientConfig.SetCertData(base64.StdEncoding.EncodeToString(config.AuthInfos[clientContext.AuthInfo].ClientCertificateData))
	tlsClientConfig.SetKeyData(base64.StdEncoding.EncodeToString(config.AuthInfos[clientContext.AuthInfo].ClientKeyData))

	clusterConfig := argocdapi.NewV1alpha1ClusterConfig()
	clusterConfig.SetTlsClientConfig(*tlsClientConfig)

	cluster := argocdapi.NewV1alpha1Cluster()
	cluster.SetServer(name)
	cluster.SetConfig(*clusterConfig)

	if _, _, err := client.ClusterServiceApi.ClusterServiceCreate(ctx).Body(*cluster).Upsert(true).Execute(); err != nil {
		return err
	}

	return nil
}

// Delete is an idempotent method to delete a cluster from ArgoCD.
func Delete(ctx context.Context, client *argocdapi.APIClient, name string) error {
	// Do a list here, not a get, Argo's schema is bobbins and doesn't handle 404s
	// as described.
	clusters, _, err := client.ClusterServiceApi.ClusterServiceList(ctx).Execute()
	if err != nil {
		return err
	}

	for _, cluster := range clusters.Items {
		if *cluster.Server != name {
			continue
		}

		if _, _, err := client.ClusterServiceApi.ClusterServiceDelete(ctx, url.QueryEscape(name)).Execute(); err != nil {
			return err
		}

		break
	}

	return nil
}
