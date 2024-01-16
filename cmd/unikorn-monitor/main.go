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

package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/pflag"

	unikornscheme "github.com/eschercloudai/unikorn/generated/clientset/unikorn/scheme"
	"github.com/eschercloudai/unikorn/pkg/monitor"

	coreclient "github.com/eschercloudai/unikorn-core/pkg/client"
	"github.com/eschercloudai/unikorn-core/pkg/constants"

	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func main() {
	// Initialize components with legacy flags.
	zapOptions := &zap.Options{}
	zapOptions.BindFlags(flag.CommandLine)

	// Initialize components with flags, then parse them.
	monitorOptions := &monitor.Options{}
	monitorOptions.AddFlags(pflag.CommandLine)

	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()

	// Get logging going first, log sinks will expect JSON formatted output for everything.
	log.SetLogger(zap.New(zap.UseFlagOptions(zapOptions)))

	logger := log.Log.WithName(constants.Application)

	// Hello World!
	logger.Info("service starting", "application", constants.Application, "version", constants.Version, "revision", constants.Revision)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Register a signal handler to trigger a graceful shutdown.
	stop := make(chan os.Signal, 1)

	signal.Notify(stop, syscall.SIGTERM)

	go func() {
		<-stop

		// Cancel anything hanging off the root context.
		cancel()
	}()

	client, err := coreclient.New(ctx, unikornscheme.AddToScheme)
	if err != nil {
		logger.Error(err, "failed to create client")

		return
	}

	monitor.Run(ctx, client, monitorOptions)
}
