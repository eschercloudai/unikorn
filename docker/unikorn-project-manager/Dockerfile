FROM scratch

# This is implcitly created by 'docker buildx build'
ARG TARGETARCH

COPY bin/${TARGETARCH}-linux-gnu/unikorn-project-manager /
COPY hack/passwd.nonroot /etc/passwd

USER 1000

ENTRYPOINT ["/unikorn-project-manager"]
