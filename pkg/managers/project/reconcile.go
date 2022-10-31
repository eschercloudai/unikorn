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

package project

import (
	"context"

	unikornv1alpha1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/constants"
	"github.com/eschercloudai/unikorn/pkg/util"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"

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
	project := &unikornv1alpha1.Project{}
	if err := r.client.Get(ctx, request.NamespacedName, project); err != nil {
		if errors.IsNotFound(err) {
			log.Info("resource deleted")

			return reconcile.Result{}, nil
		}

		return reconcile.Result{}, err
	}

	log.Info("reconciling resource")

	// See if the project namespace exists and if it doesn't, then create it.
	// We label the namespace with the project name in order to filter the results
	// and find a match.
	projectLabelRequirement, err := labels.NewRequirement(constants.ControlPlaneLabel, selection.Equals, []string{request.NamespacedName.Name})
	if err != nil {
		return reconcile.Result{}, err
	}

	selector := labels.Everything().Add(*projectLabelRequirement)

	namespaces := &corev1.NamespaceList{}
	if err := r.client.List(ctx, namespaces, &client.ListOptions{LabelSelector: selector}); err != nil {
		return reconcile.Result{}, err
	}

	if len(namespaces.Items) != 0 {
		// TODO: unlikely, but somehow we may have more than one!
		// TODO: it's also possible the resource status has been "lost", e.g. due to
		// velero backup, so we should resync.
		return reconcile.Result{}, nil
	}

	log.Info("project namespace does not exist")

	project.Status.Conditions = []unikornv1alpha1.ProjectCondition{
		{
			Type:               unikornv1alpha1.ProjectConditionProvisioned,
			Status:             corev1.ConditionFalse,
			LastTransitionTime: metav1.Now(),
			Reason:             "Provisioning",
			Message:            "Provisioning of project has started",
		},
	}

	if err := r.client.Status().Update(ctx, project); err != nil {
		return reconcile.Result{}, err
	}

	gvk, err := util.ObjectGroupVersionKind(r.client.Scheme(), project)
	if err != nil {
		return reconcile.Result{}, err
	}

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "project-",
			Labels: map[string]string{
				constants.ControlPlaneLabel: request.NamespacedName.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(project, *gvk),
			},
		},
	}

	if err := r.client.Create(ctx, namespace); err != nil {
		return reconcile.Result{}, err
	}

	project.Status.Namespace = namespace.Name
	project.Status.Conditions = []unikornv1alpha1.ProjectCondition{
		{
			Type:               unikornv1alpha1.ProjectConditionProvisioned,
			Status:             corev1.ConditionFalse,
			LastTransitionTime: metav1.Now(),
			Reason:             "Provisioned",
			Message:            "Provisioning of project has completed",
		},
	}

	if err := r.client.Status().Update(ctx, project); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}
