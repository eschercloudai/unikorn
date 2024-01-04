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

// UserApplicationBundleList defines a list of Helm application sets.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type UserApplicationBundleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []UserApplicationBundle `json:"items"`
}

// UserApplicationBundle defines a user defined collection of applications.
// This functions in much the same way as a package manager e.g. you install
// "python" and the package manager handles any dependencies and recommended
// additional packages. For entities like Clusters and ControlPlanes, The
// ApplicationBundle types are used instead.
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories=unikorn
// +kubebuilder:printcolumn:name="status",type="string",JSONPath=".status.conditions[?(@.type==\"Available\")].reason"
// +kubebuilder:printcolumn:name="age",type="date",JSONPath=".metadata.creationTimestamp"
type UserApplicationBundle struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              UserApplicationBundleSpec   `json:"spec"`
	Status            UserApplicationBundleStatus `json:"status,omitempty"`
}

// UserApplicationBundleSpec defines the requested state of applications on the chosen cluster.
type UserApplicationBundleSpec struct {
	// Pause, if true, will inhibit reconciliation.
	Pause bool `json:"pause,omitempty"`
	// ClusterName is the cluster name that the application set is for.
	// You must only define one per-cluster to avoid split brain problems.
	// The cluster must exist in the same namespace as the UserApplicationBundle.
	ClusterName *string `json:"clusterName"`
	// Applications defines the desired set of applications to install.
	// This does not need to explicitly define prerequisites, a management
	// engine should perform this for you and indicate the full list of
	// installed applications both explicit and implied in the status.
	// +listType=map
	// +listMapKey=name
	Applications []ApplicationNamedReference `json:"applications"`
}

// UserApplicationBundleStatus defines status conditions for the application set and
// individual applications within it.
type UserApplicationBundleStatus struct {
	// Conditions define overall status for the application set.
	// +listType=map
	// +listMapKey=type
	Conditions []Condition `json:"conditions,omitempty"`
	// Applications defines a list of applications that have been selected
	// to be installed my the management engine, and their status.
	// +listType=map
	// +listMapKey=name
	Applications []ApplicationStatus `json:"applicationStatuses,omitempty"`
}

type ApplicationStatus struct {
	// Name is the application name.
	Name *string `json:"name"`
	// Conditions provides per-application status.
	Conditions []Condition `json:"conditions,omitempty"`
	// Endpoint is an optional HTTPS endpoint to get access to the application.
	Endpoint *string `json:"endpoint,omitempty"`
}
