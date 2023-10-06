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
	"context"
	"flag"
	"os"

	"github.com/spf13/pflag"

	"github.com/eschercloudai/unikorn/pkg/cd"
	"github.com/eschercloudai/unikorn/pkg/cd/argocd"
	"github.com/eschercloudai/unikorn/pkg/constants"
	"github.com/eschercloudai/unikorn/pkg/errors"
	"github.com/eschercloudai/unikorn/pkg/managers/options"
	utilclient "github.com/eschercloudai/unikorn/pkg/util/client"

	klog "k8s.io/klog/v2"

	"sigs.k8s.io/controller-runtime/pkg/client"
	clientconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
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
	Reconciler(manager.Manager, cd.DriverRunnable) reconcile.Reconciler

	// RegisterWatches adds any watches that would trigger a reconcile.
	RegisterWatches(manager.Manager, controller.Controller) error

	// Upgrade allows version based upgrades of managed resources.
	// DO NOT MODIFY THE SPEC EVER.  Only things like metadata can
	// be touched.
	Upgrade(client.Client) error
}

// driverRunnable implments the cd.DriverRunnable interface.
type driverRunnable struct {
	// kind is the prover provider to create.
	kind cd.DriverKind

	// client is the Kubernetes client.
	client client.Client

	// driver will be filled in during initialization.
	driver cd.Driver
}

var _ cd.DriverRunnable = &driverRunnable{}

// NewDriverRunnable creates a new runnable task to initialize the driver when
// we are able to.
func NewDriverRunnable(client client.Client, kind cd.DriverKind) cd.DriverRunnable {
	return &driverRunnable{
		kind:   kind,
		client: client,
	}
}

// Driver returns the driver for use by a reconciler.
func (r *driverRunnable) Driver() cd.Driver {
	return r.driver
}

// NeedLeaderElection tells controller runtime to start this before leader
// election happens, so it's a race condition basically...
func (r *driverRunnable) NeedLeaderElection() bool {
	return false
}

// Start is called when the clients are initialized, and before the reconcillers
// are invoked.
func (r *driverRunnable) Start(ctx context.Context) error {
	log := log.FromContext(ctx)

	log.Info("Starting continuous deployment driver", "kind", r.kind)

	if r.kind != cd.DriverKindArgoCD {
		return errors.ErrCDDriver
	}

	r.driver = argocd.New(r.client)

	// This must block until told not to!
	<-ctx.Done()

	return nil
}

// getManager returns a generic manager.
func getManager(o *options.Options) (manager.Manager, cd.DriverRunnable, error) {
	// Create a manager with leadership election to prevent split brain
	// problems, and set the scheme so it gets propagated to the client.
	config, err := clientconfig.GetConfig()
	if err != nil {
		return nil, nil, err
	}

	scheme, err := utilclient.NewScheme()
	if err != nil {
		return nil, nil, err
	}

	options := manager.Options{
		Scheme:           scheme,
		LeaderElection:   true,
		LeaderElectionID: constants.Application,
	}

	manager, err := manager.New(config, options)
	if err != nil {
		return nil, nil, err
	}

	// Add the driver initialiser so it's setup after the client caches are
	// started but before the reconciler is called.
	driverRunnable := NewDriverRunnable(manager.GetClient(), o.CDDriver.Kind)

	if err := manager.Add(driverRunnable); err != nil {
		return nil, nil, err
	}

	return manager, driverRunnable, nil
}

// getController returns a generic controller.
func getController(o *options.Options, manager manager.Manager, f ControllerFactory, driverRunnable cd.DriverRunnable) (controller.Controller, error) {
	// This prevents a single bad reconcile from affecting all the rest by
	// boning the whole container.
	recoverPanic := true

	controllerOptions := controller.Options{
		MaxConcurrentReconciles: o.MaxConcurrentReconciles,
		RecoverPanic:            &recoverPanic,
		Reconciler:              f.Reconciler(manager, driverRunnable),
	}

	c, err := controller.New(constants.Application, manager, controllerOptions)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func doUpgrade(f ControllerFactory) error {
	client, err := utilclient.New(context.TODO())
	if err != nil {
		return err
	}

	if err := f.Upgrade(client); err != nil {
		return err
	}

	return nil
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

	if err := doUpgrade(f); err != nil {
		logger.Error(err, "resource upgrade failed")
		os.Exit(1)
	}

	manager, driverRunnable, err := getManager(o)
	if err != nil {
		logger.Error(err, "manager creation error")
		os.Exit(1)
	}

	controller, err := getController(o, manager, f, driverRunnable)
	if err != nil {
		logger.Error(err, "controller creation error")
		os.Exit(1)
	}

	if err := f.RegisterWatches(manager, controller); err != nil {
		logger.Error(err, "watcher registration error")
		os.Exit(1)
	}

	if err := manager.Start(signals.SetupSignalHandler()); err != nil {
		logger.Error(err, "manager terminated")
		os.Exit(1)
	}
}
