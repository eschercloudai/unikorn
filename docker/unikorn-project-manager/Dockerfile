FROM scratch

COPY bin/amd64-linux-gnu/unikorn-project-manager /
COPY hack/passwd.nonroot /etc/passwd

USER 1000

ENTRYPOINT ["/unikorn-project-manager"]
