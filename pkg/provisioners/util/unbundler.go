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
	"fmt"

	unikornv1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	clientlib "github.com/eschercloudai/unikorn/pkg/client"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	ErrBundleType         = errors.New("bundle has the wrong type")
	ErrApplicationMissing = errors.New("bundle missing expected application")
)

type unbundleItem struct {
	// r is where to store the resource.
	r **unikornv1.HelmApplication
	// name is the resource name in the bundle.
	name string
	// optional is an optional item, this is typically used
	// when adding packages/features to upstream bundles.
	optional bool
}

// UnbundlerOption allows idiomatic updates to new applications.
type UnbundlerOption func(*unbundleItem)

// Optional can be applied to an application to indicate that handling
// of it is conditional.
func Optional(i *unbundleItem) {
	i.optional = true
}

type Unbundler struct {
	// name of the bundle.
	name string
	// kind is the expected bundle kind.
	kind unikornv1.ApplicationBundleResourceKind
	// items are applications to unbundle.
	items []unbundleItem
}

func NewUnbundler(o unikornv1.ApplicationBundleGetter) *Unbundler {
	return &Unbundler{
		name: o.ApplicationBundleName(),
		kind: o.ApplicationBundleKind(),
	}
}

func (u *Unbundler) AddApplication(r **unikornv1.HelmApplication, name string, options ...UnbundlerOption) {
	item := unbundleItem{
		r:    r,
		name: name,
	}

	for _, o := range options {
		o(&item)
	}

	u.items = append(u.items, item)
}

func (u *Unbundler) Unbundle(ctx context.Context) error {
	c := clientlib.StaticClientFromContext(ctx)

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
			if item.optional {
				continue
			}

			return fmt.Errorf("%w: %s", ErrApplicationMissing, item.name)
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
