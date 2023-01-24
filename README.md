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
wget -O ~/bin/unikornctl https://github.com/eschercloudai/unikorn/releases/download/0.2.0/unikornctl-linux-amd64
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

#### Installing ArgoCD

ArgoCD is a **required** to use Unikorn.

Deploy argo using Helm (the release name is _hard coded_, don't change it yet please):

```
helm repo add argo https://argoproj.github.io/argo-helm
helm repo update
helm install argocd argo/argo-cd -n argocd --create-namespace
```

To add the credentials go to `Settings`, `Repositories` and `Connect Repo`, then fill in:

* **Connection method**: `SSH`
* **Name**: `unikorn`
* **Project**: `default`
* **Repository URL**: `git@github.com:eschercloudai/unikorn`
* **SSH private key data**: the contents of `~/.ssh/id_blahBlahBlah`

#### Installing Unikorn

You can install using the local repo, or with CD.

First, unless you are doing local development and want to manually load images, you will want to configure image pull secrets in order to be able to pull the images.
Create a GitHub personal access token that has the `read:packages` scope, then add it to your `~/.docker/config.json`:

```
docker login ghcr.io --username spjmurray --password ghp_blahBlahBlah
```

Then install Unikorn:

<details>
<summary>Helm</summary>

```shell
helm install unikorn charts/unikorn --namespace unikorn --create-namespace --set dockerConfig=$(base64 -w0 ~/.docker/config.json)
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
    targetRevision: 0.3.6
    helm:
      parameters:
      - name: dockerConfig
        value: # output of "base64 -w0 ~/.docker/config.json"
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

## Monitoring & Logging

Can be enabled with the `--set monitoring.enabled=true` flag.
See the [monitoring & logging](docs/monitoring.md) documentation from more information.

## Documentation

All the best tools document themselves, try:

```shell
unikornctl --help
unikornctl create --help
```
