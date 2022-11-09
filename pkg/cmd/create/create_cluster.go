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

package create

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net"
	"net/url"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	"github.com/gophercloud/utils/openstack/clientconfig"
	"github.com/spf13/cobra"

	"github.com/eschercloudai/unikorn/generated/clientset/unikorn"
	unikornv1alpha1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/cmd/errors"
	"github.com/eschercloudai/unikorn/pkg/cmd/util"
	"github.com/eschercloudai/unikorn/pkg/cmd/util/completion"
	"github.com/eschercloudai/unikorn/pkg/constants"
	uniutil "github.com/eschercloudai/unikorn/pkg/util"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	computil "k8s.io/kubectl/pkg/util/completion"

	"sigs.k8s.io/yaml"
)

const (
	defaultKubernetesVersion = "v1.25.3"

	defaultControlPlaneImage = "ubuntu-2204-kube-v1.25.3"

	defaultControlPlaneFlavor = "c.small"

	defaultControlPlaneReplicas = 3

	defaultWorkloadImage = "ubuntu-2204-kube-v1.25.3"

	defaultWorkloadFlavor = "c.small"

	defaultWorkloadReplicas = 3

	defaultAvailabilityZone = "nova"
)

var (
	//nolint:gochecknoglobals
	defaultNodeNetwork = net.IPNet{
		IP:   net.IPv4(192, 168, 0, 0),
		Mask: net.IPv4Mask(255, 255, 0, 0),
	}

	//nolint:gochecknoglobals
	defaultPodNetwork = net.IPNet{
		IP:   net.IPv4(10, 0, 0, 0),
		Mask: net.IPv4Mask(255, 0, 0, 0),
	}

	//nolint:gochecknoglobals
	defaultServiceNetwork = net.IPNet{
		IP:   net.IPv4(172, 16, 0, 0),
		Mask: net.IPv4Mask(255, 240, 0, 0),
	}

	//nolint:gochecknoglobals
	defaultDNSNameservers = []net.IP{
		net.IPv4(8, 8, 8, 8),
	}
)

// createClusterOptions defines a set of options that are required to create
// a cluster.
type createClusterOptions struct {
	// project defines the project to create the cluster under.
	project string

	// controlPlane defines the control plane name that the cluster will
	// be provisioned with.
	controlPlane string

	// name is the name of the cluster.
	name string

	// cloud indicates the clouds.yaml key to use.  If only one exists it
	// will default to that, otherwise it's a required parameter.
	cloud string

	// clouds is set during completion, and is a filtered version containing
	// only the specified cloud.
	clouds []byte

	// caCert is derived from clouds during completion.
	caCert []byte

	// kubernetesVersion defines the Kubernetes version to install.
	kubernetesVersion util.SemverFlag

	externalNetworkID string

	nodeNetwork    net.IPNet
	podNetwork     net.IPNet
	serviceNetwork net.IPNet
	dnsNameservers []net.IP

	kubernetesControlPlaneImage    string
	kubernetesControlPlaneFlavor   string
	kubernetesControlPlaneReplicas int

	kubernetesWorkloadImage    string
	kubernetesWorkloadFlavor   string
	kubernetesWorkloadReplicas int

	availabilityZone string

	sshKeyName string

	// client gives access to our custom resources.
	client unikorn.Interface
}

// newCreateClusterOptions returns a new set of cluster options with any flag.Value
// type flags initialised with defaults.
func newCreateClusterOptions() *createClusterOptions {
	return &createClusterOptions{
		kubernetesVersion: util.SemverFlag{
			Semver: defaultKubernetesVersion,
		},
	}
}

