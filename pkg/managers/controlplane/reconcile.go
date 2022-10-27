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
	"fmt"

	unikornv1alpha1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/util/provisioners/controlplane"
	provisioner "github.com/eschercloudai/unikorn/pkg/util/provisioners/generic"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
		if errors.IsNotFound(err) {
			log.Info("resource deleted")

			return reconcile.Result{}, nil
		}

		return reconcile.Result{}, err
	}

	log.V(1).Info("reconciling resource")

	// Assume here that no conditions means we've not seen it yet and it's
	// unprovisioned.
	// NOTE: like all status updates, this will bump the version of the resource
	// and cause another reconcile to be triggered unnecessarily.  It will also
	// spam the API, which isn't playing fairly, thus any updates should be
	// conditional.
	if len(controlPlane.Status.Conditions) == 0 {
		controlPlane.Status.Conditions = []unikornv1alpha1.ControlPlaneCondition{
			{
				Type:               unikornv1alpha1.ControlPlaneConditionProvisioned,
				Status:             corev1.ConditionFalse,
				LastTransitionTime: metav1.Now(),
				Reason:             "Provisioning",
				Message:            "Provisioning of control plane has started",
			},
		}

		if err := r.client.Status().Update(ctx, controlPlane); err != nil {
			return reconcile.Result{}, err
		}
	}

	// Create a new context with a status object attached, we'll use this later to
	// conditionally report provisioning status.
	var status *provisioner.ProvisionerStatus

	ctx, status = provisioner.WithStatus(ctx)

	// Provision the control plane.
	// NOTE: this should be idempotent, again, play fair, don't spam the API
	// unnecessarily.
	provisioner := controlplane.New(r.client, controlPlane)

	if err := provisioner.Provision(ctx); err != nil {
		controlPlane.Status.Conditions = []unikornv1alpha1.ControlPlaneCondition{
			{
				Type:               unikornv1alpha1.ControlPlaneConditionProvisioned,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: metav1.Now(),
				Reason:             "Errored",
				Message:            fmt.Sprintf("Provisioning of control plane has failed: %v", err),
			},
		}

		return reconcile.Result{}, err
	}

	// If and only if something has happened as flagged by the underlying provisioner
	// then update the status.
	// TODO: it's entirely possible something ephemeral happened previously and the
	// control plane is reporting an error state, at which point when the blip clears
	// up, we won't actually perform the update back into provisioned...
	if status.Provisioned {
		controlPlane.Status.Conditions = []unikornv1alpha1.ControlPlaneCondition{
			{
				Type:               unikornv1alpha1.ControlPlaneConditionProvisioned,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: metav1.Now(),
				Reason:             "Provisioned",
				Message:            "Provisioning of control plane has completed",
			},
		}

		if err := r.client.Status().Update(ctx, controlPlane); err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}
