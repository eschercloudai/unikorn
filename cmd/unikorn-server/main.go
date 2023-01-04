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
	"time"

	chi "github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"

	"github.com/eschercloudai/unikorn/pkg/constants"
	"github.com/eschercloudai/unikorn/pkg/server/authorization"
	"github.com/eschercloudai/unikorn/pkg/server/generated"
	"github.com/eschercloudai/unikorn/pkg/server/handler"
	"github.com/eschercloudai/unikorn/pkg/server/middleware"

	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func main() {
	zapOptions := &zap.Options{}
	zapOptions.BindFlags(flag.CommandLine)

	issuer := authorization.NewJWTIssuer()
	issuer.AddFlags(flag.CommandLine)

	flag.Parse()

	log.SetLogger(zap.New(zap.UseFlagOptions(zapOptions)))

	logger := log.Log.WithName(constants.Application)
	otel.SetLogger(logger)

	logger.Info("service starting", "application", constants.Application, "version", constants.Version, "revision", constants.Revision)

	// TODO: paramterize.
	authenticator := &authorization.Authenticator{
		Endpoint: "https://nl1.eschercloud.com:5000",
		Domain:   "Default",
		Issuer:   issuer,
	}

	// Middleware specified here is applied to all requests pre-routing.
	router := chi.NewRouter()
	router.Use(middleware.Logger)

	// Middleware specified here is applied to all requests post-routing.
	serverOptions := generated.ChiServerOptions{
		BaseRouter:       router,
		ErrorHandlerFunc: handler.HandleError,
		Middlewares: []generated.MiddlewareFunc{
			middleware.NewAuthorizer(issuer).Middleware,
		},
	}

	server := &http.Server{
		Addr:              ":6080",
		Handler:           generated.HandlerWithOptions(handler.New(authenticator), serverOptions),
		ReadTimeout:       time.Second,
		ReadHeaderTimeout: time.Second,
		WriteTimeout:      time.Second,
	}

	if err := server.ListenAndServe(); err != nil {
		logger.Error(err, "server died unexpectedly")
	}
}
