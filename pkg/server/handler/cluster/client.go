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

package cluster

import (
	"context"
	"net/http"
	"sort"

	unikornv1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/provisioners/vcluster"
	"github.com/eschercloudai/unikorn/pkg/server/authorization"
	"github.com/eschercloudai/unikorn/pkg/server/errors"
	"github.com/eschercloudai/unikorn/pkg/server/generated"
	"github.com/eschercloudai/unikorn/pkg/server/handler/controlplane"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Client wraps up cluster related management handling.
type Client struct {
	// client allows Kubernetes API access.
	client client.Client

	// request is the http request that invoked this client.
	request *http.Request

	// authenticator provides access to authentication services.
	authenticator *authorization.Authenticator
}

// NewClient returns a new client with required parameters.
func NewClient(client client.Client, request *http.Request, authenticator *authorization.Authenticator) *Client {
	return &Client{
		client:        client,
		request:       request,
		authenticator: authenticator,
	}
}

// List returns all clusters owned by the implicit control plane.
func (c *Client) List(ctx context.Context, controlPlaneName generated.ControlPlaneNameParameter) ([]*generated.KubernetesCluster, error) {
	controlPlane, err := controlplane.NewClient(c.client).Metadata(ctx, controlPlaneName)
	if err != nil {
		return nil, err
	}

	result := &unikornv1.KubernetesClusterList{}

	if err := c.client.List(ctx, result, &client.ListOptions{Namespace: controlPlane.Namespace}); err != nil {
		return nil, errors.OAuth2ServerError("failed to list control planes").WithError(err)
	}

	sort.Stable(result)

	return convertList(result), nil
}

// Get returns the implicit cluster identified by the JWT claims.
func (c *Client) Get(ctx context.Context, controlPlaneName generated.ControlPlaneNameParameter, name generated.ClusterNameParameter) (*generated.KubernetesCluster, error) {
	controlPlane, err := controlplane.NewClient(c.client).Metadata(ctx, controlPlaneName)
	if err != nil {
		return nil, err
	}

	result := &unikornv1.KubernetesCluster{}

	if err := c.client.Get(ctx, client.ObjectKey{Namespace: controlPlane.Namespace, Name: name}, result); err != nil {
		if kerrors.IsNotFound(err) {
			return nil, errors.HTTPNotFound()
		}

		return nil, errors.OAuth2ServerError("unable to get cluster").WithError(err)
	}

	return convert(result), nil
}

// GetKubeconfig returns the kubernetes configuation associated with a cluster.
func (c *Client) GetKubeconfig(ctx context.Context, controlPlaneName generated.ControlPlaneNameParameter, name generated.ClusterNameParameter) ([]byte, error) {
	controlPlane, err := controlplane.NewClient(c.client).Metadata(ctx, controlPlaneName)
	if err != nil {
		return nil, err
	}

	vc := vcluster.NewControllerRuntimeClient(c.client)

	vclusterConfig, err := vc.RESTConfig(ctx, controlPlane.Namespace, false)
	if err != nil {
		return nil, errors.OAuth2ServerError("failed to get control plane rest config").WithError(err)
	}

	vclusterClient, err := client.New(vclusterConfig, client.Options{})
	if err != nil {
		return nil, errors.OAuth2ServerError("failed to get control plane client").WithError(err)
	}

	objectKey := client.ObjectKey{
		Namespace: name,
		Name:      name + "-kubeconfig",
	}

	secret := &corev1.Secret{}

	if err := vclusterClient.Get(ctx, objectKey, secret); err != nil {
		if kerrors.IsNotFound(err) {
			return nil, errors.HTTPNotFound()
		}

		return nil, errors.OAuth2ServerError("unable to get cluster configuration").WithError(err)
	}

	return secret.Data["value"], nil
}

// Create creates the implicit cluster indentified by the JTW claims.
func (c *Client) Create(ctx context.Context, controlPlaneName generated.ControlPlaneNameParameter, options *generated.KubernetesCluster) error {
	controlPlane, err := controlplane.NewClient(c.client).Metadata(ctx, controlPlaneName)
	if err != nil {
		return err
	}

	if !controlPlane.Active {
		return errors.OAuth2InvalidRequest("control plane is not active")
	}

	cluster, err := c.createCluster(controlPlane, options)
	if err != nil {
		return err
	}

	if err := c.client.Create(ctx, cluster); err != nil {
		// TODO: we can do a cached lookup to save the API traffic.
		if kerrors.IsAlreadyExists(err) {
			return errors.HTTPConflict()
		}

		return errors.OAuth2ServerError("failed to create cluster").WithError(err)
	}

	return nil
}

// Delete deletes the implicit cluster indentified by the JTW claims.
func (c *Client) Delete(ctx context.Context, controlPlaneName generated.ControlPlaneNameParameter, name generated.ClusterNameParameter) error {
	controlPlane, err := controlplane.NewClient(c.client).Metadata(ctx, controlPlaneName)
	if err != nil {
		return err
	}

	if !controlPlane.Active {
		return errors.OAuth2InvalidRequest("control plane is not active")
	}

	cluster := &unikornv1.KubernetesCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: controlPlane.Namespace,
		},
	}

	if err := c.client.Delete(ctx, cluster); err != nil {
		if kerrors.IsNotFound(err) {
			return errors.HTTPNotFound()
		}

		return errors.OAuth2ServerError("failed to delete cluster").WithError(err)
	}

	return nil
}
