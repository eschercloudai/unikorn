FROM scratch

COPY bin/amd64-linux-gnu/unikorn-cluster-manager /
COPY hack/passwd.nonroot /etc/passwd

USER 1000

ENTRYPOINT ["/unikorn-cluster-manager"]
