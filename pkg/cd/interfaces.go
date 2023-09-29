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

	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// Driver is an abstraction around CD tools such as ArgoCD
// or Flux, this is a low level driver interface that configures things
// like remote clusters and Helm applications.
type Driver interface {
	// Kind allows provisioners to make decisions based on the driver
	// in use e.g. if the CD is broken in some way and needs manual
	// intervention.  Use of this is discouraged, and pull requests will
	// be rejected if there's no evidence of an upstream fix to remove
	// your hack.
	Kind() DriverKind

	// CreateOrUpdateHelmApplication creates or updates a helm application idempotently.
	CreateOrUpdateHelmApplication(ctx context.Context, id *ResourceIdentifier, app *HelmApplication) error

	// DeleteHelmApplication deletes an existing helm application.
	DeleteHelmApplication(ctx context.Context, id *ResourceIdentifier, backgroundDelete bool) error

	// CreateOrUpdateCluster creates or updates a cluster idempotently.
	CreateOrUpdateCluster(ctx context.Context, id *ResourceIdentifier, cluster *Cluster) error

	// DeleteCluster deletes an existing cluster.
	DeleteCluster(ctx context.Context, id *ResourceIdentifier) error
}

// DriverRunnable provides access to the driver from the reconcilers.
// Due to how controller-runtime works we want to register the driver
// with the reconciler, but that may require the use of a client, and
// that's not running until after the reconciler has been registered
// and Start() is called.  But, what we can do is register a Runnable
// with the manager that will get invoked after cache syncing, and
// before any reconciler is called.
type DriverRunnable interface {
	// Must implment manager.Runnable for late initialisation.
	manager.Runnable
	manager.LeaderElectionRunnable

	// Driver allows access to the driver from the reconciler.
	Driver() Driver
}
