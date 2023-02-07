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
	"bytes"
	"context"
	"io"
	"net/http"

	"github.com/getkin/kin-openapi/openapi3filter"

	"github.com/eschercloudai/unikorn/pkg/server/authorization"
	"github.com/eschercloudai/unikorn/pkg/server/errors"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

// OpenAPIValidator provides OpenAPI validation of request and response codes,
// media, and schema validation of payloads to ensure we are meeting the
// specification.
type OpenAPIValidator struct {
	// next defines the next HTTP handler in the chain.
	next http.Handler

	// authorizer provides security policy enforcement.
	authorizer *Authorizer
}

// Ensure this implements the required interfaces.
var _ http.Handler = &OpenAPIValidator{}

// NewOpenAPIValidator returns an initialized validator middleware.
func NewOpenAPIValidator(authorizer *Authorizer) *OpenAPIValidator {
	return &OpenAPIValidator{
		authorizer: authorizer,
	}
}

// bufferingResponseWriter saves the response code and body so that we can
// validate them.
type bufferingResponseWriter struct {
	// next is the parent handler.
	next http.ResponseWriter

	// code is the HTTP status code.
	code int

	// body is a copy of the HTTP response body.
	// This valus will be nil if no body was written.
	body io.ReadCloser
}

// Ensure the correct interfaces are implmeneted.
var _ http.ResponseWriter = &bufferingResponseWriter{}

// Header returns the HTTP headers.
func (w *bufferingResponseWriter) Header() http.Header {
	return w.next.Header()
}

// Write writes out a body, if WriteHeader has not been called this will
// be done with a 200 status code.
func (w *bufferingResponseWriter) Write(body []byte) (int, error) {
	buf := &bytes.Buffer{}
	buf.Write(body)

	w.body = io.NopCloser(buf)

	return w.next.Write(body)
}

// WriteHeader writes out the HTTP headers with the provided status code.
func (w *bufferingResponseWriter) WriteHeader(statusCode int) {
	w.code = statusCode

	w.next.WriteHeader(statusCode)
}

// StatusCode calculates the status code returned to the client.
func (w *bufferingResponseWriter) StatusCode() int {
	if w.code == 0 {
		return http.StatusOK
	}

	return w.code
}

// ServeHTTP implements the http.Handler interface.
func (v *OpenAPIValidator) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log := log.FromContext(r.Context())

	openapi, err := newOpenAPI()
	if err != nil {
		errors.HandleError(w, r, err)

		return
	}

	route, params, err := openapi.findRoute(r)
	if err != nil {
		errors.HandleError(w, r, err)

		return
	}

	authContext := &authorizationContext{}

	authorizationFunc := func(ctx context.Context, input *openapi3filter.AuthenticationInput) error {
		err := v.authorizer.authorizeScheme(authContext, input.RequestValidationInput.Request, input.SecurityScheme, input.Scopes)

		authContext.err = err

		return err
	}

	options := &openapi3filter.Options{
		IncludeResponseStatus: true,
		AuthenticationFunc:    authorizationFunc,
	}

	requestOptions := &openapi3filter.RequestValidationInput{
		Request:    r,
		PathParams: params,
		Route:      route,
		Options:    options,
	}

	if err := openapi3filter.ValidateRequest(r.Context(), requestOptions); err != nil {
		if authContext.err != nil {
			errors.HandleError(w, r, authContext.err)

			return
		}

		errors.OAuth2InvalidRequest("request invalid").WithError(err).Write(w, r)

		return
	}

	// Add any contextual information to bubble up to the handler.
	c := authorization.NewContextWithClaims(r.Context(), authContext.claims)
	r = r.WithContext(c)

	// Override the writer so we can inspect the contents and status.
	writer := &bufferingResponseWriter{
		next: w,
	}

	v.next.ServeHTTP(writer, r)

	responseOptions := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: requestOptions,
		Status:                 writer.StatusCode(),
		Header:                 writer.Header(),
		Body:                   writer.body,
		Options:                options,
	}

	if err := openapi3filter.ValidateResponse(c, responseOptions); err != nil {
		log.Error(err, "response openapi schema validation failure")
	}
}

// Middleware performs any authorization handling middleware.
func (v *OpenAPIValidator) Middleware(next http.Handler) http.Handler {
	v.next = next

	return v
}
