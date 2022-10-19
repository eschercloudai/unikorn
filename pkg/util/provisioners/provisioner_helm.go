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
	"context"
	"fmt"
	"os/exec"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"sigs.k8s.io/yaml"
)

// HelmProvisioner runs helm template to install a package.
type HelmProvisioner struct {
	// f is a factory used to access Kubernetes clients.
	f cmdutil.Factory

	// repo to source the chart from.
	repo string

	// chart name.
	chart string

	// namespace to provision in to.
	namespace string

	// name of the release.
	name string

	// args are arguments to pass to helm.
	args []string

	// ownerReferences allows linking all objects with a parent
	// thus facilitating cascading deletion.
	ownerReferences []metav1.OwnerReference
}

// Ensure the Provisioner interface is implemented.
var _ Provisioner = &HelmProvisioner{}

// NewHelmProvisioner returns a provisioner that installs a component or package
// with Helm.
func NewHelmProvisioner(f cmdutil.Factory, repo, chart, namespace, name string, args []string, ownerReferences []metav1.OwnerReference) Provisioner {
	return &HelmProvisioner{
		f:               f,
		repo:            repo,
		chart:           chart,
		namespace:       namespace,
		name:            name,
		args:            args,
		ownerReferences: ownerReferences,
	}
}

// Provision implements the Provision interface.
func (p *HelmProvisioner) Provision() error {
	args := []string{
		"template", p.name, p.chart,
		"--repo", p.repo,
		"--namespace", p.namespace,
	}

	args = append(args, p.args...)

	// TODO: we can probably just hook directly into the Helm library here,
	// saves having to shell out and install 3rd party tooling into our
	// containers.
	out, err := exec.Command("helm", args...).Output()
	if err != nil {
		return err
	}

	yamls := strings.Split(string(out), "\n---\n")

	objects := make([]unstructured.Unstructured, len(yamls))

	for i := range yamls {
		if err := yaml.Unmarshal([]byte(yamls[i]), &objects[i]); err != nil {
			return err
		}
	}

	client, err := p.f.DynamicClient()
	if err != nil {
		return err
	}

	restMapper, err := p.f.ToRESTMapper()
	if err != nil {
		return err
	}

	for i := range objects {
		object := &objects[i]

		object.SetOwnerReferences(p.ownerReferences)

		gvk := object.GroupVersionKind()

		mapping, err := restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			return err
		}

		// Because cascading delete don't work for these (I think, plus the code is ugly!)
		if mapping.Scope.Name() == meta.RESTScopeNameRoot {
			return fmt.Errorf("cluster scoped resources unsupported")
		}

		if _, err := client.Resource(mapping.Resource).Namespace(p.namespace).Create(context.TODO(), object, metav1.CreateOptions{}); err != nil {
			return err
		}
	}

	return nil
}
