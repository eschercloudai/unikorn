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

package provisioners

// ProvisionerMeta is a container for geneirc provisioner metadata.
type ProvisionerMeta struct {
	// Name is the name of the provisioner.
	Name string

	// Remote is the remote cluster a resource or group of resources
	// belongs to.
	Remote RemoteCluster

	// BackgroundDelete means we don't care about whether it's deprovisioned
	// successfully or not, especially useful for apps living in a
	// remote cluster that going to get terminated anyway.
	BackgroundDelete bool
}

// ProvisionerName implements the Provisioner interface.
func (p *ProvisionerMeta) ProvisionerName() string {
	return p.Name
}

func (p *ProvisionerMeta) OnRemote(remote RemoteCluster) {
	p.Remote = remote
}

func (p *ProvisionerMeta) BackgroundDeletion() {
	p.BackgroundDelete = true
}

// PropagateOptions allows provisioners to push options down to
// all their children.
func (p *ProvisionerMeta) PropagateOptions(provisioner Provisioner) {
	if p.Remote != nil {
		provisioner.OnRemote(p.Remote)
	}

	if p.BackgroundDelete {
		provisioner.BackgroundDeletion()
	}
}
