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

package get

import (
	"github.com/spf13/cobra"

	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// newGetControlPlaneCommand returns a command that is able to get or list Cluster API
// control planes found on the management cluster.
func newGetControlPlaneCommand(cf *genericclioptions.ConfigFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "control-plane",
		Short: "Get or list Cluster API control planes",
		Long:  "Get or list Cluster API control planes",
		Run: func(cmd *cobra.Command, args []string) {
		},
	}

	return cmd
}
