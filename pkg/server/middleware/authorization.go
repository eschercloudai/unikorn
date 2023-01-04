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

package middleware

import (
	"net/http"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/routers/gorillamux"

	"github.com/eschercloudai/unikorn/pkg/server/authorization"
	"github.com/eschercloudai/unikorn/pkg/server/generated"
	"github.com/eschercloudai/unikorn/pkg/server/util"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Authorizer provides OpenAPI based authorization middleware.
type Authorizer struct {
	// issuer allows creation and validation of JWT bearer tokens.
	issuer *authorization.JWTIssuer

	// next defines the next HTTP handler in the chain.
	next http.Handler
}

// Ensure this implements the required interfaces.
var _ http.Handler = &Authorizer{}

// NewAuthorizer returns a new authorizer with required parameters.
func NewAuthorizer(issuer *authorization.JWTIssuer) *Authorizer {
	return &Authorizer{
		issuer: issuer,
	}
}

// authorizeHTTP checks basic authentication information is there then
// lets this request bubble up to the handler for processing.  The ONLY
// API that uses this is the one that does basic auth to oauth token issue.
func authorizeHTTP(scheme *openapi3.SecurityScheme, r *http.Request) error {
	authorizationScheme, _, err := authorization.GetHTTPAuthenticationScheme(r)
	if err != nil {
		return err
	}

	if !strings.EqualFold(authorizationScheme, scheme.Scheme) {
		return util.HTTPUnauthorized("authorization scheme mismatch", "scheme", authorizationScheme)
	}

	return nil
}

// authorizeOAuth2 checks APIs that require and oauth2 bearer token.
func (a *Authorizer) authorizeOAuth2(scopes []string, r *http.Request) error {
	authorizationScheme, token, err := authorization.GetHTTPAuthenticationScheme(r)
	if err != nil {
		return err
	}

	if !strings.EqualFold(authorizationScheme, "bearer") {
		return util.HTTPUnauthorized("authorization scheme mismatch", "scheme", authorizationScheme)
	}

	// Check the token is from us, for us, and in date.
	claims, err := a.issuer.Verify(r, token)
	if err != nil {
		return util.HTTPUnauthorizedWithError(err, "token validation failed")
	}

	// Check the token is authorized to do what the schema says.
	for _, scope := range scopes {
		if !claims.Scope.Includes(authorization.Scope(scope)) {
			return util.HTTPUnauthorized("token missing required scope", "scope", scope)
		}
	}

	return nil
}

// authorizeScheme requires the individual scheme to match.
func (a *Authorizer) authorizeScheme(scheme *openapi3.SecurityScheme, scopes []string, r *http.Request) error {
	switch scheme.Type {
	case "http":
		return authorizeHTTP(scheme, r)
	case "oauth2":
		return a.authorizeOAuth2(scopes, r)
	}

	return util.HTTPInternalServerError("authorization scheme unsupported", "scheme", scheme.Type)
}

// authorizeSchemes requires all schemes to be fulfilled.
func (a *Authorizer) authorizeSchemes(spec *openapi3.T, requirement openapi3.SecurityRequirement, r *http.Request) error {
	if len(requirement) == 0 {
		return util.HTTPInternalServerError("no security schemes specified for route operation security requirement")
	}

	for schemeName, scopes := range requirement {
		scheme, ok := spec.Components.SecuritySchemes[schemeName]
		if !ok {
			return util.HTTPInternalServerError("security scheme missing from schema")
		}

		if scheme.Value == nil {
			return util.HTTPInternalServerError("security scheme reference not supported")
		}

		if err := a.authorizeScheme(scheme.Value, scopes, r); err != nil {
			return err
		}
	}

	return nil
}

// authorizeReqirements requires at least one set of requirements to be fulfilled.
func (a *Authorizer) authorizeReqirements(spec *openapi3.T, requirements openapi3.SecurityRequirements, r *http.Request) error {
	log := log.FromContext(r.Context())

	if len(requirements) == 0 {
		return util.HTTPInternalServerError("no security requirements specified for route operation")
	}

	for _, requirement := range requirements {
		if err := a.authorizeSchemes(spec, requirement, r); err != nil {
			log.V(1).Info("security requirements rejection", util.LogValues(err)...)

			continue
		}

		return nil
	}

	return util.HTTPUnauthorized("no security requirement was met")
}

// authorize performs any authorization of the request before allowing access to
// the resources via the handler.
func (a *Authorizer) authorize(r *http.Request) error {
	spec, err := generated.GetSwagger()
	if err != nil {
		return util.HTTPInternalServerErrorWithError(err, "unable to decode openapi schema")
	}

	router, err := gorillamux.NewRouter(spec)
	if err != nil {
		return util.HTTPInternalServerErrorWithError(err, "unable to create router")
	}

	// Authentication is dependant on route, so we insert this middleware
	// after the route, and let the router handle 404/405 errors, so this
	// logic should always work.
	route, _, err := router.FindRoute(r)
	if err != nil {
		return util.HTTPInternalServerErrorWithError(err, "unable to find route")
	}

	// You must have specified a security requirement in the schema.
	if route.Operation == nil {
		return util.HTTPInternalServerError("unable to find route operation")
	}

	if route.Operation.Security == nil {
		return util.HTTPInternalServerError("unable to find route operation security requirements")
	}

	// Check the necessary security is in place.
	if err := a.authorizeReqirements(spec, *route.Operation.Security, r); err != nil {
		return err
	}

	return nil
}

// ServeHTTP implements the http.Handler interface.
func (a *Authorizer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := a.authorize(r); err != nil {
		util.HandleError(w, r, err)

		return
	}

	a.next.ServeHTTP(w, r)
}

// Middleware performs any authorization handling middleware.
func (a *Authorizer) Middleware(next http.Handler) http.Handler {
	a.next = next

	return a
}
