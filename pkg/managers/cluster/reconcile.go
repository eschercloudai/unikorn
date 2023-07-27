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

package cluster

import (
	"context"
	"errors"
	"fmt"
	"time"

	unikornv1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/constants"
	provisionererrors "github.com/eschercloudai/unikorn/pkg/provisioners/errors"
	"github.com/eschercloudai/unikorn/pkg/provisioners/managers/cluster"

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
	object := &unikornv1.KubernetesCluster{}
	if err := r.client.Get(ctx, request.NamespacedName, object); err != nil {
		if kerrors.IsNotFound(err) {
			log.Info("resource deleted")

			return reconcile.Result{}, nil
		}

		return reconcile.Result{}, err
	}

	provisioner, err := cluster.New(ctx, r.client, object)
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

	log.Info("reconciling resource")

	// Check to see if this is (or appears to be) the first time we've seen a
	// resource and do observability as appropriate.
	if err := r.addFinalizer(ctx, object); err != nil {
		return reconcile.Result{}, err
	}

	// Create a new context with a status object attached, we'll use this later to
	// conditionally report provisioning status, and a timeout so we don't hang
	// forever in retry loops.
	provisionContext, cancel := context.WithTimeout(ctx, object.Spec.Timeout.Duration)
	defer cancel()

	perr := provisioner.Provision(provisionContext)

	// Update the status conditionally, this will remove transient errors etc.
	if err := r.handleReconcileCondition(ctx, object, perr); err != nil {
		return reconcile.Result{}, err
	}

	// If anything went wrong, requeue for another attempt.
	// NOTE: DO NOT do what CAPI do and not-specify a wait period, it will
	// suffer from an exponential back-off and kill performance.
	if perr != nil {
		//nolint:nilerr
		return reconcile.Result{RequeueAfter: constants.DefaultYieldTimeout}, nil
	}

	return reconcile.Result{}, nil
}

// addFinalizer looks to see if we've seen this resource yet, and adds a finalizer so
// we can orchestrate deletion correctly.
func (r *reconciler) addFinalizer(ctx context.Context, resource *unikornv1.KubernetesCluster) error {
	for _, finalizer := range resource.Finalizers {
		if finalizer == constants.Finalizer {
			return nil
		}
	}

	resource.Finalizers = append(resource.Finalizers, constants.Finalizer)

	if err := r.client.Update(ctx, resource); err != nil {
		return err
	}

	return nil
}

// handleReconcileDeprovision indicates the deprovision request has been picked up.
func (r *reconciler) handleReconcileDeprovision(ctx context.Context, kubernetesCluster *unikornv1.KubernetesCluster) error {
	if ok := kubernetesCluster.UpdateAvailableCondition(corev1.ConditionFalse, unikornv1.KubernetesClusterConditionReasonDeprovisioning, "Kubernetes cluster is being deprovisioned"); ok {
		if err := r.client.Status().Update(ctx, kubernetesCluster); err != nil {
			return err
		}
	}

	return nil
}

// handleReconcileCondition inspects the error, if any, that halted the provisioning and reports
// this as a ppropriate in the status.
func (r *reconciler) handleReconcileCondition(ctx context.Context, kubernetesCluster *unikornv1.KubernetesCluster, err error) error {
	var status corev1.ConditionStatus

	var reason unikornv1.KubernetesClusterConditionReason

	var message string

	switch {
	case err == nil:
		status = corev1.ConditionTrue
		reason = unikornv1.KubernetesClusterConditionReasonProvisioned
		message = "Provisioned"
	case errors.Is(err, provisionererrors.ErrYield):
		status = corev1.ConditionFalse
		reason = unikornv1.KubernetesClusterConditionReasonProvisioning
		message = "Provisioning"
	case errors.Is(err, context.Canceled):
		status = corev1.ConditionFalse
		reason = unikornv1.KubernetesClusterConditionReasonCanceled
		message = "Aborted due to controller shudown"
	case errors.Is(err, context.DeadlineExceeded):
		status = corev1.ConditionFalse
		reason = unikornv1.KubernetesClusterConditionReasonTimedout
		message = fmt.Sprintf("Aborted due to a timeout: %v", err)
	default:
		status = corev1.ConditionFalse
		reason = unikornv1.KubernetesClusterConditionReasonErrored
		message = fmt.Sprintf("Failed due to an error: %v", err)
	}

	if ok := kubernetesCluster.UpdateAvailableCondition(status, reason, message); ok {
		if err := r.client.Status().Update(ctx, kubernetesCluster); err != nil {
			return err
		}
	}

	return nil
}
