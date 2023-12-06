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

package clusterautoscaler

import (
	"context"

	unikornv1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/provisioners/application"
	"github.com/eschercloudai/unikorn/pkg/provisioners/helmapplications/clusteropenstack"
)

const (
	// applicationName is the unique name of the application.
	applicationName = "cluster-autoscaler"
)

// Provisioner encapsulates provisioning.
type Provisioner struct{}

// New returns a new initialized provisioner object.
func New() *application.Provisioner {
	provisoner := &Provisioner{}

	return application.New(applicationName).WithGenerator(provisoner)
}

// Ensure the Provisioner interface is implemented.
var _ application.Paramterizer = &Provisioner{}

func (p *Provisioner) Parameters(ctx context.Context, version string) (map[string]string, error) {
	//nolint:forcetypeassert
	cluster := application.FromContext(ctx).(*unikornv1.KubernetesCluster)

	parameters := map[string]string{
		"autoDiscovery.clusterName":  clusteropenstack.CAPIClusterName(cluster),
		"clusterAPIKubeconfigSecret": clusteropenstack.KubeconfigSecretName(cluster),
	}

	return parameters, nil
}
