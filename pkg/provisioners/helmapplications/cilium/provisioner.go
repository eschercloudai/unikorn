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

package cilium

import (
	unikornv1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/provisioners/application"
	"github.com/eschercloudai/unikorn/pkg/provisioners/util"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// applicationName is the unique name of the application.
	applicationName = "cilium"
)

// New returns a new initialized provisioner object.
func New(client client.Client, cluster *unikornv1.KubernetesCluster, helm *unikornv1.HelmApplication) *application.Provisioner {
	provisioner := &Provisioner{
		cluster: cluster,
	}

	return application.New(client, applicationName, cluster, helm).WithGenerator(provisioner).InNamespace("kube-system")
}

type Provisioner struct {
	cluster *unikornv1.KubernetesCluster
}

// Ensure the Provisioner interface is implemented.
var _ application.ValuesGenerator = &Provisioner{}

func (p *Provisioner) Values(_ *string) (interface{}, error) {
	// Scale to zero support.
	operatorValues := map[string]interface{}{
		"nodeSelector": util.ControlPlaneNodeSelector(),
	}

	// If the cluster CP has one node, then this will fail to deploy
	// as cilium has 2 as the default, we need to work some magic here.
	if *p.cluster.Spec.ControlPlane.Replicas == 1 {
		operatorValues["replicas"] = p.cluster.Spec.ControlPlane.Replicas
	}

	values := map[string]interface{}{
		"operator": operatorValues,
	}

	return values, nil
}
