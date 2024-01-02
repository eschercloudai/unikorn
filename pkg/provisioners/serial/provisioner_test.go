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

package serial_test

import (
	"context"
	"flag"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/eschercloudai/unikorn/pkg/provisioners"
	"github.com/eschercloudai/unikorn/pkg/provisioners/mock"
	"github.com/eschercloudai/unikorn/pkg/provisioners/serial"

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

// TestSerialProvision expects the serial provisioner
// to succeed when both provisioners do.
func TestSerialProvision(t *testing.T) {
	t.Parallel()

	c := gomock.NewController(t)
	defer c.Finish()

	ctx := context.Background()

	p := mock.NewMockProvisioner(c)
	p.EXPECT().Provision(ctx).Return(nil).Times(2)

	assert.NoError(t, serial.New("test", p, p).Provision(ctx))
}

// TestSerialProvisionYieldFirst ensures only the first provisioner is
// called and it returns a yield if the first provisioner does.
func TestSerialProvisionYieldFirst(t *testing.T) {
	t.Parallel()

	c := gomock.NewController(t)
	defer c.Finish()

	ctx := context.Background()

	p := mock.NewMockProvisioner(c)
	p.EXPECT().Provision(ctx).Return(provisioners.ErrYield)
	p.EXPECT().ProvisionerName().Return("")

	assert.ErrorIs(t, provisioners.ErrYield, serial.New("test", p, p).Provision(ctx))
}

// TestSerialProvisionYieldSecond ensures all provisioners are
// called and it returns a yield if the second provisioner yields.
func TestSerialProvisionYieldSecond(t *testing.T) {
	t.Parallel()

	c := gomock.NewController(t)
	defer c.Finish()

	ctx := context.Background()

	p1 := mock.NewMockProvisioner(c)
	p1.EXPECT().Provision(ctx).Return(nil)

	p2 := mock.NewMockProvisioner(c)
	p2.EXPECT().Provision(ctx).Return(provisioners.ErrYield)
	p2.EXPECT().ProvisionerName().Return("")

	assert.ErrorIs(t, provisioners.ErrYield, serial.New("test", p1, p2).Provision(ctx))
}

// TestSerialDeprovision expects the serial provisioner
// to succeed when both provisioners do.
func TestSerialDeprovision(t *testing.T) {
	t.Parallel()

	c := gomock.NewController(t)
	defer c.Finish()

	ctx := context.Background()

	p := mock.NewMockProvisioner(c)
	p.EXPECT().Deprovision(ctx).Return(nil).Times(2)

	assert.NoError(t, serial.New("test", p, p).Deprovision(ctx))
}

// TestSerialDeprovisionYieldFirst ensures all provisioners are
// called and it returns a yield if the first provisioner yields.
// Technically the second as it's done in reverse order.
func TestSerialDeprovisionYieldFirst(t *testing.T) {
	t.Parallel()

	c := gomock.NewController(t)
	defer c.Finish()

	ctx := context.Background()

	p1 := mock.NewMockProvisioner(c)
	p1.EXPECT().Deprovision(ctx).Return(provisioners.ErrYield)
	p1.EXPECT().ProvisionerName().Return("")

	p2 := mock.NewMockProvisioner(c)
	p2.EXPECT().Deprovision(ctx).Return(nil)

	assert.ErrorIs(t, provisioners.ErrYield, serial.New("test", p1, p2).Deprovision(ctx))
}

// TestSerialDeprovisionYieldSecond ensures all provisioners are
// called and it returns a yield if the second provisioner yields.
// Technically the first as it's done in reverse order.
func TestSerialDeprovisionYieldSecond(t *testing.T) {
	t.Parallel()

	c := gomock.NewController(t)
	defer c.Finish()

	ctx := context.Background()

	p := mock.NewMockProvisioner(c)
	p.EXPECT().Deprovision(ctx).Return(provisioners.ErrYield)
	p.EXPECT().ProvisionerName().Return("")

	assert.ErrorIs(t, provisioners.ErrYield, serial.New("test", p, p).Deprovision(ctx))
}
