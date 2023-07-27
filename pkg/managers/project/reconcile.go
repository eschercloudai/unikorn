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

package project

import (
	"context"
	"errors"
	"fmt"
	"time"

	unikornv1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/constants"
	"github.com/eschercloudai/unikorn/pkg/provisioners"
	"github.com/eschercloudai/unikorn/pkg/provisioners/managers/project"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type reconciler struct {
	client client.Client
}

var _ reconcile.Reconciler = &reconciler{}

func (r *reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := log.FromContext(ctx)

	// See if the resource exists or not, if not it's been deleted.
	object := &unikornv1.Project{}
	if err := r.client.Get(ctx, request.NamespacedName, object); err != nil {
		if kerrors.IsNotFound(err) {
			log.Info("resource deleted")

			return reconcile.Result{}, nil
		}

		return reconcile.Result{}, err
	}

	provisioner := project.New(r.client, object)

	// If it's being deleted, ignore if there are no finalizers, Kubernetes is in
	// charge now.  If the finalizer is still in place, run the deprovisioning.
	if object.DeletionTimestamp != nil {
		if len(object.Finalizers) == 0 {
			return reconcile.Result{}, nil
		}

		log.Info("deleting resource")

		return r.reconcileDelete(ctx, provisioner, object)
	}

	log.Info("reconciling resource")

	// Check to see if this is (or appears to be) the first time we've seen a
	// resource and do observability as appropriate.
	if ok := controllerutil.AddFinalizer(object, constants.Finalizer); ok {
		if err := r.client.Update(ctx, object); err != nil {
			return reconcile.Result{}, err
		}
	}

	perr := provisioner.Provision(ctx)

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

// reconcileDelete handles resource deletion.
func (r *reconciler) reconcileDelete(ctx context.Context, provisioner provisioners.Provisioner, resource *unikornv1.Project) (reconcile.Result, error) {
	if ok := resource.UpdateAvailableCondition(corev1.ConditionFalse, unikornv1.ProjectConditionReasonDeprovisioning, "Deprovisioning"); ok {
		if err := r.client.Status().Update(ctx, resource); err != nil {
			return reconcile.Result{}, err
		}
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	if err := provisioner.Deprovision(timeoutCtx); err != nil {
		return reconcile.Result{}, err
	}

	if ok := controllerutil.RemoveFinalizer(resource, constants.Finalizer); ok {
		if err := r.client.Update(ctx, resource); err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

// handleReconcileCondition inspects the error, if any, that halted the provisioning and reports
// this as a ppropriate in the status.
func (r *reconciler) handleReconcileCondition(ctx context.Context, project *unikornv1.Project, err error) error {
	var status corev1.ConditionStatus

	var reason unikornv1.ProjectConditionReason

	var message string

	switch {
	case err == nil:
		status = corev1.ConditionTrue
		reason = unikornv1.ProjectConditionReasonProvisioned
		message = "Provisioned"
	case errors.Is(err, context.Canceled):
		status = corev1.ConditionFalse
		reason = unikornv1.ProjectConditionReasonCanceled
		message = "Provisioning aborted due to controller shutdown"
	case errors.Is(err, context.DeadlineExceeded):
		status = corev1.ConditionFalse
		reason = unikornv1.ProjectConditionReasonTimedout
		message = fmt.Sprintf("Provisioning aborted due to a timeout: %v", err)
	default:
		status = corev1.ConditionFalse
		reason = unikornv1.ProjectConditionReasonErrored
		message = fmt.Sprintf("Provisioning failed due to an error: %v", err)
	}

	if ok := project.UpdateAvailableCondition(status, reason, message); ok {
		if err := r.client.Status().Update(ctx, project); err != nil {
			return err
		}
	}

	return nil
}
