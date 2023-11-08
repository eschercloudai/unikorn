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

package vcluster

import (
	"context"

	unikornv1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/cd"
	"github.com/eschercloudai/unikorn/pkg/constants"
	"github.com/eschercloudai/unikorn/pkg/provisioners"

	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type RemoteCluster struct {
	// namespace tells us where the vcluster lives.
	namespace string

	// labeller is used to identify the owner of and uniquely identify
	// a remote cluster instance.
	labeller unikornv1.ResourceLabeller
}

// Ensure this implements the remotecluster.Generator interface.
var _ provisioners.RemoteCluster = &RemoteCluster{}

// NewRemoteCluster return a new instance of a remote cluster generator.
func NewRemoteCluster(namespace string, labeller unikornv1.ResourceLabeller) *RemoteCluster {
	return &RemoteCluster{
		namespace: namespace,
		labeller:  labeller,
	}
}

// ID implements the remotecluster.Generator interface.
func (g *RemoteCluster) ID() *cd.ResourceIdentifier {
	// TODO: error checking.
	resourceLabels, _ := g.labeller.ResourceLabels()

	var labels []cd.ResourceIdentifierLabel

	for _, label := range constants.LabelPriorities() {
		if value, ok := resourceLabels[label]; ok {
			labels = append(labels, cd.ResourceIdentifierLabel{
				Name:  label,
				Value: value,
			})
		}
	}

	return &cd.ResourceIdentifier{
		Name:   "vcluster",
		Labels: labels,
	}
}

// Config implements the remotecluster.Generator interface.
func (g *RemoteCluster) Config(ctx context.Context) (*clientcmdapi.Config, error) {
	return NewControllerRuntimeClient().ClientConfig(ctx, g.namespace, false)
}
