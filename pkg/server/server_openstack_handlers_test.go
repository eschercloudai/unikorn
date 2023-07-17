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
	"fmt"
	"net/http"
	"time"
)

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
                        },
			{
				"name": "neutron",
				"type": "network",
				"endpoints": [
					{
						"interface": "public",
                                                "region": "RegionOne",
                                                "region_id": "RegionOne",
                                                "url": "http://` + tc.OpenstackServerEndpoint() + `/network"
					}
				]
			},
			{
                                "name": "cinder",
                                "type": "volumev3",
				"endpoints": [
                                        {
                                                "interface": "public",
                                                "region": "RegionOne",
                                                "region_id": "RegionOne",
                                                "url": "http://` + tc.OpenstackServerEndpoint() + `/blockstorage"
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

const projects = `{
	"projects": [],
	"links": {}
}`

func RegisterIdentityV3AuthProjects(tc *TestContext) {
	tc.OpenstackRouter().Get("/identity/v3/auth/projects", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(projects)); err != nil {
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
	RegisterIdentityV3AuthProjects(tc)
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

const keyPairs = `{
	"keypairs": []
}`

func RegisterComputeV2Keypairs(tc *TestContext) {
	tc.OpenstackRouter().Get("/compute/os-keypairs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(keyPairs)); err != nil {
			if debug {
				fmt.Println(err)
			}
		}
	})
}

const computeAvailabilityZones = `{
	"availabilityZoneInfo": []
}`

func RegisterComputeV2AvailabilityZone(tc *TestContext) {
	tc.OpenstackRouter().Get("/compute/os-availability-zone", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(computeAvailabilityZones)); err != nil {
			if debug {
				fmt.Println(err)
			}
		}
	})
}

const blockStorageAvailabilityZones = `{
        "availabilityZoneInfo": []
}`

func RegisterBlockStorageV3AvailabilityZone(tc *TestContext) {
	tc.OpenstackRouter().Get("/blockstorage/os-availability-zone", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(blockStorageAvailabilityZones)); err != nil {
			if debug {
				fmt.Println(err)
			}
		}
	})
}

const externalNetworks = `{
	"networks": []
}`

func RegisterNetworkV2Networks(tc *TestContext) {
	tc.OpenstackRouter().Get("/network/v2.0/networks", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(externalNetworks)); err != nil {
			if debug {
				fmt.Println(err)
			}
		}
	})
}
