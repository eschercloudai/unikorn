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

package clusterapi

import (
	unikornv1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/provisioners"
	"github.com/eschercloudai/unikorn/pkg/provisioners/application"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

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
func New(client client.Client, resource application.MutuallyExclusiveResource, helm *unikornv1.HelmApplication) provisioners.Provisioner {
	return application.New(client, applicationName, resource, helm).WithGenerator(&Provisioner{})
}

// Customize implments the application.Customizer interface.
func (p *Provisioner) Customize(version *string, object *unstructured.Unstructured) error {
	// TODO: this is very ArgoCD specific.
	ignoreDifferences := []interface{}{
		map[string]interface{}{
			"group": "rbac.authorization.k8s.io",
			"kind":  "ClusterRole",
			"jsonPointers": []interface{}{
				"/rules",
			},
		},
		map[string]interface{}{
			"group": "apiextensions.k8s.io",
			"kind":  "CustomResourceDefinition",
			"jsonPointers": []interface{}{
				"/spec/conversion/webhook/clientConfig/caBundle",
			},
		},
	}

	if err := unstructured.SetNestedField(object.Object, ignoreDifferences, "spec", "ignoreDifferences"); err != nil {
		return err
	}

	return nil
}
