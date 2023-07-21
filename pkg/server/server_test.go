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

package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	chi "github.com/go-chi/chi/v5"
	"golang.org/x/oauth2"

	unikornv1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/constants"
	"github.com/eschercloudai/unikorn/pkg/server"
	"github.com/eschercloudai/unikorn/pkg/server/authorization/jose"
	"github.com/eschercloudai/unikorn/pkg/server/authorization/keystone"
	"github.com/eschercloudai/unikorn/pkg/server/generated"
	"github.com/eschercloudai/unikorn/pkg/testutil/assert"
	clientutil "github.com/eschercloudai/unikorn/pkg/util/client"

	kerrors "k8s.io/apimachinery/pkg/api/errors"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	// privKey is the signing key used for JOSE operations.
	privKey = `-----BEGIN PRIVATE KEY-----
MIHuAgEAMBAGByqGSM49AgEGBSuBBAAjBIHWMIHTAgEBBEIB0k3++pOE0i6sEYVE
Wd2kr2FFJwaCUG+Li6fUuam6QCYGrGN9Jg0A0OY5mP7wn4fbqmHVzqIzc5rIj2Fo
iQgNmjmhgYkDgYYABABlgG7igZ59Kl7I/InXVoWY+fmUzOBeHeayXim25ThlXPuZ
yTVEbD2+5PjIRKq/UAIuYyp3e5ZJpB1Npp2pcLEDygAHqQpVkvDKCyws6jENm5dy
sMxe3kmC4XEq7JPJGLkWjTeOZp1bkLB+N0DiCxAeV12h4ckkkYFQmpjMGVlMAK79
ZA==
-----END PRIVATE KEY-----`

	// pubKey is the public key used for JOSE verification operations.
	pubKey = `-----BEGIN CERTIFICATE-----
MIIB6jCCAUygAwIBAgIRAPfmWlg3c63mehO4VfUsQ5UwCgYIKoZIzj0EAwQwIjEg
MB4GA1UEAxMXVW5pa29ybiBTZXJ2ZXIgSk9TRSBLZXkwHhcNMjMwNjI3MTIwMTEz
WhcNMjMwOTI1MTIwMTEzWjAiMSAwHgYDVQQDExdVbmlrb3JuIFNlcnZlciBKT1NF
IEtleTCBmzAQBgcqhkjOPQIBBgUrgQQAIwOBhgAEAGWAbuKBnn0qXsj8iddWhZj5
+ZTM4F4d5rJeKbblOGVc+5nJNURsPb7k+MhEqr9QAi5jKnd7lkmkHU2mnalwsQPK
AAepClWS8MoLLCzqMQ2bl3KwzF7eSYLhcSrsk8kYuRaNN45mnVuQsH43QOILEB5X
XaHhySSRgVCamMwZWUwArv1koyAwHjAOBgNVHQ8BAf8EBAMCBaAwDAYDVR0TAQH/
BAIwADAKBggqhkjOPQQDBAOBiwAwgYcCQVAfjGe5J0kQUACA0jriLiANL0U74LHz
585rKFwe85AU7zN8XRiAbTiN0qJNoA0DqS5I3t9tg7Xm5JCzz5vUW7k/AkIBjXwr
kzmu+BkD1fagFQ5sJVadfwwf0RwT4Z0lzZ8xle2Af7udWnher5JH444GJtJhPD6c
KjAL9BBqzrnOrLYodEk=
-----END CERTIFICATE-----`

	// privKeyFile is where the signing key will live.
	privKeyFile = "/tmp/unikorn-priv-key.pem"

	// pubKeyFile is where the verification key will live.
	pubKeyFile = "/tmp/unikorn-pub-key.pem"
)

var (
	// debug turns on test debugging.
	//nolint:gochecknoglobals
	debug bool
)

func projectNameFromID(projectID string) string {
	return "unikorn-server-" + projectID
}

// writeFile creates the named file and writes the data to it.
func writeFile(path, data string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}

	defer file.Close()

	if _, err := io.WriteString(file, data); err != nil {
		return err
	}

	return nil
}

// TestContext provides a common framework for test execution.
type TestContext struct {
	// openstackEndpoint records the TCP address of the mock openstack.
	openstackEndpoint net.Addr

	// openstackServer is the mock openstack server instance.
	openstackServer *http.Server

	// openstackRouter is the router used by the openstack server instance.
	// This allows you to chop and change handlers based on what responses
	// the test expects.
	openstackRouter chi.Router

	// unikornEndpoint records the TCP address of the unikorn server.
	unikornEndpoint net.Addr

	// unikornServer is the unikorn server instance.
	unikornServer *http.Server

	// kubernetesClient allows fake resources to be tested or mutated to
	// trigger various testing scenarios.
	kubernetesClient client.WithWatch
}

