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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
// +kubebuilder:printcolumn:name="namespace",type="string",JSONPath=".status.namespace"
// +kubebuilder:printcolumn:name="status",type="string",JSONPath=".status.conditions[?(@.type=\"Provisioned\")].status"
// +kubebuilder:printcolumn:name="age",type="date",JSONPath=".metadata.creationTimestamp"
type Project struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ProjectSpec   `json:"spec"`
	Status            ProjectStatus `json:"status"`
}

// ProjectSpec defines project specific metadata.
type ProjectSpec struct {
	// ProjectID is the lobally unique project identifier. This is intended to be
	// managed by an external system.
	ProjectID string `json:"projectID"`
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
	Type ProjectConditionType `json:"type"`
	// Status is the status of the condition.
	// Can be True, False, Unknown.
	Status corev1.ConditionStatus `json:"status"`
	// Last time the condition transitioned from one status to another.
	LastTrasitionTime metav1.Time `json:"lastTransitionTime"`
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

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=all;eschercloud
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="age",type="date",JSONPath=".metadata.creationTimestamp"
type ControlPlane struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ControlPlaneSpec   `json:"spec"`
	Status            ControlPlaneStatus `json:"status"`
}

// ControlPlaneSpec defines any control plane specific options.
type ControlPlaneSpec struct {
}

// ControlPlaneStatus defines the status of the project.
type ControlPlaneStatus struct {
	// Current service state of a control plane.
	Conditions []ControlPlaneCondition `json:"conditions,omitempty"`
}

type ControlPlaneConditionType string

const (
	ControlPlaneConditionProvisioned ControlPlaneConditionType = "Provisioned"
)

type ControlPlaneCondition struct {
	// Type is the type of the condition.
	Type ProjectConditionType `json:"type"`
	// Status is the status of the condition.
	// Can be True, False, Unknown.
	Status corev1.ConditionStatus `json:"status"`
	// Last time the condition transitioned from one status to another.
	LastTrasitionTime metav1.Time `json:"lastTransitionTime"`
	// Unique, one-word, CamelCase reason for the condition's last transition.
	Reason string `json:"reason"`
	// Human-readable message indicating details about last transition.
	Message string `json:"message"`
}
