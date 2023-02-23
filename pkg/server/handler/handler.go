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

//nolint:revive,stylecheck
package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/eschercloudai/unikorn/pkg/server/authorization"
	"github.com/eschercloudai/unikorn/pkg/server/errors"
	"github.com/eschercloudai/unikorn/pkg/server/generated"
	"github.com/eschercloudai/unikorn/pkg/server/handler/cluster"
	"github.com/eschercloudai/unikorn/pkg/server/handler/controlplane"
	"github.com/eschercloudai/unikorn/pkg/server/handler/project"
	"github.com/eschercloudai/unikorn/pkg/server/handler/providers"
	"github.com/eschercloudai/unikorn/pkg/server/util"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Handler struct {
	// client gives cached access to Kubernetes.
	client client.Client

	// authenticator gives access to authentication and token handling functions.
	authenticator *authorization.Authenticator

	// options allows behaviour to be defined on the CLI.
	options *Options
}

func New(client client.Client, authenticator *authorization.Authenticator, options *Options) *Handler {
	return &Handler{
		client:        client,
		authenticator: authenticator,
		options:       options,
	}
}

func (h *Handler) setCacheable(w http.ResponseWriter) {
	w.Header().Add("Cache-Control", fmt.Sprintf("max-age=%d", h.options.cacheMaxAge/time.Second))
	w.Header().Add("Cache-Control", "private")
}

func (h *Handler) setUncacheable(w http.ResponseWriter) {
	w.Header().Add("Cache-Control", "no-cache")
}

func (h *Handler) PostApiV1AuthTokensPassword(w http.ResponseWriter, r *http.Request) {
	token, claims, err := h.authenticator.Basic(r)
	if err != nil {
		errors.HandleError(w, r, err)
		return
	}

	result := &generated.Token{
		Token: token,
		Email: claims.Subject,
	}

	h.setUncacheable(w)
	util.WriteJSONResponse(w, r, http.StatusOK, result)
}

func (h *Handler) PostApiV1AuthTokensToken(w http.ResponseWriter, r *http.Request) {
	scope := &generated.TokenScope{}

	if err := util.ReadJSONBody(r, scope); err != nil {
		errors.HandleError(w, r, err)
		return
	}

	token, claims, err := h.authenticator.Token(r, scope)
	if err != nil {
		errors.HandleError(w, r, err)
		return
	}

	result := &generated.Token{
		Token: token,
		Email: claims.Subject,
	}

	h.setUncacheable(w)
	util.WriteJSONResponse(w, r, http.StatusOK, result)
}

func (h *Handler) GetApiV1Project(w http.ResponseWriter, r *http.Request) {
	result, err := project.NewClient(h.client).Get(r.Context())
	if err != nil {
		errors.HandleError(w, r, err)
		return
	}

	h.setUncacheable(w)
	util.WriteJSONResponse(w, r, http.StatusOK, result)
}

func (h *Handler) PostApiV1Project(w http.ResponseWriter, r *http.Request) {
	if err := project.NewClient(h.client).Create(r.Context()); err != nil {
		errors.HandleError(w, r, err)
		return
	}

	h.setUncacheable(w)
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) DeleteApiV1Project(w http.ResponseWriter, r *http.Request) {
	if err := project.NewClient(h.client).Delete(r.Context()); err != nil {
		errors.HandleError(w, r, err)
		return
	}

	h.setUncacheable(w)
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) GetApiV1Controlplanes(w http.ResponseWriter, r *http.Request) {
	result, err := controlplane.NewClient(h.client).List(r.Context())
	if err != nil {
		errors.HandleError(w, r, err)
		return
	}

	h.setUncacheable(w)
	util.WriteJSONResponse(w, r, http.StatusOK, result)
}

