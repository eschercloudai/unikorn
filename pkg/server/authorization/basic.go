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

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/identity/v3/tokens"

	"github.com/eschercloudai/unikorn/pkg/server/util"
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
func (a *Authenticator) Basic(w http.ResponseWriter, r *http.Request) (string, error) {
	_, token, err := GetHTTPAuthenticationScheme(r)
	if err != nil {
		return "", err
	}

	tuple, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return "", util.HTTPUnauthorized("basic authorization not base64 encoded", "error", err)
	}

	parts := strings.Split(string(tuple), ":")
	if len(parts) != 2 {
		return "", util.HTTPUnauthorized("basic authorization malformed")
	}

	username := parts[0]
	password := parts[1]

	providerClient, err := openstack.NewClient(a.Endpoint)
	if err != nil {
		return "", util.HTTPInternalServerError("unable to initialize provider client", "error", err)
	}

	serviceClient, err := openstack.NewIdentityV3(providerClient, gophercloud.EndpointOpts{})
	if err != nil {
		return "", util.HTTPInternalServerError("unable to initialize service client", "error", err)
	}

	// Do an unscoped authentication against Keystone.  The client is expected
	// to list visible projects (or indeed cache them in local web-storage) then
	// use that to do a scoped bearer token based upgrade.
	options := &tokens.AuthOptions{
		IdentityEndpoint: a.Endpoint,
		DomainName:       a.Domain,
		Username:         username,
		Password:         password,
	}

	result := tokens.Create(serviceClient, options)

	keystoneToken, err := result.ExtractToken()
	if err != nil {
		return "", util.HTTPUnauthorized("keystone authentication failed", "error", err)
	}

	jwToken, err := a.Issuer.Issue(r, username, keystoneToken.ID, nil, keystoneToken.ExpiresAt)
	if err != nil {
		return "", util.HTTPInternalServerError("unable to create JWT", "error", err)
	}

	return jwToken, nil
}
