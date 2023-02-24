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

package main

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	chi "github.com/go-chi/chi/v5"
	"github.com/spf13/pflag"
	"go.opentelemetry.io/otel"

	unikornscheme "github.com/eschercloudai/unikorn/generated/clientset/unikorn/scheme"
	"github.com/eschercloudai/unikorn/pkg/constants"
	"github.com/eschercloudai/unikorn/pkg/server/authorization"
	"github.com/eschercloudai/unikorn/pkg/server/generated"
	"github.com/eschercloudai/unikorn/pkg/server/handler"
	"github.com/eschercloudai/unikorn/pkg/server/middleware"

	"k8s.io/apimachinery/pkg/runtime"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"

	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// serverOptions allows server options to be overridden.
type serverOptions struct {
	// listenAddress tells the server what to listen on, you shouldn't
	// need to change this, its already non-privileged and the default
	// should be modified to avoid clashes with other services e.g prometheus.
	listenAddress string

	// readTimeout defines how long before we give up on the client,
	// this should be fairly short.
	readTimeout time.Duration

	// readHeaderTimeout defines how long before we give up on the client,
	// this should be fairly short.
	readHeaderTimeout time.Duration

	// writeTimeout defines how long we take to respond before we give up.
	// Ideally we'd like this to be short, but Openstack in general sucks
	// for performance.
	writeTimeout time.Duration
}

// addFlags allows server options to be modified.
func (o *serverOptions) addFlags(f *pflag.FlagSet) {
	f.StringVar(&o.listenAddress, "server-listen-address", ":6080", "API listener address.")
	f.DurationVar(&o.readTimeout, "server-read-timeout", time.Second, "How long to wait for the client to send the request body.")
	f.DurationVar(&o.readHeaderTimeout, "server-read-header-timeout", time.Second, "How long to wait for the client to send headers.")
	f.DurationVar(&o.writeTimeout, "server-write-timeout", 10*time.Second, "How long to wait for the API to respond to the client.")
}

// getClient grabs a client for the entire server instance so all handers
// share caches.
func getClient(ctx context.Context) (client.Client, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	// Create a scheme and ensure it knows about Kubernetes and Unikorn
	// resource types.
	scheme := runtime.NewScheme()

	if err := kubernetesscheme.AddToScheme(scheme); err != nil {
		return nil, err
	}

	if err := unikornscheme.AddToScheme(scheme); err != nil {
		return nil, err
	}

	cache, err := cache.New(config, cache.Options{Scheme: scheme})
	if err != nil {
		return nil, err
	}

	go func() {
		_ = cache.Start(ctx)
	}()

	clientOptions := client.Options{
		Scheme: scheme,
	}

	c, err := client.New(config, clientOptions)
	if err != nil {
		return nil, err
	}

	input := client.NewDelegatingClientInput{
		CacheReader: cache,
		Client:      c,
	}

	return client.NewDelegatingClient(input)
}

// start is the entry point to server.
func start() {
	// Initialize components with legacy flags.
	zapOptions := &zap.Options{}
	zapOptions.BindFlags(flag.CommandLine)

	// Initialize components with flags, then parse them.
	serverOptions := &serverOptions{}
	serverOptions.addFlags(pflag.CommandLine)

	issuer := authorization.NewJWTIssuer()
	issuer.AddFlags(pflag.CommandLine)

	authenticator := authorization.NewAuthenticator(issuer)
	authenticator.AddFlags(pflag.CommandLine)

	handlerOptions := &handler.Options{}
	handlerOptions.AddFlags(pflag.CommandLine)

	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()

	// Get logging going first, log sinks will expect JSON formatted output for everything.
	log.SetLogger(zap.New(zap.UseFlagOptions(zapOptions)))

	logger := log.Log.WithName(constants.Application)
	otel.SetLogger(logger)

	// Hello World!
	logger.Info("service starting", "application", constants.Application, "version", constants.Version, "revision", constants.Revision)

	// Create a root context for things to hang off of.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, err := getClient(ctx)
	if err != nil {
		logger.Error(err, "failed to create client")

		return
	}

	// Middleware specified here is applied to all requests pre-routing.
	router := chi.NewRouter()
	router.Use(middleware.Logger)
	router.NotFound(http.HandlerFunc(handler.NotFound))
	router.MethodNotAllowed(http.HandlerFunc(handler.MethodNotAllowed))

	authorizer := middleware.NewAuthorizer(issuer)

	// Middleware specified here is applied to all requests post-routing.
	// NOTE: these are applied in reverse order!!
	chiServerOptions := generated.ChiServerOptions{
		BaseRouter:       router,
		ErrorHandlerFunc: handler.HandleError,
		Middlewares: []generated.MiddlewareFunc{
			middleware.OpenAPIValidatorMiddlewareFactory(authorizer),
		},
	}

	handlerInterface := handler.New(client, authenticator, handlerOptions)
	chiServerhandler := generated.HandlerWithOptions(handlerInterface, chiServerOptions)

	server := &http.Server{
		Addr:              serverOptions.listenAddress,
		ReadTimeout:       serverOptions.readTimeout,
		ReadHeaderTimeout: serverOptions.readHeaderTimeout,
		WriteTimeout:      serverOptions.writeTimeout,
		Handler:           chiServerhandler,
	}

	// Register a signal handler to trigger a graceful shutdown.
	stop := make(chan os.Signal, 1)

	signal.Notify(stop, syscall.SIGTERM)

	go func() {
		<-stop

		// Cancel anything hanging off the root context.
		cancel()

		// Shutdown the server, Kubernetes gives us 30 seconds before a SIGKILL.
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			logger.Error(err, "server shutdown error")
		}
	}()

	if err := server.ListenAndServe(); err != nil {
		if errors.Is(err, http.ErrServerClosed) {
			return
		}

		logger.Error(err, "unexpected server error")

		return
	}
}

func main() {
	start()
}
