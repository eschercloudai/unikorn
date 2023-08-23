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

package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	argoprojv1 "github.com/eschercloudai/unikorn/pkg/apis/argoproj/v1alpha1"
	unikornv1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/provisioners"
	"github.com/eschercloudai/unikorn/pkg/provisioners/application"
	clientutil "github.com/eschercloudai/unikorn/pkg/util/client"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// testContext provides a common framework for test execution.
type testContext struct {
	// kubernetesClient allows fake resources to be tested or mutated to
	// trigger various testing scenarios.
	kubernetesClient client.WithWatch
}

func mustNewTestContext(t *testing.T) *testContext {
	t.Helper()

	scheme, err := clientutil.NewScheme()
	if err != nil {
		t.Fatal(err)
	}

	tc := &testContext{
		kubernetesClient: fake.NewClientBuilder().WithScheme(scheme).Build(),
	}

	return tc
}

// mutuallyExclusiveResource defines an abstract type that is able to uniquely
// identify itself with a set of labels.
type mutuallyExclusiveResource labels.Set

func (m mutuallyExclusiveResource) ResourceLabels() (labels.Set, error) {
	return labels.Set(m), nil
}

// mustGetApplication gets the Kubernetes Apllication resource for the
// provisioner.
func mustGetApplication(t *testing.T, p *application.Provisioner) *argoprojv1.Application {
	t.Helper()

	application, err := p.FindApplication(context.TODO())
	assert.Nil(t, err)

	return application
}

//nolint:gochecknoglobals
var (
	repo    = "foo"
	chart   = "bar"
	version = "baz"
)

// TestApplicationCreateHelm tests that given the requested input the provisioner
// creates an ArgoCD Application, and the fields are populated as expected.
func TestApplicationCreateHelm(t *testing.T) {
	t.Parallel()

	tc := mustNewTestContext(t)

	app := &unikornv1.HelmApplication{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: unikornv1.HelmApplicationSpec{
			Repo:    &repo,
			Chart:   &chart,
			Version: &version,
		},
	}

	owner := &mutuallyExclusiveResource{}
	provisioner := application.New(tc.kubernetesClient, "foo", owner, app)

	assert.Error(t, provisioner.Provision(context.TODO()), provisioners.ErrYield)

	application := mustGetApplication(t, provisioner)
	assert.Equal(t, repo, application.Spec.Source.RepoURL)
	assert.Equal(t, chart, application.Spec.Source.Chart)
	assert.Equal(t, "", application.Spec.Source.Path)
	assert.Equal(t, version, application.Spec.Source.TargetRevision)
	assert.Nil(t, application.Spec.Source.Helm)
	assert.Equal(t, "in-cluster", application.Spec.Destination.Name)
	assert.Equal(t, "", application.Spec.Destination.Namespace)
	assert.Equal(t, true, application.Spec.SyncPolicy.Automated.SelfHeal)
	assert.Equal(t, true, application.Spec.SyncPolicy.Automated.Prune)
	assert.Nil(t, application.Spec.SyncPolicy.SyncOptions)
}

