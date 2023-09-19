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

	unikornv1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/managers/common"
	"github.com/eschercloudai/unikorn/pkg/provisioners/managers/cluster"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// eventHandlerOwnerFromLabel extracts a parent resource from a resource label and
// enqueues a reconconcile request for it.  Useful when subordinate objects are
// gathered via a label selector.
type eventHandlerOwnerFromLabel struct {
	// label is the label to look for.
	label string
}

// Ensure the handler.EventHandler interface is implemented.
var _ handler.EventHandler = &eventHandlerOwnerFromLabel{}

// Create is called in response to an create event - e.g. Pod Creation.
func (e *eventHandlerOwnerFromLabel) Create(ctx context.Context, evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	e.enqueue(evt.Object, q)
}

// Update is called in response to an update event -  e.g. Pod Updated.
func (e *eventHandlerOwnerFromLabel) Update(ctx context.Context, evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	e.enqueue(evt.ObjectNew, q)
	e.enqueue(evt.ObjectOld, q)
}

// Delete is called in response to a delete event - e.g. Pod Deleted.
func (e *eventHandlerOwnerFromLabel) Delete(ctx context.Context, evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	e.enqueue(evt.Object, q)
}

// Generic is called in response to an event of an unknown type or a synthetic event triggered as a cron or
// external trigger request - e.g. reconcile Autoscaling, or a Webhook.
func (e *eventHandlerOwnerFromLabel) Generic(ctx context.Context, evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	e.enqueue(evt.Object, q)
}

// enqueue adds a reconcile request to the queue if the requested label exists and uses that
// to map to the resource name.
func (e *eventHandlerOwnerFromLabel) enqueue(object client.Object, q workqueue.RateLimitingInterface) {
	if object == nil {
		return
	}

	name, ok := object.GetLabels()[e.label]
	if !ok {
		return
	}

	request := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: object.GetNamespace(),
		},
	}

	q.Add(request)
}

// Factory provides methods that can build a type specific controller.
type Factory struct{}

var _ common.ControllerFactory = &Factory{}

// Reconciler returns a new reconciler instance.
func (*Factory) Reconciler(manager manager.Manager) reconcile.Reconciler {
	return common.NewReconciler(manager.GetClient(), cluster.New)
}

// RegisterWatches adds any watches that would trigger a reconcile.
func (*Factory) RegisterWatches(manager manager.Manager, controller controller.Controller) error {
	// Any changes to the cluster spec, trigger a reconcile.
	if err := controller.Watch(source.Kind(manager.GetCache(), &unikornv1.KubernetesCluster{}), &handler.EnqueueRequestForObject{}, &predicate.GenerationChangedPredicate{}); err != nil {
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
