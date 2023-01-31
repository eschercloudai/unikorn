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

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
)

// NetworkClient wraps the generic client because gophercloud is unsafe.
type NetworkClient struct {
	client *gophercloud.ServiceClient
}

// NewNetworkClient provides a simple one-liner to start networking.
func NewNetworkClient(provider Provider) (*NetworkClient, error) {
	providerClient, err := provider.Client()
	if err != nil {
		return nil, err
	}

	client, err := openstack.NewNetworkV2(providerClient, gophercloud.EndpointOpts{})
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
