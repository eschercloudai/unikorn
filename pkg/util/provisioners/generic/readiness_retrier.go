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

package generic

import (
	"context"

	"github.com/eschercloudai/unikorn/pkg/util/retry"
)

// ReadinessCheckWithRetry wraps a readiness check in a retry loop.
type ReadinessCheckWithRetry struct {
	// delegate is a backend readiness check to be retried.
	delegate ReadinessCheck
}

// Ensure the ReadinessCheck interface is implemented.
var _ ReadinessCheck = &ReadinessCheckWithRetry{}

// NewReadinessCheckWithRetry returns a new readiness check that will retry.
func NewReadinessCheckWithRetry(delegate ReadinessCheck) *ReadinessCheckWithRetry {
	return &ReadinessCheckWithRetry{
		delegate: delegate,
	}
}

// Check implements the ReadinessCheck interface.
func (r *ReadinessCheckWithRetry) Check(ctx context.Context) error {
	ready := func() error {
		return r.delegate.Check(ctx)
	}

	if err := retry.Forever().DoWithContext(ctx, ready); err != nil {
		return err
	}

	return nil
}
