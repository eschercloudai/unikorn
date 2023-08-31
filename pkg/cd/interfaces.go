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

package cd

//go:generate mockgen -source=interfaces.go -destination=mock/interfaces.go -package=mock

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Driver is an abstraction around CD tools such as ArgoCD
// or Flux, this is a low level driver interface that configures things
// like remote clusters and Helm applications.
type Driver interface {
	// Client gives you access to the Kubernetes client for when your CD driver
	// is incapable of working as desired and you need to take manual action.
	// Think long and hard about whether you need this, it's a hack quite frankly.
	Client() client.Client

	// CreateOrUpdateHelmApplication creates or updates a helm application idempotently.
	CreateOrUpdateHelmApplication(ctx context.Context, id *ResourceIdentifier, app *HelmApplication) error

	// DeleteHelmApplication deletes an existing helm application.
	DeleteHelmApplication(ctx context.Context, id *ResourceIdentifier, backgroundDelete bool) error

	// CreateOrUpdateCluster creates or updates a cluster idempotently.
	CreateOrUpdateCluster(ctx context.Context, id *ResourceIdentifier, cluster *Cluster) error

	// DeleteCluster deletes an existing cluster.
	DeleteCluster(ctx context.Context, id *ResourceIdentifier) error
}
