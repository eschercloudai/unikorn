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
	unikornscheme "github.com/eschercloudai/unikorn/generated/clientset/unikorn/scheme"
	unikornv1alpha1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"

	"k8s.io/apimachinery/pkg/runtime"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"

	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	identity = "unikorn-project-manager"
)

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

	if err := c.Watch(&source.Kind{Type: &unikornv1alpha1.Project{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return err
	}

	return manager.Start(signals.SetupSignalHandler())
}
