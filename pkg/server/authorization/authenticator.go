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

	"github.com/spf13/pflag"

	"github.com/eschercloudai/unikorn/pkg/providers/openstack"
	"github.com/eschercloudai/unikorn/pkg/server/errors"
	"github.com/eschercloudai/unikorn/pkg/server/generated"
)

// Authenticator provides Keystone authentication functionality.
type Authenticator struct {
	// issuer allows creation and validation of JWT bearer tokens.
	issuer *JWTIssuer

	// endpoint is the Keystone endpoint.
	endpoint string

	// domain is the default domain users live under.
	domain string
}

// NewAuthenticator returns a new authenticator with required fields populated.
// You must call AddFlags after this.
func NewAuthenticator(issuer *JWTIssuer) *Authenticator {
	return &Authenticator{
		issuer: issuer,
	}
}

// AddFlags to the specified flagset.
func (a *Authenticator) AddFlags(f *pflag.FlagSet) {
	f.StringVar(&a.endpoint, "keystone-endpoint", "https://nl1.eschercloud.com:5000", "Keystone endpoint to use for authn/authz.")
	f.StringVar(&a.domain, "keystone-user-domain-name", "Default", "Keystone user domain name for password authentication.")
}

// Endpoint returns the endpoint host.
func (a *Authenticator) Endpoint() string {
	return a.endpoint
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

	identity, err := openstack.NewIdentityClient(openstack.NewUnauthenticatedProvider(a.endpoint))
	if err != nil {
		return "", errors.OAuth2ServerError("unable to initialize identity").WithError(err)
	}

	// Do an unscoped authentication against Keystone.  The client is expected
	// to list visible projects (or indeed cache them in local web-storage) then
	// use that to do a scoped bearer token based upgrade.
	keystoneToken, user, err := identity.CreateToken(openstack.NewCreateTokenOptionsUnscopedPassword(a.domain, username, password))
	if err != nil {
		return "", errors.OAuth2AccessDenied("authentication failed").WithError(err)
	}

	claims := &UnikornClaims{
		Token: keystoneToken.ID,
		User:  user.ID,
	}

	jwToken, err := a.issuer.Issue(r, username, claims, nil, keystoneToken.ExpiresAt)
	if err != nil {
		return "", errors.OAuth2ServerError("unable to create access token").WithError(err)
	}

	return jwToken, nil
}

// Token performs token based authentication against Keystone with a scope, and returns a new token.
// Used to upgrade from unscoped, or to refresh a token.
func (a *Authenticator) Token(r *http.Request, scope *generated.TokenScope) (string, error) {
	tokenClaims, err := ClaimsFromContext(r.Context())
	if err != nil {
		return "", errors.OAuth2ServerError("failed get claims").WithError(err)
	}

	identity, err := openstack.NewIdentityClient(openstack.NewUnauthenticatedProvider(a.endpoint))
	if err != nil {
		return "", errors.OAuth2ServerError("unable to initialize identity").WithError(err)
	}

	if tokenClaims.UnikornClaims == nil {
		return "", errors.OAuth2ServerError("unable to get unikorn claims")
	}

	keystoneToken, user, err := identity.CreateToken(openstack.NewCreateTokenOptionsScopedToken(tokenClaims.UnikornClaims.Token, scope.Project.Id))
	if err != nil {
		return "", errors.OAuth2AccessDenied("authentication failed").WithError(err)
	}

	claims := &UnikornClaims{
		Token:   keystoneToken.ID,
		User:    user.ID,
		Project: scope.Project.Id,
	}

	// Add some scope to the claims to allow the token to do more.
	oAuth2Scope := &ScopeList{
		Scopes: []Scope{
			ScopeProject,
		},
	}

	jwToken, err := a.issuer.Issue(r, tokenClaims.Subject, claims, oAuth2Scope, keystoneToken.ExpiresAt)
	if err != nil {
		return "", errors.OAuth2ServerError("unable to create access token").WithError(err)
	}

	return jwToken, nil
}
