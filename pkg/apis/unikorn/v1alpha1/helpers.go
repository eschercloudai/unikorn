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
	"strings"
	"time"

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

// Paused implements the ReconcilePauser interface.
func (c *Project) Paused() bool {
	return c.Spec.Pause
}

// Paused implements the ReconcilePauser interface.
func (c *ControlPlane) Paused() bool {
	return c.Spec.Pause
}

// Paused implements the ReconcilePauser interface.
func (c *KubernetesCluster) Paused() bool {
	return c.Spec.Pause
}

// getCondition is a generic condition lookup function.
func getCondition(conditions []Condition, t ConditionType) (*Condition, error) {
	for i, condition := range conditions {
		if condition.Type == t {
			return &conditions[i], nil
		}
	}

	return nil, ErrStatusConditionLookup
}

// updateCondition either adds or updates a condition in the control plane
// status. If the condition, status and message match an existing condition
// the update is ignored.
func updateCondition(conditions *[]Condition, t ConditionType, status corev1.ConditionStatus, reason ConditionReason, message string) {
	condition := Condition{
		Type:               t,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}

	existingPtr, err := getCondition(*conditions, t)
	if err != nil {
		*conditions = append(*conditions, condition)

		return
	}

	// Do a shallow copy and set the same time, then do a shallow equality to
	// see if we need an update.
	existing := *existingPtr
	existing.LastTransitionTime = condition.LastTransitionTime

	if existing != condition {
		*existingPtr = condition
	}
}

// StatusConditionRead scans the status conditions for an existing condition whose type
// matches.
func (c *Project) StatusConditionRead(t ConditionType) (*Condition, error) {
	return getCondition(c.Status.Conditions, t)
}

// StatusConditionWrite either adds or updates a condition in the control plane status.
// If the condition, status and message match an existing condition the update is
// ignored.
func (c *Project) StatusConditionWrite(t ConditionType, status corev1.ConditionStatus, reason ConditionReason, message string) {
	updateCondition(&c.Status.Conditions, t, status, reason, message)
}

// ResourceLabels generates a set of labels to uniquely identify the resource
// if it were to be placed in a single global namespace.
func (c *Project) ResourceLabels() (labels.Set, error) {
	labels := labels.Set{
		constants.KindLabel:    constants.KindLabelValueProject,
		constants.ProjectLabel: c.Name,
	}

	return labels, nil
}

func (c *Project) ApplicationBundleKind() ApplicationBundleResourceKind {
	return ""
}

func (c *Project) ApplicationBundleName() string {
	return ""
}

// StatusConditionRead scans the status conditions for an existing condition whose type
// matches.
func (c *ControlPlane) StatusConditionRead(t ConditionType) (*Condition, error) {
	return getCondition(c.Status.Conditions, t)
}

// StatusConditionWrite either adds or updates a condition in the control plane status.
// If the condition, status and message match an existing condition the update is
// ignored.
func (c *ControlPlane) StatusConditionWrite(t ConditionType, status corev1.ConditionStatus, reason ConditionReason, message string) {
	updateCondition(&c.Status.Conditions, t, status, reason, message)
}

// ResourceLabels generates a set of labels to uniquely identify the resource
// if it were to be placed in a single global namespace.
func (c *ControlPlane) ResourceLabels() (labels.Set, error) {
	project, ok := c.Labels[constants.ProjectLabel]
	if !ok {
		return nil, ErrMissingLabel
	}

	labels := labels.Set{
		constants.KindLabel:         constants.KindLabelValueControlPlane,
		constants.ProjectLabel:      project,
		constants.ControlPlaneLabel: c.Name,
	}

	return labels, nil
}

func (c *ControlPlane) ApplicationBundleKind() ApplicationBundleResourceKind {
	return ApplicationBundleResourceKindControlPlane
}

func (c *ControlPlane) ApplicationBundleName() string {
	return *c.Spec.ApplicationBundle
}

func (c ControlPlane) Entropy() []byte {
	return []byte(c.UID)
}

func (c ControlPlane) UpgradeSpec() *ApplicationBundleAutoUpgradeSpec {
	return c.Spec.ApplicationBundleAutoUpgrade
}

