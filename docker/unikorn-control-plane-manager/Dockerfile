FROM scratch

COPY bin/amd64-linux-gnu/unikorn-control-plane-manager /
COPY manifests/controlplane /manifests/
COPY hack/passwd.nonroot /etc/passwd

USER 1000

ENTRYPOINT ["/unikorn-control-plane-manager"]
