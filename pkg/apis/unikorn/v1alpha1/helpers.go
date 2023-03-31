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

package v1alpha1

import (
	"errors"
	"net"
	"sort"
	"strings"

	"github.com/eschercloudai/unikorn/pkg/constants"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

var (
	// ErrStatusConditionLookup is raised when a condition is not found in
	// the resource status.
	ErrStatusConditionLookup = errors.New("status condition not found")

	// ErrMissingLabel is raised when an expected label is not present on
	// a resource.
	ErrMissingLabel = errors.New("expected label is missing")
)

// IPv4AddressSliceFromIPSlice is a simple converter from Go types
// to API types.
func IPv4AddressSliceFromIPSlice(in []net.IP) []IPv4Address {
	out := make([]IPv4Address, len(in))

	for i, ip := range in {
		out[i] = IPv4Address{IP: ip}
	}

	return out
}

// LookupCondition scans the status conditions for an existing condition whose type
// matches.  Returns the array index, or -1 if it doesn't exist.
func (c *Project) LookupCondition(t ProjectConditionType) (*ProjectCondition, error) {
	for i, condition := range c.Status.Conditions {
		if condition.Type == t {
			return &c.Status.Conditions[i], nil
		}
	}

	return nil, ErrStatusConditionLookup
}

// UpdateCondition either adds or updates a condition in the control plane status.
// If the condition, status and message match an existing condition the update is
// ignored.  Returns true if a modification has been made.
func (c *Project) UpdateCondition(t ProjectConditionType, status corev1.ConditionStatus, reason ProjectConditionReason, message string) bool {
	condition := ProjectCondition{
		Type:               t,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}

	existingPtr, err := c.LookupCondition(t)
	if err != nil {
		c.Status.Conditions = append(c.Status.Conditions, condition)

		return true
	}

	// Do a shallow copy and set the same time, then do a shallow equality to
	// see if we need an update.
	existing := *existingPtr
	existing.LastTransitionTime = condition.LastTransitionTime

	if existing != condition {
		*existingPtr = condition

		return true
	}

	return false
}

// UpdateAvailableCondition updates the Available condition specifically.
func (c *Project) UpdateAvailableCondition(status corev1.ConditionStatus, reason ProjectConditionReason, message string) bool {
	return c.UpdateCondition(ProjectConditionAvailable, status, reason, message)
}

// ResourceLabels generates a set of labels to uniquely identify the resource
// if it were to be placed in a single global namespace.
func (c *Project) ResourceLabels() (labels.Set, error) {
	labels := labels.Set{
		constants.ProjectLabel: c.Name,
	}

	return labels, nil
}

// LookupCondition scans the status conditions for an existing condition whose type
// matches.  Returns the array index, or -1 if it doesn't exist.
func (c *ControlPlane) LookupCondition(t ControlPlaneConditionType) (*ControlPlaneCondition, error) {
	for i, condition := range c.Status.Conditions {
		if condition.Type == t {
			return &c.Status.Conditions[i], nil
		}
	}

	return nil, ErrStatusConditionLookup
}

// UpdateCondition either adds or updates a condition in the control plane status.
// If the condition, status and message match an existing condition the update is
// ignored.  Returns true if a modification has been made.
func (c *ControlPlane) UpdateCondition(t ControlPlaneConditionType, status corev1.ConditionStatus, reason ControlPlaneConditionReason, message string) bool {
	condition := ControlPlaneCondition{
		Type:               t,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}

	existingPtr, err := c.LookupCondition(t)
	if err != nil {
		c.Status.Conditions = append(c.Status.Conditions, condition)

		return true
	}

	// Do a shallow copy and set the same time, then do a shallow equality to
	// see if we need an update.
	existing := *existingPtr
	existing.LastTransitionTime = condition.LastTransitionTime

	if existing != condition {
		*existingPtr = condition

		return true
	}

	return false
}

// UpdateAvailableCondition updates the Available condition specifically.
func (c *ControlPlane) UpdateAvailableCondition(status corev1.ConditionStatus, reason ControlPlaneConditionReason, message string) bool {
	return c.UpdateCondition(ControlPlaneConditionAvailable, status, reason, message)
}

// ResourceLabels generates a set of labels to uniquely identify the resource
// if it were to be placed in a single global namespace.
func (c *ControlPlane) ResourceLabels() (labels.Set, error) {
	project, ok := c.Labels[constants.ProjectLabel]
	if !ok {
		return nil, ErrMissingLabel
	}

	labels := labels.Set{
		constants.ProjectLabel:      project,
		constants.ControlPlaneLabel: c.Name,
	}

	return labels, nil
}

func (c *ControlPlane) ApplicationBundleName() string {
	// TODO: DELETE ME
	if c.Spec.ApplicationBundle == nil {
		return "control-plane-1.0.0"
	}

	return *c.Spec.ApplicationBundle
}

func (c ControlPlane) Entropy() []byte {
	return []byte(c.UID)
}

func (c ControlPlane) UpgradeSpec() *ApplicationBundleAutoUpgradeSpec {
	return c.Spec.ApplicationBundleAutoUpgrade
}

// LookupCondition scans the status conditions for an existing condition whose type
// matches.  Returns the array index, or -1 if it doesn't exist.
func (c *KubernetesCluster) LookupCondition(t KubernetesClusterConditionType) (*KubernetesClusterCondition, error) {
	for i, condition := range c.Status.Conditions {
		if condition.Type == t {
			return &c.Status.Conditions[i], nil
		}
	}

	return nil, ErrStatusConditionLookup
}

// UpdateCondition either adds or updates a condition in the cluster status.
// If the condition, status and message match an existing condition the update is
// ignored.  Returns true if a modification has been made.
func (c *KubernetesCluster) UpdateCondition(t KubernetesClusterConditionType, status corev1.ConditionStatus, reason KubernetesClusterConditionReason, message string) bool {
	condition := KubernetesClusterCondition{
		Type:               t,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}

	existingPtr, err := c.LookupCondition(t)
	if err != nil {
		c.Status.Conditions = append(c.Status.Conditions, condition)

		return true
	}

	// Do a shallow copy and set the same time, then do a shallow equality to
	// see if we need an update.
	existing := *existingPtr
	existing.LastTransitionTime = condition.LastTransitionTime

	if existing != condition {
		*existingPtr = condition

		return true
	}

	return false
}

// UpdateAvailableCondition updates the Available condition specifically.
func (c *KubernetesCluster) UpdateAvailableCondition(status corev1.ConditionStatus, reason KubernetesClusterConditionReason, message string) bool {
	return c.UpdateCondition(KubernetesClusterConditionAvailable, status, reason, message)
}

// ResourceLabels generates a set of labels to uniquely identify the resource
// if it were to be placed in a single global namespace.
func (c *KubernetesCluster) ResourceLabels() (labels.Set, error) {
	project, ok := c.Labels[constants.ProjectLabel]
	if !ok {
		return nil, ErrMissingLabel
	}

	controlPlane, ok := c.Labels[constants.ControlPlaneLabel]
	if !ok {
		return nil, ErrMissingLabel
	}

	labels := labels.Set{
		constants.ProjectLabel:           project,
		constants.ControlPlaneLabel:      controlPlane,
		constants.KubernetesClusterLabel: c.Name,
	}

	return labels, nil
}

func (c *KubernetesCluster) ApplicationBundleName() string {
	// TODO: DELETE ME
	if c.Spec.ApplicationBundle == nil {
		return "kubernetes-cluster-1.0.0"
	}

	return *c.Spec.ApplicationBundle
}

func (c KubernetesCluster) Entropy() []byte {
	return []byte(c.UID)
}

func (c KubernetesCluster) UpgradeSpec() *ApplicationBundleAutoUpgradeSpec {
	return c.Spec.ApplicationBundleAutoUpgrade
}

// AutoscalingEnabled indicates whether cluster autoscaling is enabled for the cluster.
func (c *KubernetesCluster) AutoscalingEnabled() bool {
	return c.Spec.Features != nil && c.Spec.Features.Autoscaling != nil && *c.Spec.Features.Autoscaling
}

// IngressEnabled indicates whether an ingress controller is required.
func (c *KubernetesCluster) IngressEnabled() bool {
	return c.Spec.Features != nil && c.Spec.Features.Ingress != nil && *c.Spec.Features.Ingress
}

// GetName is the name passed down to Helm.
func (w *KubernetesWorkloadPool) GetName() string {
	if w.Spec.Name != nil {
		return *w.Spec.Name
	}

	return w.Name
}

// Ensure type is sortable for stable deterministic output.
var _ sort.Interface = &ProjectList{}

func (l ProjectList) Len() int {
	return len(l.Items)
}

func (l ProjectList) Less(i, j int) bool {
	return strings.Compare(l.Items[i].Name, l.Items[j].Name) == -1
}

func (l ProjectList) Swap(i, j int) {
	l.Items[i], l.Items[j] = l.Items[j], l.Items[i]
}

// Ensure type is sortable for stable deterministic output.
var _ sort.Interface = &ControlPlaneList{}

func (l ControlPlaneList) Len() int {
	return len(l.Items)
}

func (l ControlPlaneList) Less(i, j int) bool {
	return strings.Compare(l.Items[i].Name, l.Items[j].Name) == -1
}

func (l ControlPlaneList) Swap(i, j int) {
	l.Items[i], l.Items[j] = l.Items[j], l.Items[i]
}

// Ensure type is sortable for stable deterministic output.
var _ sort.Interface = &KubernetesClusterList{}

func (l KubernetesClusterList) Len() int {
	return len(l.Items)
}

func (l KubernetesClusterList) Less(i, j int) bool {
	return strings.Compare(l.Items[i].Name, l.Items[j].Name) == -1
}

func (l KubernetesClusterList) Swap(i, j int) {
	l.Items[i], l.Items[j] = l.Items[j], l.Items[i]
}

// Ensure type is sortable for stable deterministic output.
var _ sort.Interface = &ApplicationBundleList{}

func (l ApplicationBundleList) Len() int {
	return len(l.Items)
}

func (l ApplicationBundleList) Less(i, j int) bool {
	// TODO: while this works now, it won't unless we parse and compare as
	// a semantic version.
	return strings.Compare(*l.Items[i].Spec.Version, *l.Items[j].Spec.Version) == -1
}

func (l ApplicationBundleList) Swap(i, j int) {
	l.Items[i], l.Items[j] = l.Items[j], l.Items[i]
}

// Get retrieves the named bundle.
func (l ApplicationBundleList) Get(name string) *ApplicationBundle {
	for i := range l.Items {
		if l.Items[i].Name == name {
			return &l.Items[i]
		}
	}

	return nil
}

// ByKind returns a new list of bundles for a specifc kind e.g. clusters or control planes.
func (l ApplicationBundleList) ByKind(kind ApplicationBundleResourceKind) *ApplicationBundleList {
	result := &ApplicationBundleList{}

	for _, bundle := range l.Items {
		if *bundle.Spec.Kind == kind {
			result.Items = append(result.Items, bundle)
		}
	}

	return result
}

// Upgradable returns a new list of bundles that are "stable" e.g. not end of life and
// not a preview.
func (l ApplicationBundleList) Upgradable() *ApplicationBundleList {
	result := &ApplicationBundleList{}

	for _, bundle := range l.Items {
		if bundle.Spec.Preview != nil && *bundle.Spec.Preview {
			continue
		}

		if bundle.Spec.EndOfLife != nil {
			continue
		}

		result.Items = append(result.Items, bundle)
	}

	return result
}

func (b *ApplicationBundle) GetApplication(name string) *ApplicationBundleApplication {
	for i := range b.Spec.Applications {
		if *b.Spec.Applications[i].Name == name {
			return &b.Spec.Applications[i]
		}
	}

	return nil
}
