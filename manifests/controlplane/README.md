# Control Plane Manifests

When this was initially written, we delegated the installation to things like `helm` or `clusterctl`.
WHy is this a bad idea?

* Reliance on external tooling in containers could lead to a greater surface area for supply-chain attacks.
* Tooling bloat.
* These tools inevitably require TLS CA certs, leading to more bloat.
* There are no guarantees that Helm doesn't change (e.g. push to master).
* It's just an arsehole to call out to `helm template` or `clusterctl` via exec system calls.
* You get very little control over what's in the manifest, if you can get it all, which means we cannot apply internal policies.

For that reason, we locally cache known good versions and include these files in containers.

## Components

The following are required at present, the links will tell you how to source the individual files.

* [Loft vcluster](vcluster/README.md)
* [JetStack Cert Manager](cert-manager/README.md)
* [Cluster API Core](cluster-api-core/README.md)
* [Cluster API Bootstrap](cluster-api-bootstrap/README.md)
* [Cluster API Control Plane](cluster-api-control-plane/README.md)
* [Cluster API Openstack Provider](cluster-api-provider-openstack/README.md)
