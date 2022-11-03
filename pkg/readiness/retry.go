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

package readiness

import (
	"context"

	"github.com/eschercloudai/unikorn/pkg/util/retry"
)

// Retry wraps a readiness check in a retry loop.
type Retry struct {
	// delegate is a backend readiness check to be retried.
	delegate Check
}

// Ensure the Check interface is implemented.
var _ Check = &Retry{}

// NewRetry returns a new readiness check that will retry.
func NewRetry(delegate Check) *Retry {
	return &Retry{
		delegate: delegate,
	}
}

// Check implements the Check interface.
func (r *Retry) Check(ctx context.Context) error {
	ready := func() error {
		return r.delegate.Check(ctx)
	}

	if err := retry.Forever().DoWithContext(ctx, ready); err != nil {
		return err
	}

	return nil
}