// TestApplicationCreateHelmExtended tests that given the requested input the provisioner
// creates an ArgoCD Application, and the fields are populated as expected.
func TestApplicationCreateHelmExtended(t *testing.T) {
	t.Parallel()

	tc := mustNewTestContext(t)

	release := "epic"
	parameter := "foo"
	value := "bah"
	theT := true

	app := &unikornv1.HelmApplication{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: unikornv1.HelmApplicationSpec{
			Repo:    &repo,
			Chart:   &chart,
			Version: &version,
			Release: &release,
			Parameters: []unikornv1.HelmApplicationSpecParameter{
				{
					Name:  &parameter,
					Value: &value,
				},
			},
			CreateNamespace: &theT,
			ServerSideApply: &theT,
		},
	}

	owner := &mutuallyExclusiveResource{}
	provisioner := application.New(tc.kubernetesClient, "foo", owner, app)

	assert.Error(t, provisioner.Provision(context.TODO()), provisioners.ErrYield)

	application := mustGetApplication(t, provisioner)
	assert.Equal(t, repo, application.Spec.Source.RepoURL)
	assert.Equal(t, chart, application.Spec.Source.Chart)
	assert.Equal(t, "", application.Spec.Source.Path)
	assert.Equal(t, version, application.Spec.Source.TargetRevision)
	assert.NotNil(t, application.Spec.Source.Helm)
	assert.Equal(t, release, application.Spec.Source.Helm.ReleaseName)
	assert.Equal(t, 1, len(application.Spec.Source.Helm.Parameters))
	assert.Equal(t, parameter, application.Spec.Source.Helm.Parameters[0].Name)
	assert.Equal(t, value, application.Spec.Source.Helm.Parameters[0].Value)
	assert.Equal(t, "in-cluster", application.Spec.Destination.Name)
	assert.Equal(t, "", application.Spec.Destination.Namespace)
	assert.Equal(t, true, application.Spec.SyncPolicy.Automated.SelfHeal)
	assert.Equal(t, true, application.Spec.SyncPolicy.Automated.Prune)
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

	app := &unikornv1.HelmApplication{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: unikornv1.HelmApplicationSpec{
			Repo:    &repo,
			Path:    &path,
			Version: &version,
		},
	}

	owner := &mutuallyExclusiveResource{}
	provisioner := application.New(tc.kubernetesClient, "foo", owner, app)

	assert.Error(t, provisioner.Provision(context.TODO()), provisioners.ErrYield)

	application := mustGetApplication(t, provisioner)
	assert.Equal(t, repo, application.Spec.Source.RepoURL)
	assert.Equal(t, "", application.Spec.Source.Chart)
	assert.Equal(t, path, application.Spec.Source.Path)
	assert.Equal(t, version, application.Spec.Source.TargetRevision)
	assert.Nil(t, application.Spec.Source.Helm)
	assert.Equal(t, "in-cluster", application.Spec.Destination.Name)
	assert.Equal(t, "", application.Spec.Destination.Namespace)
	assert.Equal(t, true, application.Spec.SyncPolicy.Automated.SelfHeal)
	assert.Equal(t, true, application.Spec.SyncPolicy.Automated.Prune)
	assert.Nil(t, application.Spec.SyncPolicy.SyncOptions)
	assert.Nil(t, application.Spec.IgnoreDifferences)
}

const (
	mutatorRelease                  = "sentinel"
	mutatorParameter                = "foo"
	mutatorValue                    = "bar"
	mutatorIgnoreDifferencesGroup   = "hippes"
	mutatorIgnoreDifferencesKind    = "treeHugger"
	mutatorIgnoreDifferencesPointer = "arrow"
)

//nolint:gochecknoglobals
var mutatorValues = map[string]string{
	mutatorParameter: mutatorValue,
}

const mutatorValuesString = mutatorParameter + ": " + mutatorValue + "\n"

// mutator does just that allows modifications of the application.
type mutator struct{}

var _ application.ReleaseNamer = &mutator{}
var _ application.Paramterizer = &mutator{}
var _ application.ValuesGenerator = &mutator{}
var _ application.Customizer = &mutator{}

func (m *mutator) ReleaseName() string {
	return "sentinel"
}

func (m *mutator) Parameters(version *string) (map[string]string, error) {
	p := map[string]string{
		mutatorParameter: mutatorValue,
	}

	return p, nil
}

func (m *mutator) Values(version *string) (interface{}, error) {
	return mutatorValues, nil
}

func (m *mutator) Customize(version *string, application *argoprojv1.Application) error {
	application.Spec.IgnoreDifferences = []argoprojv1.ApplicationIgnoreDifference{
		{
			Group: mutatorIgnoreDifferencesGroup,
			Kind:  mutatorIgnoreDifferencesKind,
			JSONPointers: []string{
				mutatorIgnoreDifferencesPointer,
			},
		},
	}

	return nil
}

