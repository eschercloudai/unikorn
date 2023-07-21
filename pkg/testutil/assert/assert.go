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

package assert

import (
	"errors"
	"net/http"
	"testing"
)

// Equal is a terse way of checking some assumption holds, offing itself if it doesn't.
func Equal[T comparable](t *testing.T, expected, actual T) {
	t.Helper()

	if expected != actual {
		t.Fatalf("assertion failure: expected %v, got %v", expected, actual)
	}
}

// NotEqual is a terse way of checking some assumption doesn't hold, offing itself if it does.
func NotEqual[T comparable](t *testing.T, expected, actual T) {
	t.Helper()

	if expected == actual {
		t.Fatalf("assertion failure: got %v unexpectedly", expected)
	}
}

// NilError is a terse way of crapping out if an error occurred.
func NilError(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Fatalf("assertion failure: unexpected error: %v", err)
	}
}

// NotNil is a terse way of crapping out if a pointer is nil.
func NotNil[T any](t *testing.T, v *T) {
	t.Helper()

	if v == nil {
		t.Fatalf("assertion failure: nil pointer")
	}
}

// MapSet checks a map exists, and also that the named key is present.
func MapSet[T any](t *testing.T, m map[string]T, key string) {
	t.Helper()

	if m == nil {
		t.Fatalf("assertion failure: map undefined")
	}

	if _, ok := m[key]; !ok {
		t.Fatalf("assertion failure: missing map key %s", key)
	}
}

// Error is a terse way of crapping out if an error didn't occur.
func Error(t *testing.T, expected, err error) {
	t.Helper()

	if err == nil {
		t.Fatalf("assertion failure: error expected")
	}

	if !errors.Is(err, expected) {
		t.Fatalf("assertion failure: error type mismatch: %v", err)
	}
}

// KubernetesError is a terse way of crapping out if an error didn't occur.
func KubernetesError(t *testing.T, callback func(error) bool, err error) {
	t.Helper()

	if err == nil {
		t.Fatalf("assertion failure: error expected")
	}

	if !callback(err) {
		t.Fatalf("assertion failure: error type mismatch: %v", err)
	}
}

// HTTPResponse is a terse way of crapping out when the response is not
// as intended.
func HTTPResponse(t *testing.T, response *http.Response, statusCode int, err error) {
	t.Helper()

	NilError(t, err)
	Equal(t, statusCode, response.StatusCode)
}
