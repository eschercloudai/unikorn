// Package generated provides primitives to interact with the openapi HTTP API.
//
// Code generated by github.com/deepmap/oapi-codegen version v1.12.4 DO NOT EDIT.
package generated

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/deepmap/oapi-codegen/pkg/runtime"
)

const (
	Oauth2AuthenticationScopes = "oauth2Authentication.Scopes"
)

// Defines values for Oauth2ErrorError.
const (
	AccessDenied            Oauth2ErrorError = "access_denied"
	InvalidClient           Oauth2ErrorError = "invalid_client"
	InvalidGrant            Oauth2ErrorError = "invalid_grant"
	InvalidRequest          Oauth2ErrorError = "invalid_request"
	InvalidScope            Oauth2ErrorError = "invalid_scope"
	MethodNotAllowed        Oauth2ErrorError = "method_not_allowed"
	NotFound                Oauth2ErrorError = "not_found"
	ServerError             Oauth2ErrorError = "server_error"
	TemporarilyUnavailable  Oauth2ErrorError = "temporarily_unavailable"
	UnauthorizedClient      Oauth2ErrorError = "unauthorized_client"
	UnsupportedGrantType    Oauth2ErrorError = "unsupported_grant_type"
	UnsupportedMediaType    Oauth2ErrorError = "unsupported_media_type"
	UnsupportedResponseType Oauth2ErrorError = "unsupported_response_type"
)

// ApplicationBundle A bundle of applications.
type ApplicationBundle struct {
	// EndOfLife When the bundle is end-of-life.
	EndOfLife *time.Time `json:"endOfLife,omitempty"`

	// Name The resource name.
	Name string `json:"name"`

	// Preview Whether the bundle is in preview.
	Preview *bool `json:"preview,omitempty"`

	// Version The bundle version.
	Version string `json:"version"`
}

// ApplicationBundleAutoUpgrade When specified enables auto upgrade of application bundles.
type ApplicationBundleAutoUpgrade struct {
	// DaysOfWeek Days of the week and time windows that permit operations to be performed in.
	DaysOfWeek *AutoUpgradeDaysOfWeek `json:"daysOfWeek,omitempty"`
}

// ApplicationBundles A list of application bundles.
type ApplicationBundles = []ApplicationBundle

// ApplicationCredentialOptions Openstack application credential create options.
type ApplicationCredentialOptions struct {
	// Name Application credential name.
	Name string `json:"name"`
}

// AutoUpgradeDaysOfWeek Days of the week and time windows that permit operations to be performed in.
type AutoUpgradeDaysOfWeek struct {
	// Friday A time window that wraps into the next day if required.
	Friday *TimeWindow `json:"friday,omitempty"`

	// Monday A time window that wraps into the next day if required.
	Monday *TimeWindow `json:"monday,omitempty"`

	// Saturday A time window that wraps into the next day if required.
	Saturday *TimeWindow `json:"saturday,omitempty"`

	// Sunday A time window that wraps into the next day if required.
	Sunday *TimeWindow `json:"sunday,omitempty"`

	// Thursday A time window that wraps into the next day if required.
	Thursday *TimeWindow `json:"thursday,omitempty"`

	// Tuesday A time window that wraps into the next day if required.
	Tuesday *TimeWindow `json:"tuesday,omitempty"`

	// Wednesday A time window that wraps into the next day if required.
	Wednesday *TimeWindow `json:"wednesday,omitempty"`
}

// ControlPlane A Unikorn control plane.
type ControlPlane struct {
	// ApplicationBundle A bundle of applications.
	ApplicationBundle ApplicationBundle `json:"applicationBundle"`

	// ApplicationBundleAutoUpgrade When specified enables auto upgrade of application bundles.
	ApplicationBundleAutoUpgrade *ApplicationBundleAutoUpgrade `json:"applicationBundleAutoUpgrade,omitempty"`

	// Name The name of the resource.
	Name string `json:"name"`

	// Status A Kubernetes resource status.
	Status *KubernetesResourceStatus `json:"status,omitempty"`
}

