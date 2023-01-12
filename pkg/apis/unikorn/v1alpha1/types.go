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

package v1alpha1

import (
	"encoding/json"
	"errors"
	"net"

	corev1 "k8s.io/api/core/v1"
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
// +kubebuilder:resource:scope=Cluster,categories=all;unikorn
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
}

// ProjectStatus defines the status of the project.
type ProjectStatus struct {
	// Namespace defines the namespace a project resides in.
	Namespace string `json:"namespace,omitempty"`

	// Current service state of a project.
	Conditions []ProjectCondition `json:"conditions,omitempty"`
}

// +kubebuilder:validation:Enum=Available
type ProjectConditionType string

const (
	// ProjectConditionAvailable if not defined or false means that the
	// control plane is not ready, or is known to be in a bad state and should
	// not be used.  When true, while not guaranteed to be fully functional, it
	// will accept Kubernetes cluster creation requests that will be take care
	// of by eventual consistency.
	ProjectConditionAvailable ProjectConditionType = "Available"
)

// ProjectConditionReason defines the possible reasons of a control plane
// condition.  These are generic and may be used by any condition.
// +kubebuilder:validation:Enum=Provisioning;Provisioned;Canceled;Timedout;Errored;Deprovisioning
type ProjectConditionReason string

const (
	// ProjectConditionReasonProvisioning is used for the Available condition
	// to indicate that a resource has been seen, it has no pre-existing condition
	// and we assume it's being provisioned for the first time.
	ProjectConditionReasonProvisioning ProjectConditionReason = "Provisioning"
	// ProjectConditionReasonProvisioned is used for the Available condition
	// to mean that the control plane is ready to be used.
	ProjectConditionReasonProvisioned ProjectConditionReason = "Provisioned"
	// ProjectConditionReasonCanceled is used by a condition to
	// indicate the controller was cancelled e.g. via a container shutdown.
	ProjectConditionReasonCanceled ProjectConditionReason = "Canceled"
	// ProjectConditionReasonTimedout is used by a condition to
	// indicate the controller timed out e.g. OpenStack is slow or broken.
	ProjectConditionReasonTimedout ProjectConditionReason = "Timedout"
	// ProjectConditionReasonErrored is used by a condition to
	// indicate an unexpected error occurred e.g. Kubernetes API transient error.
	// If we see these, consider formulating a fix, for example a retry loop.
	ProjectConditionReasonErrored ProjectConditionReason = "Errored"
	// ProjectConditionReasonDeprovisioning is used by a condition to
	// indicate the controller has picked up a deprovision event.
	ProjectConditionReasonDeprovisioning ProjectConditionReason = "Deprovisioning"
)

type ProjectCondition struct {
	// Type is the type of the condition.
	Type ProjectConditionType `json:"type"`
	// Status is the status of the condition.
	// Can be True, False, Unknown.
	Status corev1.ConditionStatus `json:"status"`
	// Last time the condition transitioned from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`
	// Unique, one-word, CamelCase reason for the condition's last transition.
	Reason ProjectConditionReason `json:"reason"`
	// Human-readable message indicating details about last transition.
	Message string `json:"message"`
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
// +kubebuilder:resource:scope=Namespaced,categories=all;unikorn
// +kubebuilder:subresource:status
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
	// Timeout defines how long a control plane is allowed to provision for before
	// a timeout is triggerd and the request aborts.
	// +kubebuilder:default="10m"
	Timeout *metav1.Duration `json:"timeout,omitempty"`
}

// ControlPlaneStatus defines the status of the project.
type ControlPlaneStatus struct {
	// Namespace defines the namespace a control plane resides in.
	Namespace string `json:"namespace,omitempty"`

	// Current service state of a control plane.
	Conditions []ControlPlaneCondition `json:"conditions,omitempty"`
}

// ControlPlaneConditionType defines the possible conditions a control plane
// can have.
// +kubebuilder:validation:Enum=Available
type ControlPlaneConditionType string

const (
	// ControlPlaneConditionAvailable if not defined or false means that the
	// control plane is not ready, or is known to be in a bad state and should
	// not be used.  When true, while not guaranteed to be fully functional, it
	// will accept Kubernetes cluster creation requests that will be take care
	// of by eventual consistency.
	ControlPlaneConditionAvailable ControlPlaneConditionType = "Available"
)

