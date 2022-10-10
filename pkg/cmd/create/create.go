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
	"github.com/spf13/cobra"

	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// NewCreateCommand creates a command that allows creation of various resources.
func NewCreateCommand(cf *genericclioptions.ConfigFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create Kubernetes clusters and resources.",
		Long:  "Create Kubernetes clusters and resources.",
	}

	commands := []*cobra.Command{
		newCreateControlPlaneCommand(cf),
		newCreateClusterCommand(cf),
	}

	cmd.AddCommand(commands...)

	return cmd
}