func (h *Handler) PostApiV1Controlplanes(w http.ResponseWriter, r *http.Request) {
	request := &generated.CreateControlPlane{}

	if err := util.ReadJSONBody(r, request); err != nil {
		errors.HandleError(w, r, err)
		return
	}

	if err := controlplane.NewClient(h.client).Create(r.Context(), request); err != nil {
		errors.HandleError(w, r, err)
		return
	}

	h.setUncacheable(w)
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) DeleteApiV1ControlplanesControlPlaneName(w http.ResponseWriter, r *http.Request, controlPlaneName generated.ControlPlaneNameParameter) {
	if err := controlplane.NewClient(h.client).Delete(r.Context(), controlPlaneName); err != nil {
		errors.HandleError(w, r, err)
		return
	}

	h.setUncacheable(w)
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) GetApiV1ControlplanesControlPlaneName(w http.ResponseWriter, r *http.Request, controlPlaneName generated.ControlPlaneNameParameter) {
	result, err := controlplane.NewClient(h.client).Get(r.Context(), controlPlaneName)
	if err != nil {
		errors.HandleError(w, r, err)
		return
	}

	h.setUncacheable(w)
	util.WriteJSONResponse(w, r, http.StatusOK, result)
}

func (h *Handler) PutApiV1ControlplanesControlPlaneName(w http.ResponseWriter, r *http.Request, controlPlaneName generated.ControlPlaneNameParameter) {
}

func (h *Handler) GetApiV1ControlplanesControlPlaneNameClusters(w http.ResponseWriter, r *http.Request, controlPlaneName generated.ControlPlaneNameParameter) {
	result, err := cluster.NewClient(h.client, h.authenticator.Endpoint()).List(r.Context(), controlPlaneName)
	if err != nil {
		errors.HandleError(w, r, err)
		return
	}

	h.setUncacheable(w)
	util.WriteJSONResponse(w, r, http.StatusOK, result)
}

func (h *Handler) PostApiV1ControlplanesControlPlaneNameClusters(w http.ResponseWriter, r *http.Request, controlPlaneName generated.ControlPlaneNameParameter) {
	request := &generated.KubernetesCluster{}

	if err := util.ReadJSONBody(r, request); err != nil {
		errors.HandleError(w, r, err)
		return
	}

	if err := cluster.NewClient(h.client, h.authenticator.Endpoint()).Create(r.Context(), controlPlaneName, request); err != nil {
		errors.HandleError(w, r, err)
		return
	}

	h.setUncacheable(w)
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) DeleteApiV1ControlplanesControlPlaneNameClustersClusterName(w http.ResponseWriter, r *http.Request, controlPlaneName generated.ControlPlaneNameParameter, clusterName generated.ClusterNameParameter) {
	if err := cluster.NewClient(h.client, h.authenticator.Endpoint()).Delete(r.Context(), controlPlaneName, clusterName); err != nil {
		errors.HandleError(w, r, err)
		return
	}

	h.setUncacheable(w)
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) GetApiV1ControlplanesControlPlaneNameClustersClusterName(w http.ResponseWriter, r *http.Request, controlPlaneName generated.ControlPlaneNameParameter, clusterName generated.ClusterNameParameter) {
	result, err := cluster.NewClient(h.client, h.authenticator.Endpoint()).Get(r.Context(), controlPlaneName, clusterName)
	if err != nil {
		errors.HandleError(w, r, err)
		return
	}

	h.setUncacheable(w)
	util.WriteJSONResponse(w, r, http.StatusOK, result)
}

func (h *Handler) PutApiV1ControlplanesControlPlaneNameClustersClusterName(w http.ResponseWriter, r *http.Request, controlPlaneName generated.ControlPlaneNameParameter, clusterName generated.ClusterNameParameter) {
}

func (h *Handler) GetApiV1ControlplanesControlPlaneNameClustersClusterNameKubeconfig(w http.ResponseWriter, r *http.Request, controlPlaneName generated.ControlPlaneNameParameter, clusterName generated.ClusterNameParameter) {
	result, err := cluster.NewClient(h.client, h.authenticator.Endpoint()).GetKubeconfig(r.Context(), controlPlaneName, clusterName)
	if err != nil {
		errors.HandleError(w, r, err)
		return
	}

	h.setUncacheable(w)
	util.WriteOctetStreamResponse(w, r, http.StatusOK, result)
}

