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
	"github.com/eschercloudai/unikorn/pkg/server/handler/providers"
	"github.com/eschercloudai/unikorn/pkg/server/util"
)

type Handler struct {
	authenticator *authorization.Authenticator
}

func New(authenticator *authorization.Authenticator) *Handler {
	return &Handler{
		authenticator: authenticator,
	}
}

func (h *Handler) PostApiV1AuthTokensPassword(w http.ResponseWriter, r *http.Request) {
	token, err := h.authenticator.Basic(r)
	if err != nil {
		errors.HandleError(w, r, err)

		return
	}

	response := &generated.Token{
		Token: token,
	}

	util.WriteJSONResponse(w, r, http.StatusOK, response)
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

	response := &generated.Token{
		Token: token,
	}

	util.WriteJSONResponse(w, r, http.StatusOK, response)
}

func (h *Handler) GetApiV1Projects(w http.ResponseWriter, r *http.Request) {
}

func (h *Handler) PostApiV1Projects(w http.ResponseWriter, r *http.Request) {
}

func (h *Handler) DeleteApiV1ProjectsProject(w http.ResponseWriter, r *http.Request, project generated.Project) {
}

func (h *Handler) GetApiV1ProjectsProject(w http.ResponseWriter, r *http.Request, project generated.Project) {
}

func (h *Handler) GetApiV1ProjectsProjectControlplanes(w http.ResponseWriter, r *http.Request, project generated.Project) {
}

func (h *Handler) PostApiV1ProjectsProjectControlplanes(w http.ResponseWriter, r *http.Request, project generated.Project) {
}

func (h *Handler) DeleteApiV1ProjectsProjectControlplanesControlPlane(w http.ResponseWriter, r *http.Request, project generated.Project, controlPlane generated.ControlPlane) {
}

func (h *Handler) GetApiV1ProjectsProjectControlplanesControlPlane(w http.ResponseWriter, r *http.Request, project generated.Project, controlPlane generated.ControlPlane) {
}

func (h *Handler) PutApiV1ProjectsProjectControlplanesControlPlane(w http.ResponseWriter, r *http.Request, project generated.Project, controlPlane generated.ControlPlane) {
}

func (h *Handler) GetApiV1ProjectsProjectControlplanesControlPlaneClusters(w http.ResponseWriter, r *http.Request, project generated.Project, controlPlane generated.ControlPlane) {
}

func (h *Handler) PostApiV1ProjectsProjectControlplanesControlPlaneClusters(w http.ResponseWriter, r *http.Request, project generated.Project, controlPlane generated.ControlPlane) {
}

func (h *Handler) DeleteApiV1ProjectsProjectControlplanesControlPlaneClustersCluster(w http.ResponseWriter, r *http.Request, project generated.Project, controlPlane generated.ControlPlane, cluster generated.Cluster) {
}

func (h *Handler) GetApiV1ProjectsProjectControlplanesControlPlaneClustersCluster(w http.ResponseWriter, r *http.Request, project generated.Project, controlPlane generated.ControlPlane, cluster generated.Cluster) {
}

func (h *Handler) PutApiV1ProjectsProjectControlplanesControlPlaneClustersCluster(w http.ResponseWriter, r *http.Request, project generated.Project, controlPlane generated.ControlPlane, cluster generated.Cluster) {
}

func (h *Handler) GetApiV1ProvidersOpenstackAvailabilityZones(w http.ResponseWriter, r *http.Request) {
	result, err := providers.NewOpenstack(h.authenticator).ListAvailabilityZones(r)
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
