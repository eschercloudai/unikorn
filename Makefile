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
GENDIR = generated
CRDDIR = crds

# Where to install things.
PREFIX = $(HOME)/bin

# List of binaries to build.
BINARIES := $(patsubst %,$(BINDIR)/%,$(COMMANDS))

# And where to install them to.
INSTALL_BINARIES := $(patsubst %,$(PREFIX)/%,$(COMMANDS))

# List of sources to trigger a build.
# TODO: Bazel may be quicker, but it's a massive hog, and a pain in the arse.
SOURCES := $(shell find . -type f -name *.go)

# Source files defining custom resource APIs
APISRC = $(shell find pkg/apis -name [^z]*.go -type f)

# Some bits about go.
GOPATH := $(shell go env GOPATH)
GOBIN := $(if $(shell go env GOBIN),$(shell go env GOBIN),$(GOPATH)/bin)

# Common linker flags.
FLAGS=-trimpath -ldflags '-X $(MODULE)/pkg/constants.Version=$(VERSION) -X $(MODULE)/pkg/constants.Revision=$(REVISION)'

# Defines the linter version.
LINT_VERSION=v1.50.0

# Defines the version of the CRD generation tools to use.
CONTROLLER_TOOLS_VERSION=v0.8.0

# Defines the version of code generator tools to use.
# This should be kept in sync with the Kubenetes library versions defined in go.mod.
CODEGEN_VERSION=v0.25.2

# This is the base directory to generate kubernetes API primitives from e.g.
# clients and CRDs.
GENAPIBASE = github.com/eschercloudai/unikorn/pkg/apis

# This is the list of APIs to generate clients for.
GENAPIS = $(GENAPIBASE)/unikorn/v1alpha1

# These are generic arguments that need to be passed to client generation.
GENARGS = --go-header-file hack/boilerplate.go.txt --output-base ../../..

# This controls the name of the client that will be generated and it will affect
# code import paths.  This overrides the default "versioned".
GENCLIENTNAME = unikorn

# This defines where clients will be generated.
GENCLIENTS = $(MODULE)/$(GENDIR)/clientset

# Main target, builds all binaries.
.PHONY: all
all: $(BINARIES) $(CRDDIR)

# Create a binary output directory, this should be an order-only prerequisite.
$(BINDIR):
	mkdir -p bin

# Create a binary from a command.
$(BINDIR)/%: $(SOURCES) $(GENDIR) | $(BINDIR)
	CGO_ENABLED=0 go build $(FLAGS) -o $@ $(CMDDIR)/$*/main.go

# Installation target, to test out things like shell completion you'll
# want to install it somewhere in your PATH.
.PHONY: install
install: $(INSTALL_BINARIES)

# Build a binary and install it.
$(PREFIX)/%: $(BINDIR)/%
	install -m 750 $< $@

# Create any CRDs defined into the target directory.
$(CRDDIR): $(APISRC)
	@mkdir -p $@
	@go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)
	$(GOBIN)/controller-gen crd crd:crdVersions=v1 paths=./pkg/apis/... output:dir=$@
	@touch $(CRDDIR)

# Generate a clientset to interact with our custom resources.
$(GENDIR): $(APISRC)
	@go install k8s.io/code-generator/cmd/deepcopy-gen@$(CODEGEN_VERSION)
	@go install k8s.io/code-generator/cmd/client-gen@$(CODEGEN_VERSION)
	$(GOBIN)/deepcopy-gen --input-dirs $(GENAPIS) -O zz_generated.deepcopy --bounding-dirs $(GENAPIBASE) $(GENARGS)
	$(GOBIN)/client-gen --clientset-name $(GENCLIENTNAME) --input-base "" --input $(GENAPIS) --output-package $(GENCLIENTS) $(GENARGS)
	@touch $(GENDIR)

# When checking out, the files timestamps are pretty much random, and make cause
# spurious rebuilds of generated content.  Call this to prevent that.
.PHONY: touch
touch:
	touch $(CRDDIR) $(GENDIR) pkg/apis/unikorn/v1alpha1/zz_generated.deepcopy.go

# Perform linting.
# This must pass or you will be denied by CI.
.PHOMY: lint
lint: $(GENDIR)
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(LINT_VERSION)
	$(GOBIN)/golangci-lint run --timeout 5m ./...

# Perform license checking.
# This must pass or you will be denied by CI.
.PHONY: license
license:
	go run ./hack/check_license/main.go
