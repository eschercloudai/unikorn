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

package openstack

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/availabilityzones"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/keypairs"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/flavors"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/images"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"github.com/eschercloudai/unikorn/pkg/constants"
)

var (
	// ErrParseError is for when we cannot parse Openstack data correctly.
	ErrParseError = errors.New("unable to parse value")
)

// ComputeClient wraps the generic client because gophercloud is unsafe.
type ComputeClient struct {
	client *gophercloud.ServiceClient
}

// NewComputeClient provides a simple one-liner to start computing.
func NewComputeClient(provider Provider) (*ComputeClient, error) {
	providerClient, err := provider.Client()
	if err != nil {
		return nil, err
	}

	client, err := openstack.NewComputeV2(providerClient, gophercloud.EndpointOpts{})
	if err != nil {
		return nil, err
	}

	c := &ComputeClient{
		client: client,
	}

	return c, nil
}

// KeyPairs returns a list of key pairs.
func (c *ComputeClient) KeyPairs(ctx context.Context) ([]keypairs.KeyPair, error) {
	tracer := otel.GetTracerProvider().Tracer(constants.Application)

	_, span := tracer.Start(ctx, "/compute/v2/os-keypairs", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	page, err := keypairs.List(c.client, &keypairs.ListOpts{}).AllPages()
	if err != nil {
		return nil, err
	}

	return keypairs.ExtractKeyPairs(page)
}

// Flavors returns a list of flavors.
func (c *ComputeClient) Flavors(ctx context.Context) ([]flavors.Flavor, error) {
	tracer := otel.GetTracerProvider().Tracer(constants.Application)

	_, span := tracer.Start(ctx, "/compute/v2/flavors", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	page, err := flavors.ListDetail(c.client, &flavors.ListOpts{}).AllPages()
	if err != nil {
		return nil, err
	}

	return flavors.ExtractFlavors(page)
}

// Flavor returns a single flavor.
func (c *ComputeClient) Flavor(ctx context.Context, name string) (*flavors.Flavor, error) {
	// Arse, OS only deals in IDs, we deal in human readable names.
	flavors, err := c.Flavors(ctx)
	if err != nil {
		return nil, err
	}

	for i, flavor := range flavors {
		if flavor.Name == name {
			f := &flavors[i]

			return f, nil
		}
	}

	return nil, fmt.Errorf("%w: unable to find flavor %s", ErrResourceNotFound, name)
}

// FlavorExtraSpecs returns extra metadata for a flavor.
func (c *ComputeClient) FlavorExtraSpecs(ctx context.Context, flavor *flavors.Flavor) (map[string]string, error) {
	tracer := otel.GetTracerProvider().Tracer(constants.Application)

	_, span := tracer.Start(ctx, "/compute/v2/flavors/"+flavor.ID+"/os-extra_specs", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	result := flavors.ListExtraSpecs(c.client, flavor.ID)
	if result.Err != nil {
		return nil, result.Err
	}

	return result.Extract()
}

// GPUMeta describes GPUs.
type GPUMeta struct {
	// GPUs is the number of GPUs, this may be the total number
	// or physical GPUs, or a single virtual GPU.  This value
	// is what will be reported for Kubernetes scheduling.
	GPUs int
}

// FlavorGPUs returns metadata about GPUs, whether it has any, the number of GPUs
// or the number of virtual GPUs.  Sadly there is absolutely no way of assiging
// metadata to flavors without having to add those same values to your host aggregates,
// so we have to have knowledge of flavors built in somewhere.
func FlavorGPUs(flavor *flavors.Flavor, extraSpecs map[string]string) (*GPUMeta, bool, error) {
	// There are some well known extra specs defined in:
	// https://docs.openstack.org/nova/latest/configuration/extra-specs.html
	//
	// MIG instances will have specs that look like:
	//   "resources:VGPU": "1", "trait:CUSTOM_A100D_2_20C": "required"
	if value, ok := extraSpecs["resources:VGPU"]; ok {
		gpus, err := strconv.Atoi(value)
		if err != nil {
			return nil, false, err
		}

		meta := &GPUMeta{
			GPUs: gpus,
		}

		return meta, true, nil
	}

	// Full GPUs will be totally different, luckily the
	//   "pci_passthrough:alias": "a100:2"
	if value, ok := extraSpecs["pci_passthrough:alias"]; ok {
		parts := strings.Split(value, ":")
		if len(parts) != 2 {
			return nil, false, fmt.Errorf("%w: GPU flavor %s metadata malformed", ErrParseError, flavor.Name)
		}

		if parts[0] != "a100" {
			return nil, false, fmt.Errorf("%w: unknown PCI device class %s", ErrParseError, parts[0])
		}

		gpus, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, false, err
		}

		meta := &GPUMeta{
			GPUs: gpus,
		}

		return meta, true, nil
	}

	return nil, false, nil
}

// Images returns a list of images.
func (c *ComputeClient) Images(ctx context.Context) ([]images.Image, error) {
	tracer := otel.GetTracerProvider().Tracer(constants.Application)

	_, span := tracer.Start(ctx, "/compute/v2/images", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	// Only return active images that are ready to be used.
	opts := &images.ListOpts{
		Status: "ACTIVE",
	}

	page, err := images.ListDetail(c.client, opts).AllPages()
	if err != nil {
		return nil, err
	}

	result, err := images.ExtractImages(page)
	if err != nil {
		return nil, err
	}

	// Filter out images that aren't compatible.
	filtered := []images.Image{}

	for _, image := range result {
		// Only accept images in scope that we know conform to our requirements.
		// TODO: we need a formal specification for this.
		if image.Metadata == nil {
			continue
		}

		// TODO: value checking shouldn't be required, but we have some duff images
		// in the system.
		if value, ok := image.Metadata["k8s"]; !ok || value == "" {
			continue
		}

		if value, ok := image.Metadata["gpu"]; !ok || value == "" {
			continue
		}

		filtered = append(filtered, image)
	}

	return filtered, nil
}

// AvailabilityZones returns a list of availability zones.
func (c *ComputeClient) AvailabilityZones(ctx context.Context) ([]availabilityzones.AvailabilityZone, error) {
	tracer := otel.GetTracerProvider().Tracer(constants.Application)

	_, span := tracer.Start(ctx, "/compute/v2/os-availability-zones", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	page, err := availabilityzones.List(c.client).AllPages()
	if err != nil {
		return nil, err
	}

	result, err := availabilityzones.ExtractAvailabilityZones(page)
	if err != nil {
		return nil, err
	}

	filtered := []availabilityzones.AvailabilityZone{}

	for _, az := range result {
		if !az.ZoneState.Available {
			continue
		}

		filtered = append(filtered, az)
	}

	return filtered, nil
}
