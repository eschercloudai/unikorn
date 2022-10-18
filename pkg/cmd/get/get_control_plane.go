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

	"github.com/eschercloudai/unikorn/generated/clientset/unikorn"
	unikornv1alpha1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/cmd/errors"
	"github.com/eschercloudai/unikorn/pkg/cmd/util"

	"github.com/spf13/cobra"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/completion"
)

type getControlPlaneOptions struct {
	// project allows scoping of control plane searching.
	project string

	// name allows explict filtering of control plane namespaces.
	names []string

	// getPrintFlags is a generic and reduced set of printing options.
	getPrintFlags *getPrintFlags

	// f is the factory used to create clients.
	f cmdutil.Factory

	// client is the Kubernetes v1 client.
	client kubernetes.Interface

	// unikornClient gives access to our custom resources.
	unikornClient unikorn.Interface
}

// newGetControlPlaneOptions returns a correctly initialized set of options.
func newGetControlPlaneOptions() *getControlPlaneOptions {
	return &getControlPlaneOptions{
		getPrintFlags: newGetPrintFlags(),
	}
}

// addFlags registers create cluster options flags with the specified cobra command.
func (o *getControlPlaneOptions) addFlags(f cmdutil.Factory, cmd *cobra.Command) {
	cmd.Flags().StringVar(&o.project, "project", "", "Kubernetes project name that contains the control plane.")

	if err := cmd.MarkFlagRequired("project"); err != nil {
		panic(err)
	}

	if err := cmd.RegisterFlagCompletionFunc("project", completion.ResourceNameCompletionFunc(f, unikornv1alpha1.ProjectResource)); err != nil {
		panic(err)
	}

	o.getPrintFlags.addFlags(cmd)
}

// complete fills in any options not does automatically by flag parsing.
func (o *getControlPlaneOptions) complete(f cmdutil.Factory, args []string) error {
	o.f = f

	var err error

	if o.client, err = f.KubernetesClientSet(); err != nil {
		return err
	}

	config, err := f.ToRESTConfig()
	if err != nil {
		return err
	}

	if o.unikornClient, err = unikorn.NewForConfig(config); err != nil {
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

	if len(o.project) == 0 {
		return fmt.Errorf(`%w: "%s"`, errors.ErrInvalidName, o.project)
	}

	return nil
}

// run executes the command.
func (o *getControlPlaneOptions) run() error {
	project, err := o.unikornClient.UnikornV1alpha1().Projects().Get(context.TODO(), o.project, metav1.GetOptions{})
	if err != nil {
		return err
	}

	namespace := project.Status.Namespace
	if len(namespace) == 0 {
		return fmt.Errorf("project namespace undefined")
	}

	// We are using the "kubectl get" library to retrieve resources.  That command
	// is generic, it accepts a kind and name(s), or a list of type/name tuples.
	// In our case, the type is implicit, so we need to prepend it to keep things
	// working as they should.
	args := []string{unikornv1alpha1.ControlPlaneResource}
	args = append(args, o.names...)

	r := o.f.NewBuilder().
		Unstructured().
		NamespaceParam(namespace).
		ResourceTypeOrNameArgs(true, args...).
		ContinueOnError().
		Latest().
		Flatten().
		TransformRequests(o.getPrintFlags.transformRequests).
		Do()

	if err := r.Err(); err != nil {
		return err
	}

	infos, err := r.Infos()
	if err != nil {
		return err
	}

	// Assume we have a single object, the r.Err above will crap out if no results are
	// found.  We know all returned results will be projects.  If doing a human printable
	// get, then a single table will be returned.  If getting by name, especially multiple
	// names, then there may be multiple results.  Coalesce these into a single list
	// as that's what is expected from standard tools.
	object := infos[0].Object

	if len(infos) > 1 {
		list := &corev1.List{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "List",
			},
		}

		for _, info := range infos {
			list.Items = append(list.Items, runtime.RawExtension{Object: info.Object})
		}

		object = list
	}

	printer, err := o.getPrintFlags.toPrinter()
	if err != nil {
		return err
	}

	if err := printer.PrintObj(object, os.Stdout); err != nil {
		return err
	}

	return nil
}

// getControlPlanesCompletionFunc is a bit messy but allows us to do the project
// to namespace indirection, as the default namespace in the Factory cannot
// be overridden and we cannot use the generic function provided by kubectl.
// Obviously this will get worse when we have vcluster to battle against as that
// needs a whole new kubeconfig.
func (o *getControlPlaneOptions) getControlPlanesCompletionFunc(f cmdutil.Factory) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		config, err := f.ToRESTConfig()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		unikornClient, err := unikorn.NewForConfig(config)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		project, err := unikornClient.UnikornV1alpha1().Projects().Get(context.TODO(), o.project, metav1.GetOptions{})
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		namespace := project.Status.Namespace
		if len(namespace) == 0 {
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

// newGetControlPlaneCommand returns a command that is able to get or list Cluster API
// control planes found on the management cluster.
func newGetControlPlaneCommand(f cmdutil.Factory) *cobra.Command {
	o := newGetControlPlaneOptions()

	cmd := &cobra.Command{
		Use:               "control-plane",
		Short:             "Get or list Cluster API control planes",
		Long:              "Get or list Cluster API control planes",
		ValidArgsFunction: o.getControlPlanesCompletionFunc(f),
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

	o.addFlags(f, cmd)

	return cmd
}