func MustNewTestContext(t *testing.T) (*TestContext, func()) {
	t.Helper()

	scheme, err := clientutil.NewScheme()
	if err != nil {
		t.Fatal(err)
	}

	kubernetesClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	openstackEndpoint, openstackServer, openstackRouter := mustSetupOpenstackServer(t)
	unikornEndpoint, unikornServer := mustSetupUnikornServer(t, openstackEndpoint, kubernetesClient)

	tc := &TestContext{
		openstackEndpoint: openstackEndpoint,
		openstackServer:   openstackServer,
		openstackRouter:   openstackRouter,
		unikornEndpoint:   unikornEndpoint,
		unikornServer:     unikornServer,
		kubernetesClient:  kubernetesClient,
	}

	shutdown := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := openstackServer.Shutdown(ctx); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		if err := unikornServer.Shutdown(ctx); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}

	return tc, shutdown
}

func (t *TestContext) OpenstackServerEndpoint() string {
	return t.openstackEndpoint.String()
}

func (t *TestContext) UnikornServerEndpoint() string {
	return t.unikornEndpoint.String()
}

func (t *TestContext) OpenstackRouter() chi.Router {
	return t.openstackRouter
}

func (t *TestContext) KubernetesClient() client.WithWatch {
	return t.kubernetesClient
}

// mustSetupOpenstackServer starts the openstack mock server running.
func mustSetupOpenstackServer(t *testing.T) (net.Addr, *http.Server, chi.Router) {
	t.Helper()

	router := chi.NewRouter()

	if debug {
		loggingMiddleware := func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Println(r.Method, r.URL.Path)
				next.ServeHTTP(w, r)
			})
		}

		router.Use(loggingMiddleware)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		t.Fatal(err)
	}

	s := &http.Server{
		ReadTimeout:       time.Second,
		ReadHeaderTimeout: time.Second,
		WriteTimeout:      time.Second,
		Handler:           router,
	}

	go func() {
		if err := s.Serve(listener); err != nil {
			if !errors.Is(err, http.ErrServerClosed) {
				fmt.Println(err)
			}
		}
	}()

	return listener.Addr(), s, router
}

