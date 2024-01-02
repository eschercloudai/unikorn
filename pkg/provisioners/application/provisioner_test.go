/*
Copyright 2022-2024 EscherCloud.

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
	"go.uber.org/mock/gomock"

	unikornv1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/cd"
	"github.com/eschercloudai/unikorn/pkg/cd/mock"
	clientlib "github.com/eschercloudai/unikorn/pkg/client"
	"github.com/eschercloudai/unikorn/pkg/constants"
	"github.com/eschercloudai/unikorn/pkg/provisioners"
	"github.com/eschercloudai/unikorn/pkg/provisioners/application"
	mockprovisioners "github.com/eschercloudai/unikorn/pkg/provisioners/mock"
	"github.com/eschercloudai/unikorn/pkg/util"
	clientutil "github.com/eschercloudai/unikorn/pkg/util/client"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	resourceBundleName = "bundle-x.y.z"
)

func newControlPlaneResource() unikornv1.ManagableResourceInterface {
	b := resourceBundleName

	return &unikornv1.ControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name: "bar",
			Labels: map[string]string{
				constants.ProjectLabel: "foo",
			},
		},
		Spec: unikornv1.ControlPlaneSpec{
			ApplicationBundle: &b,
		},
	}
}

func newKubernetesClusterResource() unikornv1.ManagableResourceInterface {
	b := resourceBundleName

	return &unikornv1.KubernetesCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "bar",
			Labels: map[string]string{
				constants.ProjectLabel:      "foo",
				constants.ControlPlaneLabel: "baz",
			},
		},
		Spec: unikornv1.KubernetesClusterSpec{
			ApplicationBundle: &b,
		},
	}
}

func newControlPlaneResourceLabels() []cd.ResourceIdentifierLabel {
	return []cd.ResourceIdentifierLabel{
		{
			Name:  constants.ControlPlaneLabel,
			Value: "bar",
		},

		{
			Name:  constants.KindLabel,
			Value: constants.KindLabelValueControlPlane,
		},

		{
			Name:  constants.ProjectLabel,
			Value: "foo",
		},
	}
}

func newKubernetesClusterResourceLabels() []cd.ResourceIdentifierLabel {
	return []cd.ResourceIdentifierLabel{
		{
			Name:  constants.KubernetesClusterLabel,
			Value: "bar",
		},
		{
			Name:  constants.ControlPlaneLabel,
			Value: "baz",
		},
		{
			Name:  constants.KindLabel,
			Value: constants.KindLabelValueKubernetesCluster,
		},

		{
			Name:  constants.ProjectLabel,
			Value: "foo",
		},
	}
}

// testContext provides a common framework for test execution.
type testContext struct {
	client client.Client
}

func mustNewTestContext(t *testing.T, objects ...client.Object) *testContext {
	t.Helper()

	scheme, err := clientutil.NewScheme()
	if err != nil {
		t.Fatal(err)
	}

	tc := &testContext{
		client: fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&unikornv1.Project{}).WithObjects(objects...).Build(),
	}

	return tc
}

const (
	applicationName         = "test"
	overrideApplicationName = "testinate"
	repo                    = "foo"
	chart                   = "bar"
	version                 = "baz"
)

func newControlPlaneBundle(applications ...*unikornv1.HelmApplication) *unikornv1.ControlPlaneApplicationBundle {
	apps := make([]unikornv1.ApplicationBundleApplication, 0, len(applications))

	for _, application := range applications {
		apps = append(apps, unikornv1.ApplicationBundleApplication{
			Name: util.ToPointer(application.Name),
			Reference: &unikornv1.ApplicationReference{
				Kind:    util.ToPointer(unikornv1.ApplicationReferenceKindHelm),
				Name:    util.ToPointer(application.Name),
				Version: application.Spec.Versions[0].Version,
			},
		})
	}

	bundle := &unikornv1.ControlPlaneApplicationBundle{
		ObjectMeta: metav1.ObjectMeta{
			Name: resourceBundleName,
		},
		Spec: unikornv1.ApplicationBundleSpec{
			Applications: apps,
		},
	}

	return bundle
}

func newKubernetesClusterBundle(applications ...*unikornv1.HelmApplication) *unikornv1.KubernetesClusterApplicationBundle {
	apps := make([]unikornv1.ApplicationBundleApplication, 0, len(applications))

	for _, application := range applications {
		apps = append(apps, unikornv1.ApplicationBundleApplication{
			Name: util.ToPointer(application.Name),
			Reference: &unikornv1.ApplicationReference{
				Kind:    util.ToPointer(unikornv1.ApplicationReferenceKindHelm),
				Name:    util.ToPointer(application.Name),
				Version: application.Spec.Versions[0].Version,
			},
		})
	}

	bundle := &unikornv1.KubernetesClusterApplicationBundle{
		ObjectMeta: metav1.ObjectMeta{
			Name: resourceBundleName,
		},
		Spec: unikornv1.ApplicationBundleSpec{
			Applications: apps,
		},
	}

	return bundle
}

// TestApplicationCreateHelm tests that given the requested input the provisioner
// creates a CD Application, and the fields are populated as expected.
func TestApplicationCreateHelm(t *testing.T) {
	t.Parallel()

	app := &unikornv1.HelmApplication{
		ObjectMeta: metav1.ObjectMeta{
			Name: applicationName,
		},
		Spec: unikornv1.HelmApplicationSpec{
			Versions: []unikornv1.HelmApplicationVersion{
				{

					Repo:    util.ToPointer(repo),
					Chart:   util.ToPointer(chart),
					Version: util.ToPointer(version),
				},
			},
		},
	}

	tc := mustNewTestContext(t, app, newControlPlaneBundle(app))

	c := gomock.NewController(t)
	defer c.Finish()

	driverAppID := &cd.ResourceIdentifier{
		Name:   applicationName,
		Labels: newControlPlaneResourceLabels(),
	}

	driverApp := &cd.HelmApplication{
		Repo:    repo,
		Chart:   chart,
		Version: version,
	}

	driver := mock.NewMockDriver(c)
	owner := newControlPlaneResource()

	ctx := context.Background()
	ctx = clientlib.NewContextWithStaticClient(ctx, tc.client)
	ctx = cd.NewContext(ctx, driver)
	ctx = application.NewContext(ctx, owner)

	driver.EXPECT().CreateOrUpdateHelmApplication(ctx, driverAppID, driverApp).Return(provisioners.ErrYield)

	provisioner := application.New(applicationName)

	assert.ErrorIs(t, provisioner.Provision(ctx), provisioners.ErrYield)
}

// TestApplicationCreateHelmExtended tests that given the requested input the provisioner
// creates an ArgoCD Application, and the fields are populated as expected.
func TestApplicationCreateHelmExtended(t *testing.T) {
	t.Parallel()

	release := "epic"
	parameter := "foo"
	value := "bah"
	remoteClusterName := "bar"
	remoteClusterLabel1 := "dog"
	remoteClusterLabel1Value := "woof"
	remoteClusterLabel2 := "cat"
	remoteClusterLabel2value := "meow"

	app := &unikornv1.HelmApplication{
		ObjectMeta: metav1.ObjectMeta{
			Name: applicationName,
		},
		Spec: unikornv1.HelmApplicationSpec{
			Versions: []unikornv1.HelmApplicationVersion{
				{
					Repo:    util.ToPointer(repo),
					Chart:   util.ToPointer(chart),
					Version: util.ToPointer(version),
					Release: util.ToPointer(release),
					Parameters: []unikornv1.HelmApplicationParameter{
						{
							Name:  util.ToPointer(parameter),
							Value: util.ToPointer(value),
						},
					},
					CreateNamespace: util.ToPointer(true),
					ServerSideApply: util.ToPointer(true),
				},
			},
		},
	}

	tc := mustNewTestContext(t, app, newKubernetesClusterBundle(app))

	c := gomock.NewController(t)
	defer c.Finish()

	remoteID := &cd.ResourceIdentifier{
		Name: remoteClusterName,
		Labels: []cd.ResourceIdentifierLabel{
			{
				Name:  remoteClusterLabel1,
				Value: remoteClusterLabel1Value,
			},
			{
				Name:  remoteClusterLabel2,
				Value: remoteClusterLabel2value,
			},
		},
	}

	r := mockprovisioners.NewMockRemoteCluster(c)
	r.EXPECT().ID().Return(remoteID)

	driverAppID := &cd.ResourceIdentifier{
		Name:   overrideApplicationName,
		Labels: newKubernetesClusterResourceLabels(),
	}

	driverApp := &cd.HelmApplication{
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
		Cluster:         remoteID,
		CreateNamespace: true,
		ServerSideApply: true,
		AllowDegraded:   true,
	}

	driver := mock.NewMockDriver(c)
	owner := newKubernetesClusterResource()

	ctx := context.Background()
	ctx = clientlib.NewContextWithStaticClient(ctx, tc.client)
	ctx = cd.NewContext(ctx, driver)
	ctx = application.NewContext(ctx, owner)

	driver.EXPECT().CreateOrUpdateHelmApplication(ctx, driverAppID, driverApp).Return(provisioners.ErrYield)

	provisioner := application.New(applicationName).WithApplicationName(overrideApplicationName).AllowDegraded()
	provisioner.OnRemote(r)

	assert.ErrorIs(t, provisioner.Provision(ctx), provisioners.ErrYield)
}

// TestApplicationCreateGit tests that given the requested input the provisioner
// creates an ArgoCD Application, and the fields are populated as expected.
func TestApplicationCreateGit(t *testing.T) {
	t.Parallel()

	path := "bar"

	app := &unikornv1.HelmApplication{
		ObjectMeta: metav1.ObjectMeta{
			Name: applicationName,
		},
		Spec: unikornv1.HelmApplicationSpec{
			Versions: []unikornv1.HelmApplicationVersion{
				{

					Repo:    util.ToPointer(repo),
					Path:    util.ToPointer(path),
					Version: util.ToPointer(version),
				},
			},
		},
	}

	tc := mustNewTestContext(t, app, newControlPlaneBundle(app))

	c := gomock.NewController(t)
	defer c.Finish()

	driverAppID := &cd.ResourceIdentifier{
		Name:   applicationName,
		Labels: newControlPlaneResourceLabels(),
	}

	driverApp := &cd.HelmApplication{
		Repo:    repo,
		Path:    path,
		Version: version,
	}

	driver := mock.NewMockDriver(c)
	owner := newControlPlaneResource()

	ctx := context.Background()
	ctx = clientlib.NewContextWithStaticClient(ctx, tc.client)
	ctx = cd.NewContext(ctx, driver)
	ctx = application.NewContext(ctx, owner)

	driver.EXPECT().CreateOrUpdateHelmApplication(ctx, driverAppID, driverApp).Return(provisioners.ErrYield)

	provisioner := application.New(applicationName)

	assert.ErrorIs(t, provisioner.Provision(ctx), provisioners.ErrYield)
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

// mutator does just that allows modifications of the application.
type mutator struct {
	postProvisionCalled bool
}

var _ application.ReleaseNamer = &mutator{}
var _ application.Paramterizer = &mutator{}
var _ application.ValuesGenerator = &mutator{}
var _ application.Customizer = &mutator{}
var _ application.PostProvisionHook = &mutator{}

func (m *mutator) ReleaseName(ctx context.Context) string {
	return "sentinel"
}

func (m *mutator) Parameters(ctx context.Context, version string) (map[string]string, error) {
	p := map[string]string{
		mutatorParameter: mutatorValue,
	}

	return p, nil
}

func (m *mutator) Values(ctx context.Context, version string) (interface{}, error) {
	return mutatorValues, nil
}

func (m *mutator) Customize(version string) ([]cd.HelmApplicationField, error) {
	differences := []cd.HelmApplicationField{
		{
			Group: mutatorIgnoreDifferencesGroup,
			Kind:  mutatorIgnoreDifferencesKind,
			JSONPointers: []string{
				mutatorIgnoreDifferencesPointer,
			},
		},
	}

	return differences, nil
}

func (m *mutator) PostProvision(_ context.Context) error {
	m.postProvisionCalled = true

	return nil
}

// TestApplicationCreateMutate tests that given the requested input the provisioner
// creates an ArgoCD Application, and the fields are populated as expected.
func TestApplicationCreateMutate(t *testing.T) {
	t.Parallel()

	namespace := "gerbils"

	app := &unikornv1.HelmApplication{
		ObjectMeta: metav1.ObjectMeta{
			Name: applicationName,
		},
		Spec: unikornv1.HelmApplicationSpec{
			Versions: []unikornv1.HelmApplicationVersion{
				{

					Repo:    util.ToPointer(repo),
					Chart:   util.ToPointer(chart),
					Version: util.ToPointer(version),
				},
			},
		},
	}

	tc := mustNewTestContext(t, app, newControlPlaneBundle(app))

	c := gomock.NewController(t)
	defer c.Finish()

	driverAppID := &cd.ResourceIdentifier{
		Name:   applicationName,
		Labels: newControlPlaneResourceLabels(),
	}

	driverApp := &cd.HelmApplication{
		Repo:      repo,
		Chart:     chart,
		Version:   version,
		Release:   mutatorRelease,
		Namespace: namespace,
		Parameters: []cd.HelmApplicationParameter{
			{
				Name:  mutatorParameter,
				Value: mutatorValue,
			},
		},
		Values: mutatorValues,
		IgnoreDifferences: []cd.HelmApplicationField{
			{
				Group: mutatorIgnoreDifferencesGroup,
				Kind:  mutatorIgnoreDifferencesKind,
				JSONPointers: []string{
					mutatorIgnoreDifferencesPointer,
				},
			},
		},
	}

	driver := mock.NewMockDriver(c)
	owner := newControlPlaneResource()

	ctx := context.Background()
	ctx = clientlib.NewContextWithStaticClient(ctx, tc.client)
	ctx = cd.NewContext(ctx, driver)
	ctx = application.NewContext(ctx, owner)

	driver.EXPECT().CreateOrUpdateHelmApplication(ctx, driverAppID, driverApp).Return(nil)

	mutator := &mutator{}
	provisioner := application.New(applicationName).WithGenerator(mutator).InNamespace(namespace)

	assert.NoError(t, provisioner.Provision(ctx))
	assert.True(t, mutator.postProvisionCalled)
}

// TestApplicationDeleteNotFound tests the provisioner returns nil when an application
// doesn't exist.
func TestApplicationDeleteNotFound(t *testing.T) {
	t.Parallel()

	c := gomock.NewController(t)
	defer c.Finish()

	driverAppID := &cd.ResourceIdentifier{
		Name:   applicationName,
		Labels: newControlPlaneResourceLabels(),
	}

	driver := mock.NewMockDriver(c)
	owner := newControlPlaneResource()

	ctx := context.Background()
	ctx = cd.NewContext(ctx, driver)
	ctx = application.NewContext(ctx, owner)

	driver.EXPECT().DeleteHelmApplication(ctx, driverAppID, false).Return(provisioners.ErrYield)

	provisioner := application.New(applicationName)

	assert.ErrorIs(t, provisioner.Deprovision(ctx), provisioners.ErrYield)
}
