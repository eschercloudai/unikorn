# Loft vcluster

This is usually installed with the `vcluster create` command.
As it happens if you look at the code, it's just invoking `helm` under the hood.
Thus to generate this manifest:

```sh
helm template unikorn vcluster --version 0.12.1 --repo https://charts.loft.sh --set service.type=LoadBalancer > manifest.yaml
```

**NOTE** the release must be called `unikorn`, the internal engine will search for this string and perform a replacement with an internal name based on the control plane.
