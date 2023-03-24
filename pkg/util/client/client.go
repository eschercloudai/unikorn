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

package client

import (
	"context"

	unikornscheme "github.com/eschercloudai/unikorn/generated/clientset/unikorn/scheme"

	"k8s.io/apimachinery/pkg/runtime"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"

	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// New returns a new controller runtime caching client, initialized with core and
// unikorn resources for typed operation.
func New(ctx context.Context) (client.Client, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	// Create a scheme and ensure it knows about Kubernetes and Unikorn
	// resource types.
	scheme := runtime.NewScheme()

	if err := kubernetesscheme.AddToScheme(scheme); err != nil {
		return nil, err
	}

	if err := unikornscheme.AddToScheme(scheme); err != nil {
		return nil, err
	}

	cache, err := cache.New(config, cache.Options{Scheme: scheme})
	if err != nil {
		return nil, err
	}

	go func() {
		_ = cache.Start(ctx)
	}()

	clientOptions := client.Options{
		Scheme: scheme,
	}

	c, err := client.New(config, clientOptions)
	if err != nil {
		return nil, err
	}

	input := client.NewDelegatingClientInput{
		CacheReader: cache,
		Client:      c,
	}

	return client.NewDelegatingClient(input)
}
