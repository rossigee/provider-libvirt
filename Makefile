# Project Setup
PROJECT_NAME := provider-libvirt
PROJECT_REPO := github.com/rossigee/$(PROJECT_NAME)

PLATFORMS ?= linux_amd64 linux_arm64
-include build/makelib/common.mk

# Setup Output
-include build/makelib/output.mk

# Setup Go
# Override golangci-lint version for modern Go support
GOLANGCILINT_VERSION ?= 2.12.2
NPROCS ?= 1
GO_TEST_PARALLEL := $(shell echo $$(( $(NPROCS) / 2 )))
GO_LINT_ARGS = ./apis/... ./internal/...
GO_VERSION = 1.26.5
# Provider requires CGO for libvirt, so don't use GO_STATIC_PACKAGES
# GO_STATIC_PACKAGES = $(GO_PROJECT)/cmd/provider
GO_CGO_PACKAGES = $(GO_PROJECT)/cmd/provider
GO_LDFLAGS += -X $(GO_PROJECT)/internal/version.Version=$(VERSION)
GO_SUBDIRS += apis
GO111MODULE = on
GO_CGO_ENABLED = 0
-include build/makelib/golang.mk

# Setup Kubernetes tools
UP_VERSION = v0.28.0
UP_CHANNEL = stable
UPTEST_VERSION = v0.11.1
-include build/makelib/k8s_tools.mk

# UP is an alias for CROSSPLANE_CLI (legacy compatibility)
UP := $(CROSSPLANE_CLI)

# Setup Images
IMAGES = provider-libvirt
REGISTRY_ORGS = ghcr.io/rossigee
-include build/makelib/imagelight.mk

# Setup XPKG - Standardized registry configuration
# Primary registry: GitHub Container Registry under rossigee
XPKG_REG_ORGS ?= ghcr.io/rossigee
XPKG_REG_ORGS_NO_PROMOTE ?= ghcr.io/rossigee

# Optional registries (can be enabled via environment variables)
# Harbor publishing has been removed - using only ghcr.io/rossigee
# To enable Upbound: export ENABLE_UPBOUND_PUBLISH=true make publish XPKG_REG_ORGS=xpkg.upbound.io/crossplane-contrib
XPKGS = provider-libvirt
-include build/makelib/xpkg.mk

# NOTE: we force image building to happen prior to xpkg build so that we ensure
# image is present in daemon.
xpkg.build.provider-libvirt: do.build.images

# The plain runtime image and the xpkg package publish to the same tag
# (IMAGES and XPKGS are both "provider-libvirt"). The xpkg push must run
# last: it's the one that adds the io.crossplane.xpkg layer annotations
# Crossplane's package manager (and `crossplane xpkg extract`) require -
# a plain `docker push` after it would silently overwrite those away.
publish.artifacts:
	$(foreach r,$(REGISTRY_ORGS), $(foreach i,$(IMAGES),@$(MAKE) img.release.publish.$(r).$(i)))
	$(foreach r,$(XPKG_REG_ORGS), $(foreach x,$(XPKGS),@$(MAKE) xpkg.release.publish.$(r).$(x)))

# Setup Package Metadata
CROSSPLANE_VERSION = 2.3.2
-include build/makelib/local.xpkg.mk
-include build/makelib/controlplane.mk

# Targets

# run `make submodules` after cloning the repository for the first time.
submodules:
	@git submodule sync
	@git submodule update --init --recursive

# NOTE: the build submodule currently overrides XDG_CACHE_HOME in order to
# force the Helm 3 to use the .work/helm directory. This causes Go on Linux
# machines to use that directory as the build cache as well. We should adjust
# this behavior in the build submodule because it is also causing Linux users
# to duplicate their build cache, but for now we just make it easier to identify
# its location in CI so that we cache between builds.
go.cachedir:
	@go env GOCACHE

# NOTE: we must ensure up is installed in tool cache prior to build as including the k8s_tools
# machinery prior to the xpkg machinery sets UP to point to tool cache.
build.init: $(UP)

# This is for running out-of-cluster locally, and is for convenience. Running
# this make target will print out the command which was used. For more control,
# try running the binary directly with different arguments.
run: go.build
	@$(INFO) Running Crossplane locally out-of-cluster . . .
	@# To see other arguments that can be provided, run the command with --help instead
	$(GO_OUT_DIR)/provider --debug

# Cross-compiling CGO for a foreign arch needs that arch's cross-compiler and
# libvirt headers/libs (installed via dpkg --add-architecture in CI). Native
# arch (amd64 on an amd64 runner) needs none of this, hence the empty defaults.
CC_amd64 :=
CXX_amd64 :=
PKG_CONFIG_PATH_amd64 :=
CC_arm64 := aarch64-linux-gnu-gcc
CXX_arm64 := aarch64-linux-gnu-g++
PKG_CONFIG_PATH_arm64 := /usr/lib/aarch64-linux-gnu/pkgconfig

# Custom build target for CGO packages (overrides the static build)
go.build:
	@$(INFO) go build $(PLATFORM) with CGO
	@mkdir -p $(GO_OUT_DIR)
	$(eval CGO_ARCH := $(word 2,$(subst _, ,$(PLATFORM))))
	$(foreach p,$(GO_CGO_PACKAGES),@CGO_ENABLED=1 CC=$(CC_$(CGO_ARCH)) CXX=$(CXX_$(CGO_ARCH)) PKG_CONFIG_PATH=$(PKG_CONFIG_PATH_$(CGO_ARCH)) $(GO) build -v -o $(GO_OUT_DIR)/$(lastword $(subst /, ,$(p)))$(GO_OUT_EXT) $(GO_BUILDFLAGS) $(p) || $(FAIL) ${\n})
	@$(OK) go build $(PLATFORM) with CGO

# NOTE: we ensure up is installed prior to running platform-specific packaging steps in xpkg.build.
xpkg.build: $(UP)

# Install CRDs into a cluster
install-crds: generate
	kubectl apply -f package/crds

# Uninstall CRDs from a cluster
uninstall-crds:
	kubectl delete -f package/crds

# Prints the path to the Crossplane CLI this build uses (fetching it first
# if necessary), so CI steps outside the Makefile (e.g. release verification)
# can reuse the exact same binary instead of guessing at download URLs.
print.crossplane-cli: $(CROSSPLANE_CLI)
	@echo $(CROSSPLANE_CLI)

# Install examples into cluster
install-examples:
	kubectl apply -f examples/

# Delete examples from cluster
delete-examples:
	kubectl delete --ignore-not-found -f examples/

.PHONY: submodules run install-crds uninstall-crds install-examples delete-examples print.crossplane-cli
