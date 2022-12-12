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

package cluster

import (
	"context"
	"encoding/base64"
	"errors"
	"strconv"

	unikornv1alpha1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/constants"
	"github.com/eschercloudai/unikorn/pkg/provisioners"
	"github.com/eschercloudai/unikorn/pkg/provisioners/application"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	ErrLabelMissing = errors.New("expected label missing")
)

// Provisioner encapsulates control plane provisioning.
type Provisioner struct {
	// client provides access to Kubernetes.
	client client.Client

	// cluster is the Kubernetes cluster we're provisioning.
	cluster *unikornv1alpha1.KubernetesCluster

	server string

	labels map[string]string
}

// New returns a new initialized provisioner object.
func New(client client.Client, cluster *unikornv1alpha1.KubernetesCluster, server string) *Provisioner {
	return &Provisioner{
		client:  client,
		cluster: cluster,
		server:  server,
	}
}

// Ensure the Provisioner interface is implemented.
var _ provisioners.Provisioner = &Provisioner{}

func (p *Provisioner) WithLabels(l map[string]string) *Provisioner {
	p.labels = l

	return p
}

func (p *Provisioner) getLabels() map[string]string {
	l := map[string]string{
		constants.ApplicationLabel: "kubernetes-cluster",
	}

	for k, v := range p.labels {
		l[k] = v
	}

	return l
}

func (p *Provisioner) generateApplication() *unstructured.Unstructured {
	// Okay, from this point on, things get a bit "meta" because whoever
	// wrote ArgoCD for some reason imported kubernetes, not client-go to
	// get access to the schema information...
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata": map[string]interface{}{
				"generateName": "kubernetes-cluster-",
				"namespace":    "argocd",
				"labels":       p.getLabels(),
			},
			"spec": map[string]interface{}{
				"project": "default",
				"source": map[string]interface{}{
					//TODO:  programmable
					"repoURL":        "https://eschercloudai.github.io/helm-cluster-api",
					"chart":          "cluster-api-cluster-openstack",
					"targetRevision": "v0.1.2",
					"helm": map[string]interface{}{
						"releaseName": p.cluster.Name,
						"parameters": []map[string]interface{}{
							{
								"name":  "openstack.cloudsYAML",
								"value": base64.StdEncoding.EncodeToString(*p.cluster.Spec.Openstack.CloudConfig),
							},
							{
								"name":  "openstack.cloud",
								"value": *p.cluster.Spec.Openstack.Cloud,
							},
							{
								"name":  "openstack.ca",
								"value": base64.StdEncoding.EncodeToString(*p.cluster.Spec.Openstack.CACert),
							},
							{
								"name":  "openstack.image",
								"value": *p.cluster.Spec.Openstack.Image,
							},
							{
								"name":  "openstack.cloudProviderConfiguration",
								"value": base64.StdEncoding.EncodeToString(*p.cluster.Spec.Openstack.CloudProviderConfig),
							},
							{
								"name":  "openstack.externalNetworkID",
								"value": *p.cluster.Spec.Network.ExternalNetworkID,
							},
							{
								"name":  "openstack.sshKeyName",
								"value": *p.cluster.Spec.Openstack.SSHKeyName,
							},
							{
								"name":  "openstack.failureDomain",
								"value": *p.cluster.Spec.Openstack.FailureDomain,
							},
							{
								"name":  "controlPlane.replicas",
								"value": strconv.Itoa(*p.cluster.Spec.ControlPlane.Replicas),
							},
							{
								"name":  "controlPlane.flavor",
								"value": *p.cluster.Spec.ControlPlane.Flavor,
							},
							{
								"name":  "workload.replicas",
								"value": strconv.Itoa(*p.cluster.Spec.Workload.Replicas),
							},
							{
								"name":  "workload.flavor",
								"value": *p.cluster.Spec.Workload.Flavor,
							},
							{
								"name":  "network.nodeCIDR",
								"value": p.cluster.Spec.Network.NodeNetwork.IPNet.String(),
							},
							{
								"name":  "network.serviceCIDRs[0]",
								"value": p.cluster.Spec.Network.ServiceNetwork.IPNet.String(),
							},
							{
								"name":  "network.podCIDRs[0]",
								"value": p.cluster.Spec.Network.PodNetwork.IPNet.String(),
							},
							{
								"name": "network.dnsNameservers[0]",
								// TODO: make dynamic.
								"value": p.cluster.Spec.Network.DNSNameservers[0].IP.String(),
							},
							{
								"name":  "kubernetes.version",
								"value": string(*p.cluster.Spec.KubernetesVersion),
							},
						},
					},
				},
				"destination": map[string]interface{}{
					"name":      p.server,
					"namespace": p.cluster.Name,
				},
				"syncPolicy": map[string]interface{}{
					"automated": map[string]interface{}{
						"selfHeal": true,
					},
					"syncOptions": []string{
						"CreateNamespace=true",
					},
				},
			},
		},
	}
}

// Provision implements the Provision interface.
func (p *Provisioner) Provision(ctx context.Context) error {
	log := log.FromContext(ctx)

	log.Info("provisioning kubernetes cluster")

	app := p.generateApplication()

	if err := application.New(p.client, app, labels.SelectorFromSet(p.getLabels())).Provision(ctx); err != nil {
		return err
	}

	log.Info("kubernetes cluster provisioned")

	return nil
}

// Deprovision implements the Provision interface.
func (p *Provisioner) Deprovision(ctx context.Context) error {
	log := log.FromContext(ctx)

	log.Info("deprovisioning kubernetes cluster")

	app := p.generateApplication()

	if err := application.New(p.client, app, labels.SelectorFromSet(p.getLabels())).Deprovision(ctx); err != nil {
		return err
	}

	log.Info("kubernetes cluster deprovisioned")

	return nil
}
