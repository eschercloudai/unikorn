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

package constants

import (
	"fmt"
	"os"
	"path"
	"time"
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

// IsProduction tells us whether we need to check for silly assumptions that
// don't exist or are mostly irrelevant in development land.
func IsProduction() bool {
	return Version != DeveloperVersion
}

const (
	// This is the default version in the Makefile.
	DeveloperVersion = "0.0.0"

	// VersionLabel is a label applied to resources so we know the application
	// version that was used to create them (and thus what metadata is valid
	// for them).  Metadata may be upgraded to a later version for any resource.
	VersionLabel = "unikorn.eschercloud.ai/version"

	// KindLabel is used to match a resource that may be owned by a particular kind.
	// For example, projects and control planes are modelled on namespaces.  For CPs
	// you have to select based on project and CP name, because of name reuse, but
	// this raises the problem that selecting a project's namespace will match multiple
	// so this provides a concrete type associated with each resource.
	KindLabel = "unikorn.eschercloud.ai/kind"

	// KindLabelValueProject is used to denote a resource belongs to this type.
	KindLabelValueProject = "project"

	// KindLabelValueControlPlane is used to denote a resource belongs to this type.
	KindLabelValueControlPlane = "controlplane"

	// KindLabelValueKubernetesCluster is used to denote a resource belongs to this type.
	KindLabelValueKubernetesCluster = "kubernetescluster"

	// ProjectLabel is a label applied to namespaces to indicate it is under
	// control of this tool.  Useful for label selection.
	ProjectLabel = "unikorn.eschercloud.ai/project"

	// ControlPlaneLabel is a label applied to resources to indicate is belongs
	// to a specific control plane.
	ControlPlaneLabel = "unikorn.eschercloud.ai/controlplane"

	// KubernetesClusterLabel is applied to resources to indicate it belongs
	// to a specific cluster.
	KubernetesClusterLabel = "unikorn.eschercloud.ai/cluster"

	// ApplicationLabel is applied to ArgoCD applications to differentiate
	// between them.
	ApplicationLabel = "unikorn.eschercloud.ai/application"

	// IngressEndpointAnnotation helps us find the ingress IP address.
	IngressEndpointAnnotation = "unikorn.eschercloud.ai/ingress-endpoint"

	// ConfigurationHashAnnotation is used where application owners refuse to
	// poll configuration updates and we (and all other users) are forced into
	// manually restarting services based on a Deployment/DaemonSet changing.
	ConfigurationHashAnnotation = "unikorn.eschercloud.ai/config-hash"

	// Finalizer is applied to resources that need to be deleted manually
	// and do other complex logic.
	Finalizer = "unikorn"

	// NvidiaGPUType is used to indicate the GPU type for cluster-autoscaler.
	NvidiaGPUType = "nvidia.com/gpu"

	// DefaultYieldTimeout allows N seconds for a provisioner to do its thing
	// and report a healthy status before yielding and giving someone else
	// a go.
	DefaultYieldTimeout = 10 * time.Second
)
