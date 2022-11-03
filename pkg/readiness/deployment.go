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
	"errors"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	ErrDeploymentUnready = errors.New("deployment readiness doesn't match desired")
)

type Deployment struct {
	// client is an intialized Kubernetes client.
	client client.Client

	// namespace is the namespace a resource resides in.
	namespace string

	// name is the name of the resource.
	name string
}

// Ensure the Check interface is implemented.
var _ Check = &Deployment{}

// NewDeployment creates a new deployment readiness check.
func NewDeployment(client client.Client, namespace, name string) *Deployment {
	return &Deployment{
		client:    client,
		namespace: namespace,
		name:      name,
	}
}

// Check implements the Check interface.
func (r *Deployment) Check(ctx context.Context) error {
	deployment := &appsv1.Deployment{}
	if err := r.client.Get(ctx, client.ObjectKey{Namespace: r.namespace, Name: r.name}, deployment); err != nil {
		return fmt.Errorf("deployment get error: %w", err)
	}

	if deployment.Status.ReadyReplicas != deployment.Status.Replicas {
		return fmt.Errorf("%w: status mismatch", ErrDeploymentUnready)
	}

	return nil
}
