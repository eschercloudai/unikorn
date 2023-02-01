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
	"github.com/eschercloudai/unikorn/pkg/server/context"
	"github.com/eschercloudai/unikorn/pkg/server/errors"
	"github.com/eschercloudai/unikorn/pkg/server/generated"
)

// authorizationContext is passed through the middleware to propagate
// information back to the top level handler.
type authorizationContext struct {
	// subject is the token subject.
	subject string

	// token is the Openstack token from password authentication.
	token string
}

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
func authorizeHTTP(r *http.Request, scheme *openapi3.SecurityScheme) error {
	authorizationScheme, _, err := authorization.GetHTTPAuthenticationScheme(r)
	if err != nil {
		return err
	}

	if !strings.EqualFold(authorizationScheme, scheme.Scheme) {
		return errors.OAuth2InvalidRequest("authorization scheme not allowed").WithValues("scheme", authorizationScheme)
	}

	return nil
}

// authorizeOAuth2 checks APIs that require and oauth2 bearer token.
func (a *Authorizer) authorizeOAuth2(ctx *authorizationContext, r *http.Request, scopes []string) error {
	authorizationScheme, token, err := authorization.GetHTTPAuthenticationScheme(r)
	if err != nil {
		return err
	}

	if !strings.EqualFold(authorizationScheme, "bearer") {
		return errors.OAuth2InvalidRequest("authorization scheme not allowed").WithValues("scheme", authorizationScheme)
	}

	// Check the token is from us, for us, and in date.
	claims, err := a.issuer.Verify(r, token)
	if err != nil {
		return errors.OAuth2AccessDenied("token validation failed").WithError(err)
	}

	// Check the token is authorized to do what the schema says.
	for _, scope := range scopes {
		if !claims.Scope.Includes(authorization.Scope(scope)) {
			return errors.OAuth2AccessDenied("token missing required scope").WithValues("scope", scope)
		}
	}

	// Set the Keystone token in the context for use by the handlers.
	// TODO: if this gets too crazy, just add the claims.
	ctx.subject = claims.Subject
	ctx.token = claims.Token

	return nil
}

// authorizeScheme requires the individual scheme to match.
func (a *Authorizer) authorizeScheme(ctx *authorizationContext, r *http.Request, scheme *openapi3.SecurityScheme, scopes []string) error {
	switch scheme.Type {
	case "http":
		return authorizeHTTP(r, scheme)
	case "oauth2":
		return a.authorizeOAuth2(ctx, r, scopes)
	}

	return errors.OAuth2InvalidRequest("authorization scheme unsupported").WithValues("scheme", scheme.Type)
}

// authorizeSchemes requires all schemes to be fulfilled.
func (a *Authorizer) authorizeSchemes(ctx *authorizationContext, r *http.Request, spec *openapi3.T, requirement openapi3.SecurityRequirement) error {
	if len(requirement) == 0 {
		return errors.OAuth2ServerError("no security schemes specified for operation")
	}

	for schemeName, scopes := range requirement {
		scheme, ok := spec.Components.SecuritySchemes[schemeName]
		if !ok {
			return errors.OAuth2ServerError("security scheme missing from schema")
		}

		if scheme.Value == nil {
			return errors.OAuth2ServerError("security scheme reference not supported")
		}

		if err := a.authorizeScheme(ctx, r, scheme.Value, scopes); err != nil {
			return err
		}
	}

	return nil
}

// authorizeReqirements requires at least one set of requirements to be fulfilled.
func (a *Authorizer) authorizeReqirements(ctx *authorizationContext, r *http.Request, spec *openapi3.T, requirements openapi3.SecurityRequirements) error {
	// Why you ask?  Well if we allowed multiple, that means any must be authoried
	// to succeed, but then that raises the problem of how do you report multiple
	// errors to the client if they all fail in a meaningful way?
	if len(requirements) != 1 {
		return errors.OAuth2ServerError("single security requirements required")
	}

	if err := a.authorizeSchemes(ctx, r, spec, requirements[0]); err != nil {
		return err
	}

	return nil
}

// authorize performs any authorization of the request before allowing access to
// the resources via the handler.
func (a *Authorizer) authorize(ctx *authorizationContext, r *http.Request) error {
	spec, err := generated.GetSwagger()
	if err != nil {
		return errors.OAuth2ServerError("unable to decode openapi schema").WithError(err)
	}

	router, err := gorillamux.NewRouter(spec)
	if err != nil {
		return errors.OAuth2ServerError("unable to create router").WithError(err)
	}

	// Authentication is dependant on route, so we insert this middleware
	// after the route, and let the router handle 404/405 errors, so this
	// logic should always work.
	route, _, err := router.FindRoute(r)
	if err != nil {
		return errors.OAuth2ServerError("unable to find route").WithError(err)
	}

	// You must have specified a security requirement in the schema.
	if route.Operation == nil {
		return errors.OAuth2ServerError("unable to find route operation")
	}

	if route.Operation.Security == nil {
		return errors.OAuth2ServerError("unable to find route operation security requirements")
	}

	// Check the necessary security is in place.
	if err := a.authorizeReqirements(ctx, r, spec, *route.Operation.Security); err != nil {
		return err
	}

	return nil
}

// ServeHTTP implements the http.Handler interface.
func (a *Authorizer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := &authorizationContext{}

	if err := a.authorize(ctx, r); err != nil {
		errors.HandleError(w, r, err)

		return
	}

	c := r.Context()

	// Add any contextual information to bubble up to the handler.
	c = context.NewContextWithSubject(c, ctx.subject)
	c = context.NewContextWithToken(c, ctx.token)

	r = r.WithContext(c)

	a.next.ServeHTTP(w, r)
}

// Middleware performs any authorization handling middleware.
func (a *Authorizer) Middleware(next http.Handler) http.Handler {
	a.next = next

	return a
}
