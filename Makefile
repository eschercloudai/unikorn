# Application version encoded in all the binaries.
VERSION = 0.0.0

# Base go module name.
MODULE := $(shell cat go.mod | grep module | awk '{print $$2}')

# Git revision.
REVISION := $(shell git rev-parse HEAD)

# Commands to build.
COMMANDS = unikornctl

# Some constants to describe the repository.
BINDIR = bin
CMDDIR = cmd
SRCDIR = src

# List of binaries to build.
BINARIES := $(patsubst %,${BINDIR}/%,${COMMANDS})

# List of sources to trigger a build.
# TODO: Bazel may be quicker, but it's a massive hog, and a pain in the arse.
SOURCES := $(shell find . -type f -name *.go)

# Some bits about go.
GOPATH := $(shell go env GOPATH)
GOBIN := ${if $(shell go env GOBIN),$(shell go env GOBIN),${GOPATH}/bin}

# Common linker flags.
FLAGS=-trimpath -ldflags '-X ${MODULE}/pkg/constants.Version=${VERSION} -X ${MODULE}/pkg/constants.Revision=${REVISION}'

# Main target, builds all binaries.
.PHONY: all
all: ${BINARIES}

# Create a binary output directory, this should be an order-only prerequisite.
${BINDIR}:
	mkdir -p bin

# Create a binary from a command.
${BINDIR}/%: ${SOURCES} | ${BINDIR}
	CGO_ENABLED=0 go build ${FLAGS} -o $@ ${CMDDIR}/$*/main.go

# Perform linting.
# This must pass or you will be denied by CI.
.PHOMY: lint
lint:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.50.0
	${GOBIN}/golangci-lint run ./...
