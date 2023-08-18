/*
Copyright 2022-2023 EscherCloud.

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

package clusterapi

import (
	argoprojv1 "github.com/eschercloudai/unikorn/pkg/apis/argoproj/v1alpha1"
	unikornv1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/provisioners/application"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// applicationName is the unique name of the application.
	applicationName = "cluster-api"
)

type Provisioner struct{}

// Ensure the Provisioner interface is implemented.
var _ application.Customizer = &Provisioner{}

// New returns a new initialized provisioner object.
func New(client client.Client, resource application.MutuallyExclusiveResource, helm *unikornv1.HelmApplication) *application.Provisioner {
	return application.New(client, applicationName, resource, helm).WithGenerator(&Provisioner{})
}

// Customize implments the application.Customizer interface.
func (p *Provisioner) Customize(version *string, application *argoprojv1.Application) error {
	// TODO: this is very ArgoCD specific.
	application.Spec.IgnoreDifferences = []argoprojv1.ApplicationIgnoreDifference{
		{
			Group: "rbac.authorization.k8s.io",
			Kind:  "ClusterRole",
			JSONPointers: []string{
				"/rules",
			},
		},
		{
			Group: "apiextensions.k8s.io",
			Kind:  "CustomResourceDefinition",
			JSONPointers: []string{
				"/spec/conversion/webhook/clientConfig/caBundle",
			},
		},
	}

	return nil
}
