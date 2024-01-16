/*
Copyright 2022-2024 EscherCloud.

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

package v1alpha1

import (
	"encoding/json"
	"errors"
	"net"

	coreunikornv1 "github.com/eschercloudai/unikorn-core/pkg/apis/unikorn/v1alpha1"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/structured-merge-diff/v4/value"
)

var (
	ErrJSONUnmarshal = errors.New("failed to unmarshal JSON")
)

// +kubebuilder:validation:Pattern="^v(?:[0-9]+\\.){2}(?:[0-9]+)$"
type SemanticVersion string

// +kubebuilder:validation:Type=string
// +kubebuilder:validation:Pattern="^(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9]?[0-9])\\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9]?[0-9])$"
type IPv4Address struct {
	net.IP
}

// Ensure the type implements json.Unmarshaler.
var _ = json.Unmarshaler(&IPv4Address{})

func (a *IPv4Address) UnmarshalJSON(b []byte) error {
	var str string
	if err := json.Unmarshal(b, &str); err != nil {
		return err
	}

	ip := net.ParseIP(str)
	if ip == nil {
		return ErrJSONUnmarshal
	}

	a.IP = ip

	return nil
}

// Ensure the type implements value.UnstructuredConverter.
var _ = value.UnstructuredConverter(&IPv4Address{})

func (a IPv4Address) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.IP.String())
}

func (a IPv4Address) ToUnstructured() interface{} {
	return a.IP.String()
}

// There is no interface defined for these. See
// https://github.com/kubernetes/kube-openapi/tree/master/pkg/generators
// for reference.
func (IPv4Address) OpenAPISchemaType() []string {
	return []string{"string"}
}

func (IPv4Address) OpenAPISchemaFormat() string {
	return ""
}

// See https://regex101.com/r/QUfWrF/1
// +kubebuilder:validation:Type=string
// +kubebuilder:validation:Pattern="^(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9]?[0-9])\\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9]?[0-9])\\/(?:3[0-2]|[1-2]?[0-9])$"
type IPv4Prefix struct {
	net.IPNet
}

// DeepCopyInto implements the interface deepcopy-gen is totally unable to
// do by itself.
func (p *IPv4Prefix) DeepCopyInto(out *IPv4Prefix) {
	if p.IPNet.IP != nil {
		in, out := &p.IPNet.IP, &out.IPNet.IP
		*out = make(net.IP, len(*in))
		copy(*out, *in)
	}

	if p.IPNet.Mask != nil {
		in, out := &p.IPNet.Mask, &out.IPNet.Mask
		*out = make(net.IPMask, len(*in))
		copy(*out, *in)
	}
}

// Ensure the type implements json.Unmarshaler.
var _ = json.Unmarshaler(&IPv4Prefix{})

func (p *IPv4Prefix) UnmarshalJSON(b []byte) error {
	var str string
	if err := json.Unmarshal(b, &str); err != nil {
		return err
	}

	_, network, err := net.ParseCIDR(str)
	if err != nil {
		return ErrJSONUnmarshal
	}

	if network == nil {
		return ErrJSONUnmarshal
	}

	p.IPNet = *network

	return nil
}

// Ensure the type implements value.UnstructuredConverter.
var _ = value.UnstructuredConverter(&IPv4Prefix{})

func (p IPv4Prefix) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.IPNet.String())
}

func (p IPv4Prefix) ToUnstructured() interface{} {
	return p.IP.String()
}

// There is no interface defined for these. See
// https://github.com/kubernetes/kube-openapi/tree/master/pkg/generators
// for reference.
func (IPv4Prefix) OpenAPISchemaType() []string {
	return []string{"string"}
}

func (IPv4Prefix) OpenAPISchemaFormat() string {
	return ""
}

// ProjectList is a typed list of projects.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ProjectList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Project `json:"items"`
}

// Project is an abstraction around control planes that provides namespacing
// of ControlPlanes.
// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Cluster,categories=unikorn
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="namespace",type="string",JSONPath=".status.namespace"
// +kubebuilder:printcolumn:name="status",type="string",JSONPath=".status.conditions[?(@.type==\"Available\")].reason"
// +kubebuilder:printcolumn:name="age",type="date",JSONPath=".metadata.creationTimestamp"
type Project struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ProjectSpec   `json:"spec"`
	Status            ProjectStatus `json:"status,omitempty"`
}

// ProjectSpec defines project specific metadata.
type ProjectSpec struct {
	// Pause, if true, will inhibit reconciliation.
	Pause bool `json:"pause,omitempty"`
}

// ProjectStatus defines the status of the project.
type ProjectStatus struct {
	// Namespace defines the namespace a project resides in.
	Namespace string `json:"namespace,omitempty"`

	// Current service state of a project.
	Conditions []coreunikornv1.Condition `json:"conditions,omitempty"`
}

// ControlPlaneList is a typed list of control planes.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ControlPlaneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ControlPlane `json:"items"`
}

// ControlPlane is an abstraction around resource provisioning, for example
// it may contain a provider like Cluster API that can provision KubernetesCluster
// resources.
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Namespaced,categories=unikorn
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="bundle",type="string",JSONPath=".spec.applicationBundle"
// +kubebuilder:printcolumn:name="namespace",type="string",JSONPath=".status.namespace"
// +kubebuilder:printcolumn:name="status",type="string",JSONPath=".status.conditions[?(@.type==\"Available\")].reason"
// +kubebuilder:printcolumn:name="age",type="date",JSONPath=".metadata.creationTimestamp"
type ControlPlane struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ControlPlaneSpec   `json:"spec"`
	Status            ControlPlaneStatus `json:"status,omitempty"`
}

// ControlPlaneSpec defines any control plane specific options.
type ControlPlaneSpec struct {
	// Pause, if true, will inhibit reconciliation.
	Pause bool `json:"pause,omitempty"`
	// Timeout defines how long a control plane is allowed to provision for before
	// a timeout is triggerd and the request aborts.
	// +kubebuilder:default="10m"
	Timeout *metav1.Duration `json:"timeout,omitempty"`
	// ApplicationBundle defines the applications used to create the control plane.
	// Change this to a new bundle to start an upgrade.
	ApplicationBundle *string `json:"applicationBundle"`
	// ApplicationBundleAutoUpgrade enables automatic upgrade of application bundles.
	// When no properties are set in the specification, the platform will automatically
	// choose an upgrade time for your resource.  This will be before a working day
	// (Mon-Fri) and before working hours (00:00-07:00 UTC).  When any property is set
	// the platform will follow the rules for the upgrade method.
	ApplicationBundleAutoUpgrade *ApplicationBundleAutoUpgradeSpec `json:"applicationBundleAutoUpgrade,omitempty"`
}

// ControlPlaneStatus defines the status of the project.
type ControlPlaneStatus struct {
	// Namespace defines the namespace a control plane resides in.
	Namespace string `json:"namespace,omitempty"`

	// Current service state of a control plane.
	Conditions []coreunikornv1.Condition `json:"conditions,omitempty"`
}

// MachineGeneric contains common things across all pool types, including
// Kubernetes control plane nodes and workload pools.
type MachineGeneric struct {
	// Version is the Kubernetes version to install.  For performance
	// reasons this should match what is already pre-installed on the
	// provided image.
	Version *SemanticVersion `json:"version"`
	// Image is the OpenStack Glance image to deploy with.
	Image *string `json:"image"`
	// Flavor is the OpenStack Nova flavor to deploy with.
	Flavor *string `json:"flavor"`
	// DiskSize is the persistent root disk size to deploy with.  This
	// overrides the default ephemeral disk size defined in the flavor.
	DiskSize *resource.Quantity `json:"diskSize,omitempty"`
	// VolumeFailureDomain allows the volume failure domain to be set
	// on a per machine deployment basis.
	VolumeFailureDomain *string `json:"volumeFailureDomain,omitempty"`
	// Replicas is the initial pool size to deploy.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=3
	Replicas *int `json:"replicas,omitempty"`
	// ServerGroupID sets the server group of the control plane in
	// order to maintain anti-affinity rules.
	ServerGroupID *string `json:"serverGroupId,omitempty"`
}

// File is a file that can be deployed to a cluster node on creation.
type File struct {
	// Path is the absolute path to create the file in.
	Path *string `json:"path"`
	// Content is the file contents.
	Content []byte `json:"content"`
}

// MachineGenericAutoscaling defines generic autoscaling configuration.
// +kubebuilder:validation:XValidation:message="maximumReplicas must be greater than minimumReplicas",rule=(self.maximumReplicas > self.minimumReplicas)
type MachineGenericAutoscaling struct {
	// MinimumReplicas defines the minimum number of replicas that
	// this pool can be scaled down to.
	// +kubebuilder:validation:Minimum=0
	MinimumReplicas *int `json:"minimumReplicas"`
	// MaximumReplicas defines the maximum numer of replicas that
	// this pool can be scaled up to.
	// +kubebuilder:validation:Minimum=1
	MaximumReplicas *int `json:"maximumReplicas"`
	// Scheduler is required when scale-from-zero support is requested
	// i.e. MimumumReplicas is 0.  This provides scheduling hints to
	// the autoscaler as it cannot derive CPU/memory constraints from
	// the machine flavor.
	Scheduler *MachineGenericAutoscalingScheduler `json:"scheduler,omitempty"`
}

// MachineGenericAutoscalingScheduler defines generic autoscaling scheduling
// constraints.
type MachineGenericAutoscalingScheduler struct {
	// CPU defines the number of CPUs for the pool flavor.
	// +kubebuilder:validation:Minimum=1
	CPU *int `json:"cpu"`
	// Memory defines the amount of memory for the pool flavor.
	// Internally this will be rounded down to the nearest Gi.
	Memory *resource.Quantity `json:"memory"`
	// GPU needs to be set when the pool contains GPU resources so
	// the autoscaler can make informed choices when scaling up.
	GPU *MachineGenericAutoscalingSchedulerGPU `json:"gpu,omitempty"`
}

// MachineGenericAutoscalingSchedulerGPU defines generic autoscaling
// scheduling constraints for GPUs.
type MachineGenericAutoscalingSchedulerGPU struct {
	// Type is the type of GPU.
	// +kubebuilder:validation:Enum=nvidia.com/gpu
	Type *string `json:"type"`
	// Count is the number of GPUs for the pool flavor.
	// +kubebuilder:validation:Minimum=1
	Count *int `json:"count"`
}

// KubernetesWorkloadPoolSpec defines the requested machine pool
// state.
type KubernetesWorkloadPoolSpec struct {
	MachineGeneric `json:",inline"`
	// Name is the name of the pool.
	Name string `json:"name"`
	// FailureDomain is the failure domain to use for the pool.
	FailureDomain *string `json:"failureDomain,omitempty"`
	// Labels is the set of node labels to apply to the pool on
	// initialisation/join.
	Labels map[string]string `json:"labels,omitempty"`
	// Files are a set of files that can be installed onto the node
	// on initialisation/join.
	Files []File `json:"files,omitempty"`
	// Autoscaling contains optional sclaing limits and scheduling
	// hints for autoscaling.
	Autoscaling *MachineGenericAutoscaling `json:"autoscaling,omitempty"`
}

// KubernetesClusterList is a typed list of kubernetes clusters.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type KubernetesClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KubernetesCluster `json:"items"`
}

// KubernetesCluster is an object representing a Kubernetes cluster.
// For now, this is a monolith for simplicity.  In future it may reference
// a provider specific implementation e.g. if CAPI goes out of favour for
// some other new starlet.
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Namespaced,categories=unikorn
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="bundle",type="string",JSONPath=".spec.applicationBundle"
// +kubebuilder:printcolumn:name="version",type="string",JSONPath=".spec.controlPlane.version"
// +kubebuilder:printcolumn:name="image",type="string",JSONPath=".spec.controlPlane.image"
// +kubebuilder:printcolumn:name="flavor",type="string",JSONPath=".spec.controlPlane.flavor"
// +kubebuilder:printcolumn:name="replicas",type="string",JSONPath=".spec.controlPlane.replicas"
// +kubebuilder:printcolumn:name="status",type="string",JSONPath=".status.conditions[?(@.type==\"Available\")].reason"
// +kubebuilder:printcolumn:name="age",type="date",JSONPath=".metadata.creationTimestamp"
type KubernetesCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              KubernetesClusterSpec   `json:"spec"`
	Status            KubernetesClusterStatus `json:"status,omitempty"`
}

// KubernetesClusterSpec defines the requested state of the Kubernetes cluster.
type KubernetesClusterSpec struct {
	// Pause, if true, will inhibit reconciliation.
	Pause bool `json:"pause,omitempty"`
	// Timeout is the maximum time to attempt to provision a cluster before aborting.
	// +kubebuilder:default="20m"
	Timeout *metav1.Duration `json:"timeout"`
	// Openstack defines global Openstack related configuration.
	Openstack *KubernetesClusterOpenstackSpec `json:"openstack"`
	// Network defines the Kubernetes networking.
	Network *KubernetesClusterNetworkSpec `json:"network"`
	// API defines Kubernetes API specific options.
	API *KubernetesClusterAPISpec `json:"api,omitempty"`
	// ControlPlane defines the control plane topology.
	ControlPlane *KubernetesClusterControlPlaneSpec `json:"controlPlane"`
	// WorkloadPools defines the workload cluster topology.
	WorkloadPools *KubernetesClusterWorkloadPoolsSpec `json:"workloadPools"`
	// Features defines add-on features that can be enabled for the cluster.
	Features *KubernetesClusterFeaturesSpec `json:"features,omitempty"`
	// ApplicationBundle defines the applications used to create the cluster.
	// Change this to a new bundle to start an upgrade.
	ApplicationBundle *string `json:"applicationBundle"`
	// ApplicationBundleAutoUpgrade enables automatic upgrade of application bundles.
	// When no properties are set in the specification, the platform will automatically
	// choose an upgrade time for your resource.  This will be before a working day
	// (Mon-Fri) and before working hours (00:00-07:00 UTC).  When any property is set
	// the platform will follow the rules for the upgrade method.
	ApplicationBundleAutoUpgrade *ApplicationBundleAutoUpgradeSpec `json:"applicationBundleAutoUpgrade,omitempty"`
}

type KubernetesClusterOpenstackSpec struct {
	// CACert is the CA used to trust the Openstack endpoint.
	CACert *[]byte `json:"caCert,omitempty"`
	// CloudConfig is a base64 encoded minimal clouds.yaml file for
	// use by the ControlPlane to provision the IaaS bits.
	CloudConfig *[]byte `json:"cloudConfig"`
	// Cloud is the clouds.yaml key that identifes the configuration
	// to use for provisioning.
	Cloud *string `json:"cloud"`
	// SSHKeyName is the SSH key name to use to provide access to the VMs.
	SSHKeyName *string `json:"sshKeyName,omitempty"`
	// FailureDomain is the global failure domain to use.  The control plane
	// will always be deployed in this region.  Individual worload pools will
	// default to this, but can override it.
	FailureDomain *string `json:"failureDomain"`
	// VolumeFailureDomain is the default failure domain to use for volumes
	// as these needn't match compute.  For legacy reasons, this will default
	// to FailureDomain, but you shouldn't reply on this behaviour.
	VolumeFailureDomain *string `json:"volumeFailureDomain,omitempty"`
	// ExternalNetworkID is the Openstack external network ID.
	ExternalNetworkID *string `json:"externalNetworkId"`
}

type KubernetesClusterAPISpec struct {
	// SubjectAlternativeNames is a list of X.509 SANs to add to the API
	// certificate.
	SubjectAlternativeNames []string `json:"subjectAlternativeNames,omitempty"`
	// AllowedPrefixes is a list of all IPv4 prefixes that are allowed to access
	// the API.
	AllowedPrefixes []IPv4Prefix `json:"allowedPrefixes,omitempty"`
}

type KubernetesClusterNetworkSpec struct {
	// NodeNetwork is the IPv4 prefix for the node network.
	NodeNetwork *IPv4Prefix `json:"nodeNetwork"`
	// PodNetwork is the IPv4 prefix for the pod network.
	PodNetwork *IPv4Prefix `json:"podNetwork"`
	// ServiceNetwork is the IPv4 prefix for the service network.
	ServiceNetwork *IPv4Prefix `json:"serviceNetwork"`
	// DNSNameservers sets the DNS nameservers for pods.
	// At present due to some technical challenges, this must contain
	// only one DNS server.
	// +listType=set
	// +kubebuilder:validation:MinItems=1
	DNSNameservers []IPv4Address `json:"dnsNameservers"`
}

type KubernetesClusterFeaturesSpec struct {
	// Autoscaling, if true, provisions a cluster autoscaler
	// and allows workload pools to specify autoscaling configuration.
	Autoscaling *bool `json:"autoscaling,omitempty"`
	// Ingress, if true, provisions an Nginx ingress controller.
	Ingress *bool `json:"ingress,omitempty"`
	// CertManager, if true, provisions cert-manager.
	CertManager *bool `json:"certManager,omitempty"`
	// KubernetesDashboard, if true, provisions the kubernetes dashboard.
	// Clients must also enable the Ingress and CertManager features.
	KubernetesDashboard *bool `json:"kubernetesDashboard,omitempty"`
	// FileStorage, if true, enables a POSIX read/write many file storage.
	FileStorage *bool `json:"fileStorage,omitempty"`
	// Prometheus, if true, installs the Prometheus Operator.
	Prometheus *bool `json:"prometheus,omitempty"`
	// NvidiaOperator, if false do not install the Nvidia Operator, otherwise
	// install if GPU flavors are detected
	NvidiaOperator *bool `json:"nvidiaOperator,omitempty"`
}

type KubernetesClusterControlPlaneSpec struct {
	MachineGeneric `json:",inline"`
}

type KubernetesClusterWorkloadPoolsPoolSpec struct {
	KubernetesWorkloadPoolSpec `json:",inline"`
}

type KubernetesClusterWorkloadPoolsSpec struct {
	// Pools contains an inline set of pools.  This field will be ignored
	// when Selector is set.  Inline pools are expected to be used for UI
	// generated clusters.
	Pools []KubernetesClusterWorkloadPoolsPoolSpec `json:"pools,omitempty"`
}

// KubernetesClusterStatus defines the observed state of the Kubernetes cluster.
type KubernetesClusterStatus struct {
	// Namespace defines the namespace a cluster resides in.
	Namespace string `json:"namespace,omitempty"`

	// Current service state of a Kubernetes cluster.
	Conditions []coreunikornv1.Condition `json:"conditions,omitempty"`
}

// ControlPlaneApplicationBundleList defines a list of application bundles.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ControlPlaneApplicationBundleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ControlPlaneApplicationBundle `json:"items"`
}

// ControlPlaneApplicationBundle defines a bundle of applications related with a particular custom
// resource e.g. a ControlPlane has vcluster, cert-manager and cluster-api applications
// associated with it.  This forms the backbone of upgrades by allowing bundles to be
// switched out in control planes etc.
// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Cluster,categories=unikorn
// +kubebuilder:printcolumn:name="kind",type="string",JSONPath=".spec.kind"
// +kubebuilder:printcolumn:name="version",type="string",JSONPath=".spec.version"
// +kubebuilder:printcolumn:name="preview",type="string",JSONPath=".spec.preview"
// +kubebuilder:printcolumn:name="end of life",type="string",JSONPath=".spec.endOfLife"
// +kubebuilder:printcolumn:name="age",type="date",JSONPath=".metadata.creationTimestamp"
type ControlPlaneApplicationBundle struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ApplicationBundleSpec   `json:"spec"`
	Status            ApplicationBundleStatus `json:"status,omitempty"`
}

// KubernetesClusterApplicationBundleList defines a list of application bundles.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type KubernetesClusterApplicationBundleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KubernetesClusterApplicationBundle `json:"items"`
}

// KubernetesClusterApplicationBundle defines a bundle of applications related with a particular custom
// resource e.g. a ControlPlane has vcluster, cert-manager and cluster-api applications
// associated with it.  This forms the backbone of upgrades by allowing bundles to be
// switched out in control planes etc.
// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Cluster,categories=unikorn
// +kubebuilder:printcolumn:name="kind",type="string",JSONPath=".spec.kind"
// +kubebuilder:printcolumn:name="version",type="string",JSONPath=".spec.version"
// +kubebuilder:printcolumn:name="preview",type="string",JSONPath=".spec.preview"
// +kubebuilder:printcolumn:name="end of life",type="string",JSONPath=".spec.endOfLife"
// +kubebuilder:printcolumn:name="age",type="date",JSONPath=".metadata.creationTimestamp"
type KubernetesClusterApplicationBundle struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ApplicationBundleSpec   `json:"spec"`
	Status            ApplicationBundleStatus `json:"status,omitempty"`
}

// ApplicationBundleSpec defines the requested resource state.
type ApplicationBundleSpec struct {
	// Version is a semantic version of the bundle, must be unique.
	Version *string `json:"version"`
	// Preview indicates that this bundle is a preview and should not be
	// used by default.
	Preview *bool `json:"preview,omitempty"`
	// EndOfLife marks when this bundle should not be advertised any more
	// by Unikorn server.  It also provides a hint that users should upgrade
	// ahead of the deadline, or that a forced upgrade should be triggered.
	EndOfLife *metav1.Time `json:"endOfLife,omitempty"`
	// Applications is a list of application references for the bundle.
	Applications []ApplicationNamedReference `json:"applications,omitempty"`
}

type ApplicationNamedReference struct {
	// Name is the name of the application.  This must match what is encoded into
	// Unikorn's application management engine.
	Name *string `json:"name"`
	// Reference is a reference to the application definition.
	Reference *coreunikornv1.ApplicationReference `json:"reference"`
}

type ApplicationBundleStatus struct{}

type ApplicationBundleAutoUpgradeSpec struct {
	// WeekDay allows specification of upgrade time windows on individual
	// days of the week.  The platform will select a random  upgrade
	// slot within the specified time windows in order to load balance and
	// mitigate against defects.
	WeekDay *ApplicationBundleAutoUpgradeWeekDaySpec `json:"weekday,omitempty"`
}

type ApplicationBundleAutoUpgradeWeekDaySpec struct {
	// Sunday, when specified, provides an upgrade window on that day.
	Sunday *ApplicationBundleAutoUpgradeWindowSpec `json:"sunday,omitempty"`
	// Monday, when specified, provides an upgrade window on that day.
	Monday *ApplicationBundleAutoUpgradeWindowSpec `json:"monday,omitempty"`
	// Tuesday, when specified, provides an upgrade window on that day.
	Tuesday *ApplicationBundleAutoUpgradeWindowSpec `json:"tuesday,omitempty"`
	// Wednesday, when specified, provides an upgrade window on that day.
	Wednesday *ApplicationBundleAutoUpgradeWindowSpec `json:"wednesday,omitempty"`
	// Thursday, when specified, provides an upgrade window on that day.
	Thursday *ApplicationBundleAutoUpgradeWindowSpec `json:"thursday,omitempty"`
	// Friday, when specified, provides an upgrade window on that day.
	Friday *ApplicationBundleAutoUpgradeWindowSpec `json:"friday,omitempty"`
	// Saturday, when specified, provides an upgrade window on that day.
	Saturday *ApplicationBundleAutoUpgradeWindowSpec `json:"saturday,omitempty"`
}

type ApplicationBundleAutoUpgradeWindowSpec struct {
	// Start is the upgrade window start hour in UTC.  Upgrades will be
	// deterministically scheduled between start and end to balance load
	// across the platform.  Windows can span days, so start=22 and end=07
	// will start at 22:00 on the selected day, and end 07:00 the following
	// one.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=23
	Start int `json:"start"`
	// End is the upgrade window end hour in UTC.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=23
	End int `json:"end"`
}
