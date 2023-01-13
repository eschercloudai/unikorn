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

package completion

import (
	"encoding/json"
	"strings"

	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/availabilityzones"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/keypairs"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/flavors"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/images"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	"github.com/gophercloud/utils/openstack/clientconfig"
	"github.com/spf13/cobra"

	"github.com/eschercloudai/unikorn/pkg/cmd/util/openstack"
)

// CloudCompletionFunc parses clouds.yaml and supplies matching cloud names.
func CloudCompletionFunc(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	clouds, err := clientconfig.LoadCloudsYAML()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var matches []string

	for name := range clouds {
		if strings.HasPrefix(name, toComplete) {
			matches = append(matches, name)
		}
	}

	return matches, cobra.ShellCompDirectiveNoFileComp
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

// OpenstackExternalNetworkCompletionFunc lists any matching external networks by ID.
// Yes this isn't particularly human friendly, but the ID is the only unique identifier.
// Names can alias which makes mapping from name to ID practically useless.
func OpenstackExternalNetworkCompletionFunc(cloud *string) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		client, err := openstack.NetworkClient(*cloud)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		// This sucks, you cannot directly query for external networks...
		page, err := networks.List(client, &networks.ListOpts{}).AllPages()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		var results []Network

		if err := networks.ExtractNetworksInto(page, &results); err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		var matches []string

		for _, network := range results {
			if network.External && strings.HasPrefix(network.Network.ID, toComplete) {
				matches = append(matches, network.Network.ID)
			}
		}

		return matches, cobra.ShellCompDirectiveNoFileComp
	}
}

// OpenstackSSHKeyCompletionFunc lists any matching ssh key pairs by name.
//
//nolint:dupl
func OpenstackSSHKeyCompletionFunc(cloud *string) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		client, err := openstack.ComputeClient(*cloud)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		page, err := keypairs.List(client, &keypairs.ListOpts{}).AllPages()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		results, err := keypairs.ExtractKeyPairs(page)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		var matches []string

		for _, keypair := range results {
			// TODO: there is a Type ("ssh") field, but it seems this library
			// is too old.
			if strings.HasPrefix(keypair.Name, toComplete) {
				matches = append(matches, keypair.Name)
			}
		}

		return matches, cobra.ShellCompDirectiveNoFileComp
	}
}

// OpenstackFlavorCompletionFunc lists any matching flavors by name.
//
//nolint:dupl
func OpenstackFlavorCompletionFunc(cloud *string) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		client, err := openstack.ComputeClient(*cloud)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		page, err := flavors.ListDetail(client, &flavors.ListOpts{}).AllPages()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		results, err := flavors.ExtractFlavors(page)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		var matches []string

		for _, flavor := range results {
			if strings.HasPrefix(flavor.Name, toComplete) {
				matches = append(matches, flavor.Name)
			}
		}

		return matches, cobra.ShellCompDirectiveNoFileComp
	}
}

// OpenstackImageCompletionFunc lists any matching images by name.
//
//nolint:dupl
func OpenstackImageCompletionFunc(cloud *string) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		client, err := openstack.ComputeClient(*cloud)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		page, err := images.ListDetail(client, &images.ListOpts{}).AllPages()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		results, err := images.ExtractImages(page)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		var matches []string

		for _, image := range results {
			if strings.HasPrefix(image.Name, toComplete) {
				matches = append(matches, image.Name)
			}
		}

		return matches, cobra.ShellCompDirectiveNoFileComp
	}
}

// OpenstackAvailabilityZoneCompletionFunc lists any matching availability zones by name.
func OpenstackAvailabilityZoneCompletionFunc(cloud *string) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		client, err := openstack.ComputeClient(*cloud)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		page, err := availabilityzones.List(client).AllPages()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		results, err := availabilityzones.ExtractAvailabilityZones(page)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		var matches []string

		for _, availabilityZone := range results {
			if strings.HasPrefix(availabilityZone.ZoneName, toComplete) {
				matches = append(matches, availabilityZone.ZoneName)
			}
		}

		return matches, cobra.ShellCompDirectiveNoFileComp
	}
}
