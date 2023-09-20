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

package kubectl

import (
	"context"
	"os/exec"

	"github.com/eschercloudai/unikorn/pkg/provisioners"

	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// Provisioner uses "kubectl apply" to provision the resources.
// We use raw config flags here as we can pass them directly to the
// underlying kubectl command.  We could use a higher level abstraction
// here, like kubectl's cmdutil.Factory, but then we'd just have to create
// a temporary kubeconfig.  We could also just hook into kubectl's apply
// logic, which would be a better solution long term, but time...
// Do not use this for production deployments, it's a supply chain
// nightmare!
// TODO: some manifests may not have a namspace, we may want to allow
// overriding this.
type Provisioner struct {
	provisioners.ProvisionerMeta

	// config allows access to the provided kubeconfig, context etc.
	// TODO: this is not aware of ClientConfigLoadingRules so environment
	// variables will be ignored for now.
	config *genericclioptions.ConfigFlags

	// path is the path to the YAML manifest.
	path string
}

// Ensure the Provisioner interface is implemented.
var _ provisioners.Provisioner = &Provisioner{}

// New returns a new provisioner that is capable of applying
// a manifest with kubectl.  The path argument may be a path on the local file
// system or a URL.
func New(config *genericclioptions.ConfigFlags, path string) *Provisioner {
	return &Provisioner{
		config: config,
		path:   path,
	}
}

// Provision implements the Provision interface.
func (p *Provisioner) Provision(_ context.Context) error {
	var args []string

	// If explcitly specified in the top level command, use these
	if len(*p.config.KubeConfig) > 0 {
		args = append(args, "--kubeconfig", *p.config.KubeConfig)
	}

	if len(*p.config.Context) > 0 {
		args = append(args, "--context", *p.config.Context)
	}

	args = append(args, "apply", "-f", p.path)

	if err := exec.Command("kubectl", args...).Run(); err != nil {
		return err
	}

	return nil
}

// Deprovision implements the Provision interface.
func (p *Provisioner) Deprovision(context.Context) error {
	return nil
}
