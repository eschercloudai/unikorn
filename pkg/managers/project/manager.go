/*
Copyright 2022-2024 EscherCloud.

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
	unikornscheme "github.com/eschercloudai/unikorn/generated/clientset/unikorn/scheme"
	unikornv1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/provisioners/managers/project"

	coreclient "github.com/eschercloudai/unikorn-core/pkg/client"
	"github.com/eschercloudai/unikorn-core/pkg/constants"
	coremanager "github.com/eschercloudai/unikorn-core/pkg/manager"
	"github.com/eschercloudai/unikorn-core/pkg/manager/options"

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

var _ coremanager.ControllerFactory = &Factory{}

// Metadata returns the application, version and revision.
func (*Factory) Metadata() (string, string, string) {
	return constants.Application, constants.Version, constants.Revision
}

// Reconciler returns a new reconciler instance.
func (*Factory) Reconciler(options *options.Options, manager manager.Manager) reconcile.Reconciler {
	return coremanager.NewReconciler(options, manager.GetClient(), project.New)
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
func (*Factory) Upgrade(_ client.Client) error {
	return nil
}

// Schemes allows controllers to add types to the client beyond
// the defaults defined in this repository.
func (*Factory) Schemes() []coreclient.SchemeAdder {
	return []coreclient.SchemeAdder{
		unikornscheme.AddToScheme,
	}
}
