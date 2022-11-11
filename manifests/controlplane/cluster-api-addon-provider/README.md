# Cluster API Addon Provider

Homepage: https://github.com/stackhpc/cluster-api-addon-provider

Used during cluster bootstrap as a replacement for the (limited) `ClusterResourceSets`

Generated via Helm using:

```
helm template \
  cluster-api-addon-provider \
  capi-addons/cluster-api-addon-provider \
  --version ">=0.1.0-dev.0.main.0,<0.1.0-dev.0.main.9999999999"
```

The resulting manifest needs the `Namespace` object also defining, as well as adding the namespace to various resources which should be namespaced.  These are:

* The `ServiceAccount`
* The `ConfigMap`
* The `Deployment`
