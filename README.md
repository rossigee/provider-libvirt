# provider-libvirt

A native Crossplane provider for libvirt that uses the Go libvirt API directly instead of terraform.

## Features

- ✅ **Pure Go Implementation** - No terraform dependencies
- ✅ **Direct libvirt API** - Uses DigitalOcean's go-libvirt for RPC communication
- ✅ **Domain Management** - Create, update, delete libvirt domains (VMs)
- ✅ **TLS Support** - Supports secure connections via qemu+ssh, qemu+tls
- ✅ **Multi-host Support** - Manage multiple libvirt hosts via ProviderConfigs

## Architecture

This provider eliminates the terraform dependency that plagued previous libvirt providers by:

1. **Direct API Access**: Uses `github.com/digitalocean/go-libvirt` for pure Go libvirt RPC communication
2. **Native Controllers**: Built with `crossplane-runtime` controllers, not terraform wrappers
3. **Efficient Networking**: Perfect for SSH/TLS connections like `qemu+ssh://user@host/system`

## Quick Start

### 1. Install the Provider

```yaml
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-libvirt
spec:
  package: ghcr.io/rossigee/provider-libvirt:latest
```

### 2. Create a ProviderConfig

```yaml
apiVersion: libvirt.crossplane.io/v1alpha1
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

### 3. Create a Domain (VM)

```yaml
apiVersion: libvirt.crossplane.io/v1alpha1
kind: Domain
metadata:
  name: test-vm
spec:
  forProvider:
    name: test-vm
    memory: 2147483648  # 2GB in bytes
    vcpu: 2
    running: true
    autostart: false
    disk:
    - file: /var/lib/libvirt/images/test-vm.qcow2
      type: virtio
    networkInterface:
    - networkName: default
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

## Comparison with terraform-based providers

| Feature | This Provider | terraform-provider-libvirt |
|---------|---------------|----------------------------|
| Dependencies | Pure Go | terraform + dmacvicar/libvirt |
| Registry Issues | ❌ None | ✅ Registry download problems |
| Container Size | Smaller | Larger (includes terraform) |
| Build Complexity | Simple | Complex (terraform + provider) |
| Connection Types | All libvirt URIs | All libvirt URIs |
| Performance | Direct RPC | RPC via terraform |

## Development

### Building

```bash
go build -o bin/provider ./cmd/provider
```

### Running locally

```bash
./bin/provider --debug
```

### Building Docker image

```bash
docker build -t provider-libvirt:latest .
```

## Supported Resources

- [x] **Domain** - Virtual machine management
- [ ] **Volume** - Storage volume management (planned)
- [ ] **Network** - Network management (planned)  
- [ ] **Pool** - Storage pool management (planned)

## Contributing

This provider is built to solve the terraform registry issues experienced with existing libvirt providers while providing a cleaner, more maintainable codebase.

## License

Licensed under the Apache License, Version 2.0.