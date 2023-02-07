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
	"flag"
	"net/http"
	"os"
	"time"

	chi "github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"

	unikornscheme "github.com/eschercloudai/unikorn/generated/clientset/unikorn/scheme"
	"github.com/eschercloudai/unikorn/pkg/constants"
	"github.com/eschercloudai/unikorn/pkg/server/authorization"
	"github.com/eschercloudai/unikorn/pkg/server/generated"
	"github.com/eschercloudai/unikorn/pkg/server/handler"
	"github.com/eschercloudai/unikorn/pkg/server/middleware"
	"github.com/eschercloudai/unikorn/pkg/util/flags"

	"k8s.io/apimachinery/pkg/runtime"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"

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
	readTimeout flags.DurationFlag

	// readHeaderTimeout defines how long before we give up on the client,
	// this should be fairly short.
	readHeaderTimeout flags.DurationFlag

	// writeTimeout defines how long we take to respond before we give up.
	// Ideally we'd like this to be short, but Openstack in general sucks
	// for performance.
	writeTimeout flags.DurationFlag
}

// newServerOptions returns default server options.
func newServerOptions() *serverOptions {
	return &serverOptions{
		readTimeout: flags.DurationFlag{
			Duration: time.Second,
		},
		readHeaderTimeout: flags.DurationFlag{
			Duration: time.Second,
		},
		writeTimeout: flags.DurationFlag{
			Duration: 10 * time.Second,
		},
	}
}

// addFlags allows server options to be modified.
func (o *serverOptions) addFlags(f *flag.FlagSet) {
	f.StringVar(&o.listenAddress, "server-listen-address", ":6080", "API listener address.")
	f.Var(&o.readTimeout, "server-read-timeout", "How long to wait for the client to send the request body.")
	f.Var(&o.readHeaderTimeout, "server-read-header-timeout", "How long to wait for the client to send headers.")
	f.Var(&o.writeTimeout, "server-write-timeout", "How long to wait for the API to respond to the client.")
}

// getClient grabs a client for the entire server instance so all handers
// share caches.
func getClient() (client.Client, error) {
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

	clientOptions := client.Options{
		Scheme: scheme,
	}

	return client.New(config, clientOptions)
}

// main is the entry point to server.
func main() {
	// Initialize components with flags, then parse them.
	serverOptions := newServerOptions()
	serverOptions.addFlags(flag.CommandLine)

	zapOptions := &zap.Options{}
	zapOptions.BindFlags(flag.CommandLine)

	issuer := authorization.NewJWTIssuer()
	issuer.AddFlags(flag.CommandLine)

	authenticator := authorization.NewAuthenticator(issuer)
	authenticator.AddFlags(flag.CommandLine)

	flag.Parse()

	// Get logging going first, log sinks will expect JSON formatted output for everything.
	log.SetLogger(zap.New(zap.UseFlagOptions(zapOptions)))

	logger := log.Log.WithName(constants.Application)
	otel.SetLogger(logger)

	// Hello World!
	logger.Info("service starting", "application", constants.Application, "version", constants.Version, "revision", constants.Revision)

	client, err := getClient()
	if err != nil {
		logger.Error(err, "failed to create client")
		os.Exit(1)
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
			middleware.NewOpenAPIValidator(authorizer).Middleware,
		},
	}

	handlerInterface := handler.New(client, authenticator)
	chiServerhandler := generated.HandlerWithOptions(handlerInterface, chiServerOptions)

	server := &http.Server{
		Addr:              serverOptions.listenAddress,
		ReadTimeout:       serverOptions.readTimeout.Duration,
		ReadHeaderTimeout: serverOptions.readHeaderTimeout.Duration,
		WriteTimeout:      serverOptions.writeTimeout.Duration,
		Handler:           chiServerhandler,
	}

	if err := server.ListenAndServe(); err != nil {
		logger.Error(err, "server died unexpectedly")
	}
}
