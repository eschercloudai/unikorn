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

_NB_: The Makefile in this repository makes use of conventions specific to GNU/Make.  If you're on a non-GNU system (i.e macOS, FreeBSD) then you need to replace `make` in the examples below with `gmake`.

### Building and Installing the Binaries

Checkout the repository:

```shell
git clone git@github.com:echercloudai/unikorn
cd unikorn
make touch # See the Makefile comments for why
```

Build the binaries and install them:

```shell
make install
```

Please note that the `install` target expects ~/bin to exist and be in your PATH.
You can customize this with `sudo make install -e PREFIX /usr/loca/bin` if that is your desire.

#### Setting Up Shell Completion

Obviously this works as `kubectl` does to avoid mistakes, do something like:

```shell
export TEMP=$(mktemp)
unikornctl completion bash > ${TEMP}
source ${TEMP}
```

For the more adventurous, you can add it to `/etc/bash_completion.d/` or whatever you use.

### Creating Docker Images

Images are built via [Docker buildx](https://docs.docker.com/build/buildx/install/), you should install this first in order to be able to reproduce the following steps.

#### Public Cloud

WHen operating in the Cloud, you'll want to push images to a public registry:

```shell
docker login
make images-push -e DOCKER_ORG=spjmurray
```

Please note, you are using a "non-standard" organization, so will need to alter some manifests later on.

#### Local Development

If you are doing local development, and using `kind` or similar, you can omit the prior step and use the following:

```shell
make images-kind-load
```

### Setting Up Kubernetes

#### Installing

Is all done via Helm, which means we can also deploy using ArgoCD.
You can install using the local repo:

```shell
helm install unikorn charts/unikorn --namespace unikorn --create-namespace --set repository=null --set tag=0.0.0
```

If you are installing this on a cloud somewhere, you will most likely need to update the images so that the registry and organization match what you are using.
The above example shows how this works with the default images that are created.

#### Monitoring

Can be enabled with the `--set monitoring.enabled=true` flag.
See the [monitoring](docs/monitoring.md) documentation from more information.

#### LoadBalancer Service Support

On local development environments, these services aren't supported out of the box.
There's a script provided that will setup Metallb for you if required e.g. kubectl access to the CAPI control plane:

```shell
go run hack/install_metallb
```

## Documentation

All the best tools document themselves, try:

```shell
unikornctl --help
unikornctl create --help
```