// addFlags registers create cluster options flags with the specified cobra command.
func (o *createClusterOptions) addFlags(f cmdutil.Factory, cmd *cobra.Command) {
	cmd.Flags().StringVar(&o.project, "project", "", "Kubernetes project name that contains the control plane.")

	if err := cmd.MarkFlagRequired("project"); err != nil {
		panic(err)
	}

	if err := cmd.RegisterFlagCompletionFunc("project", computil.ResourceNameCompletionFunc(f, unikornv1alpha1.ProjectResource)); err != nil {
		panic(err)
	}

	cmd.Flags().StringVar(&o.controlPlane, "control-plane", "", "Control plane to provision the cluster in.")

	if err := cmd.MarkFlagRequired("control-plane"); err != nil {
		panic(err)
	}

	if err := cmd.RegisterFlagCompletionFunc("control-plane", completion.ControlPlanesCompletionFunc(f, &o.project)); err != nil {
		panic(err)
	}

	cmd.Flags().StringVar(&o.cloud, "cloud", "", "Cloud config to use within clouds.yaml, must be specified if more than one exists in clouds.yaml.")

	cmd.Flags().Var(&o.kubernetesVersion, "kubernetes-version", "Kubernetes version to deploy.  Provisioning will be faster if this matches the version preloaded on images defined by the --control-plane-image and --workload-image flags.")

	cmd.Flags().StringVar(&o.externalNetworkID, "external-network-id", "", "Openstack external network ID.  If not set, Openstack will be polled and if a single network is returned this is used.  Any other situation is considered an error and it must be manually specified (see: 'openstack network list --external'.)")
	cmd.Flags().IPNetVar(&o.nodeNetwork, "node-network", defaultNodeNetwork, "Node network prefix.")
	cmd.Flags().IPNetVar(&o.podNetwork, "pod-network", defaultPodNetwork, "Pod network prefix.")
	cmd.Flags().IPNetVar(&o.serviceNetwork, "service-network", defaultServiceNetwork, "Service network prefix.")
	cmd.Flags().IPSliceVar(&o.dnsNameservers, "dns-nameservers", defaultDNSNameservers, "DNS nameservers for pods.")

	cmd.Flags().StringVar(&o.kubernetesControlPlaneImage, "kube-controlplane-image", defaultControlPlaneImage, "Kubernetes control plane Openstack image (see: 'openstack image list'.)")
	cmd.Flags().StringVar(&o.kubernetesControlPlaneFlavor, "kube-controlplane-flavor", defaultControlPlaneFlavor, "Kubernetes control plane Openstack flavor (see: 'openstack flavor list'.)")
	cmd.Flags().IntVar(&o.kubernetesControlPlaneReplicas, "kube-controlplane-replicas", defaultControlPlaneReplicas, "Kubernetes control plane replicas.")

	cmd.Flags().StringVar(&o.kubernetesWorkloadImage, "kube-workload-image", defaultWorkloadImage, "Kubernetes workload Openstack image (see: 'openstack image list'.)")
	cmd.Flags().StringVar(&o.kubernetesWorkloadFlavor, "kube-workload-flavor", defaultWorkloadFlavor, "Kubernetes workload Openstack flavor (see: 'openstack flavor list'.)")
	cmd.Flags().IntVar(&o.kubernetesWorkloadReplicas, "kube-workload-replicas", defaultWorkloadReplicas, "Kubernetes workload replicas.")

	cmd.Flags().StringVar(&o.availabilityZone, "availability-zone", defaultAvailabilityZone, "Openstack availability zone to provision into.  Only one is supported currently (see: 'openstack availability zone list'.)")

	// TODO: is this actually necessary?  Sounds like a security hole to me.
	// It's a legacy part of this template:
	// https://github.com/kubernetes-sigs/cluster-api-provider-openstack/blob/main/templates/cluster-template.yaml
	// I'm guessing we should be able to remove it using a JSON patch applied to the
	// manifest to override this requirement.
	cmd.Flags().StringVar(&o.sshKeyName, "ssh-key-name", "", "Openstack SSH key to inject onto the Kubernetes nodes (see: 'openstack keypair list'.)")

	if err := cmd.MarkFlagRequired("ssh-key-name"); err != nil {
		panic(err)
	}
}