// ControlPlanes A list of Unikorn control planes.
type ControlPlanes = []ControlPlane

// Hour An hour of the day in UTC.
type Hour = int

// JsonWebKeySet JSON web key set.
type JsonWebKeySet = []map[string]interface{}

// KubernetesCluster Unikorn Kubernetes cluster creation parameters.
type KubernetesCluster struct {
	// Api Kubernetes API settings.
	Api *KubernetesClusterAPI `json:"api,omitempty"`

	// ApplicationBundle A bundle of applications.
	ApplicationBundle ApplicationBundle `json:"applicationBundle"`

	// ApplicationBundleAutoUpgrade When specified enables auto upgrade of application bundles.
	ApplicationBundleAutoUpgrade *ApplicationBundleAutoUpgrade `json:"applicationBundleAutoUpgrade,omitempty"`

	// ControlPlane A Kubernetes cluster machine.
	ControlPlane OpenstackMachinePool `json:"controlPlane"`

	// Features A set of optional add on features for the cluster.
	Features *KubernetesClusterFeatures `json:"features,omitempty"`

	// Name Cluster name.
	Name string `json:"name"`

	// Network A kubernetes cluster network settings.
	Network KubernetesClusterNetwork `json:"network"`

	// Openstack Unikorn Kubernetes cluster creation Openstack parameters.
	Openstack KubernetesClusterOpenstack `json:"openstack"`

	// Status A Kubernetes resource status.
	Status *KubernetesResourceStatus `json:"status,omitempty"`

	// WorkloadPools A non-empty list of Kubernetes cluster workload pools.
	WorkloadPools KubernetesClusterWorkloadPools `json:"workloadPools"`
}

// KubernetesClusterAPI Kubernetes API settings.
type KubernetesClusterAPI struct {
	// AllowedPrefixes Set of address prefixes to allow access to the Kubernetes API.
	AllowedPrefixes *[]string `json:"allowedPrefixes,omitempty"`

	// SubjectAlternativeNames Set of non-standard X.509 SANs to add to the API certificate.
	SubjectAlternativeNames *[]string `json:"subjectAlternativeNames,omitempty"`
}

// KubernetesClusterAutoscaling A Kubernetes cluster workload pool autoscaling configuration.
type KubernetesClusterAutoscaling struct {
	// MaximumReplicas The maximum number of replicas to allow.
	MaximumReplicas int `json:"maximumReplicas"`

	// MinimumReplicas The minimum number of replicas to allow.
	MinimumReplicas int `json:"minimumReplicas"`
}

// KubernetesClusterFeatures A set of optional add on features for the cluster.
type KubernetesClusterFeatures struct {
	// Autoscaling Enable auto-scaling.
	Autoscaling *bool `json:"autoscaling,omitempty"`

	// CertManager Enable cert-manager.
	CertManager *bool `json:"certManager,omitempty"`

	// Ingress Enable an ingress controller.
	Ingress *bool `json:"ingress,omitempty"`

	// KubernetesDashboard Enable the Kubernetes dashboard.  Requires ingress and certManager to be enabled.
	KubernetesDashboard *bool `json:"kubernetesDashboard,omitempty"`
}

// KubernetesClusterNetwork A kubernetes cluster network settings.
type KubernetesClusterNetwork struct {
	// DnsNameservers A list of DNS name server to use.
	DnsNameservers []string `json:"dnsNameservers"`

	// NodePrefix Network prefix to provision nodes in.
	NodePrefix string `json:"nodePrefix"`

	// PodPrefix Network prefix to provision pods in.
	PodPrefix string `json:"podPrefix"`

	// ServicePrefix Network prefix to provision services in.
	ServicePrefix string `json:"servicePrefix"`
}

