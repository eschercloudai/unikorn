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
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/utils/openstack/clientconfig"
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

// ComputeClient provides a simple one-liner to start computing.
func ComputeClient(cloud string) (*gophercloud.ServiceClient, error) {
	provider, err := providerClient(cloud)
	if err != nil {
		return nil, err
	}

	return openstack.NewComputeV2(provider, gophercloud.EndpointOpts{})
}

// NetworkClient provides a simple one-liner to start networking.
func NetworkClient(cloud string) (*gophercloud.ServiceClient, error) {
	provider, err := providerClient(cloud)
	if err != nil {
		return nil, err
	}

	return openstack.NewNetworkV2(provider, gophercloud.EndpointOpts{})
}
