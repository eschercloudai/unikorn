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

package concurrent_test

import (
	"context"
	"flag"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/eschercloudai/unikorn/pkg/provisioners"
	"github.com/eschercloudai/unikorn/pkg/provisioners/concurrent"
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

// TestConcurrentProvision expects the concurrent provisioner
// to succeed when both provisioners do.
func TestConcurrentProvision(t *testing.T) {
	t.Parallel()

	c := gomock.NewController(t)
	defer c.Finish()

	ctx := context.Background()

	p1 := mock.NewMockProvisioner(c)
	p1.EXPECT().Provision(ctx).Return(nil)

	p2 := mock.NewMockProvisioner(c)
	p2.EXPECT().Provision(ctx).Return(nil)

	assert.Nil(t, concurrent.New("test", p1, p2).Provision(ctx))
}

// TestConcurrentProvisionYieldFirst ensures all provisioners are
// called and it returns a yield if the first provisioner yields.
func TestConcurrentProvisionYieldFirst(t *testing.T) {
	t.Parallel()

	c := gomock.NewController(t)
	defer c.Finish()

	ctx := context.Background()

	p1 := mock.NewMockProvisioner(c)
	p1.EXPECT().Provision(ctx).Return(provisioners.ErrYield)
	p1.EXPECT().ProvisionerName().Return("")

	p2 := mock.NewMockProvisioner(c)
	p2.EXPECT().Provision(ctx).Return(nil)

	assert.Equal(t, provisioners.ErrYield, concurrent.New("test", p1, p2).Provision(ctx))
}

// TestConcurrentProvisionYieldSecond ensures all provisioners are
// called and it returns a yield if the second provisioner yields.
func TestConcurrentProvisionYieldSecond(t *testing.T) {
	t.Parallel()

	c := gomock.NewController(t)
	defer c.Finish()

	ctx := context.Background()

	p1 := mock.NewMockProvisioner(c)
	p1.EXPECT().Provision(ctx).Return(nil)

	p2 := mock.NewMockProvisioner(c)
	p2.EXPECT().Provision(ctx).Return(provisioners.ErrYield)
	p2.EXPECT().ProvisionerName().Return("")

	assert.Equal(t, provisioners.ErrYield, concurrent.New("test", p1, p2).Provision(ctx))
}

// TestConcurrentDeprovision expects the concurrent provisioner
// to succeed when both provisioners do.
func TestConcurrentDeprovision(t *testing.T) {
	t.Parallel()

	c := gomock.NewController(t)
	defer c.Finish()

	ctx := context.Background()

	p1 := mock.NewMockProvisioner(c)
	p1.EXPECT().Deprovision(ctx).Return(nil)

	p2 := mock.NewMockProvisioner(c)
	p2.EXPECT().Deprovision(ctx).Return(nil)

	assert.Nil(t, concurrent.New("test", p1, p2).Deprovision(ctx))
}

// TestConcurrentDeprovisionYieldFirst ensures all provisioners are
// called and it returns a yield if the first provisioner yields.
func TestConcurrentDeprovisionYieldFirst(t *testing.T) {
	t.Parallel()

	c := gomock.NewController(t)
	defer c.Finish()

	ctx := context.Background()

	p1 := mock.NewMockProvisioner(c)
	p1.EXPECT().Deprovision(ctx).Return(provisioners.ErrYield)
	p1.EXPECT().ProvisionerName().Return("")

	p2 := mock.NewMockProvisioner(c)
	p2.EXPECT().Deprovision(ctx).Return(nil)

	assert.Equal(t, provisioners.ErrYield, concurrent.New("test", p1, p2).Deprovision(ctx))
}

// TestConcurrentDeprovisionYieldSecond ensures all provisioners are
// called and it returns a yield if the second provisioner yields.
func TestConcurrentDeprovisionYieldSecond(t *testing.T) {
	t.Parallel()

	c := gomock.NewController(t)
	defer c.Finish()

	ctx := context.Background()

	p1 := mock.NewMockProvisioner(c)
	p1.EXPECT().Deprovision(ctx).Return(nil)

	p2 := mock.NewMockProvisioner(c)
	p2.EXPECT().Deprovision(ctx).Return(provisioners.ErrYield)
	p2.EXPECT().ProvisionerName().Return("")

	assert.Equal(t, provisioners.ErrYield, concurrent.New("test", p1, p2).Deprovision(ctx))
}
