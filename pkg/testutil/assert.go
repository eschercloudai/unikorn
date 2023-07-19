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

package testutil

import (
	"errors"
	"net/http"
	"testing"
)

// AssertEqual is a terse way of checking some assumption holds, offing itself if it doesn't.
func AssertEqual[T comparable](t *testing.T, expected, actual T) {
	t.Helper()

	if expected != actual {
		t.Fatalf("assertion failure: expected %v, got %v", expected, actual)
	}
}

// AssertNotEqual is a terse way of checking some assumption doesn't hold, offing itself if it does.
func AssertNotEqual[T comparable](t *testing.T, expected, actual T) {
	t.Helper()

	if expected == actual {
		t.Fatalf("assertion failure: got %v unexpectedly", expected)
	}
}

// AssertNilError is a terse way of crapping out if an error occurred.
func AssertNilError(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Fatalf("assertion failure: unexpected error: %v", err)
	}
}

// AssertNotNil is a terse way of crapping out if a pointer is nil.
func AssertNotNil[T any](t *testing.T, v *T) {
	t.Helper()

	if v == nil {
		t.Fatalf("assertion failure: nil pointer")
	}
}

// AssertError is a terse way of crapping out if an error didn't occur.
func AssertError(t *testing.T, expected, err error) {
	t.Helper()

	if err == nil {
		t.Fatalf("assertion failure: error expected")
	}

	if !errors.Is(err, expected) {
		t.Fatalf("assertion failure: error type mismatch: %v", err)
	}
}

// AssertKubernetesError is a terse way of crapping out if an error didn't occur.
func AssertKubernetesError(t *testing.T, callback func(error) bool, err error) {
	t.Helper()

	if err == nil {
		t.Fatalf("assertion failure: error expected")
	}

	if !callback(err) {
		t.Fatalf("assertion failure: error type mismatch: %v", err)
	}
}

// AssertHTTPResponse is a terse way of crapping out when the response is not
// as intended.
func AssertHTTPResponse(t *testing.T, response *http.Response, statusCode int, err error) {
	t.Helper()

	AssertNilError(t, err)
	AssertEqual(t, statusCode, response.StatusCode)
}