// KubernetesClusterOpenstack Unikorn Kubernetes cluster creation Openstack parameters.
type KubernetesClusterOpenstack struct {
	// ApplicationCredentialID Application credential ID.
	ApplicationCredentialID string `json:"applicationCredentialID"`

	// ApplicationCredentialSecret Application credential secret.
	ApplicationCredentialSecret string `json:"applicationCredentialSecret"`

	// ComputeAvailabilityZone Compute availability zone for control plane, and workload pool default.
	ComputeAvailabilityZone string `json:"computeAvailabilityZone"`

	// ExternalNetworkID Openstack external network ID.
	ExternalNetworkID string `json:"externalNetworkID"`

	// SshKeyName Openstack SSH Key to install on all machines.
	SshKeyName *string `json:"sshKeyName,omitempty"`

	// VolumeAvailabilityZone Volume availability zone for control plane, and workload pool default.
	VolumeAvailabilityZone string `json:"volumeAvailabilityZone"`
}

// KubernetesClusterWorkloadPool A Kuberntes cluster workload pool.
type KubernetesClusterWorkloadPool struct {
	// Autoscaling A Kubernetes cluster workload pool autoscaling configuration.
	Autoscaling *KubernetesClusterAutoscaling `json:"autoscaling,omitempty"`

	// AvailabilityZone Workload pool availability zone.
	AvailabilityZone *string `json:"availabilityZone,omitempty"`

	// Labels Workload pool labels to apply on node creation.
	Labels *map[string]string `json:"labels,omitempty"`

	// Machine A Kubernetes cluster machine.
	Machine OpenstackMachinePool `json:"machine"`

	// Name Workload pool name.
	Name string `json:"name"`
}

// KubernetesClusterWorkloadPools A non-empty list of Kubernetes cluster workload pools.
type KubernetesClusterWorkloadPools = []KubernetesClusterWorkloadPool

// KubernetesClusters A list of Unikorn Kubernetes clusters.
type KubernetesClusters = []KubernetesCluster

// KubernetesResourceStatus A Kubernetes resource status.
type KubernetesResourceStatus struct {
	// CreationTime The time the resource was created.
	CreationTime time.Time `json:"creationTime"`

	// DeletionTime The time a control plane was deleted.
	DeletionTime *time.Time `json:"deletionTime,omitempty"`

	// Name The name of the resource.
	Name string `json:"name"`

	// Status The current status of the resource.
	Status string `json:"status"`
}

// Oauth2Error Generic error message.
type Oauth2Error struct {
	// Error A terse error string expaning on the HTTP error code.
	Error Oauth2ErrorError `json:"error"`

	// ErrorDescription Verbose message describing the error.
	ErrorDescription string `json:"error_description"`
}

// Oauth2ErrorError A terse error string expaning on the HTTP error code.
type Oauth2ErrorError string

// OpenstackApplicationCredential An Openstack application credential.
type OpenstackApplicationCredential struct {
	// Id Application credential ID.
	Id string `json:"id"`

	// Name Application credential name.
	Name string `json:"name"`

	// Secret Application credential secret, this is only present on creation.
	Secret *string `json:"secret,omitempty"`
}

// OpenstackAvailabilityZone An Openstack availability zone.
type OpenstackAvailabilityZone struct {
	// Name The availability zone name.
	Name string `json:"name"`
}

// OpenstackAvailabilityZones A list of Openstack availability zones.
type OpenstackAvailabilityZones = []OpenstackAvailabilityZone

// OpenstackExternalNetwork An Openstack external network.
type OpenstackExternalNetwork struct {
	// Id Openstack external network ID.
	Id string `json:"id"`

	// Name Opestack external network name.
	Name string `json:"name"`
}

// OpenstackExternalNetworks A list of Openstack external networks.
type OpenstackExternalNetworks = []OpenstackExternalNetwork

