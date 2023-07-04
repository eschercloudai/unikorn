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
	"github.com/eschercloudai/unikorn/pkg/server"
	"github.com/eschercloudai/unikorn/pkg/server/authorization/jose"
	"github.com/eschercloudai/unikorn/pkg/server/authorization/keystone"
	"github.com/eschercloudai/unikorn/pkg/server/generated"
	"github.com/eschercloudai/unikorn/pkg/testutil"
	clientutil "github.com/eschercloudai/unikorn/pkg/util/client"

	kerrors "k8s.io/apimachinery/pkg/api/errors"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	// userID is the mocked user ID.
	userID = "5e6bb9d8-03a1-4d26-919c-6884ff574a31"

	// projectID is the mocked project.
	projectID = "d09544ac-be0e-428b-8834-697c796b48a5"

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

func projectName(projectID string) string {
	return "unikorn-server-" + projectID
}

// v3AuthTokensSuccessResponse defines how we mock the OpenStack API.  Basically we'll
// multiplex all services through a single endpoint for simplicity.
// Important parts to pay attention to (in the context of gophercloud):
// * token.catalog.type is used to look for the service.
// * token.catalog.endpoints.interface is used to look a service endpoint, "public" is the default.
// * token.user.id is used by Unikorn for identity information in its access token.
func v3AuthTokensSuccessResponse(tc *TestContext) string {
	return `{
	"token": {
		"catalog": [
			{
				"name": "keystone",
				"type": "identity",
				"endpoints": [
					{
						"interface": "public",
						"region": "RegionOne",
						"region_id": "RegionOne",
						"url": "http://` + tc.OpenstackServerEndpoint() + `/identity"
					}
				]
			},
			{
				"name": "nova",
				"type": "compute",
				"endpoints": [
					{
						"interface": "public",
						"region": "RegionOne",
                                                "region_id": "RegionOne",
                                                "url": "http://` + tc.OpenstackServerEndpoint() + `/compute"
					}
				]
			},
			{
				"name": "glance",
				"type": "image",
				"endpoints": [
                                        {
                                                "interface": "public",
                                                "region": "RegionOne",
                                                "region_id": "RegionOne",
                                                "url": "http://` + tc.OpenstackServerEndpoint() + `/image"
                                        }
                                ]
                        }
		],
		"domain": {
			"id": "default",
			"name": "Default"
		},
		"methods": [
			"password"
		],
		"expires_at": "` + time.Now().Add(time.Hour).Format(time.RFC3339) + `",
		"user": {
			"domain": {
				"id": "default",
				"name": "Default"
			},
			"id": "` + userID + `",
			"name": "foo"
		}
	}
}`
}

// RegisterIdentityV3AuthTokensPostSuccessHandler is called when we want to login, or do a
// token exchange/rescoping.
func RegisterIdentityV3AuthTokensPostSuccessHandler(tc *TestContext) {
	tc.OpenstackRouter().Post("/identity/v3/auth/tokens", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Subject-Token", "ImAToken")
		w.WriteHeader(http.StatusCreated)
		if _, err := w.Write([]byte(v3AuthTokensSuccessResponse(tc))); err != nil {
			if debug {
				fmt.Println(err)
			}
		}
	})
}

// RegisterIdentityV3AuthTokensGetSuccessHandler is called by gophercloud to validate a
// token and to get the service catalog.
func RegisterIdentityV3AuthTokensGetSuccessHandler(tc *TestContext) {
	tc.OpenstackRouter().Get("/identity/v3/auth/tokens", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Subject-Token", "ImAToken")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(v3AuthTokensSuccessResponse(tc))); err != nil {
			if debug {
				fmt.Println(err)
			}
		}
	})
}

// identityMetadata returns versioned endpoint information for the identity service.
// Gophercloud will check the links and preferentially select v3 over v2_0.
func identityMetadata(tc *TestContext) string {
	return `{
	"versions": {
		"values": [
			{
				"id": "v3.14",
				"status": "stable",
				"links": [
					{
						"rel": "self",
						"href": "http://` + tc.OpenstackServerEndpoint() + `/identity/v3"
					}
				],
				"media-types": [
					{
						"base": "application/json",
						"type": "application/vnd.openstack.identity-v3+json"
					}
				]
			}
		]
	}
}`
}

// RegisterIdentityHandler allows gophercloud to derive the correct base path to
// use for identity operations.
func RegisterIdentityHandler(tc *TestContext) {
	tc.OpenstackRouter().Get("/identity/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(identityMetadata(tc))); err != nil {
			if debug {
				fmt.Println(err)
			}
		}
	})
}

