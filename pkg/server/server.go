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

package server

import (
	"context"
	"flag"
	"net/http"

	chi "github.com/go-chi/chi/v5"
	"github.com/spf13/pflag"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/trace"

	"github.com/eschercloudai/unikorn/pkg/server/authorization"
	"github.com/eschercloudai/unikorn/pkg/server/authorization/jose"
	"github.com/eschercloudai/unikorn/pkg/server/authorization/keystone"
	"github.com/eschercloudai/unikorn/pkg/server/authorization/oauth2"
	"github.com/eschercloudai/unikorn/pkg/server/generated"
	"github.com/eschercloudai/unikorn/pkg/server/handler"
	"github.com/eschercloudai/unikorn/pkg/server/middleware"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

type Server struct {
	// Options are server specific options e.g. listener address etc.
	Options Options

	// ZapOptions configure logging.
	ZapOptions zap.Options

	// HandlerOptions sets options for the HTTP handler.
	HandlerOptions handler.Options

	// JoseOptions sets options for JWE.
	JoseOptions jose.Options

	// KeystoneOptions sets options for OpenStack Keystone.
	KeystoneOptions keystone.Options

	// OAuth2Options sets options for the oauth2/oidc authenticator.
	OAuth2Options oauth2.Options
}

func (s *Server) AddFlags(flags *pflag.FlagSet) {
	s.Options.AddFlags(pflag.CommandLine)
	s.ZapOptions.BindFlags(flag.CommandLine)
	s.HandlerOptions.AddFlags(pflag.CommandLine)
	s.JoseOptions.AddFlags(pflag.CommandLine)
	s.KeystoneOptions.AddFlags(pflag.CommandLine)
	s.OAuth2Options.AddFlags(pflag.CommandLine)
}

func (s *Server) SetupLogging() {
	log.SetLogger(zap.New(zap.UseFlagOptions(&s.ZapOptions)))
}

// SetupOpenTelemetry adds a span processor that will print root spans to the
// logs by default, and optionally ship the spans to an OTLP listener.
// TODO: move config into an otel specific options struct.
func (s *Server) SetupOpenTelemetry(ctx context.Context) error {
	otel.SetLogger(log.Log)

	opts := []trace.TracerProviderOption{
		trace.WithSpanProcessor(&middleware.LoggingSpanProcessor{}),
	}

	if s.Options.OTLPEndpoint != "" {
		exporter, err := otlptracehttp.New(ctx,
			otlptracehttp.WithEndpoint(s.Options.OTLPEndpoint),
			otlptracehttp.WithInsecure(),
		)

		if err != nil {
			return err
		}

		opts = append(opts, trace.WithBatcher(exporter))
	}

	otel.SetTracerProvider(trace.NewTracerProvider(opts...))

	return nil
}

func (s *Server) GetServer(client client.Client) (*http.Server, error) {
	// Middleware specified here is applied to all requests pre-routing.
	router := chi.NewRouter()
	router.Use(middleware.Logger())
	router.Use(middleware.Timeout(s.Options.RequestTimeout))
	router.NotFound(http.HandlerFunc(handler.NotFound))
	router.MethodNotAllowed(http.HandlerFunc(handler.MethodNotAllowed))

	// Setup authn/authz
	issuer := jose.NewJWTIssuer(&s.JoseOptions)
	keystone := keystone.New(&s.KeystoneOptions)
	oauth2 := oauth2.New(&s.OAuth2Options, issuer, keystone)
	authenticator := authorization.NewAuthenticator(issuer, oauth2, keystone)

	// Setup middleware.
	authorizer := middleware.NewAuthorizer(issuer)

	openapi, err := middleware.NewOpenAPI()
	if err != nil {
		return nil, err
	}

	// Middleware specified here is applied to all requests post-routing.
	// NOTE: these are applied in reverse order!!
	chiServerOptions := generated.ChiServerOptions{
		BaseRouter:       router,
		ErrorHandlerFunc: handler.HandleError,
		Middlewares: []generated.MiddlewareFunc{
			middleware.OpenAPIValidatorMiddlewareFactory(authorizer, openapi),
		},
	}

	handlerInterface, err := handler.New(client, authenticator, &s.HandlerOptions)
	if err != nil {
		return nil, err
	}

	server := &http.Server{
		Addr:              s.Options.ListenAddress,
		ReadTimeout:       s.Options.ReadTimeout,
		ReadHeaderTimeout: s.Options.ReadHeaderTimeout,
		WriteTimeout:      s.Options.WriteTimeout,
		Handler:           generated.HandlerWithOptions(handlerInterface, chiServerOptions),
	}

	return server, nil
}