// OpenstackFlavor An Openstack flavor.
type OpenstackFlavor struct {
	// Cpus The number of CPUs.
	Cpus int `json:"cpus"`

	// Gpus The number of GPUs, if not set there are none.
	Gpus *int `json:"gpus,omitempty"`

	// Id The unique flavor ID.
	Id string `json:"id"`

	// Memory The amount of memory in GiB.
	Memory int `json:"memory"`

	// Name The flavor name.
	Name string `json:"name"`
}

// OpenstackFlavors A list of Openstack flavors.
type OpenstackFlavors = []OpenstackFlavor

// OpenstackImage And Openstack image.
type OpenstackImage struct {
	// Created Time when the image was created.
	Created time.Time `json:"created"`

	// Id The unique image ID.
	Id string `json:"id"`

	// Modified Time when the image was last modified.
	Modified time.Time `json:"modified"`

	// Name The image name.
	Name string `json:"name"`

	// Versions Image version metadata.
	Versions struct {
		// Kubernetes The kubernetes semantic version.
		Kubernetes string `json:"kubernetes"`

		// NvidiaDriver The nvidia driver version.
		NvidiaDriver string `json:"nvidiaDriver"`
	} `json:"versions"`
}

// OpenstackImages A list of Openstack images that are compatible with this platform.
type OpenstackImages = []OpenstackImage

// OpenstackKeyPair An Openstack key pair.
type OpenstackKeyPair struct {
	// Name The key pair name.
	Name string `json:"name"`
}

// OpenstackKeyPairs A list of Openstack key pairs.
type OpenstackKeyPairs = []OpenstackKeyPair

// OpenstackMachinePool A Kubernetes cluster machine.
type OpenstackMachinePool struct {
	// Disk An Openstack volume.
	Disk *OpenstackVolume `json:"disk,omitempty"`

	// FlavorName Openstack flavor name.
	FlavorName string `json:"flavorName"`

	// ImageName Openstack image name.
	ImageName string `json:"imageName"`

	// Replicas Number of machines.
	Replicas int `json:"replicas"`

	// Version Kubernetes version.
	Version string `json:"version"`
}

// OpenstackProject An Openstack project.
type OpenstackProject struct {
	// Description A verbose description of the project.
	Description *string `json:"description,omitempty"`

	// Id Globally unique project ID.
	Id string `json:"id"`

	// Name The name of the project within the scope of the domain.
	Name string `json:"name"`
}

// OpenstackProjects A list of Openstack projects.
type OpenstackProjects = []OpenstackProject

// OpenstackVolume An Openstack volume.
type OpenstackVolume struct {
	// AvailabilityZone Volume availability zone.
	AvailabilityZone *string `json:"availabilityZone,omitempty"`

	// Size Disk size in GiB.
	Size int `json:"size"`
}

// Project A Unikorn project.
type Project struct {
	// Status A Kubernetes resource status.
	Status *KubernetesResourceStatus `json:"status,omitempty"`
}

// StringParameter A basic string parameter.
type StringParameter = string

// TimeWindow A time window that wraps into the next day if required.
type TimeWindow struct {
	// End An hour of the day in UTC.
	End Hour `json:"end"`

	// Start An hour of the day in UTC.
	Start Hour `json:"start"`
}

// Token Oauth2 token result.
type Token struct {
	// AccessToken The opaque access token.
	AccessToken string `json:"access_token"`

	// ExpiresIn The time in seconds the token will last for.
	ExpiresIn int `json:"expires_in"`

	// IdToken An OIDC ID token.
	IdToken *string `json:"id_token,omitempty"`

	// TokenType How the access token is to be presented to the resource server.
	TokenType string `json:"token_type"`
}

