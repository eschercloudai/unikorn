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
// +kubebuilder:resource:categories=all;eschercloud
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="projectid",type="string",JSONPath=".spec.projectId"
// +kubebuilder:printcolumn:name="namespace",type="string",JSONPath=".status.namespace"
// +kubebuilder:printcolumn:name="status",type="string",JSONPath=".status.conditions[?(@.type==\"Provisioned\")].reason"
// +kubebuilder:printcolumn:name="age",type="date",JSONPath=".metadata.creationTimestamp"
type Project struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ProjectSpec   `json:"spec"`
	Status            ProjectStatus `json:"status,omitempty"`
}

// ProjectSpec defines project specific metadata.
type ProjectSpec struct {
	// ProjectID is the lobally unique project identifier. This is intended to be
	// managed by an external system.
	ProjectID string `json:"projectId"`
}

// ProjectStatus defines the status of the project.
type ProjectStatus struct {
	// Namespace defines the namespace a project resides in.
	Namespace string `json:"namespace,omitempty"`

	// Current service state of a project.
	Conditions []ProjectCondition `json:"conditions,omitempty"`
}

type ProjectConditionType string

const (
	ProjectConditionProvisioned ProjectConditionType = "Provisioned"
)

type ProjectCondition struct {
	// Type is the type of the condition.
	// +kubebuilder:validation:Enum=Provisioned
	Type ProjectConditionType `json:"type"`
	// Status is the status of the condition.
	// Can be True, False, Unknown.
	Status corev1.ConditionStatus `json:"status"`
	// Last time the condition transitioned from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`
	// Unique, one-word, CamelCase reason for the condition's last transition.
	Reason string `json:"reason"`
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
// +kubebuilder:resource:categories=all;eschercloud
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:subresource:status
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
// +kubebuilder:validation:Enum=Provisioning;Provisioned;Canceled;Timedout;Errored
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
// +kubebuilder:resource:categories=all;eschercloud
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="controlPlane",type="string",JSONPath=".spec.provisionerControlPlane"
// +kubebuilder:printcolumn:name="version",type="string",JSONPath=".spec.kubernetesVersion"
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
	// ProvisionerContolPlane is a reference to the ControlPlane object in this namespace
	// that will provision the cluster.
	ProvisionerControlPlane string `json:"provisionerControlPlane"`
	// Timeout is the maximum time to attempt to provision a cluster before aborting.
	// +kubebuilder:default="20m"
	Timeout *metav1.Duration `json:"timeout"`
	// KubernetesVersion is the Kubernetes version.
	KubernetesVersion *SemanticVersion `json:"kubernetesVersion"`
	// Openstack defines global Openstack related configuration.
	Openstack KubernetesClusterOpenstackSpec `json:"openstack"`
	// Network defines the Kubernetes networking.
	Network KubernetesClusterNetworkSpec `json:"network"`
	// ControlPlane defines the control plane topology.
	ControlPlane KubernetesClusterControlPlaneSpec `json:"controlPlane"`
	// Workload defines the workload cluster topology.
	Workload KubernetesClusterWorkloadSpec `json:"workload"`
	// ClusterAutoscaler, if specified, provisions a cluster autoscaler.
	ClusterAutoscaler *KubernetesClusterAutoscalerSpec `json:"clusterAutoscaler,omitempty"`
}

type KubernetesClusterOpenstackSpec struct {
	// CACert is the CA used to trust the Openstack endpoint.
	CACert *[]byte `json:"caCert"`
	// CloudConfig is a base64 encoded minimal clouds.yaml file for
	// use by the ControlPlane to provision the IaaS bits.
	CloudConfig *[]byte `json:"cloudConfig"`
	// CloudProviderConfig is a simple ini file that looks with a global
	// section and a auth-url key.
	CloudProviderConfig *[]byte `json:"cloudProviderConfig"`
	// Cloud is the clouds.yaml key that identifes the configuration
	// to use for provisioning.
	Cloud *string `json:"cloud"`
	// SSHKeyName is the SSH key name to use to provide access to the VMs.
	SSHKeyName *string `json:"sshKeyName"`
	// FailureDomain is the failure domain to use.
	FailureDomain *string `json:"failureDomain"`
	// Image is the Openstack image name to use for all nodes.
	Image *string `json:"image"`
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
	// ExternalNetworkID is the Openstack external network ID.
	ExternalNetworkID *string `json:"externalNetworkId"`
}

type KubernetesClusterControlPlaneSpec struct {
	// Replicas is the number desired replicas for the control plane.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=3
	Replicas *int `json:"replicas,omitempty"`
	// Flavor is the Openstack machine type to use for control
	// plane nodes.
	Flavor *string `json:"flavor"`
}

type KubernetesClusterWorkloadSpec struct {
	// Replicas is the number desired replicas for the workload nodes.
	// When enabled, this will be the minimum cluster size for the
	// cluster autoscaler.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=3
	Replicas *int `json:"replicas,omitempty"`
	// Flavor is the Openstack machine type to use for workload nodes.
	Flavor *string `json:"flavor"`
}

type KubernetesClusterAutoscalerSpec struct {
	// MaximumReplicas defines the largest a cluster should be scaled to.
	MaximumReplicas *int `json:"maximumReplicas"`
}

// KubernetesClusterStatus defines the observed state of the Kubernetes cluster.
type KubernetesClusterStatus struct {
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
// +kubebuilder:validation:Enum=Provisioning;Provisioned;Canceled;Timedout;Errored
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
