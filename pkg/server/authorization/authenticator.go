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

package authorization

import (
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/eschercloudai/unikorn/pkg/providers/openstack"
	"github.com/eschercloudai/unikorn/pkg/server/context"
	"github.com/eschercloudai/unikorn/pkg/server/errors"
	"github.com/eschercloudai/unikorn/pkg/server/generated"
)

// Authenticator provides Keystone authentication functionality.
type Authenticator struct {
	// Issuer allows creation and validation of JWT bearer tokens.
	Issuer *JWTIssuer

	// Endpoint is the Keystone endpoint.
	Endpoint string

	// Domain is the default domain users live under.
	// TODO: Hierarchical domains should be supported, but aren't needed
	// for now.
	Domain string
}

// Basic performs basic authentication against Keystone and returns a token.
func (a *Authenticator) Basic(r *http.Request) (string, error) {
	_, token, err := GetHTTPAuthenticationScheme(r)
	if err != nil {
		return "", err
	}

	tuple, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return "", errors.OAuth2InvalidRequest("basic authorization not base64 encoded").WithError(err)
	}

	parts := strings.Split(string(tuple), ":")
	if len(parts) != 2 {
		return "", errors.OAuth2InvalidRequest("basic authorization malformed")
	}

	username := parts[0]
	password := parts[1]

	identity, err := openstack.NewIdentityClient(openstack.NewUnauthenticatedProvider(a.Endpoint))
	if err != nil {
		return "", errors.OAuth2ServerError("unable to initialize identity").WithError(err)
	}

	// Do an unscoped authentication against Keystone.  The client is expected
	// to list visible projects (or indeed cache them in local web-storage) then
	// use that to do a scoped bearer token based upgrade.
	keystoneToken, err := identity.CreateToken(openstack.NewCreateTokenOptionsUnscopedPassword(a.Domain, username, password))
	if err != nil {
		return "", errors.OAuth2AccessDenied("authentication failed").WithError(err)
	}

	jwToken, err := a.Issuer.Issue(r, username, keystoneToken.ID, nil, keystoneToken.ExpiresAt)
	if err != nil {
		return "", errors.OAuth2ServerError("unable to create access token").WithError(err)
	}

	return jwToken, nil
}

// Token performs token based authentication against Keystone with a scope, and returns a new token.
// Used to upgrade from unscoped, or to refresh a token.
func (a *Authenticator) Token(r *http.Request, scope *generated.TokenScope) (string, error) {
	subject, err := context.SubjectFromContext(r.Context())
	if err != nil {
		return "", errors.OAuth2ServerError("failed get subject").WithError(err)
	}

	token, err := context.TokenFromContext(r.Context())
	if err != nil {
		return "", errors.OAuth2ServerError("failed get authorization token").WithError(err)
	}

	identity, err := openstack.NewIdentityClient(openstack.NewUnauthenticatedProvider(a.Endpoint))
	if err != nil {
		return "", errors.OAuth2ServerError("unable to initialize identity").WithError(err)
	}

	keystoneToken, err := identity.CreateToken(openstack.NewCreateTokenOptionsScopedToken(token, scope.Project.Id))
	if err != nil {
		return "", errors.OAuth2AccessDenied("authentication failed").WithError(err)
	}

	// Add some scope to the claims to allow the token to do more.
	oAuth2Scope := &ScopeList{
		Scopes: []Scope{
			ScopeProject,
		},
	}

	jwToken, err := a.Issuer.Issue(r, subject, keystoneToken.ID, oAuth2Scope, keystoneToken.ExpiresAt)
	if err != nil {
		return "", errors.OAuth2ServerError("unable to create access token").WithError(err)
	}

	return jwToken, nil
}
