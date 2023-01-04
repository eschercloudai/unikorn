FROM ubuntu:jammy as base

RUN apt update \
 && apt -y install ca-certificates

FROM scratch

# This is implcitly created by 'docker buildx build'
ARG TARGETARCH

COPY --from=base /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY bin/${TARGETARCH}-linux-gnu/unikorn-server /
COPY hack/passwd.nonroot /etc/passwd

USER 1000

ENTRYPOINT ["/unikorn-server"]
