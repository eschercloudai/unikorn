# Unikorn

A badass, opinionated, deployer of souls!

![Unikorn](https://i.stack.imgur.com/EzZiD.png)

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
make && make install
```

Please note that the `install` target expects ~/bin to exist and be in your PATH.
You can customize this with `sudo make install -e PREFIX /usr/loca/bin` if that is your desire.

### Creating Docker Images

NOTE: this is a WIP and is unnecessary at present.

```shell
docker login
make images-push -e DOCKER_ORG=spjmurray
```

### Setting Up Shell Completion

Obviously this works as `kubectl` does to avoid mistakes, do something like:

```shell
export TEMP=$(mktemp)
unikornctl completion bash > ${TEMP}
source ${TEMP}
```

For the more adventurous, you can add it to `/etc/bash_completion.d/` or whatever you use.

### Setting Up Kubernetes

We use a few CRDs to make management easier, and long term, this command is likely to be
an API server that creates resources, and a set of microservice controllers will act on
the CRs.

```shell
kubectl apply -f crds
```

## Documentation

All the best tools document themselves, try:

```shell
unikornctl --help
unikornctl create --help
```
