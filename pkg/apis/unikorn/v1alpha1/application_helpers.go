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
	"errors"
	"fmt"
)

var (
	// ErrVersionNotFound is raised when the requested version is
	// undefined in an application.
	ErrVersionNotFound = errors.New("version not found")
)

func (a *HelmApplication) GetVersion(version string) (*HelmApplicationVersion, error) {
	versions := make([]string, 0, len(a.Spec.Versions))

	for i := range a.Spec.Versions {
		if *a.Spec.Versions[i].Version == version {
			return &a.Spec.Versions[i], nil
		}

		versions = append(versions, *a.Spec.Versions[i].Version)
	}

	return nil, fmt.Errorf("%w: wanted %v have %v", ErrVersionNotFound, version, versions)
}