func (h *Handler) GetApiV1ProvidersOpenstackApplicationCredentialsApplicationCredential(w http.ResponseWriter, r *http.Request, applicationCredential generated.ApplicationCredentialParameter) {
	result, err := providers.NewOpenstack(h.authenticator).GetApplicationCredential(r, applicationCredential)
	if err != nil {
		errors.HandleError(w, r, err)
		return
	}

	h.setUncacheable(w)
	util.WriteJSONResponse(w, r, http.StatusOK, result)
}

func (h *Handler) PostApiV1ProvidersOpenstackApplicationCredentials(w http.ResponseWriter, r *http.Request) {
	options := &generated.ApplicationCredentialOptions{}

	if err := util.ReadJSONBody(r, options); err != nil {
		errors.HandleError(w, r, err)
		return
	}

	result, err := providers.NewOpenstack(h.authenticator).CreateApplicationCredential(r, options, h.options.applicationCredentialRoles)
	if err != nil {
		errors.HandleError(w, r, err)
		return
	}

	h.setUncacheable(w)
	util.WriteJSONResponse(w, r, http.StatusOK, result)
}

func (h *Handler) DeleteApiV1ProvidersOpenstackApplicationCredentialsApplicationCredential(w http.ResponseWriter, r *http.Request, applicationCredential generated.ApplicationCredentialParameter) {
	if err := providers.NewOpenstack(h.authenticator).DeleteApplicationCredential(r, applicationCredential); err != nil {
		errors.HandleError(w, r, err)
		return
	}

	h.setUncacheable(w)
}

func (h *Handler) GetApiV1ProvidersOpenstackAvailabilityZonesCompute(w http.ResponseWriter, r *http.Request) {
	result, err := providers.NewOpenstack(h.authenticator).ListAvailabilityZonesCompute(r)
	if err != nil {
		errors.HandleError(w, r, err)
		return
	}

	h.setCacheable(w)
	util.WriteJSONResponse(w, r, http.StatusOK, result)
}

func (h *Handler) GetApiV1ProvidersOpenstackAvailabilityZonesBlockStorage(w http.ResponseWriter, r *http.Request) {
	result, err := providers.NewOpenstack(h.authenticator).ListAvailabilityZonesBlockStorage(r)
	if err != nil {
		errors.HandleError(w, r, err)
		return
	}

	h.setCacheable(w)
	util.WriteJSONResponse(w, r, http.StatusOK, result)
}

func (h *Handler) GetApiV1ProvidersOpenstackExternalNetworks(w http.ResponseWriter, r *http.Request) {
	result, err := providers.NewOpenstack(h.authenticator).ListExternalNetworks(r)
	if err != nil {
		errors.HandleError(w, r, err)
		return
	}

	h.setCacheable(w)
	util.WriteJSONResponse(w, r, http.StatusOK, result)
}

func (h *Handler) GetApiV1ProvidersOpenstackFlavors(w http.ResponseWriter, r *http.Request) {
	result, err := providers.NewOpenstack(h.authenticator).ListFlavors(r)
	if err != nil {
		errors.HandleError(w, r, err)
		return
	}

	h.setCacheable(w)
	util.WriteJSONResponse(w, r, http.StatusOK, result)
}

func (h *Handler) GetApiV1ProvidersOpenstackImages(w http.ResponseWriter, r *http.Request) {
	result, err := providers.NewOpenstack(h.authenticator).ListImages(r)
	if err != nil {
		errors.HandleError(w, r, err)
		return
	}

	h.setCacheable(w)
	util.WriteJSONResponse(w, r, http.StatusOK, result)
}

func (h *Handler) GetApiV1ProvidersOpenstackKeyPairs(w http.ResponseWriter, r *http.Request) {
	result, err := providers.NewOpenstack(h.authenticator).ListKeyPairs(r)
	if err != nil {
		errors.HandleError(w, r, err)
		return
	}

	h.setCacheable(w)
	util.WriteJSONResponse(w, r, http.StatusOK, result)
}

func (h *Handler) GetApiV1ProvidersOpenstackProjects(w http.ResponseWriter, r *http.Request) {
	result, err := providers.NewOpenstack(h.authenticator).ListAvailableProjects(r)
	if err != nil {
		errors.HandleError(w, r, err)
		return
	}

	h.setCacheable(w)
	util.WriteJSONResponse(w, r, http.StatusOK, result)
}