// userInfo returns user information based on the token.
// Only email is considered by Unikorn server at present.
const userInfo = `{
	"user": {
		"domain_id": "default",
		"enabled": true,
		"id": "` + userID + `",
		"name": "foo",
		"email": "foo@bar.com"
	}
}`

// RegisterIdentityV3User allows Unikorn to lookup a user in oder to issue
// an access token.
func RegisterIdentityV3User(tc *TestContext) {
	tc.OpenstackRouter().Get("/identity/v3/users/{user_id}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(userInfo)); err != nil {
			if debug {
				fmt.Println(err)
			}
		}
	})
}

// emptyApplicationCredentials is what you get when you list credentials and
// there are none.
//
//nolint:gosec
const emptyApplicationCredentials = `{
	"links": {},
	"application_credentials": []
}`

// applicationCredentialCreate is what you get when you create an application
// credential.  Please note this is the ONLY time it will return the secret.
const applicationCredentialCreate = `{
	"application_credential": {
		"id": "69a5f849-5112-44b7-9424-64ee0f30c23d",
		"name": "foo",
		"secret": "shhhh"
	}
}`

func RegisterIdentityV3UserApplicationCredentials(tc *TestContext) {
	tc.OpenstackRouter().Get("/identity/v3/users/{user_id}/application_credentials", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(emptyApplicationCredentials)); err != nil {
			if debug {
				fmt.Println(err)
			}
		}
	})
	tc.OpenstackRouter().Post("/identity/v3/users/{user_id}/application_credentials", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if _, err := w.Write([]byte(applicationCredentialCreate)); err != nil {
			if debug {
				fmt.Println(err)
			}
		}
	})
}

// RegisterIdentityHandlers adds all the basic handlers required for token
// acquisition.
func RegisterIdentityHandlers(tc *TestContext) {
	RegisterIdentityHandler(tc)
	RegisterIdentityV3AuthTokensPostSuccessHandler(tc)
	RegisterIdentityV3AuthTokensGetSuccessHandler(tc)
	RegisterIdentityV3User(tc)
	RegisterIdentityV3UserApplicationCredentials(tc)
}

const images = `{
	"first": "/images/v2/images",
	"images": [
		{
			"id": "6daa3bee-63b8-48a3-a082-52ad680dd3c0",
			"name": "ubuntu-24.04-lts",
			"status": "active",
			"created_at": "2020-01-01T00:00:00Z",
			"k8s": "1.28.0",
			"gpu": "525.85.05",
			"digest": "MGYCMQD9kCkukyFePyvNbKe8/DLC4BZAyNJb6e5EvEqf1guR63qBr7E55/GKTVFoWBPS/v0CMQD9AK4aLdRhzWNoAC/IPT7lKQ6k20A/l/CN3cH9x8Qq9y7kfzPUOP1C15nJZsinpzk="
		}
	]
}`

func RegisterImageV2Images(tc *TestContext) {
	tc.OpenstackRouter().Get("/image/v2/images", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(images)); err != nil {
			if debug {
				fmt.Println(err)
			}
		}
	})
}

// NOTE: Extra specs are available in microversion 2.61 onward.
const flavorsDetail = `{
	"first": "/flavors/detail",
	"flavors": [
		{
			"id": "f547e5e4-5d9e-4434-bb78-d43cabcce79c",
			"name": "strawberry",
			"extra_specs": {
				"resources:VGPU": "1",
				"trait:CUSTOM_A100D_3_40C": "required"
			}
		}
	]
}`

func RegisterComputeV2FlavorsDetail(tc *TestContext) {
	tc.OpenstackRouter().Get("/compute/flavors/detail", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(flavorsDetail)); err != nil {
			if debug {
				fmt.Println(err)
			}
		}
	})
}

const serverGroupsEmpty = `{
	"first": "/os-server-groups",
	"server_groups": []
}`

const serverGroup = `{
	"server_group": {
		"id": "51ec3d7e-c52b-4b47-aa82-c99bc374ea23",
		"name": "foo",
		"policies": [
			"soft-anti-affinity"
		]
	}
}`

