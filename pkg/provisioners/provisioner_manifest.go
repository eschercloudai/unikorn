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
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/drone/envsubst"
	"github.com/go-logr/logr"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"
)

// ManifestID defines a known component that is provisoned with manifests.
type ManifestID string

const (
	// ManifestVCluster is Loft's vcluster (virtual cluster).
	ManifestVCluster ManifestID = "vcluster"

	// ManifestCertManager is Jetstack's cert-manager.
	ManifestCertManager ManifestID = "cert-manager"

	// ManifestClusterAPICore is the cluster API controller manager.
	ManifestClusterAPICore ManifestID = "cluster-api-core"

	// ManifestClusterAPIControlPlane is the cluster API control plane manager.
	ManifestClusterAPIControlPlane ManifestID = "cluster-api-control-plane"

	// ManifestClusterAPIBootstrap is the cluster API bootstrap manager.
	ManifestClusterAPIBootstrap ManifestID = "cluster-api-bootstrap"

	// ManifestClusterAPIProviderOpenstack is the cluster API OpenStack provider.
	ManifestClusterAPIProviderOpenstack ManifestID = "cluster-api-provider-openstack"
)

const (
	HelmReleaseEyecatcher = "unikorn-release"

	HelmNamespaceEyecatcher = "unikorn-namespace"
)

// manifestRegistryEntry defines where to source manifests from and other metadata
// about how they were generated and transforms we can or need to perform on them
// from their raw state.
type manifestRegistryEntry struct {
	// path is the path of the manifest directory.
	// This path is a local path, no URLs are allowed here as that opens up
	// the possibility of supply chain attacks.  All manifests need to be
	// peer reviewed for security.
	path string

	// templated tells us if the manifest is templated e.g. using
	// "helm template".  If so then it's expected to be seeded with
	// HelmReleaseEyecatcher as the release name and HelmNamespaceEyecatcher
	// as the namespace.  Templated manifests are those that can be deployed
	// multiple times in the same namespace.
	templated bool

	// withSusbstitution is a bit of a hack for manifests sourced from
	// the cluster API project.  These feature "shell" variable expansion
	// to inject values and need to be processed as such.
	withSusbstitution bool
}

var (
	// manifestRegistry records a mpping from manifest ID to its metadata.
	// At present this is global, but you could make a case for using build
	// constraints at a later point.
	//nolint:gochecknoglobals
	manifestRegistry = map[ManifestID]manifestRegistryEntry{
		ManifestVCluster: {
			path:      "/manifests/vcluster",
			templated: true,
		},
		ManifestCertManager: {
			path: "/manifests/cert-manager",
		},
		ManifestClusterAPICore: {
			path:              "/manifests/cluster-api-core",
			withSusbstitution: true,
		},
		ManifestClusterAPIControlPlane: {
			path:              "/manifests/cluster-api-control-plane",
			withSusbstitution: true,
		},
		ManifestClusterAPIBootstrap: {
			path:              "/manifests/cluster-api-bootstrap",
			withSusbstitution: true,
		},
		ManifestClusterAPIProviderOpenstack: {
			path:              "/manifests/cluster-api-provider-openstack",
			withSusbstitution: true,
		},
	}
)

// ManifestProvisioner is a provisioner that is able to parse and manage resources
// sourced from a yaml manifest.
type ManifestProvisioner struct {
	// client is a client to allow Kubernetes access.
	client client.Client

	// id is the manifest we want to provision.
	id ManifestID

	// name is a replacement name for a templated manifest.
	name string

	// namespace is a replacement namespace for a templated manifest.
	namespace string

	// ownerReferences allows all manifest resources to be linked to
	// a parent resource that will trigger cascading deletion.
	ownerReferences []metav1.OwnerReference

	// log is set on provison with context specific information for
	// this provision.
	log logr.Logger

	// entry is set on provison with a cached copy of the registry
	// metadata related to the manifest ID.
	entry manifestRegistryEntry
}

// Ensure the Provisioner interface is implemented.
var _ Provisioner = &ManifestProvisioner{}

// NewManifestProvisioner returns a new provisioner that is capable of applying
// a manifest with kubectl.  The path argument may be a path on the local file
// system or a URL.
func NewManifestProvisioner(client client.Client, id ManifestID) *ManifestProvisioner {
	return &ManifestProvisioner{
		client: client,
		id:     id,
	}
}

// WithName associates a replacement name for a templated manifest.
func (p *ManifestProvisioner) WithName(name string) *ManifestProvisioner {
	p.name = name

	return p
}

