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

package readiness

import (
	"context"
	"errors"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

var (
	// ErrConditionFormat means the formatting of the condition is wrong,
	// these are loosly defined, but there are some conventions.
	ErrConditionFormat = errors.New("status condition incorrectly formatted")

	// ErrConditionMissing means the condition isn't present.
	ErrConditionMissing = errors.New("status condition not found")

	// ErrConditionStatus means the condition has the wrong truthiness.
	ErrConditionStatus = errors.New("status condition incorrect status")
)

// StatusCondition allows any Kubernetes resource to be polled for
// a status condition that is true.  For example Deployments are ready
// when the Available status condition is set.
// TODO: we could provide a nicer interface that accepts a concrete type
// via runtime.Object and runs it through the REST mapper to derive the
// GVR.
// TODO: this only considers namespaced resources, ties into REST mapping
// also.
type StatusCondition struct {
	// client is an intialized Kubernetes dynamic client.
	client dynamic.Interface

	// gvr describes the resource type and API paths.
	gvr schema.GroupVersionResource

	// namespace is the namespace a resource resides in.
	namespace string

	// name is the name of the resource.
	name string

	// conditionType is the type of condition to look for.
	conditionType string
}

// Ensure the Check interface is implemented.
var _ Check = &StatusCondition{}

// NewStatusCondition creates a new status condition readiness check.
func NewStatusCondition(client dynamic.Interface, gvr schema.GroupVersionResource, namespace, name, conditionType string) *StatusCondition {
	return &StatusCondition{
		client:        client,
		gvr:           gvr,
		namespace:     namespace,
		name:          name,
		conditionType: conditionType,
	}
}

// Check implements the Check interface.
func (r *StatusCondition) Check(ctx context.Context) error {
	object, err := r.client.Resource(r.gvr).Namespace(r.namespace).Get(ctx, r.name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	conditions, _, err := unstructured.NestedSlice(object.Object, "status", "conditions")
	if err != nil {
		return fmt.Errorf("%w: conditions lookup error: %s", ErrConditionFormat, err.Error())
	}

	for i := range conditions {
		condition, ok := conditions[i].(map[string]interface{})
		if !ok {
			return fmt.Errorf("%w: condition type assertion error", ErrConditionFormat)
		}

		t, _, err := unstructured.NestedString(condition, "type")
		if err != nil {
			return fmt.Errorf("%w: condition type error: %s", ErrConditionFormat, err.Error())
		}

		if t != r.conditionType {
			continue
		}

		s, _, err := unstructured.NestedString(condition, "status")
		if err != nil {
			return fmt.Errorf("%w: condition status error: %s", ErrConditionFormat, err.Error())
		}

		if s != "True" {
			return ErrConditionStatus
		}

		return nil
	}

	return ErrConditionMissing
}
