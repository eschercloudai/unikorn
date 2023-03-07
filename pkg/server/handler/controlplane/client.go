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

package controlplane

import (
	"context"
	"sort"

	unikornv1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/constants"
	"github.com/eschercloudai/unikorn/pkg/server/errors"
	"github.com/eschercloudai/unikorn/pkg/server/generated"
	"github.com/eschercloudai/unikorn/pkg/server/handler/project"

	corev1 "k8s.io/api/core/v1"
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

// Meta describes the control plane.
type Meta struct {
	// Project is the owning project's metadata.
	Project *project.Meta

	// Name is the project's Kubernetes name, so a higher level resource
	// can reference it.
	Name string

	// Active defines whether the control plane is ready to be used: it's not
	// marked for deletion, and it's active according to the controller.
	// TODO: should we inherit the project's inactive status too?
	Active bool

	// Namespace is the namespace that is provisioned by the control plane.
	// Should be usable and set when the project is active.
	Namespace string
}

// active returns true if the project is usable.
func active(c *unikornv1.ControlPlane) bool {
	// Being deleted, don't use.
	// Takes precedence over condition as there's a delay between the resource
	// being deleted, and the controller acknoledging it.
	if c.DeletionTimestamp != nil {
		return false
	}

	// Unknown condition, don't use.
	condition, err := c.LookupCondition(unikornv1.ControlPlaneConditionAvailable)
	if err != nil {
		return false
	}

	// Condition not provisioned, don't use.
	if condition.Status != corev1.ConditionTrue {
		return false
	}

	return true
}

// Metadata retrieves the control plane metadata.
func (c *Client) Metadata(ctx context.Context, name string) (*Meta, error) {
	project, err := project.NewClient(c.client).Metadata(ctx)
	if err != nil {
		return nil, err
	}

	result, err := c.get(ctx, project.Namespace, name)
	if err != nil {
		return nil, err
	}

	metadata := &Meta{
		Project:   project,
		Name:      name,
		Active:    active(result),
		Namespace: result.Status.Namespace,
	}

	return metadata, nil
}

// convert converts from Kubernetes into OpenAPI types.
func convert(in *unikornv1.ControlPlane) *generated.ControlPlane {
	out := &generated.ControlPlane{
		Status: &generated.KubernetesResourceStatus{
			Name:         in.Name,
			CreationTime: in.CreationTimestamp.Time,
			Status:       "Unknown",
		},
		Name:              in.Name,
		ApplicationBundle: *in.Spec.ApplicationBundle,
	}

	if in.DeletionTimestamp != nil {
		out.Status.DeletionTime = &in.DeletionTimestamp.Time
	}

	if condition, err := in.LookupCondition(unikornv1.ControlPlaneConditionAvailable); err == nil {
		out.Status.Status = string(condition.Reason)
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
	project, err := project.NewClient(c.client).Metadata(ctx)
	if err != nil {
		return nil, err
	}

	result := &unikornv1.ControlPlaneList{}

	if err := c.client.List(ctx, result, &client.ListOptions{Namespace: project.Namespace}); err != nil {
		return nil, errors.OAuth2ServerError("failed to list control planes").WithError(err)
	}

	sort.Stable(result)

	return convertList(result), nil
}

// get returns the implicit control plane identified by the JWT claims.
func (c *Client) get(ctx context.Context, namespace, name string) (*unikornv1.ControlPlane, error) {
	result := &unikornv1.ControlPlane{}

	if err := c.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, result); err != nil {
		if kerrors.IsNotFound(err) {
			return nil, errors.HTTPNotFound()
		}

		return nil, errors.OAuth2ServerError("failed to get control plane").WithError(err)
	}

	return result, nil
}

// Get returns the implicit control plane identified by the JWT claims.
func (c *Client) Get(ctx context.Context, name generated.ControlPlaneNameParameter) (*generated.ControlPlane, error) {
	project, err := project.NewClient(c.client).Metadata(ctx)
	if err != nil {
		return nil, err
	}

	result, err := c.get(ctx, project.Namespace, name)
	if err != nil {
		return nil, err
	}

	return convert(result), nil
}

// Create creates the implicit control plane indentified by the JTW claims.
func (c *Client) Create(ctx context.Context, request *generated.ControlPlane) error {
	project, err := project.NewClient(c.client).Metadata(ctx)
	if err != nil {
		return err
	}

	if !project.Active {
		return errors.OAuth2InvalidRequest("project is not active")
	}

	// TODO: common with CLI tools.
	controlPlane := &unikornv1.ControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      request.Name,
			Namespace: project.Namespace,
			Labels: map[string]string{
				constants.VersionLabel: constants.Version,
				constants.ProjectLabel: project.Name,
			},
		},
		Spec: unikornv1.ControlPlaneSpec{
			ApplicationBundle: &request.ApplicationBundle,
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
func (c *Client) Delete(ctx context.Context, name generated.ControlPlaneNameParameter) error {
	project, err := project.NewClient(c.client).Metadata(ctx)
	if err != nil {
		return err
	}

	if !project.Active {
		return errors.OAuth2InvalidRequest("project is not active")
	}

	controlPlane := &unikornv1.ControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: project.Namespace,
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
