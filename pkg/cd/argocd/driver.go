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
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"net/url"
	"reflect"
	"strings"

	argoprojv1 "github.com/eschercloudai/unikorn/pkg/apis/argoproj/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/cd"
	"github.com/eschercloudai/unikorn/pkg/constants"
	"github.com/eschercloudai/unikorn/pkg/provisioners"
	"github.com/eschercloudai/unikorn/pkg/util"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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

type Options struct {
	K8SAPITester util.K8SAPITester
}

// Driver implements a CD driver for ArgoCD.  Applications are fairly
// straight forward as they are implemented with custom resources.  We use
// the application ID to generate a resource name, and labels to make them
// unique and add context, plus this thwarts the 63 character limit.  There
// is no custom resource for clusters, so have to use the API.
type Driver struct {
	client  client.Client
	options Options
}

var _ cd.Driver = &Driver{}

// New creates a new ArgoCD driver.
func New(client client.Client, options Options) *Driver {
	return &Driver{
		client:  client,
		options: options,
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

	if err := d.client.List(ctx, &resources, options); err != nil {
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
//
//nolint:cyclop
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

		if err := d.client.Create(ctx, required); err != nil {
			return err
		}

		resource = required
	} else {
		log.Info("updating existing application", "application", id.Name)

		// Replace the specification with what we expect.
		temp := resource.DeepCopy()
		temp.Labels = required.Labels
		temp.Spec = required.Spec

		if err := d.client.Patch(ctx, temp, client.MergeFrom(resource)); err != nil {
			return err
		}

		resource = temp
	}

	if resource.Status.Health == nil {
		return provisioners.ErrYield
	}

	// Bit of a hack, for clusters, we know they are working and gated on
	// remote cluster creation, so can allow the rest to provision while it's
	// still sorting its control plane out.
	if app.AllowDegraded && resource.Status.Health.Status == argoprojv1.Degraded {
		return nil
	}

	if resource.Status.Health.Status != argoprojv1.Healthy {
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

	if err := d.client.Patch(ctx, temp, client.MergeFrom(resource)); err != nil {
		return err
	}

	log.Info("deleting application", "application", id.Name)

	if err := d.client.Delete(ctx, resource); err != nil {
		return err
	}

	if !backgroundDelete {
		return provisioners.ErrYield
	}

	return nil
}

type ClusterTLSClientConfig struct {
	CAData   []byte `json:"caData"`
	CertData []byte `json:"certData"`
	KeyData  []byte `json:"keyData"`
}

type ClusterConfig struct {
	TLSClientConfig ClusterTLSClientConfig `json:"tlsClientConfig"`
}

// clusterSecretName mirrors what Argo does for compatibility reasons.
func clusterSecretName(host string) (string, error) {
	url, err := url.Parse(host)
	if err != nil {
		return "", err
	}

	hasher := fnv.New32a()
	hasher.Write([]byte(host))

	hostname := strings.Split(url.Host, ":")

	return fmt.Sprintf("cluster-%s-%d", hostname[0], hasher.Sum32()), nil
}

// clusterLabel we base the label on the ID to ensure uniqueness, but as this is
// Kubernetes, we are restricted to 63 characters etc. like all DNS based stuff.
func clusterLabel(id *cd.ResourceIdentifier) string {
	sum := sha256.Sum256([]byte(clusterName(id)))

	return fmt.Sprintf("cluster-%x", sum[:8])
}

// GetClusterSecret looks up the cluster secret via the ID, which is present for both
// create and delete interfaces.
func (d *Driver) GetClusterSecret(ctx context.Context, id *cd.ResourceIdentifier) (*corev1.Secret, error) {
	applicationLabels := labels.Set{
		constants.ApplicationIDLabel: clusterLabel(id),
	}

	options := &client.ListOptions{
		Namespace:     namespace,
		LabelSelector: labels.SelectorFromSet(applicationLabels),
	}

	var resources corev1.SecretList

	if err := d.client.List(ctx, &resources, options); err != nil {
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

func mustateSecret(current *corev1.Secret, labels map[string]string, data map[string][]byte) func() error {
	return func() error {
		current.Labels = labels
		current.Data = data

		return nil
	}
}

// CreateOrUpdateCluster creates or updates a cluster idempotently.
func (d *Driver) CreateOrUpdateCluster(ctx context.Context, id *cd.ResourceIdentifier, cluster *cd.Cluster) error {
	log := log.FromContext(ctx)

	configContext := cluster.Config.Contexts[cluster.Config.CurrentContext]

	clusterConfig := cluster.Config.Clusters[configContext.Cluster]

	secretName, err := clusterSecretName(clusterConfig.Server)
	if err != nil {
		return err
	}

	// This next bit is a slight hack, if we install a remote without it being
	// contactable yet, then Argo will stall installing applications on it, and
	// not reconnect until ~5 minutes later, so only install the remote when we
	// can hit the API.
	// TODO: there may be a tunable to do this for us, but this is quickest :D
	key := client.ObjectKey{
		Namespace: namespace,
		Name:      secretName,
	}

	var object corev1.Secret

	//nolint:nestif
	if err := d.client.Get(ctx, key, &object); err != nil {
		if !kerrors.IsNotFound(err) {
			return err
		}

		log.Info("awaiting cluster connectivity")

		tester := d.options.K8SAPITester

		if tester == nil {
			tester = &util.DefaultK8SAPITester{}
		}

		if err := tester.Connect(ctx, cluster.Config); err != nil {
			if !errors.Is(err, util.ErrK8SConnectionError) {
				return err
			}

			log.Info("failed to get kubernetes service")

			return provisioners.ErrYield
		}
	}

	authInfo := cluster.Config.AuthInfos[configContext.AuthInfo]

	config := &ClusterConfig{
		TLSClientConfig: ClusterTLSClientConfig{
			CAData:   clusterConfig.CertificateAuthorityData,
			CertData: authInfo.ClientCertificateData,
			KeyData:  authInfo.ClientKeyData,
		},
	}

	configData, err := json.Marshal(config)
	if err != nil {
		return err
	}

	current := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      secretName,
		},
	}

	labels := map[string]string{
		"argocd.argoproj.io/secret-type": "cluster",
		constants.ApplicationIDLabel:     clusterLabel(id),
	}
	data := map[string][]byte{
		"name":   []byte(clusterName(id)),
		"server": []byte(clusterConfig.Server),
		"config": configData,
	}

	log.Info("reconciling cluster", "id", id)

	result, err := controllerutil.CreateOrPatch(ctx, d.client, current, mustateSecret(current, labels, data))
	if err != nil {
		log.Info("cluster reconcile failed", "error", err)

		return err
	}

	log.Info("cluster reconciled", "id", id, "result", result)

	return nil
}

// DeleteCluster deletes an existing cluster.
func (d *Driver) DeleteCluster(ctx context.Context, id *cd.ResourceIdentifier) error {
	resource, err := d.GetClusterSecret(ctx, id)
	if err != nil {
		if !errors.Is(err, cd.ErrNotFound) {
			return err
		}

		return nil
	}

	if err := d.client.Delete(ctx, resource); err != nil {
		return err
	}

	return nil
}
