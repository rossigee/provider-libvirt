# Crossplane LibVirt Provider Examples

This directory contains comprehensive examples demonstrating the usage of the Crossplane LibVirt Provider with enhanced resource support and cross-resource integration.

## Features Demonstrated

### Cross-Resource Integration
- **Volume References**: Domains can reference Volume resources instead of using direct file paths
- **Network References**: Domains can reference Network resources instead of using direct network names
- **Backward Compatibility**: Legacy direct path/name configurations still work

### Resource Types

#### StoragePool
- Directory-based storage pools
- Automatic startup configuration
- Path management

#### Volume
- QCOW2 disk images
- Capacity specification
- Storage pool integration

#### Network
- NAT-based virtual networks
- DHCP configuration
- IP address range management

#### Domain
- KVM virtual machines
- Multi-disk configurations
- Multi-network interfaces
- Boot order management
- Console and graphics configuration

#### Secret
- Volume encryption secrets (LUKS/QCOW)
- Ceph authentication secrets
- iSCSI CHAP authentication secrets
- TLS certificate secrets

## Example Files

### `cross-resource-domain.yaml`
Demonstrates basic cross-resource integration:
- Creates a StoragePool, Volume, and Network
- Shows Domain configuration using resource references
- Includes legacy configuration example for comparison

### `complex-vm-setup.yaml` 
Advanced multi-VM setup with:
- Multiple storage pools (SSD and data)
- Multiple networks (DMZ and management)
- Multi-disk VM configurations
- Multi-network VM configurations
- Resource sharing between VMs

### `secret.yaml`
Comprehensive secret management examples:
- Volume encryption with LUKS passphrase
- Ceph authentication with monitor configuration
- iSCSI CHAP authentication
- TLS certificate management

## Usage

1. **Install the Provider**:
   ```bash
   kubectl apply -f package/crossplane.yaml
   ```

2. **Create Provider Configuration**:
   ```yaml
   apiVersion: libvirt.crossplane.io/v1alpha1
   kind: ProviderConfig
   metadata:
     name: default
   spec:
     credentials:
       source: Secret
       secretRef:
         namespace: crossplane-system
         name: libvirt-creds
         key: credentials
   ```

3. **Create Credentials Secret**:
   ```bash
   kubectl create secret generic libvirt-creds -n crossplane-system \
     --from-literal=credentials='{"uri":"qemu+tcp://libvirt-host:16509/system"}'
   ```

4. **Apply Examples**:
   ```bash
   kubectl apply -f examples/cross-resource-domain.yaml
   # or
   kubectl apply -f examples/complex-vm-setup.yaml
   # or
   kubectl apply -f examples/secret.yaml
   ```

## Resource Dependencies

### Creation Order
1. **StoragePool** → Must exist before Volumes
2. **Volume** → Must be Ready before Domain references
3. **Network** → Must be Ready before Domain references
4. **Secret** → Must be Ready before Volume or Domain references
5. **Domain** → Can reference ready Volumes, Networks, and Secrets

### Cross-References
- Domain disks can reference Volume resources via `volumeRef`
- Domain network interfaces can reference Network resources via `networkRef`
- Volume encryption can reference Secret resources via `secretRef`
- Resources must be in Ready state before being referenced

## Advanced Features

### Boot Configuration
```yaml
disk:
  - volumeRef:
      name: system-disk
    bootOrder: 1  # Primary boot device
  - volumeRef:
      name: data-disk
    bootOrder: 2  # Secondary boot device
```

### Network Interface Types
```yaml
networkInterface:
  - networkRef:
      name: nat-network     # Creates 'network' type interface
  - bridge: br0             # Creates 'bridge' type interface
  - networkName: default    # Creates 'network' type interface
```

### Storage Pool Types
```yaml
# Directory-based pool (most common)
spec:
  forProvider:
    type: dir
    path: /var/lib/libvirt/images/pool

# More pool types can be added as needed
```

### Secret Management
```yaml
# Volume encryption secret
spec:
  forProvider:
    type: volume
    usage: encryption
    data:
      volume:
        passphrase:
          name: "my-encryption-secret"
          namespace: "default"
          key: "passphrase"
        format: "luks"
    private: true  # Restrict access
    ephemeral: false  # Persistent

# Ceph authentication secret
spec:
  forProvider:
    type: ceph
    usage: authentication
    data:
      ceph:
        username: "ceph-user"
        key:
          name: "ceph-auth-secret"
          namespace: "default"
          key: "auth-key"
        monitors: ["mon1:6789", "mon2:6789"]
```

## Troubleshooting

### Volume Not Ready
If a Domain references a Volume that isn't ready:
```bash
kubectl describe volume <volume-name>
kubectl get volume <volume-name> -o yaml
```

### Network Not Ready
If a Domain references a Network that isn't ready:
```bash
kubectl describe network <network-name>
kubectl get network <network-name> -o yaml
```

### Domain Creation Failures
Check Domain status and events:
```bash
kubectl describe domain <domain-name>
kubectl get domain <domain-name> -o yaml
```

### Secret Not Ready
If a Volume references a Secret that isn't ready:
```bash
kubectl describe secret <secret-name>
kubectl get secret <secret-name> -o yaml
```

## Monitoring

Check resource status:
```bash
# List all resources
kubectl get storagepools,volumes,networks,secrets,domains

# Check specific resource details
kubectl describe domain <name>
kubectl describe volume <name>
kubectl describe network <name>
kubectl describe secret <name>
```

## Migration from Legacy Configuration

### Before (Direct Paths)
```yaml
disk:
  - file: /var/lib/libvirt/images/disk.qcow2
networkInterface:
  - networkName: default
```

### After (Resource References)
```yaml
disk:
  - volumeRef:
      name: my-volume
networkInterface:
  - networkRef:
      name: my-network
```

Both configurations are supported for backward compatibility.