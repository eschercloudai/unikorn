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

package argocd_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	argoprojv1 "github.com/eschercloudai/unikorn/pkg/apis/argoproj/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/cd"
	"github.com/eschercloudai/unikorn/pkg/cd/argocd"
	"github.com/eschercloudai/unikorn/pkg/provisioners"
	clientutil "github.com/eschercloudai/unikorn/pkg/util/client"

	corev1 "k8s.io/api/core/v1"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// testContext provides a common framework for test execution.
type testContext struct {
	driver *argocd.Driver
}

func mustNewTestContext(t *testing.T) *testContext {
	t.Helper()

	scheme, err := clientutil.NewScheme()
	if err != nil {
		t.Fatal(err)
	}

	tc := &testContext{
		driver: argocd.New(fake.NewClientBuilder().WithScheme(scheme).Build()),
	}

	return tc
}

// mustGetApplication gets the Kubernetes Apllication resource for the
// provisioner.
func mustGetApplication(t *testing.T, tc *testContext, id *cd.ResourceIdentifier) *argoprojv1.Application {
	t.Helper()

	application, err := tc.driver.GetHelmApplication(context.TODO(), id)
	assert.NoError(t, err)

	return application
}

const (
	repo    = "foo"
	chart   = "bar"
	version = "baz"
)

// TestApplicationCreateHelm tests that given the requested input the provisioner
// creates an ArgoCD Application, and the fields are populated as expected.
func TestApplicationCreateHelm(t *testing.T) {
	t.Parallel()

	tc := mustNewTestContext(t)

	id := &cd.ResourceIdentifier{
		Name: "test",
	}

	app := &cd.HelmApplication{
		Repo:    repo,
		Chart:   chart,
		Version: version,
	}

	assert.ErrorIs(t, tc.driver.CreateOrUpdateHelmApplication(context.TODO(), id, app), provisioners.ErrYield)

	application := mustGetApplication(t, tc, id)
	assert.Equal(t, repo, application.Spec.Source.RepoURL)
	assert.Equal(t, chart, application.Spec.Source.Chart)
	assert.Equal(t, "", application.Spec.Source.Path)
	assert.Equal(t, version, application.Spec.Source.TargetRevision)
	assert.Nil(t, application.Spec.Source.Helm)
	assert.Equal(t, "in-cluster", application.Spec.Destination.Name)
	assert.Equal(t, "", application.Spec.Destination.Namespace)
	assert.True(t, application.Spec.SyncPolicy.Automated.SelfHeal)
	assert.True(t, application.Spec.SyncPolicy.Automated.Prune)
	assert.Nil(t, application.Spec.SyncPolicy.SyncOptions)
}

// TestApplicationCreateHelmExtended tests that given the requested input the provisioner
// creates an ArgoCD Application, and the fields are populated as expected.
func TestApplicationCreateHelmExtended(t *testing.T) {
	t.Parallel()

	release := "epic"
	parameter := "foo"
	value := "bah"
	remoteClusterName := "bar"
	remoteClusterLabel1 := "baz"
	remoteClusterLabel2 := "cat"
	remoteDestination := fmt.Sprintf("%s-%s:%s", remoteClusterName, remoteClusterLabel1, remoteClusterLabel2)
	valuesKey := "dog"
	valuesValue := "woof"
	values := map[string]interface{}{
		valuesKey: valuesValue,
	}

	tc := mustNewTestContext(t)

	id := &cd.ResourceIdentifier{
		Name: "test",
	}

	clusterID := &cd.ResourceIdentifier{
		Name: remoteClusterName,
		Labels: []cd.ResourceIdentifierLabel{
			{
				Name:  "unused",
				Value: remoteClusterLabel1,
			},
			{
				Name:  "unused",
				Value: remoteClusterLabel2,
			},
		},
	}

	app := &cd.HelmApplication{
		Repo:    repo,
		Chart:   chart,
		Version: version,
		Release: release,
		Parameters: []cd.HelmApplicationParameter{
			{
				Name:  parameter,
				Value: value,
			},
		},
		Values:          values,
		Cluster:         clusterID,
		CreateNamespace: true,
		ServerSideApply: true,
	}

	assert.ErrorIs(t, tc.driver.CreateOrUpdateHelmApplication(context.TODO(), id, app), provisioners.ErrYield)

	application := mustGetApplication(t, tc, id)
	assert.Equal(t, repo, application.Spec.Source.RepoURL)
	assert.Equal(t, chart, application.Spec.Source.Chart)
	assert.Equal(t, "", application.Spec.Source.Path)
	assert.Equal(t, version, application.Spec.Source.TargetRevision)
	assert.NotNil(t, application.Spec.Source.Helm)
	assert.Equal(t, release, application.Spec.Source.Helm.ReleaseName)
	assert.Equal(t, 1, len(application.Spec.Source.Helm.Parameters))
	assert.Equal(t, parameter, application.Spec.Source.Helm.Parameters[0].Name)
	assert.Equal(t, value, application.Spec.Source.Helm.Parameters[0].Value)
	assert.Equal(t, fmt.Sprintf("%s: %s\n", valuesKey, valuesValue), application.Spec.Source.Helm.Values)
	assert.Equal(t, remoteDestination, application.Spec.Destination.Name)
	assert.Equal(t, "", application.Spec.Destination.Namespace)
	assert.True(t, application.Spec.SyncPolicy.Automated.SelfHeal)
	assert.True(t, application.Spec.SyncPolicy.Automated.Prune)
	assert.Equal(t, 2, len(application.Spec.SyncPolicy.SyncOptions))
	assert.Equal(t, argoprojv1.CreateNamespace, application.Spec.SyncPolicy.SyncOptions[0])
	assert.Equal(t, argoprojv1.ServerSideApply, application.Spec.SyncPolicy.SyncOptions[1])
}

