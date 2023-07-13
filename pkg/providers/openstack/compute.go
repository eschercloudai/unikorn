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
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/availabilityzones"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/keypairs"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/servergroups"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/flavors"
	"github.com/gophercloud/gophercloud/pagination"
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

	// Need at least 2.15 for soft-anti-affinity policy.
	// Need at least 2.64 for new server group interface.
	client.Microversion = "2.93"

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

// Flavor defines an extended set of flavor information not included
// by default in gophercloud.
type Flavor struct {
	flavors.Flavor

	ExtraSpecs map[string]string
}

// UnmarshalJSON is required because "flavors.Flavor" already defines
// this, and it will undergo method promotion.
func (f *Flavor) UnmarshalJSON(b []byte) error {
	// Unmarshal the native type using its UnmarshalJSON.
	if err := json.Unmarshal(b, &f.Flavor); err != nil {
		return err
	}

	// Create a new anonymous structure, and unmarshal the custom fields
	// into that, so we don't end up in an infinite loop.
	var s struct {
		//nolint:tagliatelle
		ExtraSpecs map[string]string `json:"extra_specs"`
	}

	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	// Copy from the anonymous struct to our flavor definition.
	f.ExtraSpecs = s.ExtraSpecs

	return nil
}

// ExtractFlavors takes raw JSON and decodes it into our custom
// flavour struct.
func ExtractFlavors(r pagination.Page) ([]Flavor, error) {
	var s struct {
		Flavors []Flavor `json:"flavors"`
	}

	//nolint:forcetypeassert
	err := (r.(flavors.FlavorPage)).ExtractInto(&s)

	return s.Flavors, err
}

// Flavors returns a list of flavors.
func (c *ComputeClient) Flavors(ctx context.Context) ([]Flavor, error) {
	tracer := otel.GetTracerProvider().Tracer(constants.Application)

	_, span := tracer.Start(ctx, "/compute/v2/flavors", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	page, err := flavors.ListDetail(c.client, &flavors.ListOpts{SortKey: "name"}).AllPages()
	if err != nil {
		return nil, err
	}

	return ExtractFlavors(page)
}

// Flavor returns a single flavor.
func (c *ComputeClient) Flavor(ctx context.Context, name string) (*Flavor, error) {
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

// GPUMeta describes GPUs.
type GPUMeta struct {
	// GPUs is the number of GPUs, this may be the total number
	// or physical GPUs, or a single virtual GPU.  This value
	// is what will be reported for Kubernetes scheduling.
	GPUs int
}

// FlavorGPUs returns metadata about GPUs, e.g. the number of GPUs.  Sadly there is absolutely
// no way of assiging metadata to flavors without having to add those same values to your host
// aggregates, so we have to have knowledge of flavors built in somewhere.
func FlavorGPUs(flavor *Flavor) (*GPUMeta, error) {
	// There are some well known extra specs defined in:
	// https://docs.openstack.org/nova/latest/configuration/extra-specs.html
	//
	// MIG instances will have specs that look like:
	//   "resources:VGPU": "1", "trait:CUSTOM_A100D_2_20C": "required"
	if value, ok := flavor.ExtraSpecs["resources:VGPU"]; ok {
		gpus, err := strconv.Atoi(value)
		if err != nil {
			return nil, err
		}

		meta := &GPUMeta{
			GPUs: gpus,
		}

		return meta, nil
	}

	// Full GPUs will be totally different, luckily the
	//   "pci_passthrough:alias": "a100:2"
	if value, ok := flavor.ExtraSpecs["pci_passthrough:alias"]; ok {
		parts := strings.Split(value, ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("%w: GPU flavor %s metadata malformed", ErrParseError, flavor.Name)
		}

		if parts[0] != "a100" {
			return nil, fmt.Errorf("%w: unknown PCI device class %s", ErrParseError, parts[0])
		}

		gpus, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, err
		}

		meta := &GPUMeta{
			GPUs: gpus,
		}

		return meta, nil
	}

	//nolint:nilnil
	return nil, nil
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

// ListServerGroups returns all server groups in the project.
func (c *ComputeClient) ListServerGroups(ctx context.Context) ([]servergroups.ServerGroup, error) {
	tracer := otel.GetTracerProvider().Tracer(constants.Application)

	_, span := tracer.Start(ctx, "/compute/v2/os-server-groups", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	page, err := servergroups.List(c.client, &servergroups.ListOpts{}).AllPages()
	if err != nil {
		return nil, err
	}

	return servergroups.ExtractServerGroups(page)
}

// CreateServerGroup creates the named server group with the given policy and returns
// the result.
func (c *ComputeClient) CreateServerGroup(ctx context.Context, name, policy string) (*servergroups.ServerGroup, error) {
	tracer := otel.GetTracerProvider().Tracer(constants.Application)

	_, span := tracer.Start(ctx, "/compute/v2/os-server-groups", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	opts := &servergroups.CreateOpts{
		Name:   name,
		Policy: policy,
	}

	return servergroups.Create(c.client, opts).Extract()
}
