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
	"errors"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	ErrStatefulSetUnready = errors.New("statefulset readiness doesn't match desired")
)

type StatefulSetReady struct {
	// client is an intialized Kubernetes client.
	client kubernetes.Interface

	// namespace is the namespace a resource resides in.
	namespace string

	// name is the name of the resource.
	name string
}

// Ensure the ReadinessCheck interface is implemented.
var _ ReadinessCheck = &StatefulSetReady{}

// NewStatefulSetReady creates a new statefulset readiness check.
func NewStatefulSetReady(client kubernetes.Interface, namespace, name string) *StatefulSetReady {
	return &StatefulSetReady{
		client:    client,
		namespace: namespace,
		name:      name,
	}
}

// Check implements the ReadinessCheck interface.
func (r *StatefulSetReady) Check() error {
	statefulset, err := r.client.AppsV1().StatefulSets(r.namespace).Get(context.TODO(), r.name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("statefulset get error: %w", err)
	}

	// k8s.io/api/apps/v1/types.go indicates this defaults to 1.
	replicas := int32(1)

	if statefulset.Spec.Replicas != nil {
		replicas = *statefulset.Spec.Replicas
	}

	if statefulset.Status.ReadyReplicas != replicas {
		return fmt.Errorf("%w: status mismatch", ErrStatefulSetUnready)
	}

	return nil
}
