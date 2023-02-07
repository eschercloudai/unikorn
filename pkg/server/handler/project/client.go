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

package project

import (
	"context"
	"fmt"

	unikornv1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/constants"
	"github.com/eschercloudai/unikorn/pkg/server/authorization"
	"github.com/eschercloudai/unikorn/pkg/server/errors"
	"github.com/eschercloudai/unikorn/pkg/server/generated"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Client wraps up project related management handling.
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

// NameFromContext translates an Openstack project ID to one we an use.
func NameFromContext(ctx context.Context) (string, error) {
	claims, err := authorization.ClaimsFromContext(ctx)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("unikorn-server-%s", claims.UnikornClaims.Project), nil
}

// get returns the implicit project identified by the JWT claims.
func (c *Client) get(ctx context.Context) (*unikornv1.Project, error) {
	name, err := NameFromContext(ctx)
	if err != nil {
		return nil, err
	}

	result := &unikornv1.Project{}

	if err := c.client.Get(ctx, client.ObjectKey{Name: name}, result); err != nil {
		if kerrors.IsNotFound(err) {
			return nil, errors.HTTPNotFound()
		}

		return nil, errors.OAuth2ServerError("failed to get project").WithError(err)
	}

	return result, err
}

// Namespace retrieves the project namespace.
func (c *Client) Namespace(ctx context.Context) (string, error) {
	result, err := c.get(ctx)
	if err != nil {
		return "", err
	}

	if result.DeletionTimestamp != nil {
		return "", errors.OAuth2InvalidRequest("project is marked for deletion")
	}

	if result.Status.Namespace == "" {
		return "", errors.OAuth2ServerError("project namespace not set")
	}

	return result.Status.Namespace, nil
}

// Get returns the implicit project identified by the JWT claims.
func (c *Client) Get(ctx context.Context) (*generated.Project, error) {
	result, err := c.get(ctx)
	if err != nil {
		return nil, err
	}

	project := &generated.Project{
		Name:         result.Name,
		CreationTime: result.CreationTimestamp.Time,
		Status:       "Unknown",
	}

	if result.DeletionTimestamp != nil {
		project.DeletionTime = &result.DeletionTimestamp.Time
	}

	if condition, err := result.LookupCondition(unikornv1.ProjectConditionAvailable); err == nil {
		project.Status = string(condition.Reason)
	}

	return project, nil
}

// Create creates the implicit project indentified by the JTW claims.
func (c *Client) Create(ctx context.Context) error {
	name, err := NameFromContext(ctx)
	if err != nil {
		return err
	}

	// TODO: common with CLI tools.
	project := &unikornv1.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				constants.VersionLabel: constants.Version,
			},
		},
	}

	if err := c.client.Create(ctx, project); err != nil {
		// TODO: we can do a cached lookup to save the API traffic.
		if kerrors.IsAlreadyExists(err) {
			return errors.HTTPConflict()
		}

		return errors.OAuth2ServerError("failed to create project").WithError(err)
	}

	return nil
}

// Delete deletes the implicit project indentified by the JTW claims.
func (c *Client) Delete(ctx context.Context) error {
	name, err := NameFromContext(ctx)
	if err != nil {
		return err
	}

	// TODO: common with CLI tools.
	project := &unikornv1.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	if err := c.client.Delete(ctx, project); err != nil {
		if kerrors.IsNotFound(err) {
			return errors.HTTPNotFound()
		}

		return errors.OAuth2ServerError("failed to delete project").WithError(err)
	}

	return nil
}
