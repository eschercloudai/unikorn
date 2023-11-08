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

package application

import (
	"context"
	"slices"
	"strings"

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

func convert(in *unikornv1.HelmApplication) *generated.Application {
	out := &generated.Application{
		Name:              in.Name,
		HumanReadableName: *in.Spec.Name,
		Description:       strings.ReplaceAll(*in.Spec.Description, "\n", " "),
		Documentation:     *in.Spec.Documentation,
		License:           *in.Spec.License,
		Icon:              in.Spec.Icon,
		Version:           *in.Spec.Version,
	}

	return out
}

func convertList(in []unikornv1.HelmApplication) []*generated.Application {
	out := make([]*generated.Application, len(in))

	for i := range in {
		out[i] = convert(&in[i])
	}

	return out
}

func (c *Client) List(ctx context.Context) ([]*generated.Application, error) {
	result := &unikornv1.HelmApplicationList{}

	if err := c.client.List(ctx, result); err != nil {
		return nil, errors.OAuth2ServerError("failed to list applications").WithError(err)
	}

	exported := result.Exported()

	slices.SortStableFunc(exported.Items, unikornv1.CompareHelmApplication)

	return convertList(exported.Items), nil
}
