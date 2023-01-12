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

	"github.com/gophercloud/utils/openstack/clientconfig"
	"github.com/spf13/cobra"

	"github.com/eschercloudai/unikorn/generated/clientset/unikorn"
	unikornv1alpha1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/cmd/errors"
	"github.com/eschercloudai/unikorn/pkg/cmd/util"
	"github.com/eschercloudai/unikorn/pkg/cmd/util/completion"
	"github.com/eschercloudai/unikorn/pkg/constants"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	computil "k8s.io/kubectl/pkg/util/completion"
	"k8s.io/kubectl/pkg/util/templates"

	"sigs.k8s.io/yaml"
)

const (
	defaultControlPlaneReplicas = 3
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

	// version defines the Kubernetes version to install.
	version util.SemverFlag

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

	// flavor defines the Openstack VM flavor Kubernetes control
	// planes use.
	flavor string

	// replicas defines the number of replicas (nodes) for
	// Kubernetes control planes.
	replicas int

	// diskSize defines the persistent volume size to provision with.
	diskSize util.QuantityFlag

	// region is the OpenStack region the cluster exists in.
	region string

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
	util.RequiredVar(cmd, &o.version, "kubernetes-version", "Kubernetes version to deploy.  Provisioning will be faster if this matches the version preloaded on images defined by the --control-plane-image and --workload-image flags.")

	// Networking options.
	util.RequiredStringVarWithCompletion(cmd, &o.externalNetworkID, "external-network", "", "Openstack external network (see: 'openstack network list --external'.)", completion.OpenstackExternalNetworkCompletionFunc(&o.cloud))
	cmd.Flags().IPNetVar(&o.nodeNetwork, "node-network", defaultNodeNetwork, "Node network prefix.")
	cmd.Flags().IPNetVar(&o.podNetwork, "pod-network", defaultPodNetwork, "Pod network prefix.")
	cmd.Flags().IPNetVar(&o.serviceNetwork, "service-network", defaultServiceNetwork, "Service network prefix.")
	cmd.Flags().IPSliceVar(&o.dnsNameservers, "dns-nameservers", defaultDNSNameservers, "DNS nameservers for pods.")

	// Kubernetes control plane options.
	util.RequiredStringVarWithCompletion(cmd, &o.flavor, "flavor", "", "Kubernetes control plane Openstack flavor (see: 'openstack flavor list'.)", completion.OpenstackFlavorCompletionFunc(&o.cloud))
	cmd.Flags().IntVar(&o.replicas, "replicas", defaultControlPlaneReplicas, "Kubernetes control plane replicas.")
	cmd.Flags().Var(&o.diskSize, "disk-size", "Kubernetes control plane disk size, defaults to that of the machine flavor.")

	// Openstack provisioning options.
	util.RequiredStringVarWithCompletion(cmd, &o.image, "image", "", "Kubernetes Openstack image (see: 'openstack image list'.)", completion.OpenstackImageCompletionFunc(&o.cloud))
	util.RequiredStringVar(cmd, &o.region, "region", "", "Openstack region to provision into.")
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
	namespace, err := util.GetControlPlaneNamespace(context.TODO(), o.client, o.project, o.controlPlane)
	if err != nil {
		return err
	}

	version := unikornv1alpha1.SemanticVersion(o.version.Semver)

	cluster := &unikornv1alpha1.KubernetesCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: o.name,
			Labels: map[string]string{
				constants.VersionLabel:      constants.Version,
				constants.ProjectLabel:      o.project,
				constants.ControlPlaneLabel: o.controlPlane,
			},
		},
		Spec: unikornv1alpha1.KubernetesClusterSpec{
			Openstack: &unikornv1alpha1.KubernetesClusterOpenstackSpec{
				CACert:            &o.caCert,
				CloudConfig:       &o.clouds,
				Cloud:             &o.cloud,
				Region:            &o.region,
				FailureDomain:     &o.availabilityZone,
				SSHKeyName:        &o.sshKeyName,
				ExternalNetworkID: &o.externalNetworkID,
			},
			Network: &unikornv1alpha1.KubernetesClusterNetworkSpec{
				NodeNetwork:    &unikornv1alpha1.IPv4Prefix{IPNet: o.nodeNetwork},
				PodNetwork:     &unikornv1alpha1.IPv4Prefix{IPNet: o.podNetwork},
				ServiceNetwork: &unikornv1alpha1.IPv4Prefix{IPNet: o.serviceNetwork},
				DNSNameservers: unikornv1alpha1.IPv4AddressSliceFromIPSlice(o.dnsNameservers),
			},
			ControlPlane: &unikornv1alpha1.KubernetesClusterControlPlaneSpec{
				MachineGeneric: unikornv1alpha1.MachineGeneric{
					Version:  &version,
					Image:    &o.image,
					Flavor:   &o.flavor,
					Replicas: &o.replicas,
					DiskSize: o.diskSize.Quantity,
				},
			},
			WorkloadPools: &unikornv1alpha1.KubernetesClusterWorkloadPoolsSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						constants.KubernetesClusterLabel: o.name,
					},
				},
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
        {{.Application}} create cluster --project foo --control-plane bar --cloud nl1-simon --ssh-key-name spjmurray --external-network c9d130bc-301d-45c0-9328-a6964af65579 --flavor c.small --version v1.24.7 --image ubuntu-2004-kube-v1.24.7 --availability-zone nova baz`)
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