// complete fills in any options not does automatically by flag parsing.
func (o *createClusterOptions) complete(f cmdutil.Factory, args []string) error {
	config, err := f.ToRESTConfig()
	if err != nil {
		return err
	}

	if o.client, err = unikorn.NewForConfig(config); err != nil {
		return err
	}

	if err := o.completeOpenstackConfig(); err != nil {
		return err
	}

	if err := o.completeOpenstackNetworking(); err != nil {
		return err
	}

	if len(args) != 1 {
		return errors.ErrIncorrectArgumentNum
	}

	o.name = args[0]

	return nil
}

// completeOpenstackConfig loads the Openstack configuration and derives some options
// from that file.
func (o *createClusterOptions) completeOpenstackConfig() error {
	clouds, err := clientconfig.LoadCloudsYAML()
	if err != nil {
		return err
	}

	// Do the automatic defaulting if only one cloud exists and it's not
	// explicitly specified.
	if len(clouds) == 1 && o.cloud == "" {
		for k := range clouds {
			o.cloud = k

			break
		}
	}

	// Ensure the cloud exists.
	cloud, ok := clouds[o.cloud]
	if !ok {
		return fmt.Errorf("%w: cloud '%s' not found in clouds.yaml", errors.ErrNotFound, o.cloud)
	}

	// Build the fitered clouds.yaml for use by the provisioner.
	filteredClouds := &clientconfig.Clouds{
		Clouds: map[string]clientconfig.Cloud{
			o.cloud: cloud,
		},
	}

	filteredCloudsYaml, err := yaml.Marshal(filteredClouds)
	if err != nil {
		return err
	}

	o.clouds = filteredCloudsYaml

	// Work out the correct CA to use.
	// Screw private clouds, public is the future!
	authURL, err := url.Parse(cloud.AuthInfo.AuthURL)
	if err != nil {
		return err
	}

	conn, err := tls.Dial("tcp", authURL.Host, nil)
	if err != nil {
		return err
	}

	defer conn.Close()

	chains := conn.ConnectionState().VerifiedChains
	chain := chains[0]
	ca := chain[len(chain)-1]

	pemBlock := &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: ca.Raw,
	}

	o.caCert = pem.EncodeToMemory(pemBlock)

	return nil
}

// Network allows us to extend gophercloud to get access to more interesting
// fields not available in the standard data types.
type Network struct {
	// Network is the gophercloud network type.  This needs to be a field,
	// not an embedded type, lest its UnmarshalJSON function get promoted...
	Network networks.Network

	// External is the bit we care about, is it an external network ID?
	External bool `json:"router:external"`
}

// UnmarshalJSON does magic quite frankly.  We unmarshal directly into the
// gophercloud network type, easy peasy.  When un marshalling into our network
// type, we need to define a temporary type to avoid an infinite loop...
func (n *Network) UnmarshalJSON(b []byte) error {
	if err := json.Unmarshal(b, &n.Network); err != nil {
		return err
	}

	type tmp Network

	var s struct {
		tmp
	}

	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	n.External = s.tmp.External

	return nil
}

func (o *createClusterOptions) completeOpenstackNetworking() error {
	if o.externalNetworkID != "" {
		return nil
	}

	clientOpts := &clientconfig.ClientOpts{
		Cloud: o.cloud,
	}

	authOpts, err := clientconfig.AuthOptions(clientOpts)
	if err != nil {
		return err
	}

	provider, err := openstack.AuthenticatedClient(*authOpts)
	if err != nil {
		return err
	}

	client, err := openstack.NewNetworkV2(provider, gophercloud.EndpointOpts{})
	if err != nil {
		return err
	}

	// This sucks, you cannot directly query for external networks...
	pager := networks.List(client, &networks.ListOpts{})

	page, err := pager.AllPages()
	if err != nil {
		return err
	}

	var results []Network

	if err := networks.ExtractNetworksInto(page, &results); err != nil {
		return err
	}

	var externalNetworks []Network

	for _, network := range results {
		if network.External {
			externalNetworks = append(externalNetworks, network)
		}
	}

	if len(externalNetworks) != 1 {
		// TODO: temporary hack, we can add this all to a completion function.
		//nolint:goerr113
		return fmt.Errorf("unable to derive external network, specify it explicitly")
	}

	o.externalNetworkID = externalNetworks[0].Network.ID

	return nil
}

