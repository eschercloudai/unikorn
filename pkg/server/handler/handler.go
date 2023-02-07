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
	"net/http"

	"github.com/eschercloudai/unikorn/pkg/server/authorization"
	"github.com/eschercloudai/unikorn/pkg/server/errors"
	"github.com/eschercloudai/unikorn/pkg/server/generated"
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
}

func New(client client.Client, authenticator *authorization.Authenticator) *Handler {
	return &Handler{
		client:        client,
		authenticator: authenticator,
	}
}

func (h *Handler) PostApiV1AuthTokensPassword(w http.ResponseWriter, r *http.Request) {
	token, err := h.authenticator.Basic(r)
	if err != nil {
		errors.HandleError(w, r, err)
		return
	}

	result := &generated.Token{
		Token: token,
	}

	util.WriteJSONResponse(w, r, http.StatusOK, result)
}

func (h *Handler) PostApiV1AuthTokensToken(w http.ResponseWriter, r *http.Request) {
	scope := &generated.TokenScope{}

	if err := util.ReadJSONBody(r, scope); err != nil {
		errors.HandleError(w, r, err)
		return
	}

	token, err := h.authenticator.Token(r, scope)
	if err != nil {
		errors.HandleError(w, r, err)
		return
	}

	result := &generated.Token{
		Token: token,
	}

	util.WriteJSONResponse(w, r, http.StatusOK, result)
}

func (h *Handler) GetApiV1Project(w http.ResponseWriter, r *http.Request) {
	result, err := project.NewClient(h.client).Get(r.Context())
	if err != nil {
		errors.HandleError(w, r, err)
		return
	}

	util.WriteJSONResponse(w, r, http.StatusOK, result)
}

func (h *Handler) PostApiV1Project(w http.ResponseWriter, r *http.Request) {
	if err := project.NewClient(h.client).Create(r.Context()); err != nil {
		errors.HandleError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) DeleteApiV1Project(w http.ResponseWriter, r *http.Request) {
	if err := project.NewClient(h.client).Delete(r.Context()); err != nil {
		errors.HandleError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) GetApiV1Controlplanes(w http.ResponseWriter, r *http.Request) {
	result, err := controlplane.NewClient(h.client).List(r.Context())
	if err != nil {
		errors.HandleError(w, r, err)
		return
	}

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

	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) DeleteApiV1ControlplanesControlPlane(w http.ResponseWriter, r *http.Request, controlPlane generated.ControlPlaneParameter) {
	if err := controlplane.NewClient(h.client).Delete(r.Context(), controlPlane); err != nil {
		errors.HandleError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) GetApiV1ControlplanesControlPlane(w http.ResponseWriter, r *http.Request, controlPlane generated.ControlPlaneParameter) {
	result, err := controlplane.NewClient(h.client).Get(r.Context(), controlPlane)
	if err != nil {
		errors.HandleError(w, r, err)
		return
	}

	util.WriteJSONResponse(w, r, http.StatusOK, result)
}

func (h *Handler) PutApiV1ControlplanesControlPlane(w http.ResponseWriter, r *http.Request, controlPlane generated.ControlPlaneParameter) {
}

func (h *Handler) GetApiV1ControlplanesControlPlaneClusters(w http.ResponseWriter, r *http.Request, controlPlane generated.ControlPlaneParameter) {
}

func (h *Handler) PostApiV1ControlplanesControlPlaneClusters(w http.ResponseWriter, r *http.Request, controlPlane generated.ControlPlaneParameter) {
}

func (h *Handler) DeleteApiV1ControlplanesControlPlaneClustersCluster(w http.ResponseWriter, r *http.Request, controlPlane generated.ControlPlaneParameter, cluster generated.ClusterParameter) {
}

func (h *Handler) GetApiV1ControlplanesControlPlaneClustersCluster(w http.ResponseWriter, r *http.Request, controlPlane generated.ControlPlaneParameter, cluster generated.ClusterParameter) {
}

func (h *Handler) PutApiV1ControlplanesControlPlaneClustersCluster(w http.ResponseWriter, r *http.Request, controlPlane generated.ControlPlaneParameter, cluster generated.ClusterParameter) {
}

func (h *Handler) GetApiV1ProvidersOpenstackAvailabilityZonesCompute(w http.ResponseWriter, r *http.Request) {
	result, err := providers.NewOpenstack(h.authenticator).ListAvailabilityZonesCompute(r)
	if err != nil {
		errors.HandleError(w, r, err)
		return
	}

	util.WriteJSONResponse(w, r, http.StatusOK, result)
}

func (h *Handler) GetApiV1ProvidersOpenstackAvailabilityZonesBlockStorage(w http.ResponseWriter, r *http.Request) {
	result, err := providers.NewOpenstack(h.authenticator).ListAvailabilityZonesBlockStorage(r)
	if err != nil {
		errors.HandleError(w, r, err)
		return
	}

	util.WriteJSONResponse(w, r, http.StatusOK, result)
}

func (h *Handler) GetApiV1ProvidersOpenstackExternalNetworks(w http.ResponseWriter, r *http.Request) {
	result, err := providers.NewOpenstack(h.authenticator).ListExternalNetworks(r)
	if err != nil {
		errors.HandleError(w, r, err)
		return
	}

	util.WriteJSONResponse(w, r, http.StatusOK, result)
}

func (h *Handler) GetApiV1ProvidersOpenstackFlavors(w http.ResponseWriter, r *http.Request) {
	result, err := providers.NewOpenstack(h.authenticator).ListFlavors(r)
	if err != nil {
		errors.HandleError(w, r, err)
		return
	}

	util.WriteJSONResponse(w, r, http.StatusOK, result)
}

func (h *Handler) GetApiV1ProvidersOpenstackImages(w http.ResponseWriter, r *http.Request) {
	result, err := providers.NewOpenstack(h.authenticator).ListImages(r)
	if err != nil {
		errors.HandleError(w, r, err)
		return
	}

	util.WriteJSONResponse(w, r, http.StatusOK, result)
}

func (h *Handler) GetApiV1ProvidersOpenstackKeyPairs(w http.ResponseWriter, r *http.Request) {
	result, err := providers.NewOpenstack(h.authenticator).ListKeyPairs(r)
	if err != nil {
		errors.HandleError(w, r, err)
		return
	}

	util.WriteJSONResponse(w, r, http.StatusOK, result)
}

func (h *Handler) GetApiV1ProvidersOpenstackProjects(w http.ResponseWriter, r *http.Request) {
	result, err := providers.NewOpenstack(h.authenticator).ListAvailableProjects(r)
	if err != nil {
		errors.HandleError(w, r, err)
		return
	}

	util.WriteJSONResponse(w, r, http.StatusOK, result)
}
