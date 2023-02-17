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

### Set up shell completion

`unikornctl` has a large amount of convenience and contextual awareness built into its various subcommands.  It's strongly recommended to set up shell completion to make your life a lot easier.

#### bash

```shell
export TEMP=$(mktemp)
unikornctl completion bash > ${TEMP}
source ${TEMP}
```

For the more adventurous, you can add it to `/etc/bash_completion.d/` or whatever you use.

#### zsh

With zsh, the [recommendation](https://jzelinskie.com/posts/dont-recommend-sourcing-shell-completion/) is to do the following:

```shell
autoload -U +X compinit && compinit
unikornctl completion zsh > $fpath/_unikornctl
```

If you have a set of existing paths in `$fpath`, create the `_unikornctl` in your own custom completion function directory.  For example, if you had custom functions in `~/.zshfunc` then you would add the following to your `~/.zshenv`:

```
fpath=( ~/.zshfunc "${fpath[@]}" )
```

And then redirect the output of `unikornctl completion zsh` to `~/.zshfunc/_unikornctl`.

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

#### Installing Prerequisites

The Unikorn server component has a couple prerequisites that are required for correct functionality.
If not installing server you can skip to the next section.

You'll need to install:

* cert-manager (used to generate keying material for JWE/JWS and for ingress TLS)
* nginx-ingress (to perform routing, avoiding CORS, and TLS termination)

<details>
<summary>Helm</summary>

```shell
helm repo add jetstack https://charts.jetstack.io
helm repo add nginx https://helm.nginx.com/stable
helm repo update
helm install cert-manager jetstack/cert-manager -v v1.10.1 -n cert-manager --create-namespace
helm install nginx-ingress nginx/nginx-ingress -v 0.16.1 -n nginx-ingress --create-namespace
```
</details>

<details>
<summary>ArgoCD</summary>

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: cert-manager
  namespace: argocd
spec:
  project: default
  source:
    chart: cert-manager
    helm:
      parameters:
      - name: installCRDs
        value: "true"
      releaseName: cert-manager
    repoURL: https://charts.jetstack.io
    targetRevision: v1.10.1
  destination:
    name: in-cluster
    namespace: cert-manager
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
    - CreateNamespace=true
---
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: nginx-ingress
  namespace: argocd
spec:
  project: default
  source:
    chart: nginx-ingress
    helm:
      parameters:
      - name: controller.service.httpPort.enable
        value: "false"
      releaseName: nginx-ingress
    repoURL: https://helm.nginx.com/stable
    targetRevision: 0.16.1
  destination:
    name: in-cluster
    namespace: nginx-ingress
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
    - CreateNamespace=true
```
</details>


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
    targetRevision: 0.3.12
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

**Note:** Unikorn server is not installed by default.
To enable it add the parameter `--set server.enabled=true`.

## Monitoring & Logging

Can be enabled with the `--set monitoring.enabled=true` flag.
See the [monitoring & logging](docs/monitoring.md) documentation from more information.

## Documentation

### CLI

All the best tools document themselves, try:

```shell
unikornctl --help
unikornctl create --help
```

### API (Unikorn Server)

Consult the [server readme](pkg/server/README.md) to get started.