// validate validates any tainted input not handled by complete() or flags
// processing.
func (o *createClusterOptions) validate() error {
	return nil
}

// run executes the command.
func (o *createClusterOptions) run() error {
	project, err := o.client.UnikornV1alpha1().Projects().Get(context.TODO(), o.project, metav1.GetOptions{})
	if err != nil {
		return err
	}

	namespace := project.Status.Namespace

	if len(namespace) == 0 {
		panic("achtung!")
	}

	controlPlane, err := o.client.UnikornV1alpha1().ControlPlanes(namespace).Get(context.TODO(), o.controlPlane, metav1.GetOptions{})
	if err != nil {
		return err
	}

	gvk, err := uniutil.ObjectGroupVersionKind(scheme.Scheme, controlPlane)
	if err != nil {
		return err
	}

	kubernetesVersion := unikornv1alpha1.SemanticVersion(o.kubernetesVersion.Semver)

	cluster := &unikornv1alpha1.KubernetesCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: o.name,
			Labels: map[string]string{
				constants.VersionLabel: constants.Version,
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(controlPlane, *gvk),
			},
		},
		Spec: unikornv1alpha1.KubernetesClusterSpec{
			ProvisionerControlPlane: controlPlane.Name,
			KubernetesVersion:       &kubernetesVersion,
			Openstack: unikornv1alpha1.KubernetesClusterOpenstackSpec{
				CACert:        &o.caCert,
				CloudConfig:   &o.clouds,
				Cloud:         &o.cloud,
				FailureDomain: &o.availabilityZone,
				SSHKeyName:    &o.sshKeyName,
			},
			Network: unikornv1alpha1.KubernetesClusterNetworkSpec{
				NodeNetwork:       &unikornv1alpha1.IPv4Prefix{IPNet: o.nodeNetwork},
				PodNetwork:        &unikornv1alpha1.IPv4Prefix{IPNet: o.podNetwork},
				ServiceNetwork:    &unikornv1alpha1.IPv4Prefix{IPNet: o.serviceNetwork},
				DNSNameservers:    unikornv1alpha1.IPv4AddressSliceFromIPSlice(o.dnsNameservers),
				ExternalNetworkID: &o.externalNetworkID,
			},
			ControlPlane: unikornv1alpha1.KubernetesClusterControlPlaneSpec{
				Replicas: &o.kubernetesControlPlaneReplicas,
				Image:    &o.kubernetesControlPlaneImage,
				Flavor:   &o.kubernetesControlPlaneFlavor,
			},
			Workload: unikornv1alpha1.KubernetesClusterWorkloadSpec{
				Replicas: &o.kubernetesWorkloadReplicas,
				Image:    &o.kubernetesWorkloadImage,
				Flavor:   &o.kubernetesWorkloadFlavor,
			},
		},
	}

	if _, err := o.client.UnikornV1alpha1().KubernetesClusters(namespace).Create(context.TODO(), cluster, metav1.CreateOptions{}); err != nil {
		return err
	}

	return nil
}

var (
	//nolint:gochecknoglobals
	createClusterExamples = util.TemplatedExample(`
        # Create a Kubernetes cluster
        {{.Application}} create cluster --project foo --control-plane bar baz`)
)

// newCreateClusterCommand creates a command that is able to provison a new Kubernetes
// cluster with a Cluster API control plane.
func newCreateClusterCommand(f cmdutil.Factory) *cobra.Command {
	o := newCreateClusterOptions()

	cmd := &cobra.Command{
		Use:     "cluster",
		Short:   "Create a Kubernetes cluster",
		Long:    "Create a Kubernetes cluster",
		Example: createClusterExamples,
		Run: func(cmd *cobra.Command, args []string) {
			util.AssertNilError(o.complete(f, args))
			util.AssertNilError(o.validate())
			util.AssertNilError(o.run())
		},
	}

	o.addFlags(f, cmd)

	return cmd
}