// ControlPlaneConditionReason defines the possible reasons of a control plane
// condition.  These are generic and may be used by any condition.
// +kubebuilder:validation:Enum=Provisioning;Provisioned;Canceled;Timedout;Errored;Deprovisioning
type ControlPlaneConditionReason string

const (
	// ControlPlaneConditionReasonProvisioning is used for the Available condition
	// to indicate that a resource has been seen, it has no pre-existing condition
	// and we assume it's being provisioned for the first time.
	ControlPlaneConditionReasonProvisioning ControlPlaneConditionReason = "Provisioning"
	// ControlPlaneConditionReasonProvisioned is used for the Available condition
	// to mean that the control plane is ready to be used.
	ControlPlaneConditionReasonProvisioned ControlPlaneConditionReason = "Provisioned"
	// ControlPlaneConditionReasonCanceled is used by a condition to
	// indicate the controller was cancelled e.g. via a container shutdown.
	ControlPlaneConditionReasonCanceled ControlPlaneConditionReason = "Canceled"
	// ControlPlaneConditionReasonTimedout is used by a condition to
	// indicate the controller timed out e.g. OpenStack is slow or broken.
	ControlPlaneConditionReasonTimedout ControlPlaneConditionReason = "Timedout"
	// ControlPlaneConditionReasonErrored is used by a condition to
	// indicate an unexpected error occurred e.g. Kubernetes API transient error.
	// If we see these, consider formulating a fix, for example a retry loop.
	ControlPlaneConditionReasonErrored ControlPlaneConditionReason = "Errored"
	// ControlPlaneConditionReasonDeprovisioning is used by a condition to
	// indicate the controller has picked up a deprovision event.
	ControlPlaneConditionReasonDeprovisioning ControlPlaneConditionReason = "Deprovisioning"
)

type ControlPlaneCondition struct {
	// Type is the type of the condition.
	Type ControlPlaneConditionType `json:"type"`
	// Status is the status of the condition.
	// Can be True, False, Unknown.
	Status corev1.ConditionStatus `json:"status"`
	// Last time the condition transitioned from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`
	// Unique, one-word, CamelCase reason for the condition's last transition.
	Reason ControlPlaneConditionReason `json:"reason"`
	// Human-readable message indicating details about last transition.
	Message string `json:"message"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type KubernetesWorkloadPoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KubernetesWorkloadPool `json:"items"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Namespaced,categories=all;unikorn
// +kubebuilder:printcolumn:name="version",type="string",JSONPath=".spec.version"
// +kubebuilder:printcolumn:name="image",type="string",JSONPath=".spec.image"
// +kubebuilder:printcolumn:name="flavor",type="string",JSONPath=".spec.flavor"
// +kubebuilder:printcolumn:name="replicas",type="string",JSONPath=".spec.replicas"
type KubernetesWorkloadPool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              KubernetesWorkloadPoolSpec   `json:"spec"`
	Status            KubernetesWorkloadPoolStatus `json:"status,omitempty"`
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
	// Replicas is the initial pool size to deploy.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=3
	Replicas *int `json:"replicas,omitempty"`
}

// File is a file that can be deployed to a cluster node on creation.
type File struct {
	// Path is the absolute path to create the file in.
	Path *string `json:"path"`
	// Content is the file contents.
	Content []byte `json:"content"`
}

