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
func (c *ComputeClient) KeyPairs() ([]keypairs.KeyPair, error) {
	page, err := keypairs.List(c.client, &keypairs.ListOpts{}).AllPages()
	if err != nil {
		return nil, err
	}

	return keypairs.ExtractKeyPairs(page)
}

// Flavors returns a list of flavors.
func (c *ComputeClient) Flavors() ([]flavors.Flavor, error) {
	page, err := flavors.ListDetail(c.client, &flavors.ListOpts{}).AllPages()
	if err != nil {
		return nil, err
	}

	return flavors.ExtractFlavors(page)
}

// Flavor returns a single flavor.
func (c *ComputeClient) Flavor(name string) (*flavors.Flavor, error) {
	// Arse, OS only deals in IDs, we deal in human readable names.
	flavors, err := c.Flavors()
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
func (c *ComputeClient) FlavorExtraSpecs(flavor *flavors.Flavor) (map[string]string, error) {
	result := flavors.ListExtraSpecs(c.client, flavor.ID)
	if result.Err != nil {
		return nil, result.Err
	}

	return result.Extract()
}

// VGPUMeta describes a virtual GPU.
type VGPUMeta struct {
	// Slices are the number of shares of the GPU.
	Slices int

	// NumSlices are the total number of shares available.
	NumSlices int
}

// GPUMeta describes GPUs.
type GPUMeta struct {
	// GPUs is the number of GPUs, this may be the total number
	// or physical GPUs, or a single virtual GPU.  This value
	// is what will be reported for Kubernetes scheduling.
	GPUs int

	// VGPU if set defines VGPU metadata.
	VGPU *VGPUMeta
}

// getVGPUMeta returns the number of VGPU slices for a particular flavor.
func getVGPUMeta(flavor *flavors.Flavor, extraSpecs map[string]string) (*VGPUMeta, error) {
	// TODO: make this a config map.
	vpguMetaMap := map[string]VGPUMeta{
		"trait:CUSTOM_A100D_1_10C": {
			Slices:    1,
			NumSlices: 7,
		},
		"trait:CUSTOM_A100D_2_20C": {
			Slices:    2,
			NumSlices: 7,
		},
		"trait:CUSTOM_A100D_3_40C": {
			Slices:    3,
			NumSlices: 7,
		},
	}

	for property := range extraSpecs {
		if meta, ok := vpguMetaMap[property]; ok {
			return &meta, nil
		}
	}

	return nil, fmt.Errorf("%w: unable to lookup vGPU metadata for flavor %s", ErrParseError, flavor.Name)
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
	// On our platform, VGPU doesn't reflect the number of VGPU slices,
	// so we have to consult the trait to work out what things look like.
	if _, ok := extraSpecs["resources:VGPU"]; ok {
		vgpu, err := getVGPUMeta(flavor, extraSpecs)
		if err != nil {
			return nil, false, err
		}

		meta := &GPUMeta{
			GPUs: 1,
			VGPU: vgpu,
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
func (c *ComputeClient) Images() ([]images.Image, error) {
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
	//nolint:prealloc
	var filtered []images.Image

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
func (c *ComputeClient) AvailabilityZones() ([]availabilityzones.AvailabilityZone, error) {
	page, err := availabilityzones.List(c.client).AllPages()
	if err != nil {
		return nil, err
	}

	return availabilityzones.ExtractAvailabilityZones(page)
}
