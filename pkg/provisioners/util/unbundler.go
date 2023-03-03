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

package util

import (
	"context"
	"errors"

	unikornv1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	ErrBundleType         = errors.New("bundle has the wrong type")
	ErrApplicationMissing = errors.New("bundle missing expected application")
)

// BundledType is a type, typically a custom resource, that has an attached
// application bundle.
type BundledType interface {
	ApplicationBundleName() string
}

type unbundleItem struct {
	// r is where to store the resource.
	r **unikornv1.HelmApplication
	// name is the resource name in the bundle.
	name string
}

type Unbundler struct {
	// name of the bundle.
	name string
	// kind is the expected bundle kind.
	kind unikornv1.ApplicationBundleResourceKind
	// items are applications to unbundle.
	items []unbundleItem
}

func NewUnbundler(o BundledType, kind unikornv1.ApplicationBundleResourceKind) *Unbundler {
	return &Unbundler{
		name: o.ApplicationBundleName(),
		kind: kind,
	}
}

func (u *Unbundler) AddApplication(r **unikornv1.HelmApplication, name string) {
	u.items = append(u.items, unbundleItem{r: r, name: name})
}

func (u *Unbundler) Unbundle(ctx context.Context, c client.Client) error {
	key := client.ObjectKey{
		Name: u.name,
	}

	bundle := &unikornv1.ApplicationBundle{}

	if err := c.Get(ctx, key, bundle); err != nil {
		return err
	}

	if *bundle.Spec.Kind != u.kind {
		return ErrBundleType
	}

	for _, item := range u.items {
		applicationReference := bundle.GetApplication(item.name)
		if applicationReference == nil {
			return ErrApplicationMissing
		}

		key := client.ObjectKey{
			Name: *applicationReference.Reference.Name,
		}

		// TODO: check the kind.
		application := &unikornv1.HelmApplication{}

		if err := c.Get(ctx, key, application); err != nil {
			return err
		}

		*item.r = application
	}

	return nil
}