// MachineGenericAutoscaling defines generic autoscaling configuration.
type MachineGenericAutoscaling struct {
	// MimumumReplicas defines the minimum number of replicas that
	// this pool can be scaled down to.
	// +kubebuilder:validation:Minimum=0
	MimumumReplicas *int `json:"minimumReplicas"`
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
	// Name allows overriding the pool name.  Workload pool resources in the same
	// namespace need unique names, but may apply to different clusters which exist
	// in their own "namespace".
	Name *string `json:"name,omitempty"`
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

type KubernetesWorkloadPoolStatus struct {
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
// +kubebuilder:resource:scope=Namespaced,categories=all;unikorn
// +kubebuilder:subresource:status
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
	// EnableAutoscaling , if specified, provisions a cluster autoscaler
	// and allows workload pools to specify autoscaling configuration.
	EnableAutoscaling *bool `json:"enableAutoscaling,omitempty"`
}

type KubernetesClusterOpenstackSpec struct {
	// CACert is the CA used to trust the Openstack endpoint.
	CACert *[]byte `json:"caCert"`
	// CloudConfig is a base64 encoded minimal clouds.yaml file for
	// use by the ControlPlane to provision the IaaS bits.
	CloudConfig *[]byte `json:"cloudConfig"`
	// Cloud is the clouds.yaml key that identifes the configuration
	// to use for provisioning.
	Cloud *string `json:"cloud"`
	// SSHKeyName is the SSH key name to use to provide access to the VMs.
	SSHKeyName *string `json:"sshKeyName"`
	// Region is the region that the cluster is provisioned in.
	Region *string `json:"region"`
	// FailureDomain is the global failure domain to use.  The control plane
	// will always be deployed in this region.  Individual worload pools will
	// default to this, but can override it.
	FailureDomain *string `json:"failureDomain"`
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
	// +kubebuilder:validation:MaxItems=1
	DNSNameservers []IPv4Address `json:"dnsNameservers"`
}

type KubernetesClusterControlPlaneSpec struct {
	MachineGeneric `json:",inline"`
}

type KubernetesClusterWorkloadPoolsSpec struct {
	// Selector is a label selector to collect KubernetesClusterWorkloadPool
	// resources.  If not specified all KubernetesClusterWorkloadPool resources
	// will be considered a member of this cluster.
	Selector *metav1.LabelSelector `json:"selector"`
}

// KubernetesClusterStatus defines the observed state of the Kubernetes cluster.
type KubernetesClusterStatus struct {
	// Namespace defines the namespace a cluster resides in.
	Namespace string `json:"namespace,omitempty"`

	// Current service state of a Kubernetes cluster.
	Conditions []KubernetesClusterCondition `json:"conditions,omitempty"`
}

// KubernetesClusterConditionType defines the possible conditions a control plane
// can have.
// +kubebuilder:validation:Enum=Available
type KubernetesClusterConditionType string

const (
	// KubernetesClusterConditionAvailable if not defined or false means that the
	// cluster is not ready, or is known to be in a bad state and should
	// not be used.  When true, while not guaranteed to be fully functional, it
	// will accept API requests.
	KubernetesClusterConditionAvailable KubernetesClusterConditionType = "Available"
)

// KubernetesClusterConditionReason defines the possible reasons of a cluster
// condition.  These are generic and may be used by any condition.
// +kubebuilder:validation:Enum=Provisioning;Provisioned;Canceled;Timedout;Errored;Deprovisioning
type KubernetesClusterConditionReason string

const (
	// KubernetesClusterConditionReasonProvisioning is used for the Available condition
	// to indicate that a resource has been seen, it has no pre-existing condition
	// and we assume it's being provisioned for the first time.
	KubernetesClusterConditionReasonProvisioning KubernetesClusterConditionReason = "Provisioning"
	// KubernetesClusterConditionReasonProvisioned is used for the Available condition
	// to mean that the control plane is ready to be used.
	KubernetesClusterConditionReasonProvisioned KubernetesClusterConditionReason = "Provisioned"
	// KubernetesClusterConditionReasonCanceled is used by a condition to
	// indicate the controller was cancelled e.g. via a container shutdown.
	KubernetesClusterConditionReasonCanceled KubernetesClusterConditionReason = "Canceled"
	// KubernetesClusterConditionReasonTimedout is used by a condition to
	// indicate the controller timed out e.g. OpenStack is slow or broken.
	KubernetesClusterConditionReasonTimedout KubernetesClusterConditionReason = "Timedout"
	// KubernetesClusterConditionReasonErrored is used by a condition to
	// indicate an unexpected error occurred e.g. Kubernetes API transient error.
	// If we see these, consider formulating a fix, for example a retry loop.
	KubernetesClusterConditionReasonErrored KubernetesClusterConditionReason = "Errored"
	// KubernetesClusterConditionReasonDeprovisioning is used by a condition to
	// indicate the controller has picked up a deprovision event.
	KubernetesClusterConditionReasonDeprovisioning KubernetesClusterConditionReason = "Deprovisioning"
)

type KubernetesClusterCondition struct {
	// Type is the type of the condition.
	Type KubernetesClusterConditionType `json:"type"`
	// Status is the status of the condition.
	// Can be True, False, Unknown.
	Status corev1.ConditionStatus `json:"status"`
	// Last time the condition transitioned from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`
	// Unique, one-word, CamelCase reason for the condition's last transition.
	Reason KubernetesClusterConditionReason `json:"reason"`
	// Human-readable message indicating details about last transition.
	Message string `json:"message"`
}
