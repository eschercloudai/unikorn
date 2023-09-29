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

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/labels"
)

// MutuallyExclusiveResource is a generic interface over all resource types,
// where the resource can be uniquely identified.  As these typically map to
// custom resource types, be extra careful you don't overload anything in
// metav1.Object or runtime.Object.
type MutuallyExclusiveResource interface {
	// ResourceLabels returns a set of labels from the resource that uniquely
	// identify it, if they all were to reside in the same namespace.
	// In database terms this would be a composite key.
	ResourceLabels() (labels.Set, error)
}

// ApplicationBundleGetter is a type, typically a custom resource, that has an attached
// application bundle.
type ApplicationBundleGetter interface {
	ApplicationBundleKind() ApplicationBundleResourceKind
	ApplicationBundleName() string
}
