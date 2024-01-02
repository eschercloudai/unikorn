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
	"k8s.io/apimachinery/pkg/runtime/schema"

	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

const (
	// GroupName is the Kubernetes API group our resources belong to.
	GroupName = "argoproj.io"
	// GroupVersion is the version of our custom resources.
	GroupVersion = "v1alpha1"
	// Group is group/version of our resources.
	Group = GroupName + "/" + GroupVersion

	// ApplicationKind is the API kind for an application.
	ApplicationKind = "Application"
	// ApplicationResource is the API endpoint for an application.
	ApplicationResource = "applications"
)

var (
	// SchemeGroupVersion defines the GV of our resources.
	//nolint:gochecknoglobals
	SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: GroupVersion}

	// SchemeBuilder creates a mapping between GVK and type.
	//nolint:gochecknoglobals
	SchemeBuilder = &scheme.Builder{GroupVersion: SchemeGroupVersion}

	// AddToScheme adds our GVK to resource mappings to an existing scheme.
	//nolint:gochecknoglobals
	AddToScheme = SchemeBuilder.AddToScheme
)

//nolint:gochecknoinits
func init() {
	SchemeBuilder.Register(&Application{}, &ApplicationList{})
}

// Resource maps a resource type to a group resource.
func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}
