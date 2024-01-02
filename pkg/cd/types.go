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

package cd

import (
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// DriverKind allows the provisioners to make CD specific hacks for when
// the underlying provider is broken in some way.
type DriverKind string

const (
	DriverKindArgoCD DriverKind = "argocd"
)

// ResourceIdentifierLabel is a single key/value pair that can
// be used to specify context in a resource identifier.
type ResourceIdentifierLabel struct {
	// Name is the label name.
	Name string

	// Value is the label value.
	Value string
}

// ResourceIdentifier allows a HelmApplication or a Cluster to be
// uniquely identified by the CD driver.  How it uses this information
// is specific to the driver.
type ResourceIdentifier struct {
	// Name can either be a unique name, in which case labels are
	// not required, or a generic name that may be used more than
	// once, in which case you will need labels to add context to
	// make it unique.
	Name string

	// Labels specify the context of a name. For example if we have
	// an application called "nginx", then it would alias with other
	// instances of it.  Labels provide a way to inject context e.g.
	// I live in namespace X, on cluster Y.
	Labels []ResourceIdentifierLabel
}

// HelmApplicationParameter defines a single key/value parameter
// to be passed to Helm.  How it is passed may be via a values.yaml
// or --set CLI flag as decided by the ContinuousDeployment driver.
type HelmApplicationParameter struct {
	// Name is a json path the to the parameter.
	Name string

	// Value is the value of the parameter.  The value may
	// be anything supported by Helm's --set flag.
	Value string
}

// HelmApplicationField identifies JSON paths within a resource type.
type HelmApplicationField struct {
	Group string

	Kind string

	JSONPointers []string
}

// HelmApplication defines a driver agnostic Helm application.
type HelmApplication struct {
	// Repo is a URL to either a Helm or Git repository.
	Repo string

	// Chart is required when using a Helm repository.
	Chart string

	// Path is required when using a Git repository.
	Path string

	// Version is either the Helm chart version, or a Git branch, tag or
	// hash.
	Version string

	// Release is the name of the release.
	Release string

	// Parameters is a set of Helm value overrides.
	Parameters []HelmApplicationParameter

	// Values is an interface to an arbitrary data structure that can
	// be marshaled into a values.yaml file. The idea here is you can
	// generate values file data types from a values.schema.json, or
	// just thown in a free-form map[string]interface{} thing.
	Values interface{}

	// Cluster identifies the cluster to install on to.
	// By definition we require the CD provider to support multiple
	// clusters to support control plane virtual clusters, and the
	// CAPI Kubernetes clusters we create.
	Cluster *ResourceIdentifier

	// Namespace identifies the namespace to install in to.
	Namespace string

	// CreateNamespace defines that the CD provider must create the
	// namespace as the chart does not.
	CreateNamespace bool

	// IgnoreDifferences can be set when the driver support self-healing
	// e.g. reversion of manual changes.  This is important as the CD
	// may do a diff, and constantly reconcile due to resource mutation
	// by an external controller.
	IgnoreDifferences []HelmApplicationField

	// ServerSideApply may be implemented by a driver.  It avoids large
	// annotations, especially for CRDs, that may fail due to buffer overflow.
	// TODO: This is the default in flux2 (v0.18.0), and can be deprecated if
	// argo can be moved to using it by default.
	ServerSideApply bool

	// AllowDegraded allows us to tolerate degraded state and allow a success
	// to be reported rather than a failure.
	AllowDegraded bool
}

// Cluster identifies a Kubernetes cluster and allows a CD driver to
// access it for management.
type Cluster struct {
	// Config is the parsed Kubernetes configuration.
	Config *clientcmdapi.Config
}
