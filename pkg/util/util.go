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

package util

import (
	"errors"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// ErrUnversionedGroupVersionKind is returned when the GVK is
	// unversioned.
	ErrUnversionedGroupVersionKind = errors.New("gvk is unversioned")

	// ErrAmbiguousGroupVersionKind is returned when more than one GVK
	// is returned for a type.
	ErrAmbiguousGroupVersionKind = errors.New("gvk is ambiguous")
)

// ObjectGroupVersionKind returns a GVK from a Kubernetes typed resource using
// scheme translation.
func ObjectGroupVersionKind(s *runtime.Scheme, o runtime.Object) (*schema.GroupVersionKind, error) {
	gvks, unversioned, err := s.ObjectKinds(o)
	if err != nil {
		return nil, err
	}

	if unversioned {
		return nil, ErrUnversionedGroupVersionKind
	}

	if len(gvks) != 1 {
		return nil, ErrAmbiguousGroupVersionKind
	}

	return &gvks[0], nil
}
