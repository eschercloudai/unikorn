FROM ubuntu:jammy as base

RUN apt update \
 && apt -y install ca-certificates

FROM scratch

# This is implcitly created by 'docker buildx build'
ARG TARGETARCH

# Required as we are talking to Openstack public endpoints.
COPY --from=base /etc/ssl/certs/ca-certificates.crt /ca-certificates.crt
COPY bin/${TARGETARCH}-linux-gnu/unikorn-server /
COPY hack/passwd.nonroot /etc/passwd

ENV SSL_CERT_FILE=/ca-certificates.crt
USER 1000

ENTRYPOINT ["/unikorn-server"]
