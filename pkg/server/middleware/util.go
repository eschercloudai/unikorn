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

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"

	"github.com/eschercloudai/unikorn/pkg/server/errors"
	"github.com/eschercloudai/unikorn/pkg/server/generated"
)

// openAPI abstracts schema access and validation.
type openAPI struct {
	// spec is the full specification.
	spec *openapi3.T

	// router is a router able to process requests and return the
	// route from the spec.
	router routers.Router
}

// newOpenRpi extracts the swagger document.
func newOpenAPI() (*openAPI, error) {
	spec, err := generated.GetSwagger()
	if err != nil {
		return nil, errors.OAuth2ServerError("unable to decode openapi schema").WithError(err)
	}

	router, err := gorillamux.NewRouter(spec)
	if err != nil {
		return nil, errors.OAuth2ServerError("unable to create router").WithError(err)
	}

	o := &openAPI{
		spec:   spec,
		router: router,
	}

	return o, nil
}

// findRoute looks up the route from the specification.
func (o *openAPI) findRoute(r *http.Request) (*routers.Route, map[string]string, error) {
	route, params, err := o.router.FindRoute(r)
	if err != nil {
		return nil, nil, errors.OAuth2ServerError("unable to find route").WithError(err)
	}

	// TODO: move to static validation.
	if route.Operation.Security == nil {
		return nil, nil, errors.OAuth2ServerError("no security requirements set")
	}

	return route, params, nil
}
