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

package delete

import (
	"github.com/spf13/cobra"

	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

// newDeleteControlPlaneCommand creates a command that deletes a Cluster API control plane.
func newDeleteControlPlaneCommand(f cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "control-plane",
		Short: "Delete a Kubernetes cluster",
		Long:  "Delete a Kubernetes cluster",
		Run: func(cmd *cobra.Command, args []string) {
		},
	}

	return cmd
}
