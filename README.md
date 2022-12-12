# Unikorn

A badass, opinionated, deployer of souls!

![Unikorn](https://i.stack.imgur.com/EzZiD.png)

## Overview

Unikorn abstracts away installation of Cluster API.

There are three resource types:

* Projects, that are a container for higher level abstractions.
* ControlPlanes, that basically are instances of Cluster API that live in Projects.
* Clusters, Kubernetes clusters.

Control planes are actually contained themselves in virtual clusters, as CAPI is pretty terrible at cleaning things up on OpenStack errors, so we make these cattle.
One Kubernetes cluster to one instance of Cluster API.
If something goes wrong, just delete the virtual cluster and restart.
In future, when things get more stable, we can support many-to-one to save on resource costs, and even do away with virtual clusters entirely.

Projects allow multiple control planes to be contained within them.
These are useful for providing a boundary for billing etc.

Unsurprisingly, as we are dealing with custom resources, we are managing the lifecycles as Kubernetes controllers ("operator pattern" to those drinking the CoreOS Koolaid).

## Installation

Consult the [developer documentation](DEVELOPER.md) for local development instructions.

### Installing the Management Binary

Download the official binary (update the version as appropriate):

```shell
wget -O ~/bin/unikornctl https://github.com/eschercloudai/unikorn/releases/download/0.1.0/unikornctl-linux-amd64
```

Set up shell completion:

```shell
export TEMP=$(mktemp)
unikornctl completion bash > ${TEMP}
source ${TEMP}
```

For the more adventurous, you can add it to `/etc/bash_completion.d/` or whatever you use.

### Installing the Service

Is all done via Helm, which means we can also deploy using ArgoCD.
As this is a private repository, we're keeping the charts private for now also, so you'll need to either checkout the correct branch for a local Helm installation, or imbue Argo with an access token to get access to the repository.

You can install using the local repo, or with CD:

<details>
<summary>Helm</summary>

```shell
helm install unikorn charts/unikorn --namespace unikorn --create-namespace
```
</details>

<details>
<summary>ArgoCD</summary>

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: unikorn
  namespace: argocd
spec:
  project: default
  source:
    path: charts/unikorn
    repoURL: git@github.com:eschercloudai/unikorn
    targetRevision: v0.1.0
  destination:
    namespace: unikorn
    server: https://kubernetes.default.svc
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
    - CreateNamespace=true
```
</details>

### Monitoring

Can be enabled with the `--set monitoring.enabled=true` flag.
See the [monitoring](docs/monitoring.md) documentation from more information.

## Documentation

All the best tools document themselves, try:

```shell
unikornctl --help
unikornctl create --help
```
