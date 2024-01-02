/*
Copyright 2022-2024 EscherCloud.

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

package retry

import (
	"context"
	"fmt"
	"time"
)

// Callback is a callback that must return nil to escape the retry loop.
type Callback func() error

// Retrier implements retry loop logic.
type Retrier struct {
	// period defines the default retry period, defaulting to 1 second.
	period time.Duration
}

// Froever returns a retrier that will retry soething forever until a nil error
// is returned.
func Forever() *Retrier {
	return &Retrier{
		period: time.Second,
	}
}

// Do starts the retry loop.  It will run until success or until an optional
// timeout expires.
func (r *Retrier) Do(f Callback) error {
	return r.DoWithContext(context.TODO(), f)
}

// DoWithContext allows you to use a global context to interrupt execution.
func (r *Retrier) DoWithContext(c context.Context, f Callback) error {
	// Check immediately to avoid a delay of period.
	rerr := f()
	if rerr == nil {
		return nil
	}

	t := time.NewTicker(r.period)
	defer t.Stop()

	for {
		select {
		case <-c.Done():
			// NOTE: we wrap the context error here, knowing it's a timeout
			// is more important than the last callback error, because of
			// reconcile yielding code.  If this becomes a problem, we may
			// need to define our own "expired" error type, that can wrap
			// the underlying error.
			//nolint:errorlint
			return fmt.Errorf("%w: %s", c.Err(), rerr)
		case <-t.C:
			if rerr = f(); rerr != nil {
				break
			}

			return nil
		}
	}
}
