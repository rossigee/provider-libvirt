# Multi-Instance Libvirt Examples

This directory contains examples for managing multiple libvirt instances from a single Kubernetes cluster.

## Setup

1. Create ProviderConfigs for each libvirt host:
   ```bash
   kubectl apply -f ../providerconfig/libvirt-host1.yaml
   kubectl apply -f ../providerconfig/libvirt-host2.yaml
   ```

2. Apply the XRD and Composition:
   ```bash
   kubectl apply -f ../compositions/kvm-xrd.yaml
   # Use the multi-instance aware composition
   kubectl apply -f ../compositions/kvm-multiinstance-composition.yaml
   ```

3. Create VMs on different hosts:
   ```bash
   kubectl apply -f vm-with-auto-naming.yaml
   ```

## Key Features

- **Automatic naming**: Resources are automatically prefixed to prevent collisions
- **Configurable storage paths**: Each host can have different storage locations
- **Network flexibility**: Specify different networks per host (default, br0, etc.)
- **Provider isolation**: Each VM explicitly targets a specific libvirt host
- **Resource parameterization**: Memory, CPU, and other specs are configurable

## Naming Strategy

The multi-instance composition automatically handles naming to prevent collisions:

```yaml
# Your simple claim
metadata:
  name: webserver
spec:
  providerConfigRef: libvirt-host1

# Results in K8s resources named:
# - libvirt-host1-webserver (Domain)
# - libvirt-host1-volume-webserver (Volume)  
# - libvirt-host1-pool-webserver (Pool)
# - libvirt-host1-cloudinit-webserver (CloudInit)
```

## Best Practices

1. Use the multi-instance composition for automatic naming
2. Use descriptive ProviderConfig names that indicate the target host
3. Configure storage paths to match the actual libvirt host filesystem
4. Ensure network names exist on the target libvirt hosts
5. Use labels to query resources by instance:
   ```bash
   kubectl get all -l libvirt.nourspeed.io/instance=libvirt-host1
   ```

## Advanced Usage

### Different Namespaces
You can have identically named claims in different namespaces:
```bash
kubectl -n team-a apply -f webserver.yaml  # -> libvirt-host1-webserver
kubectl -n team-b apply -f webserver.yaml  # -> libvirt-host2-webserver
```

### Querying Resources
Find all resources on a specific host:
```bash
kubectl get domains,volumes,pools,disks \
  -l libvirt.nourspeed.io/instance=libvirt-host1
```

See the [naming strategy documentation](../../docs/naming-strategy.md) for more details.