FROM scratch

COPY bin/amd64-linux-gnu/unikorn-control-plane-manager /
COPY manifests/controlplane /manifests/

ENTRYPOINT ["/unikorn-control-plane-manager"]
