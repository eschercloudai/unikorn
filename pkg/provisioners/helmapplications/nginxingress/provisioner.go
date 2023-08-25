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

package nginxingress

import (
	unikornv1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/cd"
	"github.com/eschercloudai/unikorn/pkg/provisioners/application"
)

const (
	// applicationName is the unique name of the application.
	applicationName = "nginx-ingress"
)

type Provisioner struct{}

// New returns a new initialized provisioner object.
func New(driver cd.Driver, resource application.MutuallyExclusiveResource, helm *unikornv1.HelmApplication) *application.Provisioner {
	return application.New(driver, applicationName, resource, helm).InNamespace("nginx-system")
}