// mustSetupUnikornServer starts the unikorn server running.
func mustSetupUnikornServer(t *testing.T, openstack net.Addr, client client.WithWatch) (net.Addr, *http.Server) {
	t.Helper()

	s := &server.Server{
		JoseOptions: jose.Options{
			TLSKeyPath:  privKeyFile,
			TLSCertPath: pubKeyFile,
		},
		KeystoneOptions: keystone.Options{
			Endpoint:      "http://" + openstack.String() + "/identity",
			Domain:        "Default",
			CACertificate: []byte("it's fake mon"),
		},
	}

	if err := s.HandlerOptions.Openstack.Key.Set("LS0tLS1CRUdJTiBQVUJMSUMgS0VZLS0tLS0KTUhZd0VBWUhLb1pJemowQ0FRWUZLNEVFQUNJRFlnQUVmOGs4RVY1TUg4M1BncThYd0JGUTd5YkU2NTEzRlh0awpHaG1jalp4WmYzbU5QOE0vb3VBbE0vZHdYWGpFeXZTNlJhVHdoT3A0aTdHL3VvbE5ZL0RJSCt1elc2VXNxR3VHClFpSW11Tm9BdzFSS1NQcEtyNWlJVXU2eEc1cDR3U3E5Ci0tLS0tRU5EIFBVQkxJQyBLRVktLS0tLQo="); err != nil {
		t.Fatal(err)
	}

	if debug {
		s.SetupLogging()

		if err := s.SetupOpenTelemetry(context.Background()); err != nil {
			t.Fatal(err)
		}
	}

	server, err := s.GetServer(client)
	if err != nil {
		t.Fatal(err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		if err := server.Serve(listener); err != nil {
			if !errors.Is(err, http.ErrServerClosed) {
				fmt.Println(err)
			}
		}
	}()

	return listener.Addr(), server
}

// TestMain is the entry point to the tests, now ideally this wouldn't exist,
// but I'll hazard a guess that making tests parallelizable will cause issues
// given the inherent dynamic nature of the ports.
func TestMain(m *testing.M) {
	flag.BoolVar(&debug, "debug", false, "Turn on test debugging output")
	flag.Parse()

	// Setup actual things are shared.
	if err := writeFile(privKeyFile, privKey); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if err := writeFile(pubKeyFile, pubKey); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Test!
	os.Exit(m.Run())
}

// JSONReader implments io.Reader that does lazy JSON marshaling.
type JSONReader struct {
	data interface{}
	buf  *bytes.Buffer
}

func NewJSONReader(data interface{}) *JSONReader {
	return &JSONReader{
		data: data,
	}
}

func (r *JSONReader) Read(p []byte) (int, error) {
	if r.buf == nil {
		data, err := json.Marshal(r.data)
		if err != nil {
			return 0, err
		}

		r.buf = bytes.NewBuffer(data)
	}

	return r.buf.Read(p)
}

// MustNewUnscopedToken does username/password authentication using oauth2
// and returns the token.
func MustNewUnscopedToken(t *testing.T, tc *TestContext) string {
	t.Helper()

	config := &oauth2.Config{
		Endpoint: oauth2.Endpoint{
			TokenURL: "http://" + tc.UnikornServerEndpoint() + "/api/v1/auth/oauth2/tokens",
		},
	}

	token, err := config.PasswordCredentialsToken(context.TODO(), "foo", "bar")
	assert.NilError(t, err)
	assert.NotEqual(t, token.AccessToken, "")

	return token.AccessToken
}

// bearerTokenInjector allows a generic client to inject a bearer token for authn/authz.
func bearerTokenInjector(token string) generated.RequestEditorFn {
	return func(ctx context.Context, req *http.Request) error {
		req.Header.Set("Authorization", "Bearer "+token)

		return nil
	}
}

// MustNewClient creates a new client, or dies on error.
func MustNewClient(t *testing.T, tc *TestContext, token string) *generated.ClientWithResponses {
	t.Helper()

	client, err := generated.NewClientWithResponses("http://"+tc.UnikornServerEndpoint(), generated.WithRequestEditorFn(bearerTokenInjector(token)))
	assert.NilError(t, err)

	return client
}

// MustNewUnscopedClient creates a new client, or dies on error, implicitly doing
// the oauth2 password grant flow.
func MustNewUnscopedClient(t *testing.T, tc *TestContext) *generated.ClientWithResponses {
	t.Helper()

	return MustNewClient(t, tc, MustNewUnscopedToken(t, tc))
}

// MustNewScopedClient creates a new client, or dies on error, implicitly doing
// the oauth2 password grant flow, then upgrading that to be scoped to a project.
func MustNewScopedClient(t *testing.T, tc *TestContext) *generated.ClientWithResponses {
	t.Helper()

	scope := &generated.TokenScope{
		Project: generated.TokenScopeProject{
			Id: projectID,
		},
	}

	response, err := MustNewUnscopedClient(t, tc).PostApiV1AuthTokensTokenWithBodyWithResponse(context.TODO(), "application/json", NewJSONReader(scope))
	assert.HTTPResponse(t, response.HTTPResponse, http.StatusCreated, err)

	return MustNewClient(t, tc, response.JSON201.AccessToken)
}

// TestApiV1AuthOAuth2TokensPassword tests the oauth2 password grant flow works.
func TestApiV1AuthOAuth2TokensPassword(t *testing.T) {
	t.Parallel()

	tc, cleanup := MustNewTestContext(t)
	defer cleanup()

	RegisterIdentityHandlers(tc)

	_ = MustNewUnscopedClient(t, tc)
}

// TestApiV1AuthTokensToken tests an unscoped token can be scoped to a project.
func TestApiV1AuthTokensToken(t *testing.T) {
	t.Parallel()

	tc, cleanup := MustNewTestContext(t)
	defer cleanup()

	RegisterIdentityHandlers(tc)

	_ = MustNewScopedClient(t, tc)
}

// TestApiV1ProjectCreate tests that a project scoped token can create a project
// with the correct name, and delete it.
func TestApiV1ProjectCreateAndDelete(t *testing.T) {
	t.Parallel()

	tc, cleanup := MustNewTestContext(t)
	defer cleanup()

	RegisterIdentityHandlers(tc)

	unikornClient := MustNewScopedClient(t, tc)

	createResponse, err := unikornClient.PostApiV1Project(context.TODO())
	assert.HTTPResponse(t, createResponse, http.StatusAccepted, err)

	defer createResponse.Body.Close()

	var project unikornv1.Project

	assert.NilError(t, tc.KubernetesClient().Get(context.TODO(), client.ObjectKey{Name: projectNameFromID(projectID)}, &project))
	assert.MapSet(t, project.Labels, constants.VersionLabel)

	deleteResponse, err := unikornClient.DeleteApiV1Project(context.TODO())
	assert.HTTPResponse(t, deleteResponse, http.StatusAccepted, err)

	defer deleteResponse.Body.Close()

	assert.KubernetesError(t, kerrors.IsNotFound, tc.KubernetesClient().Get(context.TODO(), client.ObjectKey{Name: projectNameFromID(projectID)}, &project))
}

// TestApiV1ControlPlaneCreate tests that a control plane can be created
// in a project.
func TestApiV1ControlPlanesCreate(t *testing.T) {
	t.Parallel()

	tc, cleanup := MustNewTestContext(t)
	defer cleanup()

	RegisterIdentityHandlers(tc)

	// Put some fixtures into place...
	project := mustCreateProjectFixture(t, tc, projectID)

	// Create the control plane...
	unikornClient := MustNewScopedClient(t, tc)

	request := &generated.ControlPlane{
		Name: "foo",
		ApplicationBundle: generated.ApplicationBundle{
			Name: "foo",
		},
	}

	response, err := unikornClient.PostApiV1ControlplanesWithBody(context.TODO(), "application/json", NewJSONReader(request))
	assert.HTTPResponse(t, response, http.StatusAccepted, err)

	defer response.Body.Close()

	// Check it exists in the project namespace.
	var resource unikornv1.ControlPlane

	assert.NilError(t, tc.KubernetesClient().Get(context.TODO(), client.ObjectKey{Namespace: project.Status.Namespace, Name: "foo"}, &resource))
	assert.MapSet(t, resource.Labels, constants.VersionLabel)
	assert.MapSet(t, resource.Labels, constants.ProjectLabel)
}

// TestApiV1ControlPlanesGet tests a control plane can be retrieved and
// that its fields are completed as specified by the schema.  This flexes
// compositing of resources e.g. expansion of application bundles.
func TestApiV1ControlPlanesGet(t *testing.T) {
	t.Parallel()

	tc, cleanup := MustNewTestContext(t)
	defer cleanup()

	RegisterIdentityHandlers(tc)

	// Put some fixtures into place...
	project := mustCreateProjectFixture(t, tc, projectID)
	mustCreateControlPlaneFixture(t, tc, project.Status.Namespace, "foo")
	mustCreateControlPlaneApplicationBundleFixture(t, tc)

	unikornClient := MustNewScopedClient(t, tc)

	response, err := unikornClient.GetApiV1ControlplanesControlPlaneNameWithResponse(context.TODO(), "foo")
	assert.HTTPResponse(t, response.HTTPResponse, http.StatusOK, err)
	assert.NotNil(t, response.JSON200)

	result := *response.JSON200

	assert.Equal(t, "foo", result.Name)
	assert.NotNil(t, result.Status)
	assert.Equal(t, "Provisioned", result.Status.Status)
	assert.Equal(t, controlPlaneApplicationBundleName, result.ApplicationBundle.Name)
	assert.Equal(t, controlPlaneApplicationBundleVersion, result.ApplicationBundle.Version)
}

// TestApiV1ControlPlanesList tests a control planes can be retrieved and
// that their fields are completed as specified by the schema.  This flexes
// compositing of resources e.g. expansion of application bundles.
func TestApiV1ControlPlanesList(t *testing.T) {
	t.Parallel()

	tc, cleanup := MustNewTestContext(t)
	defer cleanup()

	RegisterIdentityHandlers(tc)

	// Put some fixtures into place...
	project := mustCreateProjectFixture(t, tc, projectID)
	mustCreateControlPlaneFixture(t, tc, project.Status.Namespace, "foo")
	mustCreateControlPlaneApplicationBundleFixture(t, tc)

	unikornClient := MustNewScopedClient(t, tc)

	response, err := unikornClient.GetApiV1ControlplanesWithResponse(context.TODO())
	assert.HTTPResponse(t, response.HTTPResponse, http.StatusOK, err)
	assert.NotNil(t, response.JSON200)

	results := *response.JSON200

	assert.Equal(t, 1, len(results))
	assert.Equal(t, "foo", results[0].Name)
	assert.Equal(t, controlPlaneApplicationBundleName, results[0].ApplicationBundle.Name)
	assert.Equal(t, controlPlaneApplicationBundleVersion, results[0].ApplicationBundle.Version)
}

// TestApiV1ControlPlanesDelete tests a control plane can be deleted.
func TestApiV1ControlPlanesDelete(t *testing.T) {
	t.Parallel()

	tc, cleanup := MustNewTestContext(t)
	defer cleanup()

	RegisterIdentityHandlers(tc)

	// Put some fixtures into place...
	project := mustCreateProjectFixture(t, tc, projectID)
	mustCreateControlPlaneFixture(t, tc, project.Status.Namespace, "foo")

	unikornClient := MustNewScopedClient(t, tc)

	response, err := unikornClient.DeleteApiV1ControlplanesControlPlaneName(context.TODO(), "foo")
	assert.HTTPResponse(t, response, http.StatusAccepted, err)

	defer response.Body.Close()

	var resource unikornv1.ControlPlane

	assert.KubernetesError(t, kerrors.IsNotFound, tc.KubernetesClient().Get(context.TODO(), client.ObjectKey{Namespace: project.Status.Namespace, Name: "foo"}, &resource))
}

// TestApiV1ClustersCreate tests that a cluster can be created
// in a control plane.
func TestApiV1ClustersCreate(t *testing.T) {
	t.Parallel()

	tc, cleanup := MustNewTestContext(t)
	defer cleanup()

	RegisterIdentityHandlers(tc)
	RegisterImageV2Images(tc)
	RegisterComputeV2FlavorsDetail(tc)
	RegisterComputeV2ServerGroups(tc)

	// Put some fixtures into place...
	project := mustCreateProjectFixture(t, tc, projectID)
	controlPlane := mustCreateControlPlaneFixture(t, tc, project.Status.Namespace, "foo")

	// Create the cluster...
	unikornClient := MustNewScopedClient(t, tc)

	clusterRequest := &generated.KubernetesCluster{
		Name: "foo",
		ApplicationBundle: generated.ApplicationBundle{
			Name: "foo",
		},
		Network: generated.KubernetesClusterNetwork{
			DnsNameservers: []string{
				"8.8.8.8",
			},
			NodePrefix:    "192.168.0.0/24",
			ServicePrefix: "172.16.0.0/12",
			PodPrefix:     "10.0.0.0/8",
		},
		ControlPlane: generated.OpenstackMachinePool{
			Version:    "v1.28.0",
			Replicas:   3,
			ImageName:  "ubuntu-24.04-lts",
			FlavorName: "strawberry",
		},
		WorkloadPools: generated.KubernetesClusterWorkloadPools{
			{
				Name: "foo",
				Machine: generated.OpenstackMachinePool{
					Version:    "v1.28.0",
					Replicas:   3,
					ImageName:  "ubuntu-24.04-lts",
					FlavorName: "strawberry",
				},
			},
		},
	}

	response, err := unikornClient.PostApiV1ControlplanesControlPlaneNameClustersWithBody(context.TODO(), controlPlane.Name, "application/json", NewJSONReader(clusterRequest))
	assert.HTTPResponse(t, response, http.StatusAccepted, err)

	defer response.Body.Close()

	// Check it exists in the control plane namespace.
	var resource unikornv1.KubernetesCluster

	assert.NilError(t, tc.KubernetesClient().Get(context.TODO(), client.ObjectKey{Namespace: controlPlane.Status.Namespace, Name: "foo"}, &resource))
	assert.MapSet(t, resource.Labels, constants.VersionLabel)
	assert.MapSet(t, resource.Labels, constants.ProjectLabel)
	assert.MapSet(t, resource.Labels, constants.ControlPlaneLabel)
}

func TestApiV1ClustersGet(t *testing.T) {
	t.Parallel()

	tc, cleanup := MustNewTestContext(t)
	defer cleanup()

	RegisterIdentityHandlers(tc)
	RegisterImageV2Images(tc)
	RegisterComputeV2FlavorsDetail(tc)
	RegisterComputeV2ServerGroups(tc)

	// Put some fixtures into place...
	project := mustCreateProjectFixture(t, tc, projectID)
	controlPlane := mustCreateControlPlaneFixture(t, tc, project.Status.Namespace, "foo")
	mustCreateKubernetesClusterFixture(t, tc, controlPlane.Status.Namespace, "foo")
	mustKubernetesClusterApplicationBundleFixture(t, tc)

	// Create the cluster...
	unikornClient := MustNewScopedClient(t, tc)

	response, err := unikornClient.GetApiV1ControlplanesControlPlaneNameClustersClusterNameWithResponse(context.TODO(), controlPlane.Name, "foo")
	assert.HTTPResponse(t, response.HTTPResponse, http.StatusOK, err)
	assert.NotNil(t, response.JSON200)

	result := *response.JSON200

	assert.Equal(t, "foo", result.Name)
	assert.NotNil(t, result.Status)
	assert.Equal(t, "Provisioned", result.Status.Status)
	assert.Equal(t, kubernetesClusterApplicationBundleName, result.ApplicationBundle.Name)
	assert.Equal(t, kubernetesClusterApplicationBundleVersion, result.ApplicationBundle.Version)
	assert.Equal(t, clusterComputeFailureDomain, result.Openstack.ComputeAvailabilityZone)
	assert.Equal(t, clusterStorageFailureDomain, result.Openstack.VolumeAvailabilityZone)
	assert.Equal(t, clusterExternalNetworkID, result.Openstack.ExternalNetworkID)
	assert.NotNil(t, result.Openstack.SshKeyName)
	assert.Equal(t, clusterSSHKeyName, *result.Openstack.SshKeyName)
	assert.Equal(t, clusterNodeNetwork, result.Network.NodePrefix)
	assert.Equal(t, clusterServiceNetwork, result.Network.ServicePrefix)
	assert.Equal(t, clusterPodNetwork, result.Network.PodPrefix)
	assert.Equal(t, 1, len(result.Network.DnsNameservers))
	assert.Equal(t, clusterDNSNameserver, result.Network.DnsNameservers[0])
	assert.Equal(t, "v"+imageK8sVersion, result.ControlPlane.Version)
	assert.Equal(t, imageName, result.ControlPlane.ImageName)
	assert.Equal(t, flavorName, result.ControlPlane.FlavorName)
	assert.Equal(t, clusterControlPlaneReplicas, result.ControlPlane.Replicas)
	assert.Equal(t, 1, len(result.WorkloadPools))
	assert.Equal(t, clusterWorkloadPoolName, result.WorkloadPools[0].Name)
	assert.Equal(t, "v"+imageK8sVersion, result.WorkloadPools[0].Machine.Version)
	assert.Equal(t, imageName, result.WorkloadPools[0].Machine.ImageName)
	assert.Equal(t, flavorName, result.WorkloadPools[0].Machine.FlavorName)
	assert.Equal(t, clusterWorkloadPoolReplicas, result.WorkloadPools[0].Machine.Replicas)
}

func TestApiV1ClustersList(t *testing.T) {
	t.Parallel()

	tc, cleanup := MustNewTestContext(t)
	defer cleanup()

	RegisterIdentityHandlers(tc)
	RegisterImageV2Images(tc)
	RegisterComputeV2FlavorsDetail(tc)
	RegisterComputeV2ServerGroups(tc)

	// Put some fixtures into place...
	project := mustCreateProjectFixture(t, tc, projectID)
	controlPlane := mustCreateControlPlaneFixture(t, tc, project.Status.Namespace, "foo")
	mustCreateKubernetesClusterFixture(t, tc, controlPlane.Status.Namespace, "foo")
	mustKubernetesClusterApplicationBundleFixture(t, tc)

	// Create the cluster...
	unikornClient := MustNewScopedClient(t, tc)

	response, err := unikornClient.GetApiV1ControlplanesControlPlaneNameClustersWithResponse(context.TODO(), controlPlane.Name)
	assert.HTTPResponse(t, response.HTTPResponse, http.StatusOK, err)
	assert.NotNil(t, response.JSON200)

	results := *response.JSON200

	assert.Equal(t, 1, len(results))
	assert.Equal(t, "foo", results[0].Name)
	assert.NotNil(t, results[0].Status)
	assert.Equal(t, "Provisioned", results[0].Status.Status)
	assert.Equal(t, kubernetesClusterApplicationBundleName, results[0].ApplicationBundle.Name)
	assert.Equal(t, kubernetesClusterApplicationBundleVersion, results[0].ApplicationBundle.Version)
	assert.Equal(t, clusterComputeFailureDomain, results[0].Openstack.ComputeAvailabilityZone)
	assert.Equal(t, clusterStorageFailureDomain, results[0].Openstack.VolumeAvailabilityZone)
	assert.Equal(t, clusterExternalNetworkID, results[0].Openstack.ExternalNetworkID)
	assert.NotNil(t, results[0].Openstack.SshKeyName)
	assert.Equal(t, clusterSSHKeyName, *results[0].Openstack.SshKeyName)
	assert.Equal(t, clusterNodeNetwork, results[0].Network.NodePrefix)
	assert.Equal(t, clusterServiceNetwork, results[0].Network.ServicePrefix)
	assert.Equal(t, clusterPodNetwork, results[0].Network.PodPrefix)
	assert.Equal(t, 1, len(results[0].Network.DnsNameservers))
	assert.Equal(t, clusterDNSNameserver, results[0].Network.DnsNameservers[0])
	assert.Equal(t, "v"+imageK8sVersion, results[0].ControlPlane.Version)
	assert.Equal(t, imageName, results[0].ControlPlane.ImageName)
	assert.Equal(t, flavorName, results[0].ControlPlane.FlavorName)
	assert.Equal(t, clusterControlPlaneReplicas, results[0].ControlPlane.Replicas)
	assert.Equal(t, 1, len(results[0].WorkloadPools))
	assert.Equal(t, clusterWorkloadPoolName, results[0].WorkloadPools[0].Name)
	assert.Equal(t, "v"+imageK8sVersion, results[0].WorkloadPools[0].Machine.Version)
	assert.Equal(t, imageName, results[0].WorkloadPools[0].Machine.ImageName)
	assert.Equal(t, flavorName, results[0].WorkloadPools[0].Machine.FlavorName)
	assert.Equal(t, clusterWorkloadPoolReplicas, results[0].WorkloadPools[0].Machine.Replicas)
}

func TestApiV1ClustersDelete(t *testing.T) {
	t.Parallel()

	tc, cleanup := MustNewTestContext(t)
	defer cleanup()

	RegisterIdentityHandlers(tc)
	RegisterImageV2Images(tc)
	RegisterComputeV2FlavorsDetail(tc)
	RegisterComputeV2ServerGroups(tc)

	// Put some fixtures into place...
	project := mustCreateProjectFixture(t, tc, projectID)
	controlPlane := mustCreateControlPlaneFixture(t, tc, project.Status.Namespace, "foo")
	mustCreateKubernetesClusterFixture(t, tc, controlPlane.Status.Namespace, "foo")
	mustKubernetesClusterApplicationBundleFixture(t, tc)

	unikornClient := MustNewScopedClient(t, tc)

	response, err := unikornClient.DeleteApiV1ControlplanesControlPlaneNameClustersClusterName(context.TODO(), controlPlane.Name, "foo")
	assert.HTTPResponse(t, response, http.StatusAccepted, err)

	defer response.Body.Close()

	var resource unikornv1.KubernetesCluster

	assert.KubernetesError(t, kerrors.IsNotFound, tc.KubernetesClient().Get(context.TODO(), client.ObjectKey{Namespace: controlPlane.Status.Namespace, Name: "foo"}, &resource))
}

// TestApiV1ProvidersOpenstackProjects tests OpenStack projects can be listed.
func TestApiV1ProvidersOpenstackProjects(t *testing.T) {
	t.Parallel()

	tc, cleanup := MustNewTestContext(t)
	defer cleanup()

	RegisterIdentityHandlers(tc)

	unikornClient := MustNewScopedClient(t, tc)

	response, err := unikornClient.GetApiV1ProvidersOpenstackProjectsWithResponse(context.TODO())
	assert.HTTPResponse(t, response.HTTPResponse, http.StatusOK, err)
	assert.NotNil(t, response.JSON200)

	results := *response.JSON200

	assert.Equal(t, 1, len(results))
	assert.Equal(t, projectID, results[0].Id)
	assert.Equal(t, projectName, results[0].Name)
}

// TestApiV1ProvidersOpenstackFlavors tests OpenStack flavors can be listed.
func TestApiV1ProvidersOpenstackFlavors(t *testing.T) {
	t.Parallel()

	tc, cleanup := MustNewTestContext(t)
	defer cleanup()

	RegisterIdentityHandlers(tc)
	RegisterComputeV2FlavorsDetail(tc)

	unikornClient := MustNewScopedClient(t, tc)

	response, err := unikornClient.GetApiV1ProvidersOpenstackFlavorsWithResponse(context.TODO())
	assert.HTTPResponse(t, response.HTTPResponse, http.StatusOK, err)
	assert.NotNil(t, response.JSON200)

	results := *response.JSON200

	// NOTE: server converts from MiB to GiB of memory.
	assert.Equal(t, 1, len(results))
	assert.Equal(t, flavorID, results[0].Id)
	assert.Equal(t, flavorCpus, results[0].Cpus)
	assert.Equal(t, flavorMemory>>10, results[0].Memory)
	assert.Equal(t, flavorDisk, results[0].Disk)
}

// TestApiV1ProvidersOpenstackImages tests OpenStack images can be listed.
func TestApiV1ProvidersOpenstackImages(t *testing.T) {
	t.Parallel()

	tc, cleanup := MustNewTestContext(t)
	defer cleanup()

	RegisterIdentityHandlers(tc)
	RegisterImageV2Images(tc)

	unikornClient := MustNewScopedClient(t, tc)

	response, err := unikornClient.GetApiV1ProvidersOpenstackImagesWithResponse(context.TODO())
	assert.HTTPResponse(t, response.HTTPResponse, http.StatusOK, err)
	assert.NotNil(t, response.JSON200)

	results := *response.JSON200

	ts, err := time.Parse(time.RFC3339, imageTimestamp)
	assert.NilError(t, err)

	// NOTE: server converts kubernetes version to a proper semver so it's compatible
	// with the CAPI kubeadm controller.
	assert.Equal(t, 2, len(results))
	assert.Equal(t, imageID, results[0].Id)
	assert.Equal(t, imageName, results[0].Name)
	assert.Equal(t, ts, results[0].Created)
	assert.Equal(t, ts, results[0].Modified)
	assert.Equal(t, "v"+imageK8sVersion, results[0].Versions.Kubernetes)
	assert.Equal(t, imageGpuVersion, results[0].Versions.NvidiaDriver)
}

// TestApiV1ProvidersOpenstackAvailabilityZonesCompute tests OpenStack compute AZscan be listed.
func TestApiV1ProvidersOpenstackAvailabilityZonesCompute(t *testing.T) {
	t.Parallel()

	tc, cleanup := MustNewTestContext(t)
	defer cleanup()

	RegisterIdentityHandlers(tc)
	RegisterComputeV2AvailabilityZone(tc)

	unikornClient := MustNewScopedClient(t, tc)

	response, err := unikornClient.GetApiV1ProvidersOpenstackAvailabilityZonesComputeWithResponse(context.TODO())
	assert.HTTPResponse(t, response.HTTPResponse, http.StatusOK, err)
	assert.NotNil(t, response.JSON200)

	results := *response.JSON200

	assert.Equal(t, 1, len(results))
	assert.Equal(t, computeAvailabilityZoneName, results[0].Name)
}

// TestApiV1ProvidersOpenstackAvailabilityZonesBlockStorage tests OpenStack block storage AZscan be listed.
func TestApiV1ProvidersOpenstackAvailabilityZonesBlockStorage(t *testing.T) {
	t.Parallel()

	tc, cleanup := MustNewTestContext(t)
	defer cleanup()

	RegisterIdentityHandlers(tc)
	RegisterBlockStorageV3AvailabilityZone(tc)

	unikornClient := MustNewScopedClient(t, tc)

	response, err := unikornClient.GetApiV1ProvidersOpenstackAvailabilityZonesBlockStorageWithResponse(context.TODO())
	assert.HTTPResponse(t, response.HTTPResponse, http.StatusOK, err)
	assert.NotNil(t, response.JSON200)

	results := *response.JSON200

	assert.Equal(t, 1, len(results))
	assert.Equal(t, blockStorageAvailabilityZone, results[0].Name)
}

// TestApiV1ProvidersOpenstackExternalNetworks tests OpenStack external networks can be listed.
func TestApiV1ProvidersOpenstackExternalNetworks(t *testing.T) {
	t.Parallel()

	tc, cleanup := MustNewTestContext(t)
	defer cleanup()

	RegisterIdentityHandlers(tc)
	RegisterNetworkV2Networks(tc)

	unikornClient := MustNewScopedClient(t, tc)

	response, err := unikornClient.GetApiV1ProvidersOpenstackExternalNetworksWithResponse(context.TODO())
	assert.HTTPResponse(t, response.HTTPResponse, http.StatusOK, err)
	assert.NotNil(t, response.JSON200)

	results := *response.JSON200

	assert.Equal(t, 1, len(results))
	assert.Equal(t, externalNetworkID, results[0].Id)
}

// TestApiV1ProvidersOpenstackKeyPairs tests OpenStack key pairs can be listed.
func TestApiV1ProvidersOpenstackKeyPairs(t *testing.T) {
	t.Parallel()

	tc, cleanup := MustNewTestContext(t)
	defer cleanup()

	RegisterIdentityHandlers(tc)
	RegisterComputeV2Keypairs(tc)

	unikornClient := MustNewScopedClient(t, tc)

	response, err := unikornClient.GetApiV1ProvidersOpenstackKeyPairsWithResponse(context.TODO())
	assert.HTTPResponse(t, response.HTTPResponse, http.StatusOK, err)
	assert.NotNil(t, response.JSON200)

	results := *response.JSON200

	assert.Equal(t, 1, len(results))
	assert.Equal(t, keyPairName, results[0].Name)
}
