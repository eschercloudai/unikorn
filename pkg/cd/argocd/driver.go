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

package argocd

import (
	"context"
	"errors"
	"reflect"
	"strings"

	argoprojv1 "github.com/eschercloudai/unikorn/pkg/apis/argoproj/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/cd"
	"github.com/eschercloudai/unikorn/pkg/constants"
	"github.com/eschercloudai/unikorn/pkg/provisioners"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"
)

const (
	namespace = "argocd"
)

var (
	// ErrItemLengthMismatch is returned when items are listed but the
	// wrong number are returned.  Given we are dealing with unique applications
	// one or zero are expected.
	ErrItemLengthMismatch = errors.New("item count not as expected")
)

// Driver implements a CD driver for ArgoCD.  Applications are fairly
// straight forward as they are implemented with custom resources.  We use
// the application ID to generate a resource name, and labels to make them
// unique and add context, plus this thwarts the 63 character limit.  There
// is no custom resource for clusters, so have to use the API.
type Driver struct {
	argoCDClient Client

	kubernetesClient client.Client
}

var _ cd.Driver = &Driver{}

// NewDriver creates a new ArgoCD driver.
func NewDriver(kubernetesClient client.Client, argoCDClient Client) *Driver {
	return &Driver{
		argoCDClient:     argoCDClient,
		kubernetesClient: kubernetesClient,
	}
}

// clusterName generates a cluster name from a cluster identifier.
// Due to legacy reasons (backward compatibility) we only use the values in the labels
// and not the keys.
func clusterName(id *cd.ResourceIdentifier) string {
	name := id.Name

	if len(id.Labels) != 0 {
		values := make([]string, len(id.Labels))

		for i, label := range id.Labels {
			values[i] = label.Value
		}

		name += "-" + strings.Join(values, ":")
	}

	return name
}

// applicationLabels gets a set of labels from an application identifier.
func applicationLabels(id *cd.ResourceIdentifier) labels.Set {
	labels := labels.Set{
		constants.ApplicationLabel: id.Name,
	}

	for _, label := range id.Labels {
		labels[label.Name] = label.Value
	}

	return labels
}

// Kind returns the driver kind.
func (d *Driver) Kind() cd.DriverKind {
	return cd.DriverKindArgoCD
}

// GetHelmApplication retrieves an abstract helm application.
func (d *Driver) GetHelmApplication(ctx context.Context, id *cd.ResourceIdentifier) (*argoprojv1.Application, error) {
	options := &client.ListOptions{
		Namespace:     namespace,
		LabelSelector: labels.SelectorFromSet(applicationLabels(id)),
	}

	var resources argoprojv1.ApplicationList

	if err := d.kubernetesClient.List(ctx, &resources, options); err != nil {
		return nil, err
	}

	if len(resources.Items) == 0 {
		return nil, cd.ErrNotFound
	}

	if len(resources.Items) > 1 {
		return nil, ErrItemLengthMismatch
	}

	return &resources.Items[0], nil
}

func generateApplication(id *cd.ResourceIdentifier, app *cd.HelmApplication) (*argoprojv1.Application, error) {
	var parameters []argoprojv1.HelmParameter

	if len(app.Parameters) > 0 {
		for _, parameter := range app.Parameters {
			parameters = append(parameters, argoprojv1.HelmParameter{
				Name:  parameter.Name,
				Value: parameter.Value,
			})
		}
	}

	var values string

	if app.Values != nil {
		marshaled, err := yaml.Marshal(app.Values)
		if err != nil {
			return nil, err
		}

		values = string(marshaled)
	}

	helm := &argoprojv1.ApplicationSourceHelm{
		ReleaseName: app.Release,
		Parameters:  parameters,
		Values:      values,
	}

	destinationName := "in-cluster"

	if app.Cluster != nil {
		destinationName = clusterName(app.Cluster)
	}

	application := &argoprojv1.Application{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: id.Name + "-",
			Namespace:    namespace,
			Labels:       applicationLabels(id),
		},
		Spec: argoprojv1.ApplicationSpec{
			Project: "default",
			Source: argoprojv1.ApplicationSource{
				RepoURL:        app.Repo,
				Chart:          app.Chart,
				Path:           app.Path,
				TargetRevision: app.Version,
			},
			Destination: argoprojv1.ApplicationDestination{
				Name:      destinationName,
				Namespace: app.Namespace,
			},
			SyncPolicy: argoprojv1.ApplicationSyncPolicy{
				Automated: &argoprojv1.ApplicationSyncAutomation{
					SelfHeal: true,
					Prune:    true,
				},
			},
		},
	}

	if !reflect.ValueOf(*helm).IsZero() {
		application.Spec.Source.Helm = helm
	}

	if app.CreateNamespace {
		application.Spec.SyncPolicy.SyncOptions = append(application.Spec.SyncPolicy.SyncOptions, argoprojv1.CreateNamespace)
	}

	if app.ServerSideApply {
		application.Spec.SyncPolicy.SyncOptions = append(application.Spec.SyncPolicy.SyncOptions, argoprojv1.ServerSideApply)
	}

	for _, field := range app.IgnoreDifferences {
		application.Spec.IgnoreDifferences = append(application.Spec.IgnoreDifferences, argoprojv1.ApplicationIgnoreDifference{
			Group:        field.Group,
			Kind:         field.Kind,
			JSONPointers: field.JSONPointers,
		})
	}

	return application, nil
}

