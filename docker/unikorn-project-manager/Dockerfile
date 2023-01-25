FROM --platform=${BUILDPLATFORM:-linux/amd64} golang:1.19.1 as builder

ARG TARGETPLATFORM
ARG BUILDPLATFORM
ARG TARGETOS
ARG TARGETARCH

ARG MODULE
ARG VERSION
ARG REVISION

WORKDIR /app/
ADD . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags \
	'-X $(MODULE)/pkg/constants.Version=$(VERSION) -X $(MODULE)/pkg/constants.Revision=$(REVISION)' \
	-o unikorn-project-manager cmd/unikorn-project-manager/main.go

FROM --platform=${TARGETPLATFORM:-linux/amd64} scratch
WORKDIR /app/
COPY --from=builder /app/unikorn-project-manager /unikorn-project-manager
COPY hack/passwd.nonroot /etc/passwd
USER 1000
ENTRYPOINT ["/unikorn-project-manager"]