// TokenRequestOptions oauth2 token endpoint.
type TokenRequestOptions struct {
	// ClientId Client ID.
	ClientId *string `json:"client_id"`

	// Code Authorization code.
	Code *string `json:"code"`

	// CodeVerifier Client code verifier.
	CodeVerifier *string `json:"code_verifier"`

	// GrantType Supported grant type.
	GrantType string `json:"grant_type"`

	// Password Resource owner password.
	Password *string `json:"password"`

	// RedirectUri Client redirect URI.
	RedirectUri *string `json:"redirect_uri"`

	// Username Resource owner username.
	Username *string `json:"username"`
	union    json.RawMessage
}

// TokenRequestOptions0 defines model for .
type TokenRequestOptions0 struct {
	GrantType *interface{} `json:"grant_type,omitempty"`
}

// TokenRequestOptions1 defines model for .
type TokenRequestOptions1 struct {
	GrantType *interface{} `json:"grant_type,omitempty"`
}

// TokenScope Openstack token scope.
type TokenScope struct {
	// Project Openstack token project scope.
	Project TokenScopeProject `json:"project"`
}

// TokenScopeProject Openstack token project scope.
type TokenScopeProject struct {
	// Id Openstack project ID.
	Id string `json:"id"`
}

// ApplicationCredentialParameter A basic string parameter.
type ApplicationCredentialParameter = StringParameter

// ClusterNameParameter A basic string parameter.
type ClusterNameParameter = StringParameter

// ControlPlaneNameParameter A basic string parameter.
type ControlPlaneNameParameter = StringParameter

// ApplicationBundleResponse A list of application bundles.
type ApplicationBundleResponse = ApplicationBundles

// BadRequestResponse Generic error message.
type BadRequestResponse = Oauth2Error

// ControlPlaneResponse A Unikorn control plane.
type ControlPlaneResponse = ControlPlane

// ControlPlanesResponse A list of Unikorn control planes.
type ControlPlanesResponse = ControlPlanes

// ForbiddenResponse Generic error message.
type ForbiddenResponse = Oauth2Error

// InternalServerErrorResponse Generic error message.
type InternalServerErrorResponse = Oauth2Error

// JwksResponse JSON web key set.
type JwksResponse = JsonWebKeySet

// KubernetesClusterResponse Unikorn Kubernetes cluster creation parameters.
type KubernetesClusterResponse = KubernetesCluster

// KubernetesClustersResponse A list of Unikorn Kubernetes clusters.
type KubernetesClustersResponse = KubernetesClusters

// NullResponse defines model for nullResponse.
type NullResponse = map[string]interface{}

// OpenstackApplicationCredentialResponse An Openstack application credential.
type OpenstackApplicationCredentialResponse = OpenstackApplicationCredential

// OpenstackBlockStorageAvailabilityZonesResponse A list of Openstack availability zones.
type OpenstackBlockStorageAvailabilityZonesResponse = OpenstackAvailabilityZones

// OpenstackComputeAvailabilityZonesResponse A list of Openstack availability zones.
type OpenstackComputeAvailabilityZonesResponse = OpenstackAvailabilityZones

// OpenstackExternalNetworksResponse A list of Openstack external networks.
type OpenstackExternalNetworksResponse = OpenstackExternalNetworks

// OpenstackFlavorsResponse A list of Openstack flavors.
type OpenstackFlavorsResponse = OpenstackFlavors

// OpenstackImagesResponse A list of Openstack images that are compatible with this platform.
type OpenstackImagesResponse = OpenstackImages

// OpenstackKeyPairsResponse A list of Openstack key pairs.
type OpenstackKeyPairsResponse = OpenstackKeyPairs

// OpenstackProjectsResponse A list of Openstack projects.
type OpenstackProjectsResponse = OpenstackProjects

// ProjectResponse A Unikorn project.
type ProjectResponse = Project

// TokenResponse Oauth2 token result.
type TokenResponse = Token

// UnauthorizedResponse Generic error message.
type UnauthorizedResponse = Oauth2Error

