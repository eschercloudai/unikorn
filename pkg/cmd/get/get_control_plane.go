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
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/eschercloudai/unikorn/pkg/cmd/errors"
	"github.com/eschercloudai/unikorn/pkg/cmd/util"
	"github.com/eschercloudai/unikorn/pkg/constants"

	"github.com/spf13/cobra"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubectl/pkg/cmd/get"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

// getControlPlaneNamespace gets a single control plane namespace.
// TODO: utility function.
func getControlPlaneNamespace(client kubernetes.Interface, name string) (*corev1.Namespace, error) {
	ns, err := client.CoreV1().Namespaces().Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if _, ok := ns.Labels[constants.ControlPlaneLabel]; !ok {
		return nil, fmt.Errorf("%w: %s", errors.ErrNotFound, name)
	}

	// TODO: this is caused by doing a typed read/decode, if we do this unstructured
	// using a dynamic client we'll have a happier time.
	ns.TypeMeta.APIVersion = "v1"
	ns.TypeMeta.Kind = "Namespace"

	return ns, nil
}

// listControlPlaneNamespaces lists all control plane namespaces.
// TODO: utility function.
func listControlPlaneNamespaces(client kubernetes.Interface) (*corev1.NamespaceList, error) {
	requireControlPlaneLabel, err := labels.NewRequirement(constants.ControlPlaneLabel, selection.Exists, nil)
	if err != nil {
		return nil, err
	}

	selector := labels.NewSelector()
	selector = selector.Add(*requireControlPlaneLabel)

	nss, err := client.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}

	// TODO: this is caused by doing a typed read/decode, if we do this unstructured
	// using a dynamic client we'll have a happier time.
	nss.TypeMeta.APIVersion = "v1"
	nss.TypeMeta.Kind = "NamespaceList"

	return nss, nil
}

type getControlPlaneOptions struct {
	// name allows explict filtering of control plane namespaces.
	names []string

	// printFlags gives a rich set of functionality shamelessly stolen from
	// kubectl e.g. -o yaml etc.
	printFlags *get.PrintFlags

	// client is the Kubernetes v1 client.
	client kubernetes.Interface
}

// newGetControlPlaneOptions returns a correctly initialized set of options.
func newGetControlPlaneOptions() *getControlPlaneOptions {
	return &getControlPlaneOptions{
		printFlags: get.NewGetPrintFlags(),
	}
}

// complete fills in any options not does automatically by flag parsing.
func (o *getControlPlaneOptions) complete(f cmdutil.Factory, args []string) error {
	var err error

	if o.client, err = f.KubernetesClientSet(); err != nil {
		return err
	}

	if len(args) != 0 {
		o.names = args
	}

	return nil
}

// validate validates any tainted input not handled by complete() or flags
// processing.
func (o *getControlPlaneOptions) validate() error {
	if o.names != nil {
		for _, name := range o.names {
			if len(name) == 0 {
				return fmt.Errorf(`%w: "%s"`, errors.ErrInvalidName, name)
			}
		}
	}

	return nil
}

// run executes the command.
func (o *getControlPlaneOptions) run() error {
	var result runtime.Object

	if o.names != nil {
		if len(o.names) == 1 {
			ns, err := getControlPlaneNamespace(o.client, o.names[0])
			if err != nil {
				return err
			}

			result = ns
		} else {
			nss := &corev1.NamespaceList{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "NamespaceList",
				},
			}

			for _, name := range o.names {
				ns, err := getControlPlaneNamespace(o.client, name)
				if err != nil {
					return err
				}

				nss.Items = append(nss.Items, *ns)
			}

			result = nss
		}
	} else {
		nss, err := listControlPlaneNamespaces(o.client)
		if err != nil {
			return err
		}

		result = nss
	}

	// TODO: we can use the kubectl TablePrinter here, that consumes metav1.Table
	// resources for better control over the output.  Custom columns are okay, but
	// cannot do cool stuff ike AGE fields etc.
	printer, err := o.printFlags.ToPrinter()
	if err != nil {
		return err
	}

	if err := printer.PrintObj(result, os.Stdout); err != nil {
		return err
	}

	return nil
}

// newGetControlPlaneCommand returns a command that is able to get or list Cluster API
// control planes found on the management cluster.
func newGetControlPlaneCommand(f cmdutil.Factory) *cobra.Command {
	o := newGetControlPlaneOptions()

	cmd := &cobra.Command{
		Use:   "control-plane",
		Short: "Get or list Cluster API control planes",
		Long:  "Get or list Cluster API control planes",
		// TODO: utility function.
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			client, err := f.KubernetesClientSet()
			if err != nil {
				return nil, cobra.ShellCompDirectiveError
			}

			nss, err := listControlPlaneNamespaces(client)
			if err != nil {
				return nil, cobra.ShellCompDirectiveError
			}

			var matches []string

			for _, ns := range nss.Items {
				if strings.HasPrefix(ns.Name, toComplete) {
					matches = append(matches, ns.Name)
				}
			}

			return matches, cobra.ShellCompDirectiveNoFileComp
		},
		Aliases: []string{
			"control-planes",
			"cp",
		},
		Run: func(cmd *cobra.Command, args []string) {
			util.AssertNilError(o.complete(f, args))
			util.AssertNilError(o.validate())
			util.AssertNilError(o.run())
		},
	}

	o.printFlags.AddFlags(cmd)

	return cmd
}
