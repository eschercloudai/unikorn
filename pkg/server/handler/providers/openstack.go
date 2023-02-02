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

package providers

import (
	"net/http"
	"time"

	"github.com/eschercloudai/unikorn/pkg/providers/openstack"
	"github.com/eschercloudai/unikorn/pkg/server/authorization"
	"github.com/eschercloudai/unikorn/pkg/server/context"
	"github.com/eschercloudai/unikorn/pkg/server/errors"
	"github.com/eschercloudai/unikorn/pkg/server/generated"
)

// Openstack provides an HTTP handler for Openstack resources.
type Openstack struct {
	endpoint string
}

// NewOpenstack returns a new initialized Openstack handler.
func NewOpenstack(a *authorization.Authenticator) *Openstack {
	return &Openstack{
		endpoint: a.Endpoint(),
	}
}

func (o *Openstack) IdentityClient(r *http.Request) (*openstack.IdentityClient, error) {
	token, err := context.TokenFromContext(r.Context())
	if err != nil {
		return nil, errors.OAuth2ServerError("failed get authorization token").WithError(err)
	}

	client, err := openstack.NewIdentityClient(openstack.NewTokenProvider(o.endpoint, token))
	if err != nil {
		return nil, errors.OAuth2ServerError("failed get identity client").WithError(err)
	}

	return client, nil
}

func (o *Openstack) ComputeClient(r *http.Request) (*openstack.ComputeClient, error) {
	token, err := context.TokenFromContext(r.Context())
	if err != nil {
		return nil, errors.OAuth2ServerError("failed get authorization token").WithError(err)
	}

	client, err := openstack.NewComputeClient(openstack.NewTokenProvider(o.endpoint, token))
	if err != nil {
		return nil, errors.OAuth2ServerError("failed get compute client").WithError(err)
	}

	return client, nil
}

func (o *Openstack) NetworkClient(r *http.Request) (*openstack.NetworkClient, error) {
	token, err := context.TokenFromContext(r.Context())
	if err != nil {
		return nil, errors.OAuth2ServerError("failed get authorization token").WithError(err)
	}

	client, err := openstack.NewNetworkClient(openstack.NewTokenProvider(o.endpoint, token))
	if err != nil {
		return nil, errors.OAuth2ServerError("failed get network client").WithError(err)
	}

	return client, nil
}

func (o *Openstack) ListAvailabilityZones(r *http.Request) (interface{}, error) {
	client, err := o.ComputeClient(r)
	if err != nil {
		return nil, errors.OAuth2ServerError("failed get compute client").WithError(err)
	}

	result, err := client.AvailabilityZones()
	if err != nil {
		return nil, errors.OAuth2ServerError("failed list availability zones").WithError(err)
	}

	return result, nil
}

func (o *Openstack) ListExternalNetworks(r *http.Request) (interface{}, error) {
	client, err := o.NetworkClient(r)
	if err != nil {
		return nil, errors.OAuth2ServerError("failed get network client").WithError(err)
	}

	result, err := client.ExternalNetworks()
	if err != nil {
		return nil, errors.OAuth2ServerError("failed list external networks").WithError(err)
	}

	return result, nil
}

func (o *Openstack) ListFlavors(r *http.Request) (interface{}, error) {
	client, err := o.ComputeClient(r)
	if err != nil {
		return nil, errors.OAuth2ServerError("failed get compute client").WithError(err)
	}

	result, err := client.Flavors()
	if err != nil {
		return nil, errors.OAuth2ServerError("failed list flavors").WithError(err)
	}

	return result, nil
}

func (o *Openstack) ListImages(r *http.Request) (generated.OpenstackImages, error) {
	client, err := o.ComputeClient(r)
	if err != nil {
		return nil, errors.OAuth2ServerError("failed get compute client").WithError(err)
	}

	result, err := client.Images()
	if err != nil {
		return nil, errors.OAuth2ServerError("failed list images").WithError(err)
	}

	images := make(generated.OpenstackImages, len(result))

	for i, image := range result {
		created, err := time.Parse(time.RFC3339, image.Created)
		if err != nil {
			return nil, errors.OAuth2ServerError("failed parse image creation time").WithError(err)
		}

		modified, err := time.Parse(time.RFC3339, image.Updated)
		if err != nil {
			return nil, errors.OAuth2ServerError("failed parse image modification time").WithError(err)
		}

		// images are pre-filtered by the provider library, so these keys exist.
		kubernetesVersion, ok := image.Metadata["k8s"].(string)
		if !ok {
			return nil, errors.OAuth2ServerError("failed parse image kubernetes version")
		}

		nvidiaDriverVersion, ok := image.Metadata["gpu"].(string)
		if !ok {
			return nil, errors.OAuth2ServerError("failed parse image gpu driver version")
		}

		images[i].Id = image.ID
		images[i].Name = image.Name
		images[i].Created = created
		images[i].Modified = modified
		images[i].Versions.Kubernetes = kubernetesVersion
		images[i].Versions.NvidiaDriver = nvidiaDriverVersion
	}

	return images, nil
}

// ListAvailableProjects lists projects that the token has roles associated with.
func (o *Openstack) ListAvailableProjects(r *http.Request) (generated.OpenstackProjects, error) {
	client, err := o.IdentityClient(r)
	if err != nil {
		return nil, errors.OAuth2ServerError("failed get identity client").WithError(err)
	}

	result, err := client.ListAvailableProjects()
	if err != nil {
		return nil, errors.OAuth2ServerError("failed list projects").WithError(err)
	}

	projects := make(generated.OpenstackProjects, len(result))

	for i, project := range result {
		projects[i].Id = project.ID
		projects[i].Name = project.Name

		if project.Description != "" {
			projects[i].Description = &result[i].Description
		}
	}

	return projects, nil
}

func (o *Openstack) ListKeyPairs(r *http.Request) (interface{}, error) {
	client, err := o.ComputeClient(r)
	if err != nil {
		return nil, errors.OAuth2ServerError("failed get compute client").WithError(err)
	}

	result, err := client.KeyPairs()
	if err != nil {
		return nil, errors.OAuth2ServerError("failed list key pairs").WithError(err)
	}

	return result, nil
}
