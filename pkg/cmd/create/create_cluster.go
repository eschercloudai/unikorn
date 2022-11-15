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
	"encoding/pem"
	"fmt"
	"net"
	"net/url"
	"strings"

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
	"k8s.io/kubectl/pkg/util/templates"

	"sigs.k8s.io/yaml"
)

const (
	defaultControlPlaneReplicas = 3

	defaultWorkloadReplicas = 3
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

	// cloudProvider is set during completion, and is a really simple file containing
	// the Keystone endpoint.
	cloudProvider []byte

	// caCert is derived from clouds during completion.
	caCert []byte

	// kubernetesVersion defines the Kubernetes version to install.
	kubernetesVersion util.SemverFlag

	// externalNetworkID is an internet facing Openstack network to provision
	// VIPs on for load balancers and stuff.
	externalNetworkID string

	// nodeNetwork is the network prefix Kubernetes nodes live on.
	nodeNetwork net.IPNet

	// podNetwork is the network prefix Kubernetes pods live on.
	podNetwork net.IPNet

	// service Network is the network prefix Kubernetes services live on.
	serviceNetwork net.IPNet

	// dnsNameservers is a list of nameservers for pods and nodes to use.
	dnsNameservers []net.IP

	// image defines the Openstack image for Kubernetes nodes.
	image string

	// kubernetesControlPlaneFlavor defines the Openstack VM flavor Kubernetes control
	// planes use.
	kubernetesControlPlaneFlavor string

	// kubernetesControlPlaneReplicas defines the number of replicas (nodes) for
	// Kubernetes control planes.
	kubernetesControlPlaneReplicas int

	// kubernetesWorkloadFlavor defines the Openstack VM flavor Kubernetes workload
	// clusters use.
	kubernetesWorkloadFlavor string

	// kubernetesWorkloadReplicas defines the number of replicas (nodes) for
	// Kubernetes workload clusters
	kubernetesWorkloadReplicas int

	// availabilityZone defines in what Openstack failure domain the Kubernetes
	// cluster will be provisioned in.
	availabilityZone string

	// sshKeyName defines the SSH key to inject onto the Kubernetes cluster.
	// TODO: this is a legacy thing, and a security hole.  I'm pretty sure
	// cloud-init will do all the provisioning.  If we need access for support
	// then there are better ways of achieving this.
	sshKeyName string

	// client gives access to our custom resources.
	client unikorn.Interface
}

