/*
Copyright 2022-2024 EscherCloud.

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

package application

import (
	"context"

	unikornv1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
)

type key int

//nolint:gochecknoglobals
var resourceKey key

func NewContext(ctx context.Context, resource unikornv1.ManagableResourceInterface) context.Context {
	return context.WithValue(ctx, resourceKey, resource)
}

func FromContext(ctx context.Context) unikornv1.ManagableResourceInterface {
	//nolint:forcetypeassert
	return ctx.Value(resourceKey).(unikornv1.ManagableResourceInterface)
}
