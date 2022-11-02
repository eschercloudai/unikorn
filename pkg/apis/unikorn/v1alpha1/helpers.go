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
	"errors"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	ErrStatusConditionLookup = errors.New("status condition not found")
)

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

func (c *ControlPlane) UpdateAvailableCondition(status corev1.ConditionStatus, reason ControlPlaneConditionReason, message string) bool {
	return c.UpdateCondition(ControlPlaneConditionAvailable, status, reason, message)
}