// TestApplicationCreateMutate tests that given the requested input the provisioner
// creates an ArgoCD Application, and the fields are populated as expected.
func TestApplicationCreateMutate(t *testing.T) {
	t.Parallel()

	tc := mustNewTestContext(t)

	namespace := "gerbils"

	app := &unikornv1.HelmApplication{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: unikornv1.HelmApplicationSpec{
			Repo:    &repo,
			Chart:   &chart,
			Version: &version,
		},
	}

	owner := &mutuallyExclusiveResource{}
	generator := &mutator{}
	provisioner := application.New(tc.kubernetesClient, "foo", owner, app).WithGenerator(generator).InNamespace(namespace)

	assert.Error(t, provisioner.Provision(context.TODO()), provisioners.ErrYield)

	application := mustGetApplication(t, provisioner)
	assert.Equal(t, repo, application.Spec.Source.RepoURL)
	assert.Equal(t, chart, application.Spec.Source.Chart)
	assert.Equal(t, "", application.Spec.Source.Path)
	assert.Equal(t, version, application.Spec.Source.TargetRevision)
	assert.NotNil(t, application.Spec.Source.Helm)
	assert.Equal(t, mutatorRelease, application.Spec.Source.Helm.ReleaseName)
	assert.Equal(t, 1, len(application.Spec.Source.Helm.Parameters))
	assert.Equal(t, mutatorParameter, application.Spec.Source.Helm.Parameters[0].Name)
	assert.Equal(t, mutatorValue, application.Spec.Source.Helm.Parameters[0].Value)
	assert.Equal(t, mutatorValuesString, application.Spec.Source.Helm.Values)
	assert.Equal(t, namespace, application.Spec.Destination.Namespace)
	assert.Equal(t, true, application.Spec.SyncPolicy.Automated.SelfHeal)
	assert.Equal(t, true, application.Spec.SyncPolicy.Automated.Prune)
	assert.Nil(t, application.Spec.SyncPolicy.SyncOptions)
	assert.Equal(t, 1, len(application.Spec.IgnoreDifferences))
	assert.Equal(t, mutatorIgnoreDifferencesGroup, application.Spec.IgnoreDifferences[0].Group)
	assert.Equal(t, mutatorIgnoreDifferencesKind, application.Spec.IgnoreDifferences[0].Kind)
	assert.Equal(t, 1, len(application.Spec.IgnoreDifferences[0].JSONPointers))
	assert.Equal(t, mutatorIgnoreDifferencesPointer, application.Spec.IgnoreDifferences[0].JSONPointers[0])
}

// TestApplicationUpdateAndDelete tests that given the requested input the provisioner
// creates an ArgoCD Application, and the fields are populated as expected.
func TestApplicationUpdateAndDelete(t *testing.T) {
	t.Parallel()

	tc := mustNewTestContext(t)

	app := &unikornv1.HelmApplication{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: unikornv1.HelmApplicationSpec{
			Repo:    &repo,
			Chart:   &chart,
			Version: &version,
		},
	}

	owner := &mutuallyExclusiveResource{}
	provisioner := application.New(tc.kubernetesClient, "foo", owner, app)

	assert.Error(t, provisioner.Provision(context.TODO()), provisioners.ErrYield)

	newVersion := "the best"
	app.Spec.Version = &newVersion

	assert.Error(t, provisioner.Provision(context.TODO()), provisioners.ErrYield)

	application := mustGetApplication(t, provisioner)
	assert.Nil(t, application.DeletionTimestamp)
	assert.Equal(t, repo, application.Spec.Source.RepoURL)
	assert.Equal(t, chart, application.Spec.Source.Chart)
	assert.Equal(t, "", application.Spec.Source.Path)
	assert.Equal(t, newVersion, application.Spec.Source.TargetRevision)
	assert.Nil(t, application.Spec.Source.Helm)
	assert.Equal(t, "in-cluster", application.Spec.Destination.Name)
	assert.Equal(t, "", application.Spec.Destination.Namespace)
	assert.Equal(t, true, application.Spec.SyncPolicy.Automated.SelfHeal)
	assert.Equal(t, true, application.Spec.SyncPolicy.Automated.Prune)
	assert.Nil(t, application.Spec.SyncPolicy.SyncOptions)

	assert.Error(t, provisioner.Deprovision(context.TODO()), provisioners.ErrYield)

	application = mustGetApplication(t, provisioner)
	assert.NotNil(t, application.DeletionTimestamp)
}