// addFlags registers create cluster options flags with the specified cobra command.
func (o *createClusterOptions) addFlags(f cmdutil.Factory, cmd *cobra.Command) {
	// Unikorn options.
	util.RequiredStringVarWithCompletion(cmd, &o.project, "project", "", "Kubernetes project name that contains the control plane.", computil.ResourceNameCompletionFunc(f, unikornv1alpha1.ProjectResource))
	util.RequiredStringVarWithCompletion(cmd, &o.controlPlane, "control-plane", "", "Control plane to provision the cluster in.", completion.ControlPlanesCompletionFunc(f, &o.project))

	// Openstack configuration options.
	util.RequiredStringVarWithCompletion(cmd, &o.cloud, "cloud", "", "Cloud config to use within clouds.yaml.", completion.CloudCompletionFunc)

	// Kubernetes options.
	util.RequiredVar(cmd, &o.kubernetesVersion, "kubernetes-version", "Kubernetes version to deploy.  Provisioning will be faster if this matches the version preloaded on images defined by the --control-plane-image and --workload-image flags.")

	// Networking options.
	util.RequiredStringVarWithCompletion(cmd, &o.externalNetworkID, "external-network", "", "Openstack external network (see: 'openstack network list --external'.)", completion.OpenstackExternalNetworkCompletionFunc(&o.cloud))
	cmd.Flags().IPNetVar(&o.nodeNetwork, "node-network", defaultNodeNetwork, "Node network prefix.")
	cmd.Flags().IPNetVar(&o.podNetwork, "pod-network", defaultPodNetwork, "Pod network prefix.")
	cmd.Flags().IPNetVar(&o.serviceNetwork, "service-network", defaultServiceNetwork, "Service network prefix.")
	cmd.Flags().IPSliceVar(&o.dnsNameservers, "dns-nameservers", defaultDNSNameservers, "DNS nameservers for pods.")

	// Kubernetes control plane options.
	util.RequiredStringVarWithCompletion(cmd, &o.kubernetesControlPlaneFlavor, "kube-controlplane-flavor", "", "Kubernetes control plane Openstack flavor (see: 'openstack flavor list'.)", completion.OpenstackFlavorCompletionFunc(&o.cloud))
	cmd.Flags().IntVar(&o.kubernetesControlPlaneReplicas, "kube-controlplane-replicas", defaultControlPlaneReplicas, "Kubernetes control plane replicas.")

	// Kubernetes workload cluster options.
	util.RequiredStringVarWithCompletion(cmd, &o.kubernetesWorkloadFlavor, "kube-workload-flavor", "", "Kubernetes workload Openstack flavor (see: 'openstack flavor list'.)", completion.OpenstackFlavorCompletionFunc(&o.cloud))
	cmd.Flags().IntVar(&o.kubernetesWorkloadReplicas, "kube-workload-replicas", defaultWorkloadReplicas, "Kubernetes workload replicas.")

	// Openstack provisioning options.
	util.RequiredStringVarWithCompletion(cmd, &o.image, "image", "", "Kubernetes Openstack image (see: 'openstack image list'.)", completion.OpenstackImageCompletionFunc(&o.cloud))
	util.RequiredStringVarWithCompletion(cmd, &o.availabilityZone, "availability-zone", "", "Openstack availability zone to provision into.  Only one is supported currently (see: 'openstack availability zone list'.)", completion.OpenstackAvailabilityZoneCompletionFunc(&o.cloud))
	util.RequiredStringVarWithCompletion(cmd, &o.sshKeyName, "ssh-key-name", "", "Openstack SSH key to inject onto the Kubernetes nodes (see: 'openstack keypair list'.)", completion.OpenstackSSHKeyCompletionFunc(&o.cloud))
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

	// Set the cloud provider config.
	// TODO: this can use application credentials not user ones, see:
	// https://github.com/kubernetes-sigs/cluster-api-provider-openstack/blob/main/templates/env.rc
	cloudProvider := []string{
		`[Global]`,
		fmt.Sprintf(`auth-url="%s"`, cloud.AuthInfo.AuthURL),
		fmt.Sprintf(`username="%s"`, cloud.AuthInfo.Username),
		fmt.Sprintf(`password="%s"`, cloud.AuthInfo.Password),
		fmt.Sprintf(`domain-name="%s"`, cloud.AuthInfo.DomainName),
		fmt.Sprintf(`tenant-name="%s"`, cloud.AuthInfo.ProjectName),
		fmt.Sprintf(`region="%s"`, cloud.RegionName),
	}

	o.cloudProvider = []byte(strings.Join(cloudProvider, "\n"))

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
				CACert:              &o.caCert,
				CloudConfig:         &o.clouds,
				CloudProviderConfig: &o.cloudProvider,
				Cloud:               &o.cloud,
				FailureDomain:       &o.availabilityZone,
				SSHKeyName:          &o.sshKeyName,
				Image:               &o.image,
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
				Flavor:   &o.kubernetesControlPlaneFlavor,
			},
			Workload: unikornv1alpha1.KubernetesClusterWorkloadSpec{
				Replicas: &o.kubernetesWorkloadReplicas,
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
	createClusterLong = templates.LongDesc(`
	Create a Kubernetes cluster

	This command will use standard lookup rules to find a clouds.yaml file on
	your local system.  You need to supply a --cloud parameter to select the
	cloud and user account to provision with.  Only the selected cloud will be
	passed to CAPI for security reasons.  It's also recommended that you use
	the shell completion for --cloud first, as that'll allow further completion
	functions to poll OpenStack to get images, flavors etc.

	Tab completion is your friend here as it's a very chunky command, with lots
	of required flags, let that be your shepherd.`)

	//nolint:gochecknoglobals
	createClusterExamples = util.TemplatedExample(`
        # Create a Kubernetes cluster
        {{.Application}} create cluster --project foo --control-plane bar --cloud nl1-simon --ssh-key-name spjmurray --external-network c9d130bc-301d-45c0-9328-a6964af65579 --kube-controlplane-flavor c.small --kube-workload-flavor g.medium_a100_MIG_2g.20gb --kubernetes-version v1.24.7 --image ubuntu-2004-kube-v1.24.7 --availability-zone nova baz`)
)

// newCreateClusterCommand creates a command that is able to provison a new Kubernetes
// cluster with a Cluster API control plane.
func newCreateClusterCommand(f cmdutil.Factory) *cobra.Command {
	o := createClusterOptions{}

	cmd := &cobra.Command{
		Use:     "cluster",
		Short:   "Create a Kubernetes cluster",
		Long:    createClusterLong,
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
