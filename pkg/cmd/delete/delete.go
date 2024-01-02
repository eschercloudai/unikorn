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

package delete

import (
	"github.com/spf13/cobra"

	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

// NewDeleteCommand creates a command that is responsible for deleting various resources.
func NewDeleteCommand(f cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete Unikorn resources",
		Long:  "Delete Unikorn resources",
	}

	commands := []*cobra.Command{
		newDeleteProjectCommand(f),
		newDeleteControlPlaneCommand(f),
		newDeleteClusterCommand(f),
	}

	cmd.AddCommand(commands...)

	return cmd
}