func RegisterComputeV2ServerGroups(tc *TestContext) {
	tc.OpenstackRouter().Get("/compute/os-server-groups", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(serverGroupsEmpty)); err != nil {
			if debug {
				fmt.Println(err)
			}
		}
	})
	tc.OpenstackRouter().Post("/compute/os-server-groups", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(serverGroup)); err != nil {
			if debug {
				fmt.Println(err)
			}
		}
	})
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
	testutil.AssertNilError(t, err)
	testutil.AssertNotEqual(t, token.AccessToken, "")

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
	testutil.AssertNilError(t, err)

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
	testutil.AssertHTTPResponse(t, response.HTTPResponse, http.StatusCreated, err)

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
	testutil.AssertHTTPResponse(t, createResponse, http.StatusAccepted, err)

	defer createResponse.Body.Close()

	var project unikornv1.Project

	testutil.AssertNilError(t, tc.KubernetesClient().Get(context.TODO(), client.ObjectKey{Name: projectName(projectID)}, &project))

	deleteResponse, err := unikornClient.DeleteApiV1Project(context.TODO())
	testutil.AssertHTTPResponse(t, deleteResponse, http.StatusAccepted, err)

	defer deleteResponse.Body.Close()

	testutil.AssertKubernetesError(t, kerrors.IsNotFound, tc.KubernetesClient().Get(context.TODO(), client.ObjectKey{Name: projectName(projectID)}, &project))
}

// TestApiV1ControlPlaneCreateAndDelete tests that a control plane can be created
// in a project, and then deleted.
func TestApiV1ControlPlanesCreateAndDelete(t *testing.T) {
	t.Parallel()

	tc, cleanup := MustNewTestContext(t)
	defer cleanup()

	RegisterIdentityHandlers(tc)

	// Put some fixtures into place...
	project := mustCreateProjectFixture(t, tc, projectID)

	// Create the control plane...
	unikornClient := MustNewScopedClient(t, tc)

	controlPlaneRequest := &generated.ControlPlane{
		Name: "foo",
		ApplicationBundle: generated.ApplicationBundle{
			Name: "foo",
		},
	}

	createResponse, err := unikornClient.PostApiV1ControlplanesWithBody(context.TODO(), "application/json", NewJSONReader(controlPlaneRequest))
	testutil.AssertHTTPResponse(t, createResponse, http.StatusAccepted, err)

	defer createResponse.Body.Close()

	// Check it exists in the project namespace.
	// TODO: check the required metadata has been added by server.
	var controlPlaneResource unikornv1.ControlPlane

	testutil.AssertNilError(t, tc.KubernetesClient().Get(context.TODO(), client.ObjectKey{Namespace: project.Status.Namespace, Name: "foo"}, &controlPlaneResource))

	deleteResponse, err := unikornClient.DeleteApiV1ControlplanesControlPlaneName(context.TODO(), "foo")
	testutil.AssertHTTPResponse(t, deleteResponse, http.StatusAccepted, err)

	defer deleteResponse.Body.Close()

	testutil.AssertKubernetesError(t, kerrors.IsNotFound, tc.KubernetesClient().Get(context.TODO(), client.ObjectKey{Namespace: project.Status.Namespace, Name: "foo"}, &controlPlaneResource))
}

// TestApiV1ClustersCreateAndDelete tests that a cluster can be created
// in a control plane, and then deleted.
func TestApiV1ClustersCreateAndDelete(t *testing.T) {
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

	createResponse, err := unikornClient.PostApiV1ControlplanesControlPlaneNameClustersWithBody(context.TODO(), controlPlane.Name, "application/json", NewJSONReader(clusterRequest))
	testutil.AssertHTTPResponse(t, createResponse, http.StatusAccepted, err)

	defer createResponse.Body.Close()

	// Check it exists in the control plane namespace.
	// TODO: check the required metadata has been added by server.
	var clusterResource unikornv1.KubernetesCluster

	testutil.AssertNilError(t, tc.KubernetesClient().Get(context.TODO(), client.ObjectKey{Namespace: controlPlane.Status.Namespace, Name: "foo"}, &clusterResource))

	deleteResponse, err := unikornClient.DeleteApiV1ControlplanesControlPlaneNameClustersClusterName(context.TODO(), controlPlane.Name, "foo")
	testutil.AssertHTTPResponse(t, deleteResponse, http.StatusAccepted, err)

	defer deleteResponse.Body.Close()

	testutil.AssertKubernetesError(t, kerrors.IsNotFound, tc.KubernetesClient().Get(context.TODO(), client.ObjectKey{Namespace: controlPlane.Status.Namespace, Name: "foo"}, &clusterResource))
}
