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

package conditional_test

import (
	"context"
	"flag"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/eschercloudai/unikorn/pkg/provisioners"
	"github.com/eschercloudai/unikorn/pkg/provisioners/conditional"
	"github.com/eschercloudai/unikorn/pkg/provisioners/mock"

	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestMain(m *testing.M) {
	var debug bool

	flag.BoolVar(&debug, "debug", false, "Enables debug logging")
	flag.Parse()

	if debug {
		log.SetLogger(zap.New(zap.WriteTo(os.Stdout)))
	}

	m.Run()
}

func predicateTrue() bool {
	return true
}

func predicateFalse() bool {
	return false
}

// TestConditionalProvision tests that things are provisioned if asked to be.
func TestConditionalProvision(t *testing.T) {
	t.Parallel()

	c := gomock.NewController(t)
	defer c.Finish()

	ctx := context.Background()

	p := mock.NewMockProvisioner(c)
	p.EXPECT().Provision(ctx).Return(nil)

	assert.NoError(t, conditional.New("test", predicateTrue, p).Provision(ctx))
}

// TestConditionalProvisionFalse tests that things are deprovisioned if asked to
// be e.g. was asked for, but now isn't.
func TestConditionalProvisionFalse(t *testing.T) {
	t.Parallel()

	c := gomock.NewController(t)
	defer c.Finish()

	ctx := context.Background()

	p := mock.NewMockProvisioner(c)
	p.EXPECT().Deprovision(ctx).Return(nil)

	assert.NoError(t, conditional.New("test", predicateFalse, p).Provision(ctx))
}

// TestConditionalProvisionError tests errors are propagated when an error occurs
// with provisioning.
func TestConditionalProvisionError(t *testing.T) {
	t.Parallel()

	c := gomock.NewController(t)
	defer c.Finish()

	ctx := context.Background()

	p := mock.NewMockProvisioner(c)
	p.EXPECT().Provision(ctx).Return(provisioners.ErrYield)

	assert.ErrorIs(t, conditional.New("test", predicateTrue, p).Provision(ctx), provisioners.ErrYield)
}

// TestConditionalProvisionFalseError tests errors are propagated when an error occurs
// with deprovisioning.
func TestConditionalProvisionFalseError(t *testing.T) {
	t.Parallel()

	c := gomock.NewController(t)
	defer c.Finish()

	ctx := context.Background()

	p := mock.NewMockProvisioner(c)
	p.EXPECT().Deprovision(ctx).Return(provisioners.ErrYield)

	assert.ErrorIs(t, conditional.New("test", predicateFalse, p).Provision(ctx), provisioners.ErrYield)
}

// TestConditionalDeprovision tests that things are deprovisioned when already provisioned.
func TestConditionalDeprovision(t *testing.T) {
	t.Parallel()

	c := gomock.NewController(t)
	defer c.Finish()

	ctx := context.Background()

	p := mock.NewMockProvisioner(c)
	p.EXPECT().Deprovision(ctx).Return(nil)

	assert.NoError(t, conditional.New("test", predicateTrue, p).Deprovision(ctx))
}

// TestConditionalDeprovisionFalse tests that things are deprovisioned when they aren't
// already provisioned. For example, the service may be down, you've set the predicate
// to false, and try delete the resource.  When the service comes back up, if this was
// predicated then the provisioner would not be garbage collected and leak resources.
func TestConditionalDeprovisionFalse(t *testing.T) {
	t.Parallel()

	c := gomock.NewController(t)
	defer c.Finish()

	ctx := context.Background()

	p := mock.NewMockProvisioner(c)
	p.EXPECT().Deprovision(ctx).Return(nil)

	assert.NoError(t, conditional.New("test", predicateFalse, p).Deprovision(ctx))
}

// TestConditionalDeprovisionError tests error propagation when enabled.
func TestConditionalDeprovisionError(t *testing.T) {
	t.Parallel()

	c := gomock.NewController(t)
	defer c.Finish()

	ctx := context.Background()

	p := mock.NewMockProvisioner(c)
	p.EXPECT().Deprovision(ctx).Return(provisioners.ErrYield)

	assert.ErrorIs(t, conditional.New("test", predicateTrue, p).Deprovision(ctx), provisioners.ErrYield)
}

// TestConditionalDeprovisionError tests error propagation when disabled, see
// TestConditionalDeprovisionFalse for why this is necessary.
func TestConditionalDeprovisionFalseError(t *testing.T) {
	t.Parallel()

	c := gomock.NewController(t)
	defer c.Finish()

	ctx := context.Background()

	p := mock.NewMockProvisioner(c)
	p.EXPECT().Deprovision(ctx).Return(provisioners.ErrYield)

	assert.ErrorIs(t, conditional.New("test", predicateFalse, p).Deprovision(ctx), provisioners.ErrYield)
}
