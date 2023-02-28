/*
Copyright 2022 EscherCloud.

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

package openstack

import (
	"context"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/pagination"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"github.com/eschercloudai/unikorn/pkg/constants"
)

// BlockStorageClient wraps the generic client because gophercloud is unsafe.
type BlockStorageClient struct {
	client *gophercloud.ServiceClient
}

// NewBlockStorageClient provides a simple one-liner to start computing.
func NewBlockStorageClient(provider Provider) (*BlockStorageClient, error) {
	providerClient, err := provider.Client()
	if err != nil {
		return nil, err
	}

	client, err := openstack.NewBlockStorageV3(providerClient, gophercloud.EndpointOpts{})
	if err != nil {
		return nil, err
	}

	c := &BlockStorageClient{
		client: client,
	}

	return c, nil
}

type AvailabilityZonePage struct {
	pagination.SinglePageBase
}

type AvailabilityZoneInfo struct {
	Info []AvailabilityZone `json:"availabilityZoneInfo"`
}

type AvailabilityZone struct {
	ZoneName  string                `json:"zoneName"`
	ZoneState AvailabilityZoneState `json:"zoneState"`
}

type AvailabilityZoneState struct {
	Available bool `json:"available"`
}

// AvailabilityZones retrieves block storage availability zones.
// Obviously this is undocumented by Openstack, and unimplemented by
// gophercloud, so we have to do it ourselves.
// TODO: upstream me.
func (c *BlockStorageClient) AvailabilityZones(ctx context.Context) ([]AvailabilityZone, error) {
	url := c.client.ServiceURL("os-availability-zone")

	tracer := otel.GetTracerProvider().Tracer(constants.Application)

	_, span := tracer.Start(ctx, url, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	pager := pagination.NewPager(c.client, url, func(r pagination.PageResult) pagination.Page {
		return AvailabilityZonePage{pagination.SinglePageBase(r)}
	})

	page, err := pager.AllPages()
	if err != nil {
		return nil, err
	}

	result := &AvailabilityZoneInfo{}

	//nolint:forcetypeassert
	if err := (page.(AvailabilityZonePage)).ExtractInto(result); err != nil {
		return nil, err
	}

	filtered := []AvailabilityZone{}

	for _, az := range result.Info {
		if !az.ZoneState.Available {
			continue
		}

		filtered = append(filtered, az)
	}

	return filtered, nil
}