// ApplicationCredentialRequest Openstack application credential create options.
type ApplicationCredentialRequest = ApplicationCredentialOptions

// CreateControlPlaneRequest A Unikorn control plane.
type CreateControlPlaneRequest = ControlPlane

// CreateKubernetesClusterRequest Unikorn Kubernetes cluster creation parameters.
type CreateKubernetesClusterRequest = KubernetesCluster

// TokenScopeRequest Openstack token scope.
type TokenScopeRequest = TokenScope

// PostApiV1AuthOauth2TokensFormdataRequestBody defines body for PostApiV1AuthOauth2Tokens for application/x-www-form-urlencoded ContentType.
type PostApiV1AuthOauth2TokensFormdataRequestBody = TokenRequestOptions

// PostApiV1AuthTokensTokenJSONRequestBody defines body for PostApiV1AuthTokensToken for application/json ContentType.
type PostApiV1AuthTokensTokenJSONRequestBody = TokenScope

// PostApiV1ControlplanesJSONRequestBody defines body for PostApiV1Controlplanes for application/json ContentType.
type PostApiV1ControlplanesJSONRequestBody = ControlPlane

// PutApiV1ControlplanesControlPlaneNameJSONRequestBody defines body for PutApiV1ControlplanesControlPlaneName for application/json ContentType.
type PutApiV1ControlplanesControlPlaneNameJSONRequestBody = ControlPlane

// PostApiV1ControlplanesControlPlaneNameClustersJSONRequestBody defines body for PostApiV1ControlplanesControlPlaneNameClusters for application/json ContentType.
type PostApiV1ControlplanesControlPlaneNameClustersJSONRequestBody = KubernetesCluster

// PutApiV1ControlplanesControlPlaneNameClustersClusterNameJSONRequestBody defines body for PutApiV1ControlplanesControlPlaneNameClustersClusterName for application/json ContentType.
type PutApiV1ControlplanesControlPlaneNameClustersClusterNameJSONRequestBody = KubernetesCluster

// PostApiV1ProvidersOpenstackApplicationCredentialsJSONRequestBody defines body for PostApiV1ProvidersOpenstackApplicationCredentials for application/json ContentType.
type PostApiV1ProvidersOpenstackApplicationCredentialsJSONRequestBody = ApplicationCredentialOptions

// AsTokenRequestOptions0 returns the union data inside the TokenRequestOptions as a TokenRequestOptions0
func (t TokenRequestOptions) AsTokenRequestOptions0() (TokenRequestOptions0, error) {
	var body TokenRequestOptions0
	err := json.Unmarshal(t.union, &body)
	return body, err
}

// FromTokenRequestOptions0 overwrites any union data inside the TokenRequestOptions as the provided TokenRequestOptions0
func (t *TokenRequestOptions) FromTokenRequestOptions0(v TokenRequestOptions0) error {
	b, err := json.Marshal(v)
	t.union = b
	return err
}

// MergeTokenRequestOptions0 performs a merge with any union data inside the TokenRequestOptions, using the provided TokenRequestOptions0
func (t *TokenRequestOptions) MergeTokenRequestOptions0(v TokenRequestOptions0) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}

	merged, err := runtime.JsonMerge(b, t.union)
	t.union = merged
	return err
}

// AsTokenRequestOptions1 returns the union data inside the TokenRequestOptions as a TokenRequestOptions1
func (t TokenRequestOptions) AsTokenRequestOptions1() (TokenRequestOptions1, error) {
	var body TokenRequestOptions1
	err := json.Unmarshal(t.union, &body)
	return body, err
}

// FromTokenRequestOptions1 overwrites any union data inside the TokenRequestOptions as the provided TokenRequestOptions1
func (t *TokenRequestOptions) FromTokenRequestOptions1(v TokenRequestOptions1) error {
	b, err := json.Marshal(v)
	t.union = b
	return err
}

