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
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/eschercloudai/unikorn/generated/clientset/unikorn"
	unikornv1alpha1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/cmd/errors"
	"github.com/eschercloudai/unikorn/pkg/cmd/util"
	"github.com/eschercloudai/unikorn/pkg/cmd/util/completion"
	"github.com/eschercloudai/unikorn/pkg/util/provisioners/vcluster"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	computil "k8s.io/kubectl/pkg/util/completion"
)

type getKubeConfigOptions struct {
	project string

	name string

	// client is the Kubernetes v1 client.
	client kubernetes.Interface

	// unikornClient gives access to our custom resources.
	unikornClient unikorn.Interface
}

// addFlags registers create cluster options flags with the specified cobra command.
func (o *getKubeConfigOptions) addFlags(f cmdutil.Factory, cmd *cobra.Command) {
	cmd.Flags().StringVar(&o.project, "project", "", "Kubernetes project name that contains the control plane.")

	if err := cmd.MarkFlagRequired("project"); err != nil {
		panic(err)
	}

	if err := cmd.RegisterFlagCompletionFunc("project", computil.ResourceNameCompletionFunc(f, unikornv1alpha1.ProjectResource)); err != nil {
		panic(err)
	}
}

// complete fills in any options not does automatically by flag parsing.
func (o *getKubeConfigOptions) complete(f cmdutil.Factory, args []string) error {
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

	if len(args) != 1 {
		return errors.ErrInvalidName
	}

	o.name = args[0]

	return nil
}

// validate validates any tainted input not handled by complete() or flags
// processing.
func (o *getKubeConfigOptions) validate() error {
	if len(o.name) == 0 {
		return fmt.Errorf(`%w: "%s"`, errors.ErrInvalidName, o.name)
	}

	if len(o.project) == 0 {
		return fmt.Errorf(`%w: "%s"`, errors.ErrInvalidName, o.project)
	}

	return nil
}

// run executes the command.
func (o *getKubeConfigOptions) run() error {
	project, err := o.unikornClient.UnikornV1alpha1().Projects().Get(context.TODO(), o.project, metav1.GetOptions{})
	if err != nil {
		return err
	}

	namespace := project.Status.Namespace
	if len(namespace) == 0 {
		return errors.ErrProjectNamespaceUndefined
	}

	configPath, cleanup, err := vcluster.WriteConfig(context.TODO(), vcluster.NewKubectlGetter(o.client), namespace, o.name)
	if err != nil {
		return err
	}

	defer cleanup()

	f, err := os.Open(configPath)
	if err != nil {
		return err
	}

	out, err := io.ReadAll(f)
	if err != nil {
		return err
	}

	f.Close()

	fmt.Println(string(out))

	return nil
}

// newGetKubeConfigCommand creates a command that gets a Cluster API control plane.
func newGetKubeConfigCommand(f cmdutil.Factory) *cobra.Command {
	o := getKubeConfigOptions{}

	cmd := &cobra.Command{
		Use:               "kubeconfig",
		Short:             "Delete a Kubernetes cluster",
		Long:              "Delete a Kubernetes cluster",
		ValidArgsFunction: completion.ControlPlanesCompletionFunc(f, &o.project),
		Run: func(cmd *cobra.Command, args []string) {
			util.AssertNilError(o.complete(f, args))
			util.AssertNilError(o.validate())
			util.AssertNilError(o.run())
		},
	}

	o.addFlags(f, cmd)

	return cmd
}
