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

package client

import (
	"context"
	"fmt"

	argocdapi "github.com/eschercloudai/argocd-client-go/pkg/api"
	argocdclient "github.com/eschercloudai/argocd-client-go/pkg/client"

	"github.com/eschercloudai/unikorn/pkg/constants"

	corev1 "k8s.io/api/core/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewInClusterOptions tries to derive the in-cluster ArgoCD options.
func NewInClusterOptions(ctx context.Context, c client.Client, namespace string) (*argocdclient.Options, error) {
	adminSecret := &corev1.Secret{}
	if err := c.Get(ctx, client.ObjectKey{Name: "argocd-initial-admin-secret", Namespace: namespace}, adminSecret); err != nil {
		return nil, err
	}

	serverSecret := &corev1.Secret{}
	if err := c.Get(ctx, client.ObjectKey{Name: "argocd-secret", Namespace: namespace}, serverSecret); err != nil {
		return nil, err
	}

	options := &argocdclient.Options{
		Host:      fmt.Sprintf("argocd-server.%s:443", namespace),
		Username:  "admin",
		Password:  string(adminSecret.Data["password"]),
		CA:        serverSecret.Data["tls.crt"],
		UserAgent: constants.VersionString(),
	}

	return options, nil
}

// NewInCluster creates a new in-cluster client.
func NewInCluster(ctx context.Context, c client.Client, namespace string) (*argocdapi.APIClient, error) {
	options, err := NewInClusterOptions(ctx, c, namespace)
	if err != nil {
		return nil, err
	}

	argocd, err := argocdclient.New(ctx, options)
	if err != nil {
		return nil, err
	}

	return argocd, nil
}
