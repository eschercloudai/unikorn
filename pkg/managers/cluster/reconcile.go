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

	unikornv1alpha1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/provisioners/cluster"

	"k8s.io/apimachinery/pkg/api/errors"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type reconciler struct {
	client client.Client
}

var _ reconcile.Reconciler = &reconciler{}

func (r *reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := log.FromContext(ctx)

	// See if the resource exists or not, if not it's been deleted, but do nothing
	// as cascading deletes will handle the cleanup.
	kubernetesCluster := &unikornv1alpha1.KubernetesCluster{}
	if err := r.client.Get(ctx, request.NamespacedName, kubernetesCluster); err != nil {
		if errors.IsNotFound(err) {
			log.Info("resource deleted")

			return reconcile.Result{}, nil
		}

		return reconcile.Result{}, err
	}

	// If it's being deleted, ignore it, we don't need to take any additional action.
	if kubernetesCluster.DeletionTimestamp != nil {
		log.V(1).Info("resource deleting")

		return reconcile.Result{}, nil
	}

	log.Info("reconciling resource")

	// Create a new context with a status object attached, we'll use this later to
	// conditionally report provisioning status, and a timeout so we don't hang
	// forever in retry loops.
	provisionContext, cancel := context.WithTimeout(ctx, kubernetesCluster.Spec.Timeout.Duration)
	defer cancel()

	provisioner := cluster.New(r.client, kubernetesCluster)

	if err := provisioner.Provision(provisionContext); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}
