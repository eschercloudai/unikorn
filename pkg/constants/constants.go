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

package constants

import (
	"fmt"
	"os"
	"path"
)

var (
	// Application is the application name.
	//nolint:gochecknoglobals
	Application = path.Base(os.Args[0])

	// Version is the application version set via the Makefile.
	//nolint:gochecknoglobals
	Version string

	// Revision is the git revision set via the Makefile.
	//nolint:gochecknoglobals
	Revision string
)

// VersionString returns a canonical version string.  It's based on
// HTTP's User-Agent so can be used to set that too, if this ever has to
// call out ot other micro services.
func VersionString() string {
	return fmt.Sprintf("%s/%s (revision/%s)", Application, Version, Revision)
}

const (
	// VersionLabel is a label applied to resources so we know the application
	// version that was used to create them (and thus what metadata is valid
	// for them).  Metadata may be upgraded to a later version for any resource.
	VersionLabel = "unikorn.eschercloud.ai/version"

	// ControlPlaneLabel is a label applied to namespaces to indicate it is under
	// control of this tool.  Useful for label selection.
	ControlPlaneLabel = "namespace.unikorn.eschercloud.ai/control-plane"
)
