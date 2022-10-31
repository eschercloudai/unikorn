# Unikorn

A badass, opinionated, deployer of souls!

![Unikorn](https://i.stack.imgur.com/EzZiD.png)

## Overview

Unikorn abstracts away installation of Cluster API.

There are two resource types:

* Projects, that are a container for higher level abstractions.
* ControlPlanes, that basically are instances of Cluster API that live in Projects.

Control planes are actually contained themselves in virtual clusters, as CAPI is pretty terrible at cleaning things up on OpenStack errors, so we make these cattle.
One Kubernetes cluster to one instance of Cluster API.
If something goes wrong, just delete the virtual cluster and restart.
In future, when things get more stable, we can support many-to-one to save on resource costs, and even do away with virtual clusters entirely.

Projects allow multiple control planes to be contained within them.
These are useful for providing a boundary for billing etc.

Unsurprisingly, as we are dealing with custom resources, we are managing the lifecycles as Kubernetes controllers ("operator pattern" to those drinking the CoreOS Koolaid).

## Installation

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

#### Installing CRDs

We use a few CRDs to make management easier, and long term, this command is likely to be
an API server that creates resources, and a set of microservice controllers will act on
the CRs.

```shell
kubectl apply -f crds
```

#### Installing Cert Manager CRDs

tl;dr do this.
The long version is that dynamic clients will perform an API resource lookup on start up.
This will initialse REST mapping from APIVersion/Kind to REST endpoints.
Now, it transpires that you need to install Cert-Manager resources to provision Cluster API.
In order to do that, the REST mapper needs to know about `Certificate` custom resources on startup.
To facilitate this, just install them on the management cluster:

```shell
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.10.0/cert-manager.crds.yaml
```

While it is possible to redo a discovery when a CRD is provisioned, the controller-runtime makes it practically impossible without forcing a container restart.

#### Installing the Unikorn Controllers

There are a couple manifests -- one per controller -- in the `manifests` directory.
To install them:

```shell
kubectl apply -f manifests
```

If you are installing this on a cloud somewhere, you will most likely need to update the images so that the registry and organization match what you are using.

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
