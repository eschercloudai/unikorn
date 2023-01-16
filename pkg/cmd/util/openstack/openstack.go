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
	"encoding/json"
	"errors"
	"fmt"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/availabilityzones"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/keypairs"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/flavors"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/images"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	"github.com/gophercloud/utils/openstack/clientconfig"
)

var (
	ErrResourceNotFound = errors.New("requested resource not found")
)

// providerClient abstracts away a load of cruft when using gophercloud.
// The provider client is used directly with each service.
func providerClient(cloud string) (*gophercloud.ProviderClient, error) {
	clientOpts := &clientconfig.ClientOpts{
		Cloud: cloud,
	}

	authOpts, err := clientconfig.AuthOptions(clientOpts)
	if err != nil {
		return nil, err
	}

	provider, err := openstack.AuthenticatedClient(*authOpts)
	if err != nil {
		return nil, err
	}

	return provider, nil
}

// ComputeClient wraps the generic client because gophercloud is unsafe.
type ComputeClient struct {
	client *gophercloud.ServiceClient
}

// NewComputeClient provides a simple one-liner to start computing.
func NewComputeClient(cloud string) (*ComputeClient, error) {
	provider, err := providerClient(cloud)
	if err != nil {
		return nil, err
	}

	client, err := openstack.NewComputeV2(provider, gophercloud.EndpointOpts{})
	if err != nil {
		return nil, err
	}

	c := &ComputeClient{
		client: client,
	}

	return c, nil
}

// NetworkClient wraps the generic client because gophercloud is unsafe.
type NetworkClient struct {
	client *gophercloud.ServiceClient
}

// NewNetworkClient provides a simple one-liner to start networking.
func NewNetworkClient(cloud string) (*NetworkClient, error) {
	provider, err := providerClient(cloud)
	if err != nil {
		return nil, err
	}

	client, err := openstack.NewNetworkV2(provider, gophercloud.EndpointOpts{})
	if err != nil {
		return nil, err
	}

	c := &NetworkClient{
		client: client,
	}

	return c, nil
}

// Network allows us to extend gophercloud to get access to more interesting
// fields not available in the standard data types.
type Network struct {
	// Network is the gophercloud network type.  This needs to be a field,
	// not an embedded type, lest its UnmarshalJSON function get promoted...
	Network networks.Network

	// External is the bit we care about, is it an external network ID?
	External bool `json:"router:external"`
}

// UnmarshalJSON does magic quite frankly.  We unmarshal directly into the
// gophercloud network type, easy peasy.  When un marshalling into our network
// type, we need to define a temporary type to avoid an infinite loop...
func (n *Network) UnmarshalJSON(b []byte) error {
	if err := json.Unmarshal(b, &n.Network); err != nil {
		return err
	}

	type tmp Network

	var s struct {
		tmp
	}

	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	n.External = s.tmp.External

	return nil
}

// ExternalNetworks returns a list of external networks.
func (c *NetworkClient) ExternalNetworks() ([]Network, error) {
	// This sucks, you cannot directly query for external networks...
	page, err := networks.List(c.client, &networks.ListOpts{}).AllPages()
	if err != nil {
		return nil, err
	}

	var results []Network

	if err := networks.ExtractNetworksInto(page, &results); err != nil {
		return nil, err
	}

	return results, nil
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

// Images returns a list of images.
func (c *ComputeClient) Images() ([]images.Image, error) {
	page, err := images.ListDetail(c.client, &images.ListOpts{}).AllPages()
	if err != nil {
		return nil, err
	}

	return images.ExtractImages(page)
}

// AvailabilityZones returns a list of availability zones.
func (c *ComputeClient) AvailabilityZones() ([]availabilityzones.AvailabilityZone, error) {
	page, err := availabilityzones.List(c.client).AllPages()
	if err != nil {
		return nil, err
	}

	return availabilityzones.ExtractAvailabilityZones(page)
}
