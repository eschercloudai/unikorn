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

package create

import (
	"github.com/eschercloudai/unikorn/pkg/cmd/util"

	"github.com/spf13/cobra"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/kubectl/pkg/util/templates"
)

var (
	createControlPlaneLong = templates.LongDesc(`
        Create a Cluster API control plane.

        Control planes are modelled on Kubernetes namespaces, this gives
        us a primitive to label, and annotate, to aid in life-cycle management.

        Each control plane namespace will contain an instance of a loft.io
        vcluster.  The use of vclusters allows a level of isolation between
        users in a multi-tenancy environment.  It also allows trivial deletion
        of resources contained within that vcluster as that is not subject
        to finalizers and the like (Cluster API is poorly tested in failure
        scenarios.)`)

	createControlPlaneExample = util.TemplatedExample(`
        # Create a control plane with a generated name.
        {{.Application}} create control-plane

        # Create a control plane with an explcit name.
        {{.Application}} create control-plane foo`)
)

// newCreateControlPlaneCommand creates a command that can create control planes.
// The initial intention is to have a per-user/organization control plane that
// contains Cluster API in a virtual cluster
func newCreateControlPlaneCommand(cf *genericclioptions.ConfigFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "control-plane",
		Short:   "Create a Cluster API control plane.",
		Long:    createControlPlaneLong,
		Example: createControlPlaneExample,
		Run: func(cmd *cobra.Command, args []string) {
		},
	}

	return cmd
}
