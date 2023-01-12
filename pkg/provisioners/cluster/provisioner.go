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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
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

// WithLabels associates set of labels with the provisioner to uniquely identify
// the provisioner instance.
// TODO: this is fairly common, share it using interface aggregation.
func (p *Provisioner) WithLabels(l map[string]string) *Provisioner {
	p.labels = l

	return p
}

// getLabels returns an application specific set of labels to uniquely identify the
// application.
func (p *Provisioner) getLabels(app string) map[string]interface{} {
	l := map[string]interface{}{
		constants.ApplicationLabel: app,
	}

	for k, v := range p.labels {
		l[k] = v
	}

	return l
}

// generateMachineHelmValues translates the API's idea of a machine into what's
// expected by the underlying Helm chart.
func (p *Provisioner) generateMachineHelmValues(machine *unikornv1alpha1.MachineGeneric) map[string]interface{} {
	object := map[string]interface{}{
		"image":  *machine.Image,
		"flavor": *machine.Flavor,
	}

	if machine.DiskSize != nil {
		object["diskSize"] = machine.DiskSize.Value() >> 30
	}

	return object
}

// getWorkloadPools athers all workload pools that belong to this cluster.
// By default that's all the things, in reality most sane people will add label
// selectors.
func (p *Provisioner) getWorkloadPools(ctx context.Context) (*unikornv1alpha1.KubernetesWorkloadPoolList, error) {
	selector := labels.Everything()

	if p.cluster.Spec.WorkloadPools != nil && p.cluster.Spec.WorkloadPools.Selector != nil {
		s, err := metav1.LabelSelectorAsSelector(p.cluster.Spec.WorkloadPools.Selector)
		if err != nil {
			return nil, err
		}

		selector = s
	}

	workloadPools := &unikornv1alpha1.KubernetesWorkloadPoolList{}

	if err := p.client.List(ctx, workloadPools, &client.ListOptions{LabelSelector: selector}); err != nil {
		return nil, err
	}

	return workloadPools, nil
}

// generateWorloadPoolHelmValues translates the API's idea of a workload pool into
// what's expected by the underlying Helm chart.
func (p *Provisioner) generateWorloadPoolHelmValues(ctx context.Context) (map[string]interface{}, error) {
	workloadPoolResources, err := p.getWorkloadPools(ctx)
	if err != nil {
		return nil, err
	}

	workloadPools := map[string]interface{}{}

	for _, workloadPool := range workloadPoolResources.Items {
		object := map[string]interface{}{
			"version":  string(*workloadPool.Spec.Version),
			"replicas": *workloadPool.Spec.Replicas,
			"machine":  p.generateMachineHelmValues(&workloadPool.Spec.MachineGeneric),
		}

		if len(workloadPool.Spec.Labels) != 0 {
			labels := map[string]interface{}{}

			for key, value := range workloadPool.Spec.Labels {
				labels[key] = value
			}

			object["labels"] = labels
		}

		if len(workloadPool.Spec.Files) != 0 {
			files := make([]interface{}, len(workloadPool.Spec.Files))

			for i, file := range workloadPool.Spec.Files {
				files[i] = map[string]interface{}{
					"path":    *file.Path,
					"content": base64.StdEncoding.EncodeToString(file.Content),
				}
			}

			object["files"] = files
		}

		// TODO: scheduling.
		workloadPools[workloadPool.GetName()] = object
	}

	return workloadPools, nil
}

// generateApplication creates an ArgoCD application for a cluster.
func (p *Provisioner) generateApplication(ctx context.Context) (*unstructured.Unstructured, error) {
	workloadPools, err := p.generateWorloadPoolHelmValues(ctx)
	if err != nil {
		return nil, err
	}

	nameservers := make([]interface{}, len(p.cluster.Spec.Network.DNSNameservers))

	for i, nameserver := range p.cluster.Spec.Network.DNSNameservers {
		nameservers[i] = nameserver.IP.String()
	}

	// TODO: generate types from the Helm values schema.
	// TODO: add in API configuration.
	valuesRaw := map[string]interface{}{
		"openstack": map[string]interface{}{
			"cloud":             *p.cluster.Spec.Openstack.Cloud,
			"cloudsYAML":        base64.StdEncoding.EncodeToString(*p.cluster.Spec.Openstack.CloudConfig),
			"ca":                base64.StdEncoding.EncodeToString(*p.cluster.Spec.Openstack.CACert),
			"sshKeyName":        *p.cluster.Spec.Openstack.SSHKeyName,
			"region":            *p.cluster.Spec.Openstack.Region,
			"failureDomain":     *p.cluster.Spec.Openstack.FailureDomain,
			"externalNetworkID": *p.cluster.Spec.Openstack.ExternalNetworkID,
		},
		"cluster": map[string]interface{}{
			"taints": []interface{}{
				// This prevents things like coreDNS from coming up until
				// the CNI is installed.
				map[string]interface{}{
					"key":    "node.cilium.io/agent-not-ready",
					"effect": "NoSchedule",
					"value":  "true",
				},
			},
		},
		"controlPlane": map[string]interface{}{
			"version":  string(*p.cluster.Spec.ControlPlane.Version),
			"replicas": *p.cluster.Spec.ControlPlane.Replicas,
			"machine":  p.generateMachineHelmValues(&p.cluster.Spec.ControlPlane.MachineGeneric),
		},
		"workloadPools": workloadPools,
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
					"targetRevision": "v0.3.1",
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
						"prune":    true,
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
				"floating-network-id": *p.cluster.Spec.Openstack.ExternalNetworkID,
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
						"prune":    true,
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
						"prune":    true,
					},
				},
			},
		},
	}
}

