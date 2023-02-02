# Unikorn Developer

Developer-centric documentation.

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

When operating in the Cloud, you'll want to push images to a public registry:

```shell
docker login
make images-push -e DOCKER_ORG=spjmurray VERSION=$(git rev-parse HEAD)
```

Please note, you are using a "non-standard" organization, so will need to alter installation later on.
Additionally, we typically use unique "versions" aka tags for each build, thus avoiding caching.
You may want to just change the image pull policy in the Helm chart (i.e. write said functionality).

Your Helm configuration later should look like `--set repository=null --set organization=spjmurray --set tag=$(git rev-parse HEAD)`.

#### Local Development

If you are doing local development, and using `kind` or similar, you can omit the prior step and use the following:

```shell
make images-kind-load
```

The recommendation on macOS is to make use of [Colima](https://github.com/abiosoft/colima) and to create a single-node Kubernetes cluster sufficient for local development and test with the following:

```
colima start --cpu 4 --memory 8 --network-address --kubernetes
```

Images are built in the same VM and the same containerd namespace as the Kubernetes cluster that Colima provisions on your behalf, so there is no need for an equivalent command like `make images-kind-load` - images are immediately available in the cluster.

By default, development images will have a tag/version of `0.0.0` so as not to be confused with official releases.

Your Helm configuration later should look like `--set tag=0.0.0`.

### Setting Up Kubernetes

#### Installing

ArgoCD is a prerequisite, install with the following commands:

```
helm repo add argo https://argoproj.github.io/argo-helm
helm repo update
helm install argocd argo/argo-cd -n argocd --create-namespace
```

Once ArgoCD is up and running, retrieve the password for the `admin` account with the following command:

```
kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d
```

And then set up port forwarding so that the dashboard can be accessed locally via https://localhost:8080

```
kubectl -n argocd port-forward svc/argocd-server 8080:80 --address 0.0.0.0
```

Finally, create an ArgoCD Application definition in order to deploy Unikorn:

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
    repoURL: https://github.com/eschercloudai/unikorn.git
    targetRevision: 0.3.9
    helm:
      values: |
        tag: 0.0.0
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

#### LoadBalancer Service Support

On local development environments, these services aren't supported out of the box.
There's a script provided that will setup Metallb for you if require e.g. kubectl access to the CAPI control plane:

```shell
go run hack/install_metallb
```

There is no need to run this step when using a cluster provisioned using macOS via Colima, support is automatically provided out of the box for LoadBalancer Services as these inherit the host's IP address thanks to the use of [Klipper](https://docs.k3s.io/networking#service-load-balancer) which is bundled with K3s.