// TestApplicationCreateGit tests that given the requested input the provisioner
// creates an ArgoCD Application, and the fields are populated as expected.
func TestApplicationCreateGit(t *testing.T) {
	t.Parallel()

	tc := mustNewTestContext(t)

	path := "bar"

	id := &cd.ResourceIdentifier{
		Name: "test",
	}

	app := &cd.HelmApplication{
		Repo:    repo,
		Path:    path,
		Version: version,
	}

	assert.ErrorIs(t, tc.driver.CreateOrUpdateHelmApplication(context.TODO(), id, app), provisioners.ErrYield)

	application := mustGetApplication(t, tc, id)
	assert.Equal(t, repo, application.Spec.Source.RepoURL)
	assert.Equal(t, "", application.Spec.Source.Chart)
	assert.Equal(t, path, application.Spec.Source.Path)
	assert.Equal(t, version, application.Spec.Source.TargetRevision)
	assert.Nil(t, application.Spec.Source.Helm)
	assert.Equal(t, "in-cluster", application.Spec.Destination.Name)
	assert.Equal(t, "", application.Spec.Destination.Namespace)
	assert.True(t, application.Spec.SyncPolicy.Automated.SelfHeal)
	assert.True(t, application.Spec.SyncPolicy.Automated.Prune)
	assert.Nil(t, application.Spec.SyncPolicy.SyncOptions)
	assert.Nil(t, application.Spec.IgnoreDifferences)
}

// TestApplicationUpdateAndDelete tests that given the requested input the provisioner
// creates an ArgoCD Application, and the fields are populated as expected.
func TestApplicationUpdateAndDelete(t *testing.T) {
	t.Parallel()

	tc := mustNewTestContext(t)

	id := &cd.ResourceIdentifier{
		Name: "test",
	}

	app := &cd.HelmApplication{
		Repo:    repo,
		Chart:   chart,
		Version: version,
	}

	assert.ErrorIs(t, tc.driver.CreateOrUpdateHelmApplication(context.TODO(), id, app), provisioners.ErrYield)

	newVersion := "the best"
	app.Version = newVersion

	assert.ErrorIs(t, tc.driver.CreateOrUpdateHelmApplication(context.TODO(), id, app), provisioners.ErrYield)

	application := mustGetApplication(t, tc, id)
	assert.Nil(t, application.DeletionTimestamp)
	assert.Equal(t, repo, application.Spec.Source.RepoURL)
	assert.Equal(t, chart, application.Spec.Source.Chart)
	assert.Equal(t, "", application.Spec.Source.Path)
	assert.Equal(t, newVersion, application.Spec.Source.TargetRevision)
	assert.Nil(t, application.Spec.Source.Helm)
	assert.Equal(t, "in-cluster", application.Spec.Destination.Name)
	assert.Equal(t, "", application.Spec.Destination.Namespace)
	assert.True(t, application.Spec.SyncPolicy.Automated.SelfHeal)
	assert.True(t, application.Spec.SyncPolicy.Automated.Prune)
	assert.Nil(t, application.Spec.SyncPolicy.SyncOptions)

	assert.ErrorIs(t, tc.driver.DeleteHelmApplication(context.TODO(), id, false), provisioners.ErrYield)

	application = mustGetApplication(t, tc, id)
	assert.NotNil(t, application.DeletionTimestamp)
}