// getWorkloadPoolMachineDeploymentNames gets a list of machine deployments that should
// exist for this cluster.
// TODO: this is horrific and relies on knowing the internal workings of the Helm chart
// not just the public API!!!
func (p *Provisioner) getWorkloadPoolMachineDeploymentNames(ctx context.Context) ([]string, error) {
	pools, err := p.getWorkloadPools(ctx)
	if err != nil {
		return nil, err
	}

	names := make([]string, len(pools.Items))

	for i, pool := range pools.Items {
		names[i] = fmt.Sprintf("%s-pool-%s", p.cluster.Name, pool.GetName())
	}

	return names, nil
}

// getMachineDeployments gets all live machine deployments for the cluster.
func (p *Provisioner) getMachineDeployments(ctx context.Context, c client.Client) ([]unstructured.Unstructured, error) {
	deployments := &unstructured.UnstructuredList{
		Object: map[string]interface{}{
			"apiVersion": "cluster.x-k8s.io/v1beta1",
			"kind":       "MachineDeployment",
		},
	}

	options := &client.ListOptions{
		Namespace: p.cluster.Name,
	}

	if err := c.List(ctx, deployments, options); err != nil {
		return nil, err
	}

	var filtered []unstructured.Unstructured

	for _, deployment := range deployments.Items {
		ownerReferences := deployment.GetOwnerReferences()

		for _, ownerReference := range ownerReferences {
			if ownerReference.Kind != "Cluster" || ownerReference.Name != p.cluster.Name {
				continue
			}

			filtered = append(filtered, deployment)
		}
	}

	return filtered, nil
}

// machineDeploymentExists tells whether the deployment exists in the
// expected list of names.
func machineDeploymentExists(deployment *unstructured.Unstructured, names []string) bool {
	for _, name := range names {
		if name == deployment.GetName() {
			return true
		}
	}

	return false
}

// deleteOrphanedMachineDeployments does just that. So what happens when you
// delete a workload pool is that the application notes it's no longer in the
// manifest, BUT, and I like big buts, cluster-api has added an owner reference,
// so Argo thinks it's an implicitly created resource now.  So, what we need to
// do is manually delete any orphaned MachineDeployments.
func (p *Provisioner) deleteOrphanedMachineDeployments(ctx context.Context) error {
	log := log.FromContext(ctx)

	vclusterConfig, err := vcluster.RESTConfig(ctx, p.client, p.cluster.Namespace)
	if err != nil {
		return fmt.Errorf("%w: failed to get cluster kubeconfig", err)
	}

	vclusterClient, err := client.New(vclusterConfig, client.Options{})
	if err != nil {
		return fmt.Errorf("%w: failed to create cluster client", err)
	}

	names, err := p.getWorkloadPoolMachineDeploymentNames(ctx)
	if err != nil {
		return err
	}

	deployments, err := p.getMachineDeployments(ctx, vclusterClient)
	if err != nil {
		return err
	}

	for i := range deployments {
		deployment := &deployments[i]

		if machineDeploymentExists(deployment, names) {
			continue
		}

		log.Info("deleting orphaned machine deployment", "name", deployment.GetName())

		if err := vclusterClient.Delete(ctx, deployment); err != nil {
			return err
		}
	}

	return nil
}

// provisionCluster creates a Kubernetes cluster application.
func (p *Provisioner) provisionCluster(ctx context.Context) error {
	log := log.FromContext(ctx)

	log.Info("provisioning kubernetes cluster")

	object, err := p.generateApplication(ctx)
	if err != nil {
		return err
	}

	if err := application.New(p.client, object).Provision(ctx); err != nil {
		return err
	}

	if err := p.deleteOrphanedMachineDeployments(ctx); err != nil {
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

	object, err = p.generateApplication(ctx)
	if err != nil {
		return err
	}

	if err := application.New(p.client, object).Deprovision(ctx); err != nil {
		return err
	}

	log.Info("kubernetes cluster deprovisioned")

	return nil
}
