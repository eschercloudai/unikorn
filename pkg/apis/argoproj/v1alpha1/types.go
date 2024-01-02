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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ApplicationList is a typed list of projects.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ApplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Application `json:"items"`
}

// Application is an abstraction around Argo applications, this is needed
// because the ArgoCD code base is a shambolic mess and you cannot import
// the types directly without a lot of messing about.
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Application struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ApplicationSpec   `json:"spec"`
	Status            ApplicationStatus `json:"status,omitempty"`
}

// ApplicationSpec defines project specific metadata.
type ApplicationSpec struct {
	// Project is the ArgoCD project to provision in.
	Project string `json:"project"`
	// Source defines where to get the application configuration from.
	Source ApplicationSource `json:"source"`
	// Destination defines where to provision the application.
	Destination ApplicationDestination `json:"destination"`
	// SyncPolicy defines how to keep the application in sync.
	SyncPolicy ApplicationSyncPolicy `json:"syncPolicy"`
	// IgnoreDifferences ignores differences in resources when syncing.
	IgnoreDifferences []ApplicationIgnoreDifference `json:"ignoreDifferences,omitempty"`
}

type ApplicationSource struct {
	// RepoURL is either a helm repository URL or a URL to git source.
	//nolint:tagliatelle
	RepoURL string `json:"repoURL"`
	// Chart is specified when the repo is a helm repo to identify the
	// specific chart.
	Chart string `json:"chart,omitempty"`
	// Path is specified when the repo is git source and identifies the
	// path where the helm chart is located within the repo.
	Path string `json:"path,omitempty"`
	// TargetRevision identifies a helm chart version, or a git tag/branch.
	TargetRevision string `json:"targetRevision"`
	// Helm defines helm parameters.
	Helm *ApplicationSourceHelm `json:"helm,omitempty"`
}

type ApplicationSourceHelm struct {
	// ReleaseName sets the helm release, defaults to the application
	// name otherwise.
	ReleaseName string `json:"releaseName,omitempty"`
	// Values is a verbatim values file to pass to helm.
	Values string `json:"values,omitempty"`
	// Parameters are a set of key value pairs to pass to helm
	// via the --set flag.
	Parameters []HelmParameter `json:"parameters,omitempty"`
}

type HelmParameter struct {
	// Name is a json path to a value to change.
	Name string `json:"name"`
	// Value is the value to set the parameter to.
	Value string `json:"value"`
}

type ApplicationDestination struct {
	// Name is the ArgoCD cluster to provision the application in.
	Name string `json:"name"`
	// Namespace is the namespace to provision the application in.
	Namespace string `json:"namespace"`
}

type ApplicationSyncOption string

const (
	// CreateNamespace identifies that argo need to create the namespace
	// to successfully provision the application.
	CreateNamespace ApplicationSyncOption = "CreateNamespace=true"
	// ServerSideApply assumes ArgoCD is broken and messes up diffs whereas
	// letting Kubernetes do it is better.
	ServerSideApply ApplicationSyncOption = "ServerSideApply=true"
)

type ApplicationSyncPolicy struct {
	// SyncOptions define any synchronization options.
	SyncOptions []ApplicationSyncOption `json:"syncOptions"`
	// Automated, if set, allows periodic synchronization.
	Automated *ApplicationSyncAutomation `json:"automated,omitempty"`
}

type ApplicationSyncAutomation struct {
	// SelfHeal enables self-healing.
	SelfHeal bool `json:"selfHeal,omitempty"`
	// Prune removes orphaned resources.
	Prune bool `json:"prune,omitempty"`
}

type ApplicationIgnoreDifference struct {
	// Group is the resource API group.
	Group string `json:"group"`
	// Kind is the resource kind.
	Kind string `json:"kind"`
	// JSONPointers is a list of JSON pointers to ignore in diffs.
	JSONPointers []string `json:"jsonPointers"`
}

// ApplicationStatus defines the status of the project.
type ApplicationStatus struct {
	// Health defines the application's health status.
	Health *ApplicationHealth `json:"health"`
}

type ApplicationHealthStatus string

const (
	// Healthy is when the chart is synchronized and all resources have
	// got to a healthy status e.g. deployments scaled up etc.
	Healthy ApplicationHealthStatus = "Healthy"

	// Degraded is when things are osensibly working, but not fully healthy
	// yet.
	Degraded ApplicationHealthStatus = "Degraded"
)

type ApplicationHealth struct {
	// Status reports the health status.
	Status ApplicationHealthStatus `json:"status"`
}
