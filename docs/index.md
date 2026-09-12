# Provider Libvirt Documentation

A Crossplane v2 provider for managing Libvirt/KVM virtual infrastructure. All resources are namespaced `libvirt.m.crossplane.io/v1beta1` with multi-tenancy support.

> **Note**: This is a grandfathered provider (exception to the native-implementation rule).

## Quick Links

- [Configuration](configuration.md) — Authentication and connection setup
- [Getting Started](getting-started.md) — Installation and first resources
- [Development](development.md) — Building, testing, and contributing
- [Testing](testing.md) — Testing procedures

## Resource Documentation

### Compute & Storage

| Resource | API Group | Description |
|----------|-----------|-------------|
| Domain | `libvirt.m.crossplane.io/v1beta1` | Virtual machines |
| Volume | `libvirt.m.crossplane.io/v1beta1` | Storage volumes |
| StoragePool | `libvirt.m.crossplane.io/v1beta1` | Storage pools |

### Networking & Devices

| Resource | API Group | Description |
|----------|-----------|-------------|
| Network | `libvirt.m.crossplane.io/v1beta1` | Virtual networks |
| NodeDevice | `libvirt.m.crossplane.io/v1beta1` | Node devices (passthrough) |
| Secret | `libvirt.m.crossplane.io/v1beta1` | Libvirt secrets (e.g. Ceph keys) |

### Provider

| Resource | API Group | Description |
|----------|-----------|-------------|
| ProviderConfig | `libvirt.m.crossplane.io/v1beta1` | Connection URI (cluster-scoped) |

## API Coverage Gaps

libvirt API surface not yet modeled: snapshots/checkpoints as resources, network port management, storage volume cloning/upload progress, NWFilter rules, interface statistics, and storage pool auto-start configuration.
