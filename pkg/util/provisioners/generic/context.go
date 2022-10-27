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
	"errors"
)

var (
	// ErrProvisionerStatusNotSet is returned when an attempt to get the
	// provisioner status is made with GetStatus, but no corresponding
	// call to WithStatus has been made.
	ErrProvisionerStatusNotSet = errors.New("provisioner status not set")

	// ErrProvisionerStatusTypeMismatch is returned if a type assertion
	// fails.
	ErrProvisionerStatusTypeMismatch = errors.New("provisioner status type mismatch")
)

type ProvisionerStatusKeyType string

const (
	// provisionerStatusKey is the key to associate a status object with
	// in a context.
	provisionerStatusKey ProvisionerStatusKeyType = "provisioner-status"
)

// ProvisionerStatus allows provisioners to communicate with top level
// managers what's been going on.
type ProvisionerStatus struct {
	// Provisioned indicates that something was provisioned.
	Provisioned bool
}

// WithStatus attaches a provisioner status to a new context.
func WithStatus(ctx context.Context) (context.Context, *ProvisionerStatus) {
	status := &ProvisionerStatus{}

	return context.WithValue(ctx, provisionerStatusKey, status), status
}

// GetStatus fetches a reference to a provisioner status.
func GetStatus(ctx context.Context) (*ProvisionerStatus, error) {
	value := ctx.Value(provisionerStatusKey)
	if value == nil {
		return nil, ErrProvisionerStatusNotSet
	}

	status, ok := value.(*ProvisionerStatus)
	if !ok {
		return nil, ErrProvisionerStatusTypeMismatch
	}

	return status, nil
}
