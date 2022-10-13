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

	"github.com/eschercloudai/unikorn/generated/clientset/unikorn"
	unikornv1alpha1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/cmd/errors"
	"github.com/eschercloudai/unikorn/pkg/cmd/util"
	"github.com/eschercloudai/unikorn/pkg/constants"

	"github.com/spf13/cobra"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"
)

type createProjectOptions struct {
	// name is the name of the project to create.
	name string

	// projectID is the external management plane's project identifier.
	projectID string

	// client is the Kubernetes v1 client.
	client kubernetes.Interface

	// unikornClient gives access to our custom resources.
	unikornClient unikorn.Interface
}

// addFlags registers create cluster options flags with the specified cobra command.
func (o *createProjectOptions) addFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&o.projectID, "project", "", "management plane project identifier.")

	if err := cmd.MarkFlagRequired("project"); err != nil {
		panic(err)
	}
}

// complete fills in any options not does automatically by flag parsing.
func (o *createProjectOptions) complete(f cmdutil.Factory, args []string) error {
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
func (o *createProjectOptions) validate() error {
	if len(o.name) == 0 {
		return errors.ErrInvalidName
	}

	return nil
}

// run executes the command.
func (o *createProjectOptions) run() error {
	project := &unikornv1alpha1.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name: o.name,
			Labels: map[string]string{
				constants.VersionLabel: constants.Version,
			},
		},
		Spec: unikornv1alpha1.ProjectSpec{
			ProjectID: o.projectID,
		},
	}

	var err error

	project, err = o.unikornClient.UnikornV1alpha1().Projects().Create(context.TODO(), project, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	// Pretend from this line orward it's an asynchronous controller/operator
	// like thing.

	project.Status.Conditions = []unikornv1alpha1.ProjectCondition{
		{
			Type:               unikornv1alpha1.ProjectConditionProvisioned,
			Status:             corev1.ConditionFalse,
			LastTransitionTime: metav1.Now(),
			Reason:             "Provisioning",
			Message:            "Provisioning of project has started",
		},
	}

	project, err = o.unikornClient.UnikornV1alpha1().Projects().UpdateStatus(context.TODO(), project, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	gvks, _, err := scheme.Scheme.ObjectKinds(project)
	if err != nil {
		return err
	}

	if len(gvks) != 1 {
		panic("unexpectedly got multiple gvks for object")
	}

	gvk := gvks[0]

	// The namespace name is auto generated, and will be reflected in the Project
	// status.  Internally we'll use a label to reference the project in order to
	// look it up (for restoring status and checking for existence).  Finally set
	// an owner reference so the namespace is automagically deleted by project
	// deletion.
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "controlplane-",
			Labels: map[string]string{
				constants.ControlPlaneLabel: o.name,
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(project, gvk),
			},
		},
	}

	namespace, err = o.client.CoreV1().Namespaces().Create(context.TODO(), namespace, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	project.Status.Namespace = namespace.Name
	project.Status.Conditions = []unikornv1alpha1.ProjectCondition{
		{
			Type:               unikornv1alpha1.ProjectConditionProvisioned,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
			Reason:             "Provisioned",
			Message:            "Provisioning of project has completed",
		},
	}

	_, err = o.unikornClient.UnikornV1alpha1().Projects().UpdateStatus(context.TODO(), project, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}

var (
	createProjectLong = templates.LongDesc(`
        Create a project.

	Projects are modelled as custom resources, as they are domain specific
	abstractions.  We tried intially modelling control planes on namespaces,
	with projects being an annotations, but it turns out this is a whole
	world of pain.

	Projects map 1:1 to a namespace, and these project namespaces also contain
	custom control plane resources.  Thus we can simply off-board users/projects
	with a single delete.  We also no longer need to do an indexed search of
	control planes by project, as control planes are encapsulated in projects.

	Projects are cluster scoped.`)

	createProjectExample = util.TemplatedExample(`
        # Create a control plane named my-project-name.
        {{.Application}} create project my-project-name`)
)

// newCreateProjectCommand creates a command that can create control planes.
// The initial intention is to have a per-user/organization control plane that
// contains Cluster API in a virtual cluster
func newCreateProjectCommand(f cmdutil.Factory) *cobra.Command {
	o := &createProjectOptions{}

	cmd := &cobra.Command{
		Use:     "project [flags] my-project-name",
		Short:   "Create a project.",
		Long:    createProjectLong,
		Example: createProjectExample,
		Run: func(cmd *cobra.Command, args []string) {
			util.AssertNilError(o.complete(f, args))
			util.AssertNilError(o.validate())
			util.AssertNilError(o.run())
		},
	}

	o.addFlags(cmd)

	return cmd
}
