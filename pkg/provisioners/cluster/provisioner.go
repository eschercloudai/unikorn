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

	"github.com/gophercloud/utils/openstack/clientconfig"
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
	"sigs.k8s.io/yaml"
)

var (
	// ErrLabelMissing is returned when a required label is not present on
	// the cluster resource.
	ErrLabelMissing = errors.New("expected label missing")

	// ErrCloudConfiguration is returned when the cloud configuration is not
	// correctly formatted.
	ErrCloudConfiguration = errors.New("invalid cloud configuration")
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

// generateApplication creates an ArgoCD application for a cluster.
func (p *Provisioner) generateApplication() (*unstructured.Unstructured, error) {
	nameservers := make([]interface{}, len(p.cluster.Spec.Network.DNSNameservers))

	for i, nameserver := range p.cluster.Spec.Network.DNSNameservers {
		nameservers[i] = nameserver.IP.String()
	}

	// TODO: generate types from the Helm values schema.
	valuesRaw := map[string]interface{}{
		"openstack": map[string]interface{}{
			"cloud":             *p.cluster.Spec.Openstack.Cloud,
			"cloudsYAML":        base64.StdEncoding.EncodeToString(*p.cluster.Spec.Openstack.CloudConfig),
			"ca":                base64.StdEncoding.EncodeToString(*p.cluster.Spec.Openstack.CACert),
			"sshKeyName":        *p.cluster.Spec.Openstack.SSHKeyName,
			"region":            "nl1",
			"failureDomain":     *p.cluster.Spec.Openstack.FailureDomain,
			"externalNetworkID": *p.cluster.Spec.Network.ExternalNetworkID,
		},
		"cluster": map[string]interface{}{
			"taints": []interface{}{
				map[string]interface{}{
					"key":    "node.cilium.io/agent-not-ready",
					"effect": "NoSchedule",
					"value":  "true",
				},
			},
		},
		"controlPlane": map[string]interface{}{
			"version":  string(*p.cluster.Spec.KubernetesVersion),
			"image":    *p.cluster.Spec.Openstack.Image,
			"flavor":   *p.cluster.Spec.ControlPlane.Flavor,
			"diskSize": 40,
			"replicas": *p.cluster.Spec.ControlPlane.Replicas,
		},
		"workloadPools": map[string]interface{}{
			"default": map[string]interface{}{
				"version":  string(*p.cluster.Spec.KubernetesVersion),
				"image":    *p.cluster.Spec.Openstack.Image,
				"flavor":   *p.cluster.Spec.Workload.Flavor,
				"diskSize": 100,
				"replicas": *p.cluster.Spec.Workload.Replicas,
			},
		},
		"network": map[string]interface{}{
			"nodeCIDR": p.cluster.Spec.Network.NodeNetwork.IPNet.String(),
			"serviceCIDRs": []interface{}{
				p.cluster.Spec.Network.ServiceNetwork.IPNet.String(),
			},
			"podCIDRs": []interface{}{
				p.cluster.Spec.Network.PodNetwork.IPNet.String(),
			},
			"dnsNameservers": nameservers,
		},
	}

	values, err := yaml.Marshal(valuesRaw)
	if err != nil {
		return nil, err
	}

	// Okay, from this point on, things get a bit "meta" because whoever
	// wrote ArgoCD for some reason imported kubernetes, not client-go to
	// get access to the schema information...
	object := &unstructured.Unstructured{
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
					"targetRevision": "v0.2.2",
					"helm": map[string]interface{}{
						"releaseName": p.cluster.Name,
						"values":      string(values),
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

	return object, nil
}

// generateOpenstackCloudProviderApplication creates an ArgoCD application for
// the Openstack controller manager.
func (p *Provisioner) generateOpenstackCloudProviderApplication(server string) (*unstructured.Unstructured, error) {
	var clouds clientconfig.Clouds

	if err := yaml.Unmarshal(*p.cluster.Spec.Openstack.CloudConfig, &clouds); err != nil {
		return nil, err
	}

	cloud, ok := clouds.Clouds[*p.cluster.Spec.Openstack.Cloud]
	if !ok {
		return nil, fmt.Errorf("%w: cloud '%s' not found in clouds.yaml", ErrCloudConfiguration, *p.cluster.Spec.Openstack.Cloud)
	}

	valuesRaw := map[string]interface{}{
		"cloudConfig": map[string]interface{}{
			"global": map[string]interface{}{
				"auth-url":    cloud.AuthInfo.AuthURL,
				"username":    cloud.AuthInfo.Username,
				"password":    cloud.AuthInfo.Password,
				"domain-name": cloud.AuthInfo.DomainName,
				"tenant-name": cloud.AuthInfo.ProjectName,
			},
			"loadBalancer": map[string]interface{}{
				"floating-network-id": *p.cluster.Spec.Network.ExternalNetworkID,
			},
		},
		"tolerations": []interface{}{
			map[string]interface{}{
				"key":    "node-role.kubernetes.io/master",
				"effect": "NoSchedule",
			},
			map[string]interface{}{
				"key":    "node-role.kubernetes.io/control-plane",
				"effect": "NoSchedule",
			},
			map[string]interface{}{
				"key":    "node.cloudprovider.kubernetes.io/uninitialized",
				"effect": "NoSchedule",
				"value":  "true",
			},
			map[string]interface{}{
				"key":    "node.cilium.io/agent-not-ready",
				"effect": "NoSchedule",
				"value":  "true",
			},
		},
	}

	values, err := yaml.Marshal(valuesRaw)
	if err != nil {
		return nil, err
	}

	object := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata": map[string]interface{}{
				"generateName": "openstack-cloud-provider-",
				"namespace":    "argocd",
				"labels":       p.getLabels("openstack-cloud-provider"),
			},
			"spec": map[string]interface{}{
				"project": "default",
				"source": map[string]interface{}{
					//TODO: programmable
					//TODO: the revision does have a Kubernetes app version...
					"repoURL":        "https://kubernetes.github.io/cloud-provider-openstack",
					"chart":          "openstack-cloud-controller-manager",
					"targetRevision": "1.4.0",
					"helm": map[string]interface{}{
						"values": string(values),
					},
				},
				"destination": map[string]interface{}{
					"name":      server,
					"namespace": "ocp-system",
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

	return object, nil
}

// generateCiliumApplication creates an ArgoCD application for the
// Cilium CNI plugin.
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

	object, err := p.generateApplication()
	if err != nil {
		return err
	}

	if err := application.New(p.client, object).Provision(ctx); err != nil {
		return err
	}

	log.Info("kubernetes cluster provisioned")

	return nil
}

// getKubernetesClusterConfig retrieves the Kubernetes configuration from
// a cluster API cluster.
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

	// Retry getting the secret until it exists.
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

// provisionAddOns runs in parallel with provisionCluster.  The cluster API machine
// deployment will not become healthy until the Kubernetes nodes report as healty
// and that requires a CNI to be installed, and the cloud provider.  Obviously this
// isn't made easy by CAPI, many have tried, many have failed.  We need to poll the
// CAPI deployment until the Kubernetes config is available, install it in ArgoCD, then
// deploy the Cilium and cloud provider applications on the remote.
func (p *Provisioner) provisionAddOns(ctx context.Context) error {
	log := log.FromContext(ctx)

	log.Info("provisioning addons")

	config, err := p.getKubernetesClusterConfig(ctx)
	if err != nil {
		return err
	}

	server := config.Clusters[config.Contexts[config.CurrentContext].Cluster].Server

	argocd, err := argocdclient.NewInCluster(ctx, p.client, "argocd")
	if err != nil {
		return err
	}

	// Retry adding the cluster until ArgoCD deems it's ready, it'll 500 until that
	// condition is met.
	upsertCluster := func() error {
		if err := argocdcluster.Upsert(ctx, argocd, server, config); err != nil {
			return err
		}

		return nil
	}

	if err := retry.Forever().DoWithContext(ctx, upsertCluster); err != nil {
		return err
	}

	group, gctx := errgroup.WithContext(ctx)

	group.Go(func() error { return p.provisionOpenstackCloudProvider(gctx, server) })
	group.Go(func() error { return p.provisionCilium(gctx, server) })

	if err := group.Wait(); err != nil {
		return err
	}

	log.Info("addons provisioned")

	return nil
}

// provisionCilium applies the Cilium CNI to the Kubnernetes cluster.
func (p *Provisioner) provisionCilium(ctx context.Context, server string) error {
	log := log.FromContext(ctx)

	log.Info("provisioning cilium")

	if err := application.New(p.client, p.generateCiliumApplication(server)).Provision(ctx); err != nil {
		return err
	}

	log.Info("cilium provisioned")

	return nil
}

// provisionOpenstackCloudProvider applies the openstack cloud controller
// to the Kubnernetes cluster.
func (p *Provisioner) provisionOpenstackCloudProvider(ctx context.Context, server string) error {
	log := log.FromContext(ctx)

	log.Info("provisioning openstack cloud provider")

	object, err := p.generateOpenstackCloudProviderApplication(server)
	if err != nil {
		return err
	}

	if err := application.New(p.client, object).Provision(ctx); err != nil {
		return err
	}

	log.Info("openstack cloud provider provisioned")

	return nil
}

// Provision implements the Provision interface.
func (p *Provisioner) Provision(ctx context.Context) error {
	group, gctx := errgroup.WithContext(ctx)

	group.Go(func() error { return p.provisionCluster(gctx) })
	group.Go(func() error { return p.provisionAddOns(gctx) })

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

	log.Info("deprovisioning openstack cloud provider")

	object, err := p.generateOpenstackCloudProviderApplication(server)
	if err != nil {
		return err
	}

	if err := application.New(p.client, object).Deprovision(ctx); err != nil {
		return err
	}

	log.Info("openstack cloud provider deprovisioned")

	log.Info("deprovisioning kubernetes cluster")

	argocd, err := argocdclient.NewInCluster(ctx, p.client, "argocd")
	if err != nil {
		return err
	}

	if err := argocdcluster.Delete(ctx, argocd, server); err != nil {
		return err
	}

	object, err = p.generateApplication()
	if err != nil {
		return err
	}

	if err := application.New(p.client, object).Deprovision(ctx); err != nil {
		return err
	}

	log.Info("kubernetes cluster deprovisioned")

	return nil
}
