# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Crossplane provider for Libvirt, built using the Upjet code generation framework. It provides Kubernetes Custom Resource Definitions (CRDs) for managing libvirt resources like domains, volumes, networks, and storage pools through Crossplane's declarative infrastructure approach.

## Key Architecture

### Provider Structure
- **Terraform Integration**: Uses Upjet to generate Kubernetes controllers from the terraform-provider-libvirt
- **Resource Types**: Manages libvirt resources (domains, volumes, networks, pools, cloudinit disks) as Kubernetes CRDs
- **Controller Pattern**: Each resource type has its own controller in `internal/controller/`
- **API Versioning**: Uses both v1alpha1 (for managed resources) and v1beta1 (for provider configuration)

### Code Organization
- `apis/`: Generated API types for all resource kinds
- `config/`: Configuration for each resource type and provider setup
- `internal/controller/`: Generated controllers for each resource type
- `internal/clients/`: Terraform setup and credential handling
- `examples/`: YAML examples including compositions for multi-resource deployments
- `package/`: Generated CRD definitions

### Key Components
- **ProviderConfig**: Manages libvirt connection credentials (URI-based authentication)
- **Compositions**: Complex resource templates (see `examples/compositions/kvm-composition.yaml`)
- **External Name Strategy**: Maps Kubernetes resource names to libvirt resource identifiers

## Common Development Commands

### Build and Generate
```bash
# Run code generation (regenerates APIs and controllers)
go run cmd/generator/main.go "$PWD"

# Build the provider binary
make build

# Build and run locally against k8s cluster
make run
```

### Development Workflow
```bash
# Full build, push, and install pipeline
make all

# Run locally with debug logging
./bin/provider --debug
```

### Testing
```bash
# Run end-to-end tests (requires UPTEST_EXAMPLE_LIST env var)
make uptest

# Local development testing
make local-deploy
make e2e
```

## Important Technical Details

### Code Generation
- The provider uses Upjet's code generation from terraform-provider-libvirt schema
- Generated files are prefixed with `zz_` and should not be manually edited
- Schema is pulled from terraform provider version 0.7.6
- Generation requires terraform binary and provider schema

### Resource Configuration
- Each resource type has a config file in `config/` directory that customizes generation
- External name configurations are defined in `config/external_name.go`
- Resource-specific overrides are in files like `config/domain/config.go`

### Provider Setup
- Uses dmacvicar/terraform-provider-libvirt as the underlying terraform provider
- Credentials are provided via ProviderConfig with URI-based connection strings
- Supports terraform version 1.2.1 with specific provider version constraints

### Key Files
- `config/provider.go`: Main provider configuration and resource registration
- `internal/clients/libvirt.go`: Terraform setup and credential extraction
- `cmd/provider/main.go`: Provider binary entry point with CLI flags

## Recent Updates and Known Issues

### Webhook Validators
- Webhook validators in `internal/webhook/` have been updated to return `(admission.Warnings, error)` tuples
- Field names updated to follow Go conventions: `VolumeId` → `VolumeID`, `BaseVolumeId` → `BaseVolumeID`
- Webhook server configuration fixed in main.go to properly set the port

### Build Process
- The xpkg build may show an error about missing Docker blob when using `make`
- This is due to the `--controller` flag in the Upjet makefile trying to reference non-existent image layers
- Despite the error, all build artifacts (binary, Docker image, xpkg) are created successfully
- Workaround scripts available: `build-xpkg.sh`, `build.sh` for clean builds

### Deployment
- Use `deploy-to-cluster.sh` for automated deployment to Kubernetes clusters
- Requires Crossplane to be installed (script will install if missing)
- Provider expects libvirt URI credentials in a Secret (e.g., `qemu+ssh://user@host/system`)
- Example manifests:
  - `provider-install.yaml`: Provider installation with debug mode
  - `providerconfig-example.yaml`: ProviderConfig template with credential setup
  - `test-deployment.yaml`: Example Network resource for testing

### Multi-Instance Support
- Provider supports multiple libvirt instances via different ProviderConfigs
- Resource naming includes instance identifiers to prevent collisions
- Validation webhooks prevent cross-instance resource references