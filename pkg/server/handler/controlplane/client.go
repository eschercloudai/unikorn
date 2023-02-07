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

package controlplane

import (
	"context"

	unikornv1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/constants"
	"github.com/eschercloudai/unikorn/pkg/server/errors"
	"github.com/eschercloudai/unikorn/pkg/server/generated"
	"github.com/eschercloudai/unikorn/pkg/server/handler/project"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Client wraps up control plane related management handling.
type Client struct {
	// client allows Kubernetes API access.
	client client.Client
}

// NewClient returns a new client with required parameters.
func NewClient(client client.Client) *Client {
	return &Client{
		client: client,
	}
}

// convert converts from Kubernetes into OpenAPI types.
func convert(in *unikornv1.ControlPlane) *generated.ControlPlane {
	out := &generated.ControlPlane{
		Name:         in.Name,
		CreationTime: in.CreationTimestamp.Time,
		Status:       "Unknown",
	}

	if in.DeletionTimestamp != nil {
		out.DeletionTime = &in.DeletionTimestamp.Time
	}

	if condition, err := in.LookupCondition(unikornv1.ControlPlaneConditionAvailable); err == nil {
		out.Status = string(condition.Reason)
	}

	return out
}

// convertList converts from Kubernetes into OpenAPI types.
func convertList(in *unikornv1.ControlPlaneList) []*generated.ControlPlane {
	out := make([]*generated.ControlPlane, len(in.Items))

	for i := range in.Items {
		out[i] = convert(&in.Items[i])
	}

	return out
}

// List returns all control planes owned by the implicit control plane.
func (c *Client) List(ctx context.Context) ([]*generated.ControlPlane, error) {
	namespace, err := project.NewClient(c.client).Namespace(ctx)
	if err != nil {
		return nil, err
	}

	result := &unikornv1.ControlPlaneList{}

	if err := c.client.List(ctx, result, &client.ListOptions{Namespace: namespace}); err != nil {
		return nil, errors.OAuth2ServerError("failed to list control planes").WithError(err)
	}

	return convertList(result), nil
}

// Get returns the implicit control plane identified by the JWT claims.
func (c *Client) Get(ctx context.Context, name generated.ControlPlaneParameter) (*generated.ControlPlane, error) {
	namespace, err := project.NewClient(c.client).Namespace(ctx)
	if err != nil {
		return nil, err
	}

	result := &unikornv1.ControlPlane{}

	if err := c.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, result); err != nil {
		if kerrors.IsNotFound(err) {
			return nil, errors.HTTPNotFound()
		}

		return nil, errors.OAuth2ServerError("failed to get control plane").WithError(err)
	}

	return convert(result), nil
}

// Create creates the implicit control plane indentified by the JTW claims.
func (c *Client) Create(ctx context.Context, request *generated.CreateControlPlane) error {
	namespace, err := project.NewClient(c.client).Namespace(ctx)
	if err != nil {
		return err
	}

	projectName, err := project.NameFromContext(ctx)
	if err != nil {
		return err
	}

	// TODO: common with CLI tools.
	controlPlane := &unikornv1.ControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      request.Name,
			Namespace: namespace,
			Labels: map[string]string{
				constants.VersionLabel: constants.Version,
				constants.ProjectLabel: projectName,
			},
		},
	}

	if err := c.client.Create(ctx, controlPlane); err != nil {
		// TODO: we can do a cached lookup to save the API traffic.
		if kerrors.IsAlreadyExists(err) {
			return errors.HTTPConflict()
		}

		return errors.OAuth2ServerError("failed to create control plane").WithError(err)
	}

	return nil
}

// Delete deletes the implicit control plane indentified by the JTW claims.
func (c *Client) Delete(ctx context.Context, name generated.ControlPlaneParameter) error {
	namespace, err := project.NewClient(c.client).Namespace(ctx)
	if err != nil {
		return err
	}

	// TODO: common with CLI tools.
	controlPlane := &unikornv1.ControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	if err := c.client.Delete(ctx, controlPlane); err != nil {
		if kerrors.IsNotFound(err) {
			return errors.HTTPNotFound()
		}

		return errors.OAuth2ServerError("failed to delete control plane").WithError(err)
	}

	return nil
}
