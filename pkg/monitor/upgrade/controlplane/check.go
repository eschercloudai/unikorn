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

package controlplane

import (
	"context"
	"fmt"
	"sort"

	unikornv1 "github.com/eschercloudai/unikorn/pkg/apis/unikorn/v1alpha1"
	"github.com/eschercloudai/unikorn/pkg/constants"
	"github.com/eschercloudai/unikorn/pkg/monitor/upgrade/errors"
	"github.com/eschercloudai/unikorn/pkg/monitor/upgrade/util"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Checker struct {
	client client.Client
}

func New(client client.Client) *Checker {
	return &Checker{
		client: client,
	}
}

//nolint:cyclop
func (c *Checker) Check(ctx context.Context) error {
	log := log.FromContext(ctx)

	log.Info("checking for control plane upgrades")

	allBundles := &unikornv1.ApplicationBundleList{}

	if err := c.client.List(ctx, allBundles, &client.ListOptions{}); err != nil {
		return err
	}

	// Select only the pertinent bundles for this type.
	allBundlesByKind := allBundles.ByKind(unikornv1.ApplicationBundleResourceKindControlPlane)

	// Extract the potential upgrade target bundles, these are sorted by version, so
	// the newest is on the top, we shall see why later...
	bundles := allBundlesByKind.Upgradable()
	if len(bundles.Items) == 0 {
		return errors.ErrNoBundles
	}

	sort.Stable(bundles)

	// Pick the most recent as out upgrade target.
	upgradeTarget := &bundles.Items[0]

	resources := &unikornv1.ControlPlaneList{}

	if err := c.client.List(ctx, resources, &client.ListOptions{}); err != nil {
		return err
	}

	for _, resource := range resources.Items {
		if resource.Spec.ApplicationBundleAutoUpgrade == nil {
			log.Info("resource auto-upgrade disabled, ignoring", "project", resource.Labels[constants.ProjectLabel], "controlplane", resource.Name)
			continue
		}

		bundle := allBundlesByKind.Get(resource.ApplicationBundleName())
		if bundle == nil {
			return fmt.Errorf("%w: %s", errors.ErrMissingBundle, *resource.Spec.ApplicationBundle)
		}

		// If the current bundle is in preview, then don't offer to upgrade.
		if bundle.Spec.Preview != nil && *bundle.Spec.Preview {
			log.Info("bundle in preview, ignoring", "project", resource.Labels[constants.ProjectLabel], "controlplane", resource.Name)
			continue
		}

		// If the current bundle is the best option already, we are done.
		if bundle.Name == upgradeTarget.Name {
			log.Info("bundle already latest, ignoring", "project", resource.Labels[constants.ProjectLabel], "controlplane", resource.Name)
			continue
		}

		// Is it allowed to happen now?  Base it on the UID for ultimate randomness,
		// you can cause a stampede if all the resources are called "default".
		window := util.GenerateTimeWindow(string(resource.UID))
		if !window.In() {
			log.Info("not in upgrade window, ignoring", "project", resource.Labels[constants.ProjectLabel], "controlplane", resource.Name, "start", window.Start, "end", window.End)
			continue
		}

		log.Info("bundle upgrading", "project", resource.Labels[constants.ProjectLabel], "controlplane", resource.Name, "from", *bundle.Spec.Version, "to", *upgradeTarget.Spec.Version)
	}

	return nil
}
