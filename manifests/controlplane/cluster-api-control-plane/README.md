# Cluster API Control Plane

This is typically installed with `clusterctl init`.
Upon further inspection, this just installs directly from GitHub.
Grab the core components with:

```sh
wget https://github.com/kubernetes-sigs/cluster-api/releases/download/v1.2.4/control-plane-components.yaml -O manifest.yaml
```
