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

package cmd

import (
	"github.com/eschercloudai/unikorn/pkg/cmd/create"
	"github.com/eschercloudai/unikorn/pkg/cmd/delete"
	"github.com/eschercloudai/unikorn/pkg/cmd/get"
	"github.com/eschercloudai/unikorn/pkg/constants"

	"github.com/spf13/cobra"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/kubectl/pkg/util/templates"
)

var (
	rootLongDesc = templates.LongDesc(`
	EscherCloudAI Kubernetes Provisioning.

	This CLI toolset provides dynamic provisioning of Kubernetes clusters
	and Cluster API control planes.  It also provides various Kubernetes
	cluster life-cycle management functions.  For additional details on
	how the individual components operatate, see the individual 'create'
	help topics.

	This tool is designed with flexibility in mind, so doesn't force any
	sort of cardinality between control planes and clusters, however as
	the control plane management is somewhat new, there are bugs that
	suggest a 1:1 mapping is best intiial for teardown operations.  In the
	future, some upstream hardening will allow 1:N and better resource
	utilisation.`)
)

// newRootCommand returns the root command and all its subordinates.
// This sets global flags for standard Kubernetes configuration options
// such as --kubeconfig, --context, --namespace, etc.
func newRootCommand(cf *genericclioptions.ConfigFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   constants.Application,
		Short: "EscherCloudAI Kubernetes Provisioning.",
		Long:  rootLongDesc,
	}

	cf.AddFlags(cmd.PersistentFlags())

	commands := []*cobra.Command{
		newVersionCommand(),
		create.NewCreateCommand(cf),
		delete.NewDeleteCommand(cf),
		get.NewGetCommand(cf),
	}

	cmd.AddCommand(commands...)

	return cmd
}

// Generate creates a hierarchy of cobra commands for the application.  It can
// also be used to walk the structure and generate HTML documentation for example.
func Generate() *cobra.Command {
	cf := genericclioptions.NewConfigFlags(true)

	cmd := newRootCommand(cf)

	return cmd
}
