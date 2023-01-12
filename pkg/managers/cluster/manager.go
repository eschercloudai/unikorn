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
	unikornscheme "github.com/eschercloudai/unikorn/generated/clientset/unikorn/scheme"
	unikornv1alpha1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/constants"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/util/workqueue"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	identity = "unikorn-cluster-manager"
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
func (e *eventHandlerOwnerFromLabel) Create(evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	e.enqueue(evt.Object, q)
}

// Update is called in response to an update event -  e.g. Pod Updated.
func (e *eventHandlerOwnerFromLabel) Update(evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	e.enqueue(evt.ObjectNew, q)
	e.enqueue(evt.ObjectOld, q)
}

// Delete is called in response to a delete event - e.g. Pod Deleted.
func (e *eventHandlerOwnerFromLabel) Delete(evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	e.enqueue(evt.Object, q)
}

// Generic is called in response to an event of an unknown type or a synthetic event triggered as a cron or
// external trigger request - e.g. reconcile Autoscaling, or a Webhook.
func (e *eventHandlerOwnerFromLabel) Generic(evt event.GenericEvent, q workqueue.RateLimitingInterface) {
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

func Run() error {
	// Create a scheme and ensure it knows about Kubernetes and Unikorn
	// resource types.
	scheme := runtime.NewScheme()

	if err := kubernetesscheme.AddToScheme(scheme); err != nil {
		return err
	}

	if err := unikornscheme.AddToScheme(scheme); err != nil {
		return err
	}

	// Create a manager with leadership election to prevent split brain
	// problems, and set the scheme so it gets propagated to the client.
	config, err := config.GetConfig()
	if err != nil {
		return err
	}

	options := manager.Options{
		Scheme:           scheme,
		LeaderElection:   true,
		LeaderElectionID: identity,
	}

	manager, err := manager.New(config, options)
	if err != nil {
		return err
	}

	// Create a controller that responds to our ControlPlane objects.
	controllerOptions := controller.Options{
		Reconciler: &reconciler{
			client: manager.GetClient(),
		},
	}

	c, err := controller.New(identity, manager, controllerOptions)
	if err != nil {
		return err
	}

	// Any changes to the cluster spec, trigger a reconcile.
	if err := c.Watch(&source.Kind{Type: &unikornv1alpha1.KubernetesCluster{}}, &handler.EnqueueRequestForObject{}, &predicate.GenerationChangedPredicate{}); err != nil {
		return err
	}

	// Any changes to workload pools that are selected by the cluster, trigger a reconcile.
	if err := c.Watch(&source.Kind{Type: &unikornv1alpha1.KubernetesWorkloadPool{}}, &eventHandlerOwnerFromLabel{label: constants.KubernetesClusterLabel}, &predicate.GenerationChangedPredicate{}); err != nil {
		return err
	}

	return manager.Start(signals.SetupSignalHandler())
}
