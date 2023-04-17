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

package common

import (
	"flag"
	"os"

	"github.com/spf13/pflag"

	unikornscheme "github.com/eschercloudai/unikorn/generated/clientset/unikorn/scheme"
	"github.com/eschercloudai/unikorn/pkg/constants"
	"github.com/eschercloudai/unikorn/pkg/managers/options"

	"k8s.io/apimachinery/pkg/runtime"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	klog "k8s.io/klog/v2"

	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ControllerFactory allows creation of a Unikorn controller with
// minimal code.
type ControllerFactory interface {
	// Reconciler returns a new reconciler instance.
	Reconciler(manager.Manager) reconcile.Reconciler

	// RegisterWatches adds any watches that would trigger a reconcile.
	RegisterWatches(controller.Controller) error
}

// getScheme returns a scheme that knows about core Kubernetes and Unikorn types
// that it can use to map between structured and unstructured resource definitions.
// TODO: we'd really love to include ArgoCD here, but its dependency hell.
// See https://github.com/argoproj/gitops-engine/issues/56 for a never ending
// commentary on the underlying problem.
func getScheme() (*runtime.Scheme, error) {
	scheme := runtime.NewScheme()

	if err := kubernetesscheme.AddToScheme(scheme); err != nil {
		return nil, err
	}

	if err := unikornscheme.AddToScheme(scheme); err != nil {
		return nil, err
	}

	return scheme, nil
}

// getManager returns a generic manager.
func getManager() (manager.Manager, error) {
	// Create a manager with leadership election to prevent split brain
	// problems, and set the scheme so it gets propagated to the client.
	config, err := config.GetConfig()
	if err != nil {
		return nil, err
	}

	scheme, err := getScheme()
	if err != nil {
		return nil, err
	}

	options := manager.Options{
		Scheme:           scheme,
		LeaderElection:   true,
		LeaderElectionID: constants.Application,
	}

	manager, err := manager.New(config, options)
	if err != nil {
		return nil, err
	}

	return manager, nil
}

// getController returns a generic controller.
func getController(o *options.Options, manager manager.Manager, f ControllerFactory) (controller.Controller, error) {
	// This prevents a single bad reconcile from affecting all the rest by
	// boning the whole container.
	recoverPanic := true

	controllerOptions := controller.Options{
		Reconciler:              f.Reconciler(manager),
		MaxConcurrentReconciles: o.MaxConcurrentReconciles,
		RecoverPanic:            &recoverPanic,
	}

	c, err := controller.New(constants.Application, manager, controllerOptions)
	if err != nil {
		return nil, err
	}

	return c, nil
}

// Run provides common manager initialization and execution.
func Run(f ControllerFactory) {
	zapOptions := &zap.Options{}
	zapOptions.BindFlags(flag.CommandLine)

	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

	o := &options.Options{}
	o.AddFlags(pflag.CommandLine)

	pflag.Parse()

	logr := zap.New(zap.UseFlagOptions(zapOptions))

	log.SetLogger(logr)
	klog.SetLogger(logr)

	logger := log.Log.WithName("init")
	logger.Info("service starting", "application", constants.Application, "version", constants.Version, "revision", constants.Revision)

	manager, err := getManager()
	if err != nil {
		logger.Error(err, "manager creation error")
		os.Exit(1)
	}

	controller, err := getController(o, manager, f)
	if err != nil {
		logger.Error(err, "controller creation error")
		os.Exit(1)
	}

	if err := f.RegisterWatches(controller); err != nil {
		logger.Error(err, "watcher registration error")
		os.Exit(1)
	}

	if err := manager.Start(signals.SetupSignalHandler()); err != nil {
		logger.Error(err, "manager terminated")
		os.Exit(1)
	}
}
