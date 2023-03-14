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

package controlplane

import (
	"context"
	"errors"
	"fmt"
	"time"

	unikornv1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/constants"
	provisionererrors "github.com/eschercloudai/unikorn/pkg/provisioners/errors"
	"github.com/eschercloudai/unikorn/pkg/provisioners/managers/controlplane"

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
	object := &unikornv1.ControlPlane{}
	if err := r.client.Get(ctx, request.NamespacedName, object); err != nil {
		if kerrors.IsNotFound(err) {
			log.Info("resource deleted")

			return reconcile.Result{}, nil
		}

		return reconcile.Result{}, err
	}

	provisioner, err := controlplane.New(ctx, r.client, object)
	if err != nil {
		return reconcile.Result{}, err
	}

	// If it's being deleted, ignore it, we don't need to take any additional action.
	if object.DeletionTimestamp != nil {
		if len(object.Finalizers) == 0 {
			return reconcile.Result{}, nil
		}

		log.Info("resource deleting")

		if err := r.handleReconcileDeprovision(ctx, object); err != nil {
			return reconcile.Result{}, err
		}

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

	log.V(1).Info("reconciling resource")

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

	// Provision the control plane.
	if err := provisioner.Provision(provisionContext); err != nil {
		// If the provisioner has voluntarily yielded, requeue it and look at
		// it later to allow others to use the worker, or indeed pickup delete
		// requests, updates... probably not a great idea :D
		// NOTE: DO NOT do what CAPI do and not-specify a wait period, it will
		// suffer from an exponential back-off and kill performance.
		if errors.Is(err, provisionererrors.ErrYield) {
			log.Info("reconcile yielding")

			return reconcile.Result{RequeueAfter: constants.DefaultYieldTimeout}, nil
		}

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
func (r *reconciler) handleReconcileFirstVisit(ctx context.Context, controlPlane *unikornv1.ControlPlane) error {
	condition, err := controlPlane.LookupCondition(unikornv1.ControlPlaneConditionAvailable)
	if err != nil {
		controlPlane.Finalizers = []string{
			constants.Finalizer,
		}

		if err := r.client.Update(ctx, controlPlane); err != nil {
			return err
		}

		controlPlane.UpdateAvailableCondition(corev1.ConditionFalse, unikornv1.ControlPlaneConditionReasonProvisioning, "Provisioning control plane")

		if err := r.client.Status().Update(ctx, controlPlane); err != nil {
			return err
		}

		return nil
	}

	if condition.Reason == unikornv1.ControlPlaneConditionReasonProvisioned {
		controlPlane.UpdateAvailableCondition(corev1.ConditionTrue, unikornv1.ControlPlaneConditionReasonUpdating, "Updating control plane")

		if err := r.client.Status().Update(ctx, controlPlane); err != nil {
			return err
		}

		return nil
	}

	return nil
}

// handleReconcileDeprovision indicates the deprovision request has been picked up.
func (r *reconciler) handleReconcileDeprovision(ctx context.Context, controlPlane *unikornv1.ControlPlane) error {
	if ok := controlPlane.UpdateAvailableCondition(corev1.ConditionFalse, unikornv1.ControlPlaneConditionReasonDeprovisioning, "Control plane is being deprovisioned"); ok {
		if err := r.client.Status().Update(ctx, controlPlane); err != nil {
			return err
		}
	}

	return nil
}

// handleReconcileComplete indicates that the reconcile is complete and the control
// plane is ready to be used.
func (r *reconciler) handleReconcileComplete(ctx context.Context, controlPlane *unikornv1.ControlPlane) error {
	if ok := controlPlane.UpdateAvailableCondition(corev1.ConditionTrue, unikornv1.ControlPlaneConditionReasonProvisioned, "Provisioning of control plane has completed"); ok {
		if err := r.client.Status().Update(ctx, controlPlane); err != nil {
			return err
		}
	}

	return nil
}

// handleReconcileError inspects the error type that halted the provisioning and reports
// this as a ppropriate in the status.
func (r *reconciler) handleReconcileError(ctx context.Context, controlPlane *unikornv1.ControlPlane, err error) error {
	var reason unikornv1.ControlPlaneConditionReason

	var message string

	switch {
	case errors.Is(err, context.Canceled):
		reason = unikornv1.ControlPlaneConditionReasonCanceled
		message = "Provisioning aborted due to controller shudown"
	case errors.Is(err, context.DeadlineExceeded):
		reason = unikornv1.ControlPlaneConditionReasonTimedout
		message = fmt.Sprintf("Provisioning aborted due to a timeout: %v", err)
	default:
		reason = unikornv1.ControlPlaneConditionReasonErrored
		message = fmt.Sprintf("Provisioning failed due to an error: %v", err)
	}

	if ok := controlPlane.UpdateAvailableCondition(corev1.ConditionFalse, reason, message); ok {
		if err := r.client.Status().Update(ctx, controlPlane); err != nil {
			return err
		}
	}

	return nil
}
