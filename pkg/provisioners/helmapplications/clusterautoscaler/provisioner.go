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

package clusterautoscaler

import (
	unikornv1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/provisioners"
	"github.com/eschercloudai/unikorn/pkg/provisioners/application"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// applicationName is the unique name of the application.
	applicationName = "cluster-autoscaler"
)

// Provisioner encapsulates control plane provisioning.
type Provisioner struct {
	// clusterName defines the CAPI cluster name.
	clusterName string

	// clusterKubeconfigSecretName defines the secret that contains the
	// kubeconfig for the cluster.
	clusterKubeconfigSecretName string
}

// New returns a new initialized provisioner object.
func New(client client.Client, resource application.MutuallyExclusiveResource, helm *unikornv1.HelmApplication, clusterName, clusterKubeconfigSecretName string) provisioners.Provisioner {
	provisoner := &Provisioner{
		clusterName:                 clusterName,
		clusterKubeconfigSecretName: clusterKubeconfigSecretName,
	}

	return application.New(client, applicationName, resource, helm).WithGenerator(provisoner)
}

// Ensure the Provisioner interface is implemented.
var _ application.Paramterizer = &Provisioner{}

func (p *Provisioner) Parameters(version *string) (map[string]string, error) {
	parameters := map[string]string{
		"autoDiscovery.clusterName":  p.clusterName,
		"clusterAPIKubeconfigSecret": p.clusterKubeconfigSecretName,
	}

	return parameters, nil
}
