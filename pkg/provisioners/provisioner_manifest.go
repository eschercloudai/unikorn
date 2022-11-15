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
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/drone/envsubst"
	jsonpatch "github.com/evanphx/json-patch"
	"github.com/go-logr/logr"

	"github.com/eschercloudai/unikorn/pkg/util"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
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

	// ManifestClusterAPIProviderOpenstack is the cluster API OpenStack provider.
	ManifestClusterAPIAddonProvider ManifestID = "cluster-api-addon-provider"

	// ManifestProviderOpenstackKubernetesCluster is a Kubernetes cluster.
	ManifestProviderOpenstackKubernetesCluster ManifestID = "cluster-api-cluster-openstack"
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

	// withSubstitution is a bit of a hack for manifests sourced from
	// the cluster API project.  These feature "shell" variable expansion
	// to inject values and need to be processed as such.
	withSubstitution bool
}

var (
	// manifestRegistry records a mapping from manifest ID to its metadata.
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
			path:             "/manifests/cluster-api-core",
			withSubstitution: true,
		},
		ManifestClusterAPIControlPlane: {
			path:             "/manifests/cluster-api-control-plane",
			withSubstitution: true,
		},
		ManifestClusterAPIBootstrap: {
			path:             "/manifests/cluster-api-bootstrap",
			withSubstitution: true,
		},
		ManifestClusterAPIProviderOpenstack: {
			path:             "/manifests/cluster-api-provider-openstack",
			withSubstitution: true,
		},
		ManifestClusterAPIAddonProvider: {
			path:             "/manifests/cluster-api-addon-provider",
			withSubstitution: true,
		},
		ManifestProviderOpenstackKubernetesCluster: {
			path:             "/manifests/cluster-api-cluster-openstack",
			withSubstitution: true,
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

	// envMapper allows manifest with environment subsitution to
	// map variables to values.
	envMapper func(string) string
}

// nullEnvMapper is a dummy mapper for when none is specified, by the manifest
// requires one.
func nullEnvMapper(_ string) string {
	return ""
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

// WithEnvMapper associates a mapping function that substitutes variables with values.
func (p *ManifestProvisioner) WithEnvMapper(f func(string) string) *ManifestProvisioner {
	p.envMapper = f

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
// TODO: we can actually just do some magic here to make it look like it's a
// substitution, generalise things etc.
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

// patch allows us to take vendor provided manifests and do some custom
// overrides.  This is safer in the long run, rather than manually hacking
// stuff as we pick up the vendor changes for free, don't accidentally forget
// to add some manual hacks.  Instead patches will fail to apply if things
// go too far out of whack.
type patch struct {
	// APIVersion defines the resource group/version to apply the patches to.
	APIVersion string `json:"apiVersion"`

	// Kind defines the resource kind to apply the patches to.
	Kind string `json:"kind"`

	// Name defines the resource name to apply the patches to.
	// This is optional, so by omitting it you can apply the patch to all
	// resources that match the GVK only.
	Name *string `json:"name"`

	// Patch is the patchset, basically a valid array of JSON Patch objects.
	Patch jsonpatch.Patch `json:"patches"`
}

// processPatches looks for a patch definition file to accompany a manifest.
// If one exists, then parse the manifest into objects, if the object matches
// any patch's constraints, then apply the patchset.  This should be called
// before any environment variable subsitution, as the patches themselves may
// contain variables.
func (p *ManifestProvisioner) processPatches(s string) (string, error) {
	path := filepath.Join(p.entry.path, "patches.json")

	patchFile, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			p.log.V(1).Info("no patch file found for manifest")

			return s, nil
		}

		return "", err
	}

	defer patchFile.Close()

	p.log.V(1).Info("applying patches")

	bytes, err := io.ReadAll(patchFile)
	if err != nil {
		return "", err
	}

	var patches []patch

	if err := json.Unmarshal(bytes, &patches); err != nil {
		return "", err
	}

	yamls := util.SplitYAML(s)

	for i := range yamls {
		patchedYAML, err := p.processPatchesYAML(yamls[i], patches)
		if err != nil {
			return "", err
		}

		yamls[i] = patchedYAML
	}

	return strings.Join(yamls, "\n---\n"), nil
}

// processPatchesYAML applies patches to a single YAML object.
func (p *ManifestProvisioner) processPatchesYAML(s string, patches []patch) (string, error) {
	var object unstructured.Unstructured

	if err := yaml.Unmarshal([]byte(s), &object); err != nil {
		return "", err
	}

	jsonObject, err := json.Marshal(object.Object)
	if err != nil {
		return "", err
	}

	for _, patch := range patches {
		if object.GetAPIVersion() != patch.APIVersion || object.GetKind() != patch.Kind {
			continue
		}

		patchedJSON, err := patch.Patch.Apply(jsonObject)
		if err != nil {
			return "", err
		}

		p.log.V(1).Info("applied patch", "object", string(jsonObject), "result", string(patchedJSON))

		jsonObject = patchedJSON
	}

	patchedYAML, err := yaml.JSONToYAML(jsonObject)
	if err != nil {
		return "", err
	}

	return string(patchedYAML), nil
}

// processSubstitution optionally substitutes "shell" environment variables
// in a manifest.
// TODO: we should pre-process and ensure all environment subsitutions will
// actual resolve to something.
func (p *ManifestProvisioner) processSubstitution(s string) (string, error) {
	if !p.entry.withSubstitution {
		return s, nil
	}

	p.log.V(1).Info("applying environment substitution")

	envMapper := nullEnvMapper

	if p.envMapper != nil {
		envMapper = p.envMapper
	}

	s, err := envsubst.Eval(s, envMapper)
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

// applyNamespace does any namespace defaulting required of the manifest object.
func (p *ManifestProvisioner) applyNamespace(object *unstructured.Unstructured) error {
	gvk := object.GroupVersionKind()

	mapping, err := p.client.RESTMapper().RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return err
	}

	if mapping.Scope.Name() != meta.RESTScopeNameRoot && object.GetNamespace() == "" {
		if p.namespace == "" {
			panic("object has no namespace and none provided")
		}

		object.SetNamespace(p.namespace)
	}

	return nil
}

// provision creates any objects required by the manifest.
func (p *ManifestProvisioner) provision(ctx context.Context, objects []unstructured.Unstructured) error {
	for i := range objects {
		object := &objects[i]

		if err := p.applyNamespace(object); err != nil {
			return err
		}

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

	contents, err = p.processPatches(contents)
	if err != nil {
		return err
	}

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
