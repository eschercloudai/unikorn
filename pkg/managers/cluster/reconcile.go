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
	"errors"
	"fmt"
	"time"

	unikornv1alpha1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/constants"
	"github.com/eschercloudai/unikorn/pkg/provisioners/cluster"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type reconciler struct {
	client client.Client
}

var _ reconcile.Reconciler = &reconciler{}

//nolint:cyclop
func (r *reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := log.FromContext(ctx)

	// See if the resource exists or not, if not it's been deleted, but do nothing
	// as cascading deletes will handle the cleanup.
	object := &unikornv1alpha1.KubernetesCluster{}
	if err := r.client.Get(ctx, request.NamespacedName, object); err != nil {
		if kerrors.IsNotFound(err) {
			log.Info("resource deleted")

			return reconcile.Result{}, nil
		}

		return reconcile.Result{}, err
	}

	provisioner := cluster.New(r.client, object)

	// If it's being deleted, ignore it, we don't need to take any additional action.
	if object.DeletionTimestamp != nil {
		if len(object.Finalizers) == 0 {
			return reconcile.Result{}, nil
		}

		log.Info("resource deleting")

		// TODO: we need to add a status condition to say we are deleting.
		// And obviously report any errors of course.
		timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()

		if err := provisioner.Deprovision(timeoutCtx); err != nil {
			return reconcile.Result{}, err
		}

		object.Finalizers = nil

		if err := r.client.Update(ctx, object); err != nil {
			return reconcile.Result{}, err
		}

		return reconcile.Result{}, nil
	}

	log.Info("reconciling resource")

	// Check to see if this is (or appears to be) the first time we've seen a
	// resource and do observability as appropriate.
	if err := r.handleReconcileFirstVisit(ctx, object); err != nil {
		return reconcile.Result{}, err
	}

	// Create a new context with a status object attached, we'll use this later to
	// conditionally report provisioning status, and a timeout so we don't hang
	// forever in retry loops.
	provisionContext, cancel := context.WithTimeout(ctx, object.Spec.Timeout.Duration)
	defer cancel()

	if err := provisioner.Provision(provisionContext); err != nil {
		if err := r.handleReconcileError(ctx, object, err); err != nil {
			return reconcile.Result{}, err
		}

		return reconcile.Result{}, err
	}

	if err := r.handleReconcileComplete(ctx, object); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

// handleReconcileFirstVisit checks to see if the Available condition is present in the
// status, if not we assume it's the first time we've seen this an set the condition to
// Provisioning.
func (r *reconciler) handleReconcileFirstVisit(ctx context.Context, kubernetesCluster *unikornv1alpha1.KubernetesCluster) error {
	if _, err := kubernetesCluster.LookupCondition(unikornv1alpha1.KubernetesClusterConditionAvailable); err != nil {
		kubernetesCluster.Finalizers = []string{
			constants.Finalizer,
		}

		kubernetesCluster.UpdateAvailableCondition(corev1.ConditionFalse, unikornv1alpha1.KubernetesClusterConditionReasonProvisioning, "Provisioning of kubernetes cluster has started")

		if err := r.client.Update(ctx, kubernetesCluster); err != nil {
			return err
		}
	}

	return nil
}

// handleReconcileComplete indicates that the reconcile is complete and the control
// plane is ready to be used.
func (r *reconciler) handleReconcileComplete(ctx context.Context, kubernetesCluster *unikornv1alpha1.KubernetesCluster) error {
	if ok := kubernetesCluster.UpdateAvailableCondition(corev1.ConditionTrue, unikornv1alpha1.KubernetesClusterConditionReasonProvisioned, "Provisioning of kubernetes cluster has completed"); ok {
		if err := r.client.Status().Update(ctx, kubernetesCluster); err != nil {
			return err
		}
	}

	return nil
}

// handleReconcileError inspects the error type that halted the provisioning and reports
// this as a ppropriate in the status.
func (r *reconciler) handleReconcileError(ctx context.Context, kubernetesCluster *unikornv1alpha1.KubernetesCluster, err error) error {
	var reason unikornv1alpha1.KubernetesClusterConditionReason

	var message string

	switch {
	case errors.Is(err, context.Canceled):
		reason = unikornv1alpha1.KubernetesClusterConditionReasonCanceled
		message = "Provisioning aborted due to controller shudown"
	case errors.Is(err, context.DeadlineExceeded):
		reason = unikornv1alpha1.KubernetesClusterConditionReasonTimedout
		message = fmt.Sprintf("Provisioning aborted due to a timeout: %v", err)
	default:
		reason = unikornv1alpha1.KubernetesClusterConditionReasonErrored
		message = fmt.Sprintf("Provisioning failed due to an error: %v", err)
	}

	if ok := kubernetesCluster.UpdateAvailableCondition(corev1.ConditionFalse, reason, message); ok {
		if err := r.client.Status().Update(ctx, kubernetesCluster); err != nil {
			return err
		}
	}

	return nil
}
