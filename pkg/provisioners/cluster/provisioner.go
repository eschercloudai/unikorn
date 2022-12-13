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
	"fmt"
	"strconv"

	"golang.org/x/sync/errgroup"

	unikornv1alpha1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	argocdclient "github.com/eschercloudai/unikorn/pkg/argocd/client"
	argocdcluster "github.com/eschercloudai/unikorn/pkg/argocd/cluster"
	"github.com/eschercloudai/unikorn/pkg/constants"
	"github.com/eschercloudai/unikorn/pkg/provisioners"
	"github.com/eschercloudai/unikorn/pkg/provisioners/application"
	"github.com/eschercloudai/unikorn/pkg/provisioners/vcluster"
	"github.com/eschercloudai/unikorn/pkg/util/retry"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

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

func (p *Provisioner) getLabels(app string) map[string]interface{} {
	l := map[string]interface{}{
		constants.ApplicationLabel: app,
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
				"labels":       p.getLabels("kubernetes-cluster"),
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

func (p *Provisioner) generateCiliumApplication(server string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata": map[string]interface{}{
				"generateName": "cilium-",
				"namespace":    "argocd",
				"labels":       p.getLabels("cilium"),
			},
			"spec": map[string]interface{}{
				"project": "default",
				"source": map[string]interface{}{
					//TODO:  programmable
					"repoURL":        "https://helm.cilium.io/",
					"chart":          "cilium",
					"targetRevision": "1.12.4",
				},
				"destination": map[string]interface{}{
					"name":      server,
					"namespace": "kube-system",
				},
				"syncPolicy": map[string]interface{}{
					"automated": map[string]interface{}{
						"selfHeal": true,
					},
				},
			},
		},
	}
}

// provisionCluster creates a Kubernetes cluster application.
func (p *Provisioner) provisionCluster(ctx context.Context) error {
	log := log.FromContext(ctx)

	log.Info("provisioning kubernetes cluster")

	if err := application.New(p.client, p.generateApplication()).Provision(ctx); err != nil {
		return err
	}

	log.Info("kubernetes cluster provisioned")

	return nil
}

func (p *Provisioner) getKubernetesClusterConfig(ctx context.Context) (*clientcmdapi.Config, error) {
	vclusterConfig, err := vcluster.RESTConfig(ctx, p.client, p.cluster.Namespace)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to get cluster kubeconfig", err)
	}

	vclusterClient, err := client.New(vclusterConfig, client.Options{})
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create cluster client", err)
	}

	secret := &corev1.Secret{}

	secretKey := client.ObjectKey{
		Namespace: p.cluster.Name,
		Name:      p.cluster.Name + "-kubeconfig",
	}

	getSecret := func() error {
		return vclusterClient.Get(ctx, secretKey, secret)
	}

	if err := retry.Forever().DoWithContext(ctx, getSecret); err != nil {
		return nil, err
	}

	config, err := clientcmd.NewClientConfigFromBytes(secret.Data["value"])
	if err != nil {
		return nil, err
	}

	rawConfig, err := config.RawConfig()
	if err != nil {
		return nil, err
	}

	return &rawConfig, nil
}

// provisionCilium runs in parallel with provisionCluster.  The cluster API machine
// deployment will not become healthy until the Kubernetes nodes report as healty
// and that requires a CNI to be installed.  Obviously this isn't made easy by
// CAPI, many have tried, many have failed.  We need to poll the CAPI deployment
// until the Kubernetes config is available, install it in ArgoCD, then deploy
// the Cilium application on the remote.
func (p *Provisioner) provisionCilium(ctx context.Context) error {
	log := log.FromContext(ctx)

	log.Info("provisioning cilium")

	config, err := p.getKubernetesClusterConfig(ctx)
	if err != nil {
		return err
	}

	server := config.Clusters[config.Contexts[config.CurrentContext].Cluster].Server

	argocd, err := argocdclient.NewInCluster(ctx, p.client, "argocd")
	if err != nil {
		return err
	}

	if err := argocdcluster.Upsert(ctx, argocd, server, config); err != nil {
		return err
	}

	if err := application.New(p.client, p.generateCiliumApplication(server)).Provision(ctx); err != nil {
		return err
	}

	log.Info("cilium provisioned")

	return nil
}

// Provision implements the Provision interface.
func (p *Provisioner) Provision(ctx context.Context) error {
	group, gctx := errgroup.WithContext(ctx)

	group.Go(func() error { return p.provisionCluster(gctx) })
	group.Go(func() error { return p.provisionCilium(gctx) })

	if err := group.Wait(); err != nil {
		return err
	}

	return nil
}

// Deprovision implements the Provision interface.
func (p *Provisioner) Deprovision(ctx context.Context) error {
	log := log.FromContext(ctx)

	config, err := p.getKubernetesClusterConfig(ctx)
	if err != nil {
		return err
	}

	server := config.Clusters[config.Contexts[config.CurrentContext].Cluster].Server

	log.Info("deprovisioning cilium")

	if err := application.New(p.client, p.generateCiliumApplication(server)).Deprovision(ctx); err != nil {
		return err
	}

	log.Info("cilium deprovisioned")

	log.Info("deprovisioning kubernetes cluster")

	argocd, err := argocdclient.NewInCluster(ctx, p.client, "argocd")
	if err != nil {
		return err
	}

	if err := argocdcluster.Delete(ctx, argocd, server); err != nil {
		return err
	}

	if err := application.New(p.client, p.generateApplication()).Deprovision(ctx); err != nil {
		return err
	}

	log.Info("kubernetes cluster deprovisioned")

	return nil
}
