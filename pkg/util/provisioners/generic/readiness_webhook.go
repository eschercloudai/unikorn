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

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// WebhookReady does a dry run create and fails of the webhook isn't functional yet.
type WebhookReady struct {
	// client allows Kubernetes API access.
	client client.Client

	// object is an unstructured object that should be accepted by a dry run create
	// through the webhook.
	object *unstructured.Unstructured
}

// Ensure the ReadinessCheck interface is implemented.
var _ ReadinessCheck = &WebhookReady{}

// NewWebhookReady returns a new readiness check that will retry.
func NewWebhookReady(client client.Client, object *unstructured.Unstructured) *WebhookReady {
	return &WebhookReady{
		client: client,
		object: object,
	}
}

// Check implements the ReadinessCheck interface.
func (r *WebhookReady) Check(ctx context.Context) error {
	return r.client.Create(ctx, r.object, client.DryRunAll)
}
