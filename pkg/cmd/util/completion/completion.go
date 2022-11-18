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

package completion

import (
	"context"
	"strings"

	"github.com/spf13/cobra"

	"github.com/eschercloudai/unikorn/generated/clientset/unikorn"
	"github.com/eschercloudai/unikorn/pkg/cmd/util"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

// ControlPlanesCompletionFunc is a bit messy but allows us to do the project
// to namespace indirection, as the default namespace in the Factory cannot
// be overridden and we cannot use the generic function provided by kubectl.
// Obviously this will get worse when we have vcluster to battle against as that
// needs a whole new kubeconfig.
func ControlPlanesCompletionFunc(f cmdutil.Factory, project *string) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		config, err := f.ToRESTConfig()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		unikornClient, err := unikorn.NewForConfig(config)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		namespace, err := util.GetProjectNamespace(context.TODO(), unikornClient, *project)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		controlPlanes, err := unikornClient.UnikornV1alpha1().ControlPlanes(namespace).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		var matches []string

		for _, cp := range controlPlanes.Items {
			if strings.HasPrefix(cp.Name, toComplete) {
				matches = append(matches, cp.Name)
			}
		}

		return matches, cobra.ShellCompDirectiveNoFileComp
	}
}

// ClustersCompletionFunc returns a list of clusters that belong to a control plane in
// a project that match a prefix.
func ClustersCompletionFunc(f cmdutil.Factory, project, controlPlane *string) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		config, err := f.ToRESTConfig()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		unikornClient, err := unikorn.NewForConfig(config)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		namespace, err := util.GetControlPlaneNamespace(context.TODO(), unikornClient, *project, *controlPlane)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		clusters, err := unikornClient.UnikornV1alpha1().KubernetesClusters(namespace).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		var matches []string

		for _, cluster := range clusters.Items {
			if strings.HasPrefix(cluster.Name, toComplete) {
				matches = append(matches, cluster.Name)
			}
		}

		return matches, cobra.ShellCompDirectiveNoFileComp
	}
}
