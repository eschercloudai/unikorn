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
	"fmt"
	"time"

	"github.com/eschercloudai/unikorn/generated/clientset/unikorn"
	unikornv1alpha1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/cmd/errors"
	"github.com/eschercloudai/unikorn/pkg/cmd/util"
	"github.com/eschercloudai/unikorn/pkg/constants"
	"github.com/eschercloudai/unikorn/pkg/util/provisioners"
	"github.com/eschercloudai/unikorn/pkg/util/retry"
	"github.com/eschercloudai/unikorn/pkg/util/vcluster"

	"github.com/spf13/cobra"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/completion"
	"k8s.io/kubectl/pkg/util/templates"
)

type createControlPlaneOptions struct {
	// f gives us access to clients.
	f cmdutil.Factory

	// name is the name of the control plane to create.
	name string

	// project is the project name.
	project string

	// timeout is how long to wait for everything to provision.
	timeout time.Duration

	// client is the Kubernetes v1 client.
	client kubernetes.Interface

	// unikornClient gives access to our custom resources.
	unikornClient unikorn.Interface
}

// addFlags registers create cluster options flags with the specified cobra command.
func (o *createControlPlaneOptions) addFlags(f cmdutil.Factory, cmd *cobra.Command) {
	cmd.Flags().StringVar(&o.project, "project", "", "Kubernetes project name that contains the control plane.")
	cmd.Flags().DurationVar(&o.timeout, "timeout", 5*time.Minute, "Duration to wait for provisioning.")

	if err := cmd.MarkFlagRequired("project"); err != nil {
		panic(err)
	}

	if err := cmd.RegisterFlagCompletionFunc("project", completion.ResourceNameCompletionFunc(f, unikornv1alpha1.ProjectResource)); err != nil {
		panic(err)
	}
}

// complete fills in any options not does automatically by flag parsing.
func (o *createControlPlaneOptions) complete(f cmdutil.Factory, args []string) error {
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
	c, cancel := context.WithTimeout(context.TODO(), o.timeout)
	defer cancel()

	project, err := o.unikornClient.UnikornV1alpha1().Projects().Get(context.TODO(), o.project, metav1.GetOptions{})
	if err != nil {
		return err
	}

	namespace := project.Status.Namespace

	if len(namespace) == 0 {
		panic("achtung!")
	}

	controlPlane := &unikornv1alpha1.ControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name: o.name,
			Labels: map[string]string{
				constants.VersionLabel: constants.Version,
			},
		},
	}

	controlPlane, err = o.unikornClient.UnikornV1alpha1().ControlPlanes(namespace).Create(context.TODO(), controlPlane, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	// Pretend from this line onward it's an asynchronous controller/operator
	// like thing.

	fmt.Println("🦄 Provisioning control plane ...")

	controlPlane.Status.Conditions = []unikornv1alpha1.ControlPlaneCondition{
		{
			Type:               unikornv1alpha1.ControlPlaneConditionProvisioned,
			Status:             corev1.ConditionFalse,
			LastTransitionTime: metav1.Now(),
			Reason:             "Provisioning",
			Message:            "Provisioning of control plane has started",
		},
	}

	fmt.Println("🦄 Provisioning virtual cluster ...")

	controlPlane, err = o.unikornClient.UnikornV1alpha1().ControlPlanes(namespace).UpdateStatus(context.TODO(), controlPlane, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	gvks, _, err := scheme.Scheme.ObjectKinds(controlPlane)
	if err != nil {
		return err
	}

	if len(gvks) != 1 {
		panic("unexpectedly got multiple gvks for object")
	}

	gvk := gvks[0]

	// TODO: We probably want the control plane resource to defer deletion until its
	// children are done, thus preventing race conditions on delete and recreate.
	args := []string{
		"--set=service.type=LoadBalancer",
	}

	ownerReferences := []metav1.OwnerReference{
		*metav1.NewControllerRef(controlPlane, gvk),
	}

	vclusterProvisioner := provisioners.NewHelmProvisioner(o.f, "https://charts.loft.sh", "vcluster", namespace, o.name, args, ownerReferences)

	if err := vclusterProvisioner.Provision(); err != nil {
		return err
	}

	statefulsetReadiness := provisioners.NewStatefulSetReady(o.client, namespace, o.name)

	if err := retry.WithContext(c).Do(statefulsetReadiness.Check); err != nil {
		return err
	}

	fmt.Println("🦄 Extracting Kubernetes config ...")

	configPath, cleanup, err := vcluster.WriteConfig(c, o.client, namespace, o.name)
	if err != nil {
		return err
	}

	defer cleanup()

	fmt.Println("🦄 Provisioning Cluster API ...")

	// TODO: we need a better provisioner for this.
	clusterAPIProvisioner := provisioners.NewBinaryProvisioner(nil, "clusterctl", "init", "--kubeconfig", configPath, "--infrastructure", "openstack", "--wait-providers")

	if err := clusterAPIProvisioner.Provision(); err != nil {
		return err
	}

	controlPlane.Status.Conditions = []unikornv1alpha1.ControlPlaneCondition{
		{
			Type:               unikornv1alpha1.ControlPlaneConditionProvisioned,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
			Reason:             "Provisioned",
			Message:            "Provisioning of control plane has completed",
		},
	}

	_, err = o.unikornClient.UnikornV1alpha1().ControlPlanes(namespace).UpdateStatus(context.TODO(), controlPlane, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	fmt.Println("🦄 Neigh")

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
func newCreateControlPlaneCommand(f cmdutil.Factory) *cobra.Command {
	o := &createControlPlaneOptions{}

	cmd := &cobra.Command{
		Use:     "control-plane [flags] my-control-plane-name",
		Short:   "Create a Cluster API control plane.",
		Long:    createControlPlaneLong,
		Example: createControlPlaneExample,
		Run: func(cmd *cobra.Command, args []string) {
			util.AssertNilError(o.complete(f, args))
			util.AssertNilError(o.validate())
			util.AssertNilError(o.run())
		},
	}

	o.addFlags(f, cmd)

	return cmd
}
