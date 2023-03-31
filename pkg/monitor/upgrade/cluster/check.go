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

package cluster

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
	logger := log.FromContext(ctx)

	logger.Info("checking for kubernetes cluster upgrades")

	allBundles := &unikornv1.ApplicationBundleList{}

	if err := c.client.List(ctx, allBundles, &client.ListOptions{}); err != nil {
		return err
	}

	// Select only the pertinent bundles for this type.
	allBundlesByKind := allBundles.ByKind(unikornv1.ApplicationBundleResourceKindKubernetesCluster)

	// Extract the potential upgrade target bundles, these are sorted by version, so
	// the newest is on the top, we shall see why later...
	bundles := allBundlesByKind.Upgradable()
	if len(bundles.Items) == 0 {
		return errors.ErrNoBundles
	}

	sort.Stable(bundles)

	// Pick the most recent as out upgrade target.
	upgradeTarget := &bundles.Items[0]

	resources := &unikornv1.KubernetesClusterList{}

	if err := c.client.List(ctx, resources, &client.ListOptions{}); err != nil {
		return err
	}

	for _, resource := range resources.Items {
		logger := logger.WithValues("project", resource.Labels[constants.ProjectLabel], "controlplane", resource.Labels[constants.ControlPlaneLabel], "cluster", resource.Name)

		rctx := log.IntoContext(ctx, logger)

		if resource.Spec.ApplicationBundleAutoUpgrade == nil {
			logger.Info("resource auto-upgrade disabled, ignoring")
			continue
		}

		bundle := allBundlesByKind.Get(resource.ApplicationBundleName())
		if bundle == nil {
			return fmt.Errorf("%w: %s", errors.ErrMissingBundle, *resource.Spec.ApplicationBundle)
		}

		// If the current bundle is in preview, then don't offer to upgrade.
		if bundle.Spec.Preview != nil && *bundle.Spec.Preview {
			logger.Info("bundle in preview, ignoring")
			continue
		}

		// If the current bundle is the best option already, we are done.
		if bundle.Name == upgradeTarget.Name {
			logger.Info("bundle already latest, ignoring")
			continue
		}

		// Is it allowed to happen now?  Base it on the UID for ultimate randomness,
		// you can cause a stampede if all the resources are called "default".
		window := util.TimeWindowFromResource(rctx, resource)
		if window == nil {
			logger.Info("no time window in scope, ignoring")
			continue
		}

		if !window.In() {
			logger.Info("not in upgrade window, ignoring", "start", window.Start, "end", window.End)
			continue
		}

		logger.Info("bundle upgrading", "from", *bundle.Spec.Version, "to", *upgradeTarget.Spec.Version)
	}

	return nil
}
