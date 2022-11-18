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
	"github.com/eschercloudai/unikorn/pkg/provisioners/util"
	"github.com/eschercloudai/unikorn/pkg/provisioners/vcluster"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
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
}

// New returns a new initialized provisioner object.
func New(client client.Client, cluster *unikornv1alpha1.KubernetesCluster) *Provisioner {
	return &Provisioner{
		client:  client,
		cluster: cluster,
	}
}

// Ensure the Provisioner interface is implemented.
var _ provisioners.Provisioner = &Provisioner{}

// Provision implements the Provision interface.
func (p *Provisioner) Provision(ctx context.Context) error {
	// Create a new client that's able to talk to the vcluster.
	vclusterConfig, err := vcluster.RESTConfig(ctx, p.client, p.cluster.Namespace)
	if err != nil {
		return err
	}

	// Do not inherit the scheme or REST mapper here, it's a different cluster!
	vclusterClient, err := client.New(vclusterConfig, client.Options{})
	if err != nil {
		return err
	}

	namespace, err := p.provisionNamespace(ctx, vclusterClient)
	if err != nil {
		return err
	}

	// Provision the actual cluster in the namespace.
	envMapper := func(env string) string {
		// TODO: the manifest looks broken as regards DNS nameservers...
		mapping := map[string]string{
			"CLUSTER_NAME":                           p.cluster.Name,
			"OPENSTACK_NODE_NETWORK":                 p.cluster.Spec.Network.NodeNetwork.IPNet.String(),
			"KUBERNETES_POD_NETWORK":                 p.cluster.Spec.Network.PodNetwork.IPNet.String(),
			"KUBERNETES_SERVICE_NETWORK":             p.cluster.Spec.Network.ServiceNetwork.IPNet.String(),
			"OPENSTACK_CLOUD":                        *p.cluster.Spec.Openstack.Cloud,
			"OPENSTACK_DNS_NAMESERVERS":              p.cluster.Spec.Network.DNSNameservers[0].IP.String(),
			"OPENSTACK_EXTERNAL_NETWORK_ID":          *p.cluster.Spec.Network.ExternalNetworkID,
			"CONTROL_PLANE_MACHINE_COUNT":            strconv.Itoa(*p.cluster.Spec.ControlPlane.Replicas),
			"OPENSTACK_CLOUD_PROVIDER_CONF_B64":      base64.StdEncoding.EncodeToString(*p.cluster.Spec.Openstack.CloudProviderConfig),
			"OPENSTACK_CLOUD_YAML_B64":               base64.StdEncoding.EncodeToString(*p.cluster.Spec.Openstack.CloudConfig),
			"OPENSTACK_CLOUD_CACERT_B64":             base64.StdEncoding.EncodeToString(*p.cluster.Spec.Openstack.CACert),
			"KUBERNETES_VERSION":                     string(*p.cluster.Spec.KubernetesVersion),
			"OPENSTACK_CONTROL_PLANE_MACHINE_FLAVOR": *p.cluster.Spec.ControlPlane.Flavor,
			"OPENSTACK_IMAGE_NAME":                   *p.cluster.Spec.Openstack.Image,
			"OPENSTACK_SSH_KEY_NAME":                 *p.cluster.Spec.Openstack.SSHKeyName,
			"WORKER_MACHINE_COUNT":                   strconv.Itoa(*p.cluster.Spec.Workload.Replicas),
			"OPENSTACK_FAILURE_DOMAIN":               *p.cluster.Spec.Openstack.FailureDomain,
			"OPENSTACK_NODE_MACHINE_FLAVOR":          *p.cluster.Spec.Workload.Flavor,
		}

		if value, ok := mapping[env]; ok {
			return value
		}

		return ""
	}

	provisioner := provisioners.NewManifestProvisioner(vclusterClient, provisioners.ManifestProviderOpenstackKubernetesCluster).WithNamespace(namespace.Name).WithEnvMapper(envMapper)

	if err := provisioner.Provision(ctx); err != nil {
		return err
	}

	return nil
}

// provisionNamespace creates a namespace for the cluster.  This ensures people cannot
// do nefarious things like provision stuff into kube-system etc.
func (p *Provisioner) provisionNamespace(ctx context.Context, client client.Client) (*corev1.Namespace, error) {
	namespace, err := util.GetResourceNamespace(ctx, client, constants.KubernetesClusterLabel, p.cluster.Name)
	if err == nil {
		return namespace, nil
	}

	// Some other error, propagate it back up the stack.
	if !errors.Is(err, util.ErrNamespaceLookup) {
		return nil, err
	}

	// Create a new control plane namespace.
	namespace = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "cluster-",
			Labels: map[string]string{
				constants.KubernetesClusterLabel: p.cluster.Name,
			},
		},
	}

	if err := provisioners.NewResourceProvisioner(client, namespace).Provision(ctx); err != nil {
		return nil, err
	}

	p.cluster.Status.Namespace = namespace.Name

	if err := p.client.Status().Update(ctx, p.cluster); err != nil {
		return nil, err
	}

	return namespace, nil
}

// Deprovision implements the Provision interface.
func (p *Provisioner) Deprovision(ctx context.Context) error {
	vclusterConfig, err := vcluster.RESTConfig(ctx, p.client, p.cluster.Namespace)
	if err != nil {
		return err
	}

	// Do not inherit the scheme or REST mapper here, it's a different cluster!
	vclusterClient, err := client.New(vclusterConfig, client.Options{})
	if err != nil {
		return err
	}

	namespace, err := util.GetResourceNamespace(ctx, vclusterClient, constants.KubernetesClusterLabel, p.cluster.Name)
	if err != nil {
		// Already dead.
		if errors.Is(err, util.ErrNamespaceLookup) {
			return nil
		}

		return err
	}

	// Cluster API is "special".  In order to deprovision it needs all the secrets we provided
	// to be still in place (in order to talk to Openstack).  So we need to remove all things
	// we added with the exception of secrets.  Once that's done we can nuke the namespace.
	// Like us, they use finalizers to do the cleanup.
	envMapper := func(env string) string {
		mapping := map[string]string{
			"CLUSTER_NAME": p.cluster.Name,
		}

		if value, ok := mapping[env]; ok {
			return value
		}

		return ""
	}

	provisioner := provisioners.NewManifestProvisioner(vclusterClient, provisioners.ManifestProviderOpenstackKubernetesCluster).WithNamespace(p.cluster.Name).WithEnvMapper(envMapper)

	if err := provisioner.Deprovision(ctx); err != nil {
		return err
	}

	// Deprovision the namespace and await deletion.
	if err := provisioners.NewResourceProvisioner(vclusterClient, namespace).Deprovision(ctx); err != nil {
		return err
	}

	return nil
}
