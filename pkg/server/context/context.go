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

package context

import (
	"context"

	"github.com/eschercloudai/unikorn/pkg/server/errors"
)

// contextKey defines a new context key type unique to this package.
type contextKey string

const (
	// subjectKey is the key used to store the request subject (user).
	subjectKey contextKey = "subject"

	// tokenKey is the key used to store an Openstack token.
	tokenKey contextKey = "token"
)

// NewContext stores the value into a new context.
func newContextString(ctx context.Context, key contextKey, s string) context.Context {
	if s == "" {
		return ctx
	}

	return context.WithValue(ctx, key, s)
}

// fromContextString lookups a key and tries to convert to a string.
func fromContextString(ctx context.Context, key contextKey) (string, error) {
	value := ctx.Value(key)
	if value == nil {
		return "", errors.OAuth2ServerError("context key not present").WithValues("key", key)
	}

	s, ok := value.(string)
	if !ok {
		return "", errors.OAuth2ServerError("context value not a string").WithValues("key", key)
	}

	return s, nil
}

// NewContextWithSubject adds a value to the context.
func NewContextWithSubject(ctx context.Context, value string) context.Context {
	return newContextString(ctx, subjectKey, value)
}

// SubjectFromContext extracts the value from the context.
func SubjectFromContext(ctx context.Context) (string, error) {
	return fromContextString(ctx, subjectKey)
}

// NewContextWithToken adds a value to the context.
func NewContextWithToken(ctx context.Context, value string) context.Context {
	return newContextString(ctx, tokenKey, value)
}

// TokenFromContext extracts the value from the context.
func TokenFromContext(ctx context.Context) (string, error) {
	return fromContextString(ctx, tokenKey)
}
