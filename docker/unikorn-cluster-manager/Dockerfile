FROM ubuntu:jammy as base

RUN apt update \
 && apt -y install ca-certificates

FROM scratch

# This is implcitly created by 'docker buildx build'
ARG TARGETARCH

# Required by the SNAT hack, so CAPI can talk to Kubernetes when a firewall
# is placed in the way.
COPY --from=base /etc/ssl/certs/ca-certificates.crt /ca-certificates.crt
COPY bin/${TARGETARCH}-linux-gnu/unikorn-cluster-manager /
COPY hack/passwd.nonroot /etc/passwd

ENV SSL_CERT_FILE=/ca-certificates.crt
USER 1000

ENTRYPOINT ["/unikorn-cluster-manager"]
