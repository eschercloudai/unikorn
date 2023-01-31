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

package util

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/eschercloudai/unikorn/pkg/server/generated"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	// ErrRequest is raised for all handler errors.
	ErrRequest = errors.New("request error")
)

// HTTPError wraps ErrRequest with more contextual information that is used to
// propagate and create suitable responses.
type HTTPError struct {
	// code is the HTTP error code.
	code int

	// message is a verbose message to log/return to the user.
	message string

	// values are key value pairs for logging.
	values []interface{}
}

// newHTTPError returns a new HTTP error.
func newHTTPError(code int, message string, values ...interface{}) *HTTPError {
	return &HTTPError{
		code:    code,
		message: message,
		values:  values,
	}
}

// Unwrap implements Go 1.13 errors.
func (e *HTTPError) Unwrap() error {
	return ErrRequest
}

// Error implements the error interface.
func (e *HTTPError) Error() string {
	return e.message
}

// Write returns the error code and message to the client.
func (e *HTTPError) Write(w http.ResponseWriter, r *http.Request) {
	log := log.FromContext(r.Context())

	ge := &generated.GenericError{
		Description: e.message,
	}

	w.Header().Add("Content-Type", "application/json")

	w.WriteHeader(e.code)

	body, err := json.Marshal(ge)
	if err != nil {
		log.Error(err, "failed to marshal error response")

		return
	}

	if _, err := w.Write(body); err != nil {
		log.Error(err, "failed to wirte error response")

		return
	}
}

// HTTPUnauthorized wraps up a 401 error.
func HTTPUnauthorized(message string, values ...interface{}) *HTTPError {
	return newHTTPError(http.StatusUnauthorized, message, values...)
}

// HTTPUnauthorizedWithError wraps up a 401 error.
func HTTPUnauthorizedWithError(err error, message string, values ...interface{}) *HTTPError {
	values = append(values, "detail", err.Error())

	return HTTPUnauthorized(message, values...)
}

// HTTPInternalServerError wraps up a 500 error.
func HTTPInternalServerError(message string, values ...interface{}) *HTTPError {
	return newHTTPError(http.StatusInternalServerError, message, values...)
}

// HTTPInternalServerErrorWithError wraps up a 500 error.
func HTTPInternalServerErrorWithError(err error, message string, values ...interface{}) *HTTPError {
	values = append(values, "detail", err.Error())

	return HTTPInternalServerError(message, values...)
}

// toHTTPError is a handy unwrapper to get a HTTP error from a generic one.
func toHTTPError(err error) *HTTPError {
	var httpErr *HTTPError

	if !errors.As(err, &httpErr) {
		return nil
	}

	return httpErr
}

// HandleError is the top level error handler that should be called from all
// path handlers on error.
func HandleError(w http.ResponseWriter, r *http.Request, err error) {
	log := log.FromContext(r.Context())

	log.Info("raising error", LogValues(err)...)

	if httpError := toHTTPError(err); httpError != nil {
		httpError.Write(w, r)

		return
	}

	// TODO: need to watch for logging errors in CI/CD and fix them.
	log.Error(err, "error not wrapped as HTTPError type")
	HTTPInternalServerError("unhandled error").Write(w, r)
}

// LogValues gets a key/value set of values for logging context.
func LogValues(err error) []interface{} {
	if err := toHTTPError(err); err != nil {
		return append(err.values, "error", err.message)
	}

	return []interface{}{"error", err.Error()}
}
