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

// HelmApplicationList defines a list of Helm applications.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type HelmApplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HelmApplication `json:"items"`
}

// HelmApplication defines a Helm application.
// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Cluster,categories=unikorn
// +kubebuilder:printcolumn:name="repo",type="string",JSONPath=".spec.repo"
// +kubebuilder:printcolumn:name="chart",type="string",JSONPath=".spec.chart"
// +kubebuilder:printcolumn:name="version",type="string",JSONPath=".spec.version"
// +kubebuilder:printcolumn:name="age",type="date",JSONPath=".metadata.creationTimestamp"
type HelmApplication struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              HelmApplicationSpec   `json:"spec"`
	Status            HelmApplicationStatus `json:"status,omitempty"`
}

type HelmApplicationSpec struct {
	// Name is the human readable application name.
	Name *string `json:"name"`
	// Description describes what the application does.
	Description *string `json:"description"`
	// Documentation defines a URL to 3rd party documentation.
	Documentation *string `json:"documentation"`
	// License describes the licence the application is released under.
	License *string `json:"license"`
	// Icon is a base64 encoded icon for the application.
	Icon []byte `json:"icon"`
	// Tags allows an application to be given a free-form set of labels
	// that can provide grouping, filtering or other contexts.  For
	// example "networking", "monitoring", "database" etc.
	// TODO: make me required.
	Tags []string `json:"tags,omitempty"`
	// Exported defines whether the application should be exported to
	// the user visiable application manager.
	Exported *bool `json:"exported,omitempty"`
	// Repo is either a Helm chart repository, or git repository.
	// TODO: remove me!
	Repo *string `json:"repo,omitempty"`
	// Chart is the chart name in the repository.
	// TODO: remove me!
	Chart *string `json:"chart,omitempty"`
	// Path is the path if the repo is a git repo.
	// TODO: remove me!
	Path *string `json:"path,omitempty"`
	// Version is the chart version, or a branch when a path is provided.
	// TODO: remove me!
	Version *string `json:"version,omitempty"`
	// Release is the explicit release name for when chart resource names are dynamic.
	// Typically we need predicatable names for things that are going to be remote
	// clusters to derive endpoints or Kubernetes configurations.
	// TODO: remove me!
	Release *string `json:"release,omitempty"`
	// Parameters is a set of static --set parameters to pass to the chart.
	// TODO: remove me!
	Parameters []HelmApplicationParameter `json:"parameters,omitempty"`
	// CreateNamespace indicates whether the chart requires a namespace to be
	// created by the tooling, rather than the chart itself.
	// TODO: remove me!
	CreateNamespace *bool `json:"createNamespace,omitempty"`
	// ServerSideApply allows you to bypass using kubectl apply.  This is useful
	// in situations where CRDs are too big and blow the annotation size limit.
	// We'd like to have this on by default, but mutating admission webhooks and
	// controllers modifying the spec mess this up.
	// TODO: remove me!
	ServerSideApply *bool `json:"serverSideApply,omitempty"`
	// Interface is the name of a Unikorn function that configures the application.
	// In particular it's used when reading values from a custom resource and mapping
	// them to Helm values.  This allows us to version Helm interfaces in the context
	// of "do we need to do something differently", without having to come up with a
	// generalized solution that purely exists as Kubernetes resource specifications.
	// For example, building a Openstack Cloud Provider configuration from a clouds.yaml
	// is going to be bloody tricky without some proper code to handle it.
	// TODO: remove me!
	Interface *string `json:"interface,omitempty"`
	// Versions are the application versions that are supported.
	Versions []HelmApplicationVersion `json:"versions,omitempty"`
}

// +kubebuilder:validation:XValidation:rule="has(self.chart) || has(self.path)",message="either chart or path must be specified"
// +kubebuilder:validation:XValidation:rule="!(has(self.chart) && has(self.path))",message="only one of chart or path may be specified"
type HelmApplicationVersion struct {
	// Repo is either a Helm chart repository, or git repository.
	// If not set, uses the application default.
	Repo *string `json:"repo"`
	// Chart is the chart name in the repository.
	// If not set, uses the application default.
	Chart *string `json:"chart,omitempty"`
	// Path is the path if the repo is a git repo.
	// If not set, uses the application default.
	Path *string `json:"path,omitempty"`
	// Version is the chart version, or a branch when a path is provided.
	Version *string `json:"version"`
	// Release is the explicit release name for when chart resource names are dynamic.
	// Typically we need predicatable names for things that are going to be remote
	// clusters to derive endpoints or Kubernetes configurations.
	// If not set, uses the application default.
	Release *string `json:"release,omitempty"`
	// Parameters is a set of static --set parameters to pass to the chart.
	// If not set, uses the application default.
	Parameters []HelmApplicationParameter `json:"parameters,omitempty"`
	// CreateNamespace indicates whether the chart requires a namespace to be
	// created by the tooling, rather than the chart itself.
	// If not set, uses the application default.
	CreateNamespace *bool `json:"createNamespace,omitempty"`
	// ServerSideApply allows you to bypass using kubectl apply.  This is useful
	// in situations where CRDs are too big and blow the annotation size limit.
	// We'd like to have this on by default, but mutating admission webhooks and
	// controllers modifying the spec mess this up.
	// If not set, uses the application default.
	ServerSideApply *bool `json:"serverSideApply,omitempty"`
	// Interface is the name of a Unikorn function that configures the application.
	// In particular it's used when reading values from a custom resource and mapping
	// them to Helm values.  This allows us to version Helm interfaces in the context
	// of "do we need to do something differently", without having to come up with a
	// generalized solution that purely exists as Kubernetes resource specifications.
	// For example, building a Openstack Cloud Provider configuration from a clouds.yaml
	// is going to be bloody tricky without some proper code to handle it.
	// If not set, uses the application default.
	Interface *string `json:"interface,omitempty"`
	// Dependencies capture hard dependencies on other applications that must
	// be installed before this one.
	Dependencies []HelmApplicationDependency `json:"dependencies,omitempty"`
	// Recommends capture soft dependencies on other applications that may be
	// installed after this one. Typically ths could be storage classes for a
	// storage provider etc.
	Recommends []HelmApplicationDependency `json:"recommends,omitempty"`
}

type HelmApplicationParameter struct {
	// Name is the name of the parameter.
	Name *string `json:"name"`
	// Value is the value of the parameter.
	Value *string `json:"value"`
}

type HelmApplicationDependency struct {
	// Name the name of the application to depend on.
	Name *string `json:"name"`
}

type HelmApplicationStatus struct{}
