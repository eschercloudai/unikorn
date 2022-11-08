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
	"crypto/tls"
	"encoding/pem"
	"fmt"
	"net/url"

	"github.com/gophercloud/utils/openstack/clientconfig"
	"github.com/spf13/cobra"

	unikornv1alpha1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/cmd/errors"
	"github.com/eschercloudai/unikorn/pkg/cmd/util"
	"github.com/eschercloudai/unikorn/pkg/cmd/util/completion"

	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	computil "k8s.io/kubectl/pkg/util/completion"

	"sigs.k8s.io/yaml"
)

// createClusterOptions defines a set of options that are required to create
// a cluster.
type createClusterOptions struct {
	// project defines the project to create the cluster under.
	project string

	// controlPlane defines the control plane name that the cluster will
	// be provisioned with.
	controlPlane string

	// cloud indicates the clouds.yaml key to use.  If only one exists it
	// will default to that, otherwise it's a required parameter.
	cloud string

	// clouds is set during completion, and is a filtered version containing
	// only the specified cloud.
	clouds []byte

	// caCert is derived from clouds during completion.
	caCert []byte

	// name is the name of the cluster.
	name string
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

	cmd.Flags().StringVar(&o.cloud, "cloud", "", "Cloud config to use within clouds.yaml, must be specified if more than one exists in clouds.yaml")
}

// complete fills in any options not does automatically by flag parsing.
func (o *createClusterOptions) complete(args []string) error {
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

	if len(args) != 1 {
		return errors.ErrIncorrectArgumentNum
	}

	o.name = args[0]

	return nil
}

// validate validates any tainted input not handled by complete() or flags
// processing.
func (o *createClusterOptions) validate() error {
	return nil
}

// run executes the command.
func (o *createClusterOptions) run() error {
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
	o := &createClusterOptions{}

	cmd := &cobra.Command{
		Use:     "cluster",
		Short:   "Create a Kubernetes cluster",
		Long:    "Create a Kubernetes cluster",
		Example: createClusterExamples,
		Run: func(cmd *cobra.Command, args []string) {
			util.AssertNilError(o.complete(args))
			util.AssertNilError(o.validate())
			util.AssertNilError(o.run())
		},
	}

	o.addFlags(f, cmd)

	return cmd
}
