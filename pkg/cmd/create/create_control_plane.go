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
	"context"

	"github.com/eschercloudai/unikorn/pkg/cmd/errors"
	"github.com/eschercloudai/unikorn/pkg/cmd/util"
	"github.com/eschercloudai/unikorn/pkg/constants"

	"github.com/spf13/cobra"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubectl/pkg/util/templates"
)

type createControlPlaneOptions struct {
	// name is the name of the control plane to create.
	name string

	// client is the Kubernetes v1 client.
	client kubernetes.Interface
}

// complete fills in any options not does automatically by flag parsing.
func (o *createControlPlaneOptions) complete(cf *genericclioptions.ConfigFlags, args []string) error {
	config, err := cf.ToRESTConfig()
	if err != nil {
		return err
	}

	if o.client, err = kubernetes.NewForConfig(config); err != nil {
		return err
	}

	if len(args) != 1 {
		return errors.ErrIncorrectArgumentNum
	}

	o.name = args[0]

	return nil
}

// validate validates any tainted input not handled by complete() or flags
// processing.
func (o *createControlPlaneOptions) validate() error {
	if len(o.name) == 0 {
		return errors.ErrInvalidName
	}

	return nil
}

// run executes the command.
func (o *createControlPlaneOptions) run() error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: o.name,
			Labels: map[string]string{
				constants.VersionLabel:      constants.Version,
				constants.ControlPlaneLabel: "true",
			},
		},
	}

	// TODO: we can probably make use of the dynamic client here and just say
	// here's a list of stuff, create it, rather than mess around with huge
	// swathes of typed API code.  That way we can have say an explicit
	// provisioner, a helm provisioner, a binary provisioner...
	if _, err := o.client.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{}); err != nil {
		return err
	}

	return nil
}

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
        # Create a control plane named my-control-plane-name.
        {{.Application}} create control-plane my-control-plane-name`)
)

// newCreateControlPlaneCommand creates a command that can create control planes.
// The initial intention is to have a per-user/organization control plane that
// contains Cluster API in a virtual cluster
func newCreateControlPlaneCommand(cf *genericclioptions.ConfigFlags) *cobra.Command {
	o := &createControlPlaneOptions{}

	cmd := &cobra.Command{
		Use:     "control-plane [flags] my-control-plane-name",
		Short:   "Create a Cluster API control plane.",
		Long:    createControlPlaneLong,
		Example: createControlPlaneExample,
		Run: func(cmd *cobra.Command, args []string) {
			util.AssertNilError(o.complete(cf, args))
			util.AssertNilError(o.validate())
			util.AssertNilError(o.run())
		},
	}

	return cmd
}
