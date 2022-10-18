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

package retry

import (
	"context"
	"time"
)

// RetryFunc is a callback that must return nil to escape the retry loop.
type RetryFunc func() error

// Retrier implements retry loop logic.
type Retrier struct {
	// context is used to terminate the retry loop on either a timeout
	// or a cancellation call from another routine.  See WithContext()
	// and WithTimeout for additional behaviour.  If not set it will
	// retry forever.
	context context.Context

	// cancel is associated with a context to free resources.
	cancel func()

	// period defines the default retry period, defaulting to 1 second.
	period time.Duration
}

// Froever returns a retrier that will retry soething forever until a nil error
// is returned.
func Forever() *Retrier {
	return &Retrier{
		context: context.TODO(),
		period:  time.Second,
	}
}

// WithContext allows a global context to be registered with this retry function,
// e.g. if a timeout spans the whole transaction, and not just this single retry.
func WithContext(c context.Context) *Retrier {
	return &Retrier{
		context: c,
		period:  time.Second,
	}
}

// WithTimeout returns a retrier that will execute for a specifc length of time.
func WithTimeout(timeout time.Duration) *Retrier {
	c, cancel := context.WithTimeout(context.TODO(), timeout)

	return &Retrier{
		context: c,
		cancel:  cancel,
		period:  time.Second,
	}
}

// WithPeriod defines how often to perform the retry.
func (r *Retrier) WithPeriod(period time.Duration) *Retrier {
	r.period = period
	return r
}

// WithTimeout wraps the existing context with a timeout specific to this retry
// invocation.  This should only be used with WithContext(ctx).WithTimeout() to
// augment a global timeout with a local one as this call does not respect existing
// cancel functions.
func (r *Retrier) WithTimeout(timeout time.Duration) *Retrier {
	r.context, r.cancel = context.WithTimeout(r.context, timeout)
	return r
}

// Do starts the retry loop.  It will run until a context times out or is cancelled,
// or the retry function returns nil indicating success.
func (r *Retrier) Do(f RetryFunc) error {
	if r.cancel != nil {
		defer r.cancel()
	}

	t := time.NewTicker(r.period)
	defer t.Stop()

	for {
		select {
		case <-r.context.Done():
			return r.context.Err()
		case <-t.C:
			if err := f(); err != nil {
				break
			}

			return nil
		}
	}
}
