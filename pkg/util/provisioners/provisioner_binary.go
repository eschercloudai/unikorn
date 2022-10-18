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

package provisioners

import (
	"os/exec"

	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// BinaryProvisioner runs a binary to install a package.
// This is considered bad practice as it's essentially a black box, and we
// have limited control over how things are installed from a compliance
// perspective.
type BinaryProvisioner struct {
	// config allows access to the provided kubeconfig, context etc.
	// TODO: this is not aware of ClientConfigLoadingRules so environment
	// variables will be ignored for now.
	config *genericclioptions.ConfigFlags

	// command is the command to run.
	command string

	// args are any required arguments.
	args []string
}

// Ensure the Provisioner interface is implemented.
var _ Provisioner = &BinaryProvisioner{}

// NewBinaryProvisioner returns a provisioner that installs a component or package
// with a binary installer.
func NewBinaryProvisioner(config *genericclioptions.ConfigFlags, command string, args ...string) Provisioner {
	return &BinaryProvisioner{
		config:  config,
		command: command,
		args:    args,
	}
}

// Provision implements the Provision interface.
func (p *BinaryProvisioner) Provision() error {
	var args []string

	/* TODO: there is no way to get this information from a cobra command...
	// If explcitly specified in the top level command, use these
	// TODO: some binaries may choose not to implement these flags, or more
	// annoyingly call them something else.
	if len(*p.config.KubeConfig) > 0 {
		args = append(args, "--kubeconfig", *p.config.KubeConfig)
	}

	if len(*p.config.Context) > 0 {
		args = append(args, "--context", *p.config.Context)
	}
	*/

	args = append(args, p.args...)

	if err := exec.Command(p.command, args...).Run(); err != nil {
		return err
	}

	return nil
}
