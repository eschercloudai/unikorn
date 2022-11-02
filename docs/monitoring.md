# Monitoring and Logging

All Unikorn controllers support prometheus metrics, and promtail/loki compatible logs out of the box.
Logs in particular are tagged with the instance they relate to and a specific GUID associated with the reconcile request.
As such a nice database is in order to make sense of them (unless you can read JSON...)

## Performance Metrics

When writing provisioners and other things, it's useful to keep a histogram of run-times so that you can spot any anomalies.
This is especially pertinent when OpenStack is in play as it's pretty flaky, and things like load-balancers and ingresses can get stuck very easily.
All metric names should be prefixed with `unikorn_` for easy identification, then some other context to make things totally obvious.

You will most likely want to install Prometheus and Grafana:

```shell
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm install prometheus prometheus-community/kube-prometheus-stack --namespace monitoring --create-namespace
```

This will pretty much work out of the box.
See [`manifests/prometheus.yaml`](https://github.com/eschercloudai/unikorn/blob/main/manifests/prometheus.yaml) for an example of use, using the wonderful Prometheus Operator.

## Logs

Coming soon!

For now, use `kubectl logs -f deployment/unikorn-*`.
You can boost the verbosity by adding the follwing to the controll manifests:

```yaml
args:
- -zap-log-level=debug
```

Additionalal options/details you can get from `./bin/amd64-linux-gnu/unikorn-control-plane-manager --help`.
