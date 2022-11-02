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

package controlplane

import (
	"context"
	"errors"
	"fmt"

	unikornv1alpha1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/util/provisioners/controlplane"

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

func (r *reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := log.FromContext(ctx)

	// See if the resource exists or not, if not it's been deleted, but do nothing
	// as cascading deletes will handle the cleanup.
	controlPlane := &unikornv1alpha1.ControlPlane{}
	if err := r.client.Get(ctx, request.NamespacedName, controlPlane); err != nil {
		if kerrors.IsNotFound(err) {
			log.Info("resource deleted")

			return reconcile.Result{}, nil
		}

		return reconcile.Result{}, err
	}

	// If it's being deleted, ignore it, we don't need to take any additional action.
	if controlPlane.DeletionTimestamp != nil {
		log.V(1).Info("resource deleting")

		return reconcile.Result{}, nil
	}

	log.V(1).Info("reconciling resource")

	// Check to see if this is (or appears to be) the first time we've seen a
	// resource and do observability as appropriate.
	if err := r.handleReconcileFirstVisit(ctx, controlPlane); err != nil {
		return reconcile.Result{}, err
	}

	// Create a new context with a status object attached, we'll use this later to
	// conditionally report provisioning status, and a timeout so we don't hang
	// forever in retry loops.
	provisionContext, cancel := context.WithTimeout(ctx, controlPlane.Spec.Timeout.Duration)
	defer cancel()

	// Provision the control plane.
	if err := controlplane.New(r.client, controlPlane).Provision(provisionContext); err != nil {
		if err := r.handleReconcileError(ctx, controlPlane, err); err != nil {
			return reconcile.Result{}, err
		}

		return reconcile.Result{}, err
	}

	if err := r.handleReconcileComplete(ctx, controlPlane); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

// handleReconcileFirstVisit checks to see if the Available condition is present in the
// status, if not we assume it's the first time we've seen this an set the condition to
// Provisioning.
func (r *reconciler) handleReconcileFirstVisit(ctx context.Context, controlPlane *unikornv1alpha1.ControlPlane) error {
	if _, err := controlPlane.LookupCondition(unikornv1alpha1.ControlPlaneConditionAvailable); err != nil {
		controlPlane.UpdateAvailableCondition(corev1.ConditionFalse, unikornv1alpha1.ControlPlaneConditionReasonProvisioning, "Provisioning of control plane has started")

		if err := r.client.Status().Update(ctx, controlPlane); err != nil {
			return err
		}
	}

	return nil
}

// handleReconcileComplete indicates that the reconcile is complete and the control
// plane is ready to be used.
func (r *reconciler) handleReconcileComplete(ctx context.Context, controlPlane *unikornv1alpha1.ControlPlane) error {
	if ok := controlPlane.UpdateAvailableCondition(corev1.ConditionTrue, unikornv1alpha1.ControlPlaneConditionReasonProvisioned, "Provisioning of control plane has completed"); ok {
		if err := r.client.Status().Update(ctx, controlPlane); err != nil {
			return err
		}
	}

	return nil
}

// handleReconcileError inspects the error type that halted the provisioning and reports
// this as a ppropriate in the status.
func (r *reconciler) handleReconcileError(ctx context.Context, controlPlane *unikornv1alpha1.ControlPlane, err error) error {
	var reason unikornv1alpha1.ControlPlaneConditionReason

	var message string

	switch {
	case errors.Is(err, context.Canceled):
		reason = unikornv1alpha1.ControlPlaneConditionReasonCanceled
		message = "Provisioning aborted due to controller shudown"
	case errors.Is(err, context.DeadlineExceeded):
		reason = unikornv1alpha1.ControlPlaneConditionReasonTimedout
		message = fmt.Sprintf("Provisioning aborted due to a timeout: %v", err)
	default:
		reason = unikornv1alpha1.ControlPlaneConditionReasonErrored
		message = fmt.Sprintf("Provisioning failed due to an error: %v", err)
	}

	if ok := controlPlane.UpdateAvailableCondition(corev1.ConditionFalse, reason, message); ok {
		if err := r.client.Status().Update(ctx, controlPlane); err != nil {
			return err
		}
	}

	return nil
}