// WithNamespace associates a replacement namespace for a templated manifest.
func (p *ManifestProvisioner) WithNamespace(namespace string) *ManifestProvisioner {
	p.namespace = namespace

	return p
}

// WithOwnerReferences associates an owner reference list with a manifest for
// cascading deletion.
func (p *ManifestProvisioner) WithOwnerReferences(ownerReferences []metav1.OwnerReference) *ManifestProvisioner {
	p.ownerReferences = ownerReferences

	return p
}

// readManifest loads the manifest file from the local filesystem.
func (p *ManifestProvisioner) readManifest() (string, error) {
	path := filepath.Join(p.entry.path, "manifest.yaml")

	p.log.V(1).Info("loading manifest", "path", path)

	// Load in the main manifest file.
	manifest, err := os.Open(path)
	if err != nil {
		return "", err
	}

	defer manifest.Close()

	bytes, err := io.ReadAll(manifest)
	if err != nil {
		return "", err
	}

	return string(bytes), nil
}

// processTemplate optionally substitutes the name and namespace in a templated
// manifest.
func (p *ManifestProvisioner) processTemplate(s string) string {
	if !p.entry.templated {
		return s
	}

	p.log.V(1).Info("applying template", "name", p.name)

	if p.name == "" {
		panic("no name for templated manifest")
	}

	s = strings.ReplaceAll(s, HelmReleaseEyecatcher, p.name)

	p.log.V(1).Info("applying template", "namespace", p.namespace)

	if p.namespace == "" {
		panic("no namespace for templated manifest")
	}

	return strings.ReplaceAll(s, HelmNamespaceEyecatcher, p.namespace)
}

// processSubstitution optionally substitutes "shell" environment variables
// in a manifest.
func (p *ManifestProvisioner) processSubstitution(s string) (string, error) {
	if !p.entry.withSusbstitution {
		return s, nil
	}

	p.log.V(1).Info("applying environment substitution")

	s, err := envsubst.EvalEnv(s)
	if err != nil {
		return "", err
	}

	return s, nil
}

// parse splits the manifest up into YAML objects and unmarshals them into a
// generic unstructured format.
func (p *ManifestProvisioner) parse(s string) ([]unstructured.Unstructured, error) {
	sections := strings.Split(s, "\n---\n")

	var yamls []string

	// Discard any empty sections.
	for _, section := range sections {
		if strings.TrimSpace(section) != "" {
			yamls = append(yamls, section)
		}
	}

	objects := make([]unstructured.Unstructured, len(yamls))

	for i := range yamls {
		if err := yaml.Unmarshal([]byte(yamls[i]), &objects[i]); err != nil {
			return nil, err
		}
	}

	return objects, nil
}

// provision creates any objects required by the manifest.
func (p *ManifestProvisioner) provision(ctx context.Context, objects []unstructured.Unstructured) error {
	for i := range objects {
		object := &objects[i]

		if p.ownerReferences != nil {
			object.SetOwnerReferences(p.ownerReferences)
		}

		// Create the object if it doesn't exist.
		// TODO: the fallthrough case here should be update and upgrade,
		// but that's a ways off!
		objectKey := client.ObjectKeyFromObject(object)

		// NOTE: while we don't strictly need the existing resource yet, it'll
		// moan if you don't provide something to store into.
		existing, ok := object.NewEmptyInstance().(*unstructured.Unstructured)
		if !ok {
			panic("unstructured empty instance fail")
		}

		if err := p.client.Get(ctx, objectKey, existing); err != nil {
			if !errors.IsNotFound(err) {
				return err
			}

			p.log.Info("creating object", "key", objectKey)

			if err := p.client.Create(ctx, object); err != nil {
				return err
			}

			continue
		}
	}

	return nil
}

// Provision implements the Provision interface.
func (p *ManifestProvisioner) Provision(ctx context.Context) error {
	p.log = log.FromContext(ctx).WithValues("manifest", p.id)

	// Find the manifest descriptor.
	var ok bool

	p.entry, ok = manifestRegistry[p.id]
	if !ok {
		panic("no registry entry")
	}

	// Do any processing filters.
	contents, err := p.readManifest()
	if err != nil {
		return err
	}

	contents = p.processTemplate(contents)

	contents, err = p.processSubstitution(contents)
	if err != nil {
		return err
	}

	objects, err := p.parse(contents)
	if err != nil {
		return err
	}

	if err := p.provision(ctx, objects); err != nil {
		return err
	}

	return nil
}
