# provider-libvirt

A **Crossplane v2 native provider** for libvirt that uses the Go libvirt API directly instead of terraform.

## Container Registry

- **Primary**: `ghcr.io/rossigee/provider-libvirt:v0.11.9`

## Features

- ✅ **Native implementation** - Uses the `libvirt.org/go/libvirt` CGO binding
- ✅ **Direct libvirt API** - Connects to libvirt over its supported URI transports
- ✅ **Crossplane v2 Native** - Supports namespaced resources with `.m.` API groups
- ✅ **Complete Resource Management** - Domains, Volumes, Networks, StoragePools, NodeDevices, Secrets
- ✅ **Cross-Resource References** - Domain disks can reference Volume resources, network interfaces reference Network resources
- ✅ **Cloud-Init Integration** - Volume provisioning from URLs and files for automated VM setup
- ✅ **TLS Support** - Supports secure connections via qemu+ssh, qemu+tls
- ✅ **Multi-host Support** - Manage multiple libvirt hosts via ProviderConfigs
- ✅ **Advanced Storage** - Support for multiple storage pool types (dir, fs, nfs, iscsi, lvm, rbd, gluster, zfs)
- ✅ **Enterprise Networking** - NAT, bridge, routed, and isolated network configurations

## Architecture

This provider uses native Crossplane controllers with the `libvirt.org/go/libvirt` binding. The release build requires CGO and the libvirt development headers; the runtime image includes the libvirt client libraries.

## Getting Started

### 1. Install the Provider

```yaml
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-libvirt
spec:
  package: ghcr.io/rossigee/provider-libvirt:v0.11.9
```

### 2. Create a ProviderConfig

```yaml
apiVersion: libvirt.m.crossplane.io/v1beta1
kind: ProviderConfig
metadata:
  name: my-libvirt-host
spec:
  credentials:
    source: Secret
    secretRef:
      name: libvirt-credentials
      namespace: crossplane-system
      key: uri
---
apiVersion: v1
kind: Secret
metadata:
  name: libvirt-credentials
  namespace: crossplane-system
type: Opaque
data:
  uri: cXFlbXUrc3NoOi8vdXNlckBob3N0L3N5c3RlbQ== # qemu+ssh://user@host/system
```

### 3. Create Storage and Network Resources

```yaml
# Create a storage pool
apiVersion: libvirt.m.crossplane.io/v1beta1
kind: StoragePool
metadata:
  name: vm-storage
  namespace: vms
spec:
  forProvider:
    name: vm-storage
    type: dir
    target:
      path: /var/lib/libvirt/images/vm-storage
  providerConfigRef:
    name: my-libvirt-host
---
# Create a volume with cloud-init from URL
apiVersion: libvirt.m.crossplane.io/v1beta1
kind: Volume
metadata:
  name: ubuntu-disk
  namespace: vms
spec:
  forProvider:
    name: ubuntu-22.04-server
    pool: vm-storage
    format: qcow2
    capacity: 21474836480  # 20GB
    source:
      url: https://cloud-images.ubuntu.com/releases/22.04/release/ubuntu-22.04-server-cloudimg-amd64.img
  providerConfigRef:
    name: my-libvirt-host
---
# Create a virtual network
apiVersion: libvirt.m.crossplane.io/v1beta1
kind: Network
metadata:
  name: vm-network
  namespace: vms
spec:
  forProvider:
    name: vm-network
    mode: nat
    ip:
      address: 192.168.100.1/24
    dhcp:
      enabled: true
      range:
        start: 192.168.100.100
        end: 192.168.100.200
  providerConfigRef:
    name: my-libvirt-host
```

### 4. Create a Domain (VM) with Resource References

```yaml
apiVersion: libvirt.m.crossplane.io/v1beta1
kind: Domain
metadata:
  name: test-vm
  namespace: vms
spec:
  forProvider:
    name: test-vm
    memory: 2147483648  # 2GB in bytes
    vcpu: 2
    running: true
    autostart: false
    # Reference the Volume resource (v2 native cross-resource feature)
    disk:
    - volumeRef:
        name: ubuntu-disk
      type: virtio
      bootOrder: 1
    # Reference the Network resource (v2 native cross-resource feature)
    networkInterface:
    - networkRef:
        name: vm-network
      model: virtio
      waitForLease: true
    console:
      type: pty
    graphics:
      type: spice
      listenAddress: 127.0.0.1
      autoport: true
  providerConfigRef:
    name: my-libvirt-host
```

