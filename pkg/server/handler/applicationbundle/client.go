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

package applicationbundle

import (
	"context"
	"slices"

	unikornv1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/server/errors"
	"github.com/eschercloudai/unikorn/pkg/server/generated"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Client wraps up application bundle related management handling.
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

func convert(in *unikornv1.ApplicationBundle) *generated.ApplicationBundle {
	out := &generated.ApplicationBundle{
		Name:    in.Name,
		Version: *in.Spec.Version,
		Preview: in.Spec.Preview,
	}

	if in.Spec.EndOfLife != nil {
		out.EndOfLife = &in.Spec.EndOfLife.Time
	}

	return out
}

func convertList(in []unikornv1.ApplicationBundle) []*generated.ApplicationBundle {
	out := make([]*generated.ApplicationBundle, len(in))

	for i := range in {
		out[i] = convert(&in[i])
	}

	return out
}

func (c *Client) listByKind(ctx context.Context, kind unikornv1.ApplicationBundleResourceKind) ([]*generated.ApplicationBundle, error) {
	result := &unikornv1.ApplicationBundleList{}

	if err := c.client.List(ctx, result); err != nil {
		return nil, errors.OAuth2ServerError("failed to list application bundles").WithError(err)
	}

	resultByKind := result.ByKind(kind)

	slices.SortStableFunc(resultByKind.Items, unikornv1.CompareApplicationBundle)

	return convertList(resultByKind.Items), nil
}

// TODO: while not exposed, it doesn't check the kind yet, which could be seen as
// an escalation of sorts.
func (c *Client) Get(ctx context.Context, name string) (*generated.ApplicationBundle, error) {
	result := &unikornv1.ApplicationBundle{}

	if err := c.client.Get(ctx, client.ObjectKey{Name: name}, result); err != nil {
		return nil, errors.HTTPNotFound().WithError(err)
	}

	return convert(result), nil
}

func (c *Client) ListControlPlane(ctx context.Context) ([]*generated.ApplicationBundle, error) {
	return c.listByKind(ctx, unikornv1.ApplicationBundleResourceKindControlPlane)
}

func (c *Client) ListCluster(ctx context.Context) ([]*generated.ApplicationBundle, error) {
	return c.listByKind(ctx, unikornv1.ApplicationBundleResourceKindKubernetesCluster)
}