// StatusConditionRead scans the status conditions for an existing condition whose type
// matches.
func (c *KubernetesCluster) StatusConditionRead(t ConditionType) (*Condition, error) {
	return getCondition(c.Status.Conditions, t)
}

// StatusConditionWrite either adds or updates a condition in the cluster status.
// If the condition, status and message match an existing condition the update is
// ignored.
func (c *KubernetesCluster) StatusConditionWrite(t ConditionType, status corev1.ConditionStatus, reason ConditionReason, message string) {
	updateCondition(&c.Status.Conditions, t, status, reason, message)
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
		constants.KindLabel:              constants.KindLabelValueKubernetesCluster,
		constants.ProjectLabel:           project,
		constants.ControlPlaneLabel:      controlPlane,
		constants.KubernetesClusterLabel: c.Name,
	}

	return labels, nil
}

func (c *KubernetesCluster) ApplicationBundleKind() ApplicationBundleResourceKind {
	return ApplicationBundleResourceKindKubernetesCluster
}

func (c *KubernetesCluster) ApplicationBundleName() string {
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

// CertManagerEnabled indicates whether cert-manager is required.
func (c *KubernetesCluster) CertManagerEnabled() bool {
	return c.Spec.Features != nil && c.Spec.Features.CertManager != nil && *c.Spec.Features.CertManager
}

// KubernetesDashboardEnabled indicates whether the Kubernetes dashboard is required.
func (c *KubernetesCluster) KubernetesDashboardEnabled() bool {
	return c.Spec.Features != nil && c.Spec.Features.KubernetesDashboard != nil && *c.Spec.Features.KubernetesDashboard
}

// FileStorageEnabled indicates whether a POSIX file storage CSI is required.
func (c *KubernetesCluster) FileStorageEnabled() bool {
	return c.Spec.Features != nil && c.Spec.Features.FileStorage != nil && *c.Spec.Features.FileStorage
}

// PrometheusEnabled indicates whether the Prometheus Operator is required.
func (c *KubernetesCluster) PrometheusEnabled() bool {
	return c.Spec.Features != nil && c.Spec.Features.Prometheus != nil && *c.Spec.Features.Prometheus
}

// NvidiaOperatorEnabled indicates whether to install the Nvidia GPU operator.
func (c *KubernetesCluster) NvidiaOperatorEnabled() bool {
	return c.Spec.Features != nil && c.Spec.Features.NvidiaOperator != nil && *c.Spec.Features.NvidiaOperator
}

func CompareControlPlane(a, b ControlPlane) int {
	return strings.Compare(a.Name, b.Name)
}

func CompareKubernetesCluster(a, b KubernetesCluster) int {
	return strings.Compare(a.Name, b.Name)
}

func CompareApplicationBundle(a, b ApplicationBundle) int {
	// TODO: while this works now, it won't unless we parse and compare as
	// a semantic version.
	return strings.Compare(*a.Spec.Version, *b.Spec.Version)
}

func CompareHelmApplication(a, b HelmApplication) int {
	return strings.Compare(a.Name, b.Name)
}

// Exported returns all applications that are exported, and thus end-user installable.
func (l HelmApplicationList) Exported() HelmApplicationList {
	result := HelmApplicationList{}

	for i := range l.Items {
		if l.Items[i].Spec.Exported != nil && *l.Items[i].Spec.Exported {
			result.Items = append(result.Items, l.Items[i])
		}
	}

	return result
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

// Weekdays returns the days of the week that are set in the spec.
func (s ApplicationBundleAutoUpgradeWeekDaySpec) Weekdays() []time.Weekday {
	var result []time.Weekday

	if s.Sunday != nil {
		result = append(result, time.Sunday)
	}

	if s.Monday != nil {
		result = append(result, time.Monday)
	}

	if s.Tuesday != nil {
		result = append(result, time.Tuesday)
	}

	if s.Wednesday != nil {
		result = append(result, time.Wednesday)
	}

	if s.Thursday != nil {
		result = append(result, time.Thursday)
	}

	if s.Friday != nil {
		result = append(result, time.Friday)
	}

	if s.Saturday != nil {
		result = append(result, time.Saturday)
	}

	return result
}
