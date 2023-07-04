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

package server_test

import (
	"context"
	"testing"

	unikornv1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/constants"
	"github.com/eschercloudai/unikorn/pkg/testutil"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// mustCreateProjectFixture creates a project, and its randomly named namespace
// just as if unikorn-project-manager had picked up the create request, preformed it
// and also updated the status.
func mustCreateProjectFixture(t *testing.T, tc *TestContext, projectID string) *unikornv1.Project {
	t.Helper()

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "project-",
			Labels: map[string]string{
				constants.ProjectLabel: projectName(projectID),
			},
		},
	}

	testutil.AssertNilError(t, tc.KubernetesClient().Create(context.TODO(), namespace, &client.CreateOptions{}))

	project := &unikornv1.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name: projectName(projectID),
		},
		Status: unikornv1.ProjectStatus{
			Namespace: namespace.Name,
			Conditions: []unikornv1.ProjectCondition{
				{
					Type:   unikornv1.ProjectConditionAvailable,
					Status: corev1.ConditionTrue,
					Reason: unikornv1.ProjectConditionReasonProvisioned,
				},
			},
		},
	}

	testutil.AssertNilError(t, tc.KubernetesClient().Create(context.TODO(), project, &client.CreateOptions{}))

	return project
}

// mustCreateControlPlaneFixture creates a control plane , and its randomly named namespace
// just as if unikorn-controlplane-manager had picked up the create request, preformed it
// and also updated the status.
func mustCreateControlPlaneFixture(t *testing.T, tc *TestContext, projectNamespace, name string) *unikornv1.ControlPlane {
	t.Helper()

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "controlplane-",
			Labels: map[string]string{
				constants.ProjectLabel:      projectName(projectID),
				constants.ControlPlaneLabel: name,
			},
		},
	}

	testutil.AssertNilError(t, tc.KubernetesClient().Create(context.TODO(), namespace, &client.CreateOptions{}))

	controlPlane := &unikornv1.ControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: projectNamespace,
			Name:      name,
		},
		Status: unikornv1.ControlPlaneStatus{
			Namespace: namespace.Name,
			Conditions: []unikornv1.ControlPlaneCondition{
				{
					Type:   unikornv1.ControlPlaneConditionAvailable,
					Status: corev1.ConditionTrue,
					Reason: unikornv1.ControlPlaneConditionReasonProvisioned,
				},
			},
		},
	}

	testutil.AssertNilError(t, tc.KubernetesClient().Create(context.TODO(), controlPlane, &client.CreateOptions{}))

	return controlPlane
}