// CreateOrUpdateHelmApplication creates or updates a helm application idempotently.
func (d *Driver) CreateOrUpdateHelmApplication(ctx context.Context, id *cd.ResourceIdentifier, app *cd.HelmApplication) error {
	log := log.FromContext(ctx)

	required, err := generateApplication(id, app)
	if err != nil {
		return err
	}

	resource, err := d.GetHelmApplication(ctx, id)
	if err != nil && !errors.Is(err, cd.ErrNotFound) {
		return err
	}

	if resource == nil {
		log.Info("creating new application", "application", id.Name)

		if err := d.kubernetesClient.Create(ctx, required); err != nil {
			return err
		}

		resource = required
	} else {
		log.Info("updating existing application", "application", id.Name)

		// Replace the specification with what we expect.
		temp := resource.DeepCopy()
		temp.Labels = required.Labels
		temp.Spec = required.Spec

		if err := d.kubernetesClient.Patch(ctx, temp, client.MergeFrom(resource)); err != nil {
			return err
		}

		resource = temp
	}

	// NOTE: This isn't necessarily accurate, take CAPI clusters for instance,
	// that's just a bunch of CRs, and they are instantly healthy until
	// CAPI/CAPO take note and start making status updates...
	if resource.Status.Health == nil || resource.Status.Health.Status != argoprojv1.Healthy {
		return provisioners.ErrYield
	}

	return nil
}

// DeleteHelmApplication deletes an existing helm application.
func (d *Driver) DeleteHelmApplication(ctx context.Context, id *cd.ResourceIdentifier, backgroundDelete bool) error {
	log := log.FromContext(ctx)

	resource, err := d.GetHelmApplication(ctx, id)
	if err != nil {
		if errors.Is(err, cd.ErrNotFound) {
			log.Info("application deleted", "application", id.Name)

			return nil
		}

		return err
	}

	if !resource.GetDeletionTimestamp().IsZero() {
		if backgroundDelete {
			return nil
		}

		log.Info("waiting for application deletion", "application", id.Name)

		return provisioners.ErrYield
	}

	log.Info("adding application finalizer", "application", id.Name)

	// Apply a finalizer to ensure synchronous deletion. See
	// https://argo-cd.readthedocs.io/en/stable/user-guide/app_deletion/
	temp := resource.DeepCopy()
	temp.SetFinalizers([]string{"resources-finalizer.argocd.argoproj.io"})

	// Try to work around a race during deletion as per
	// https://github.com/argoproj/argo-cd/issues/12943
	temp.Spec.SyncPolicy.Automated = nil

	if err := d.kubernetesClient.Patch(ctx, temp, client.MergeFrom(resource)); err != nil {
		return err
	}

	log.Info("deleting application", "application", id.Name)

	if err := d.kubernetesClient.Delete(ctx, resource); err != nil {
		return err
	}

	if !backgroundDelete {
		return provisioners.ErrYield
	}

	return nil
}

// CreateOrUpdateCluster creates or updates a cluster idempotently.
func (d *Driver) CreateOrUpdateCluster(ctx context.Context, id *cd.ResourceIdentifier, cluster *cd.Cluster) error {
	// TODO: a whole load of error checking!
	server := cluster.Config.Clusters[cluster.Config.Contexts[cluster.Config.CurrentContext].Cluster].Server

	if err := d.argoCDClient.UpsertCluster(ctx, clusterName(id), server, cluster.Config); err != nil {
		return err
	}

	return nil
}

// DeleteCluster deletes an existing cluster.
func (d *Driver) DeleteCluster(ctx context.Context, id *cd.ResourceIdentifier) error {
	if err := d.argoCDClient.DeleteCluster(ctx, clusterName(id)); err != nil {
		return err
	}

	return nil
}