// MergeTokenRequestOptions1 performs a merge with any union data inside the TokenRequestOptions, using the provided TokenRequestOptions1
func (t *TokenRequestOptions) MergeTokenRequestOptions1(v TokenRequestOptions1) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}

	merged, err := runtime.JsonMerge(b, t.union)
	t.union = merged
	return err
}

func (t TokenRequestOptions) MarshalJSON() ([]byte, error) {
	b, err := t.union.MarshalJSON()
	if err != nil {
		return nil, err
	}
	object := make(map[string]json.RawMessage)
	if t.union != nil {
		err = json.Unmarshal(b, &object)
		if err != nil {
			return nil, err
		}
	}

	if t.ClientId != nil {
		object["client_id"], err = json.Marshal(t.ClientId)
		if err != nil {
			return nil, fmt.Errorf("error marshaling 'client_id': %w", err)
		}
	}

	if t.Code != nil {
		object["code"], err = json.Marshal(t.Code)
		if err != nil {
			return nil, fmt.Errorf("error marshaling 'code': %w", err)
		}
	}

	if t.CodeVerifier != nil {
		object["code_verifier"], err = json.Marshal(t.CodeVerifier)
		if err != nil {
			return nil, fmt.Errorf("error marshaling 'code_verifier': %w", err)
		}
	}

	object["grant_type"], err = json.Marshal(t.GrantType)
	if err != nil {
		return nil, fmt.Errorf("error marshaling 'grant_type': %w", err)
	}

	if t.Password != nil {
		object["password"], err = json.Marshal(t.Password)
		if err != nil {
			return nil, fmt.Errorf("error marshaling 'password': %w", err)
		}
	}

	if t.RedirectUri != nil {
		object["redirect_uri"], err = json.Marshal(t.RedirectUri)
		if err != nil {
			return nil, fmt.Errorf("error marshaling 'redirect_uri': %w", err)
		}
	}

	if t.Username != nil {
		object["username"], err = json.Marshal(t.Username)
		if err != nil {
			return nil, fmt.Errorf("error marshaling 'username': %w", err)
		}
	}
	b, err = json.Marshal(object)
	return b, err
}

func (t *TokenRequestOptions) UnmarshalJSON(b []byte) error {
	err := t.union.UnmarshalJSON(b)
	if err != nil {
		return err
	}
	object := make(map[string]json.RawMessage)
	err = json.Unmarshal(b, &object)
	if err != nil {
		return err
	}

	if raw, found := object["client_id"]; found {
		err = json.Unmarshal(raw, &t.ClientId)
		if err != nil {
			return fmt.Errorf("error reading 'client_id': %w", err)
		}
	}

	if raw, found := object["code"]; found {
		err = json.Unmarshal(raw, &t.Code)
		if err != nil {
			return fmt.Errorf("error reading 'code': %w", err)
		}
	}

	if raw, found := object["code_verifier"]; found {
		err = json.Unmarshal(raw, &t.CodeVerifier)
		if err != nil {
			return fmt.Errorf("error reading 'code_verifier': %w", err)
		}
	}

	if raw, found := object["grant_type"]; found {
		err = json.Unmarshal(raw, &t.GrantType)
		if err != nil {
			return fmt.Errorf("error reading 'grant_type': %w", err)
		}
	}

	if raw, found := object["password"]; found {
		err = json.Unmarshal(raw, &t.Password)
		if err != nil {
			return fmt.Errorf("error reading 'password': %w", err)
		}
	}

	if raw, found := object["redirect_uri"]; found {
		err = json.Unmarshal(raw, &t.RedirectUri)
		if err != nil {
			return fmt.Errorf("error reading 'redirect_uri': %w", err)
		}
	}

	if raw, found := object["username"]; found {
		err = json.Unmarshal(raw, &t.Username)
		if err != nil {
			return fmt.Errorf("error reading 'username': %w", err)
		}
	}

	return err
}
