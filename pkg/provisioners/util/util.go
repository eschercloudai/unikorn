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

package util

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"

	clientlib "github.com/eschercloudai/unikorn/pkg/client"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	ErrNamespaceLookup = errors.New("unable to lookup namespace")
)

func GetResourceNamespace(ctx context.Context, l labels.Set) (*corev1.Namespace, error) {
	c := clientlib.StaticClientFromContext(ctx)

	namespaces := &corev1.NamespaceList{}
	if err := c.List(ctx, namespaces, &client.ListOptions{LabelSelector: l.AsSelector()}); err != nil {
		return nil, err
	}

	if len(namespaces.Items) != 1 {
		return nil, fmt.Errorf("%w: labels %v", ErrNamespaceLookup, l)
	}

	return &namespaces.Items[0], nil
}

// GetConfigurationHash is used to restart badly behaved apps that don't respect configuration
// changes.
func GetConfigurationHash(config any) (string, error) {
	configJSON, err := json.Marshal(config)
	if err != nil {
		return "", err
	}

	configHash := sha256.Sum256(configJSON)

	return fmt.Sprintf("%x", configHash), nil
}
