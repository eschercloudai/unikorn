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

	"golang.org/x/mod/semver"

	unikornv1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/constants"
	"github.com/eschercloudai/unikorn/pkg/managers/common"
	"github.com/eschercloudai/unikorn/pkg/provisioners/util"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// Factory provides methods that can build a type specific controller.
type Factory struct{}

var _ common.ControllerFactory = &Factory{}

// Reconciler returns a new reconciler instance.
func (*Factory) Reconciler(manager manager.Manager) reconcile.Reconciler {
	return &reconciler{
		client: manager.GetClient(),
	}
}

// RegisterWatches adds any watches that would trigger a reconcile.
func (*Factory) RegisterWatches(manager manager.Manager, controller controller.Controller) error {
	if err := controller.Watch(source.Kind(manager.GetCache(), &unikornv1.Project{}), &handler.EnqueueRequestForObject{}, &predicate.GenerationChangedPredicate{}); err != nil {
		return err
	}

	return nil
}

// Upgrade can perform metadata upgrades of all versioned resources on restart/upgrade
// of the controller.  This must not affect the spec in any way as it causes split brain
// and potential fail.
func (*Factory) Upgrade(c client.Client) error {
	ctx := context.TODO()

	resources := &unikornv1.ProjectList{}

	if err := c.List(ctx, resources, &client.ListOptions{}); err != nil {
		return err
	}

	for i := range resources.Items {
		if err := upgrade(ctx, c, &resources.Items[i]); err != nil {
			return err
		}
	}

	return nil
}

// semverLess returns true if a is less than b.
func semverLess(a, b string) bool {
	// Note we use un-prefixed tags, the library expects different.
	return semver.Compare("v"+a, "v"+b) < 0
}

func upgrade(ctx context.Context, c client.Client, resource *unikornv1.Project) error {
	version, ok := resource.Labels[constants.VersionLabel]
	if !ok {
		return unikornv1.ErrMissingLabel
	}

	// Skip development versions.  This may lead to people unwittingly using old
	// resources that don't match the requirements of a newer version, but it's
	// better than trying to upgrade to a newer version accidentally when it's
	// already at that version, and legacy resource selection won't work at all.
	if version == "0.0.0" {
		return nil
	}

	newResource := resource.DeepCopy()

	// In 0.3.27, the underlying namespace for a project was augmented with the
	// "unikorn.eschercloud.ai/kind" label to avoid aliasing with control plane
	// namespaces as "unikorn.eschercloud.ai/project" was added to them to provide
	// scoping.
	if semverLess(version, "0.3.27") {
		namespace, err := util.GetResourceNamespaceLegacy(ctx, c, constants.ProjectLabel, resource.Name)
		if err != nil {
			return err
		}

		namespace.Labels[constants.KindLabel] = constants.KindLabelValueProject

		if err := c.Update(ctx, namespace, &client.UpdateOptions{}); err != nil {
			return err
		}

		newResource.Labels[constants.VersionLabel] = "0.3.27"
	}

	if err := c.Patch(ctx, newResource, client.MergeFrom(resource)); err != nil {
		return err
	}

	return nil
}