## Crossplane v2 Native Features

This provider is built specifically for **Crossplane v2** and includes features not available in older providers:

### ✅ Namespace Isolation
```yaml
# Resources can be deployed in specific namespaces for multi-tenancy
apiVersion: libvirt.m.crossplane.io/v1beta1
kind: Domain
metadata:
  name: my-vm
  namespace: production  # Namespace isolation
```

### ✅ Cross-Resource References
```yaml
# Domains can reference other resources declaratively
disk:
- volumeRef:
    name: my-volume  # References Volume resource
networkInterface:
- networkRef:
    name: my-network  # References Network resource
```

### ✅ Cloud-Init Integration
```yaml
# Volumes can be provisioned from URLs for automated VM setup
source:
  url: https://cloud-images.ubuntu.com/releases/22.04/release/ubuntu-22.04-server-cloudimg-amd64.img
  file: /path/to/local/cloud-init.iso  # Or from local files
```

## Comparison with terraform-based providers

| Feature | This Provider | terraform-provider-libvirt |
|---------|---------------|----------------------------|
| **Architecture** | Native Crossplane v2 + libvirt CGO | Terraform + provider |
| **Registry Issues** | None | Registry download problems |
| **Namespace Support** | v2 native namespaced resources | Cluster-scoped only |
| **Cross-Resource Refs** | Native Kubernetes references | String-based references |
| **Cloud-Init** | URL/file provisioning | Manual setup required |
| **Build Complexity** | CGO toolchain and libvirt headers | Terraform and provider tooling |

## Development

### Prerequisites

- Go 1.27.1
- Docker for containerization
- libvirt development libraries and target headers for CGO builds

### Building

```bash
# Initialize build system
git submodule update --init --recursive

# Generate CRDs and build
make generate
make build

# Run full validation (lint, test, build)
make reviewable
```

### Running locally

```bash
./bin/provider --debug
```

### Building and Publishing

Release publication is tag-only. After the release PR is merged and `master` is green:

```bash
git fetch origin master
test "$(git rev-parse HEAD)" = "$(git rev-parse origin/master)"
test "$(<VERSION)" = "v0.11.9"
test -z "$(git show-ref --tags v0.11.9)"
test -z "$(git ls-remote --tags origin refs/tags/v0.11.9)"
git tag -a v0.11.9 -m "Release v0.11.9" HEAD
git push origin refs/tags/v0.11.9
```

### Testing

The provider includes comprehensive tests:

```bash
# Run unit tests
make test

# Run linting
make lint

# Full validation
make reviewable
```

**Current release**: `v0.11.9`

## Resource Types

All resources are namespaced (`libvirt.m.crossplane.io/v1beta1`) — there is
no separate cluster-scoped API version for this provider.

| Resource | Description |
|----------|-------------|
| Domain | Virtual machine management with full lifecycle control |
| Volume | Storage volume management with cloud-init URL/file provisioning |
| Network | Network management (NAT, bridge, routed, isolated modes) |
| StoragePool | Storage pool management (dir, fs, nfs, iscsi, lvm, rbd, gluster, zfs) |
| NodeDevice | Hardware device management for GPU/USB passthrough |
| Secret | Secure credential and certificate management |

### Resource Cross-References

The provider supports declaring dependencies between resources:

- **Volume → StoragePool**: Volumes reference storage pools for organization
- **Domain → Volume**: Domain disks can reference Volume resources via `volumeRef`
- **Domain → Network**: Network interfaces can reference Network resources via `networkRef`

## Contributing

This provider is built to solve the terraform registry issues experienced with existing libvirt providers while providing a cleaner, more maintainable codebase.

## License

Licensed under the Apache License, Version 2.0.