// TestApplicationDeleteNotFound tests the provisioner returns nil when an application
// doesn't exist.
func TestApplicationDeleteNotFound(t *testing.T) {
	t.Parallel()

	tc := mustNewTestContext(t)

	id := &cd.ResourceIdentifier{
		Name: "test",
	}

	assert.NoError(t, tc.driver.DeleteHelmApplication(context.TODO(), id, false))
}

const (
	clusterServer = "https://localhost:8443"
)

func clusterCA() []byte {
	return []byte("foo")
}

func clusterClientCert() []byte {
	return []byte("bar")
}

func clusterClientKey() []byte {
	return []byte("baz")
}

func getKubeconfig() *clientcmdapi.Config {
	return &clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{
			"default": {
				Server:                   clusterServer,
				CertificateAuthorityData: clusterCA(),
			},
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			"default": {
				ClientCertificateData: clusterClientCert(),
				ClientKeyData:         clusterClientKey(),
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			"default": {
				Cluster:  "default",
				AuthInfo: "default",
			},
		},
		CurrentContext: "default",
	}
}

// mustGetClusterSecret gets the cluster secret for the id.
func mustGetClusterSecret(t *testing.T, tc *testContext, id *cd.ResourceIdentifier) *corev1.Secret {
	t.Helper()

	secret, err := tc.driver.GetClusterSecret(context.TODO(), id)
	assert.NoError(t, err)

	return secret
}

// TestClusterCreate ensures we can successfully create a new cluster, read it back and
// the contents are correct.
func TestClusterCreate(t *testing.T) {
	t.Parallel()

	tc := mustNewTestContext(t)

	id := &cd.ResourceIdentifier{
		Name: "test",
	}

	cluster := &cd.Cluster{
		Config: getKubeconfig(),
	}

	assert.NoError(t, tc.driver.CreateOrUpdateCluster(context.TODO(), id, cluster))

	secret := mustGetClusterSecret(t, tc, id)

	assert.Equal(t, []byte(clusterServer), secret.Data["server"])

	var config argocd.ClusterConfig

	assert.NoError(t, json.Unmarshal(secret.Data["config"], &config))
	assert.Equal(t, clusterCA(), config.TLSClientConfig.CAData)
	assert.Equal(t, clusterClientCert(), config.TLSClientConfig.CertData)
	assert.Equal(t, clusterClientKey(), config.TLSClientConfig.KeyData)
}

// TestClusterUpdateAndDelete tests updates are reflected in the cluster e.g. certificate
// rotation, and deletion does what it's supposed to.
func TestClusterUpdateAndDelete(t *testing.T) {
	t.Parallel()

	tc := mustNewTestContext(t)

	id := &cd.ResourceIdentifier{
		Name: "test",
	}

	cluster := &cd.Cluster{
		Config: getKubeconfig(),
	}

	assert.NoError(t, tc.driver.CreateOrUpdateCluster(context.TODO(), id, cluster))

	newCAData := []byte("squirrel")

	cluster.Config.Clusters["default"].CertificateAuthorityData = newCAData

	assert.NoError(t, tc.driver.CreateOrUpdateCluster(context.TODO(), id, cluster))

	secret := mustGetClusterSecret(t, tc, id)

	var config argocd.ClusterConfig

	assert.NoError(t, json.Unmarshal(secret.Data["config"], &config))
	assert.Equal(t, newCAData, config.TLSClientConfig.CAData)

	assert.NoError(t, tc.driver.DeleteCluster(context.TODO(), id))

	_, err := tc.driver.GetClusterSecret(context.TODO(), id)
	assert.ErrorIs(t, err, cd.ErrNotFound)
}

// TestClusterDeleteNotFound tests cluster deletion is idempotent when the cluster
// secret doesn't exist.
func TestClusterDeleteNotFound(t *testing.T) {
	t.Parallel()

	tc := mustNewTestContext(t)

	id := &cd.ResourceIdentifier{
		Name: "test",
	}

	assert.NoError(t, tc.driver.DeleteCluster(context.TODO(), id))
}
