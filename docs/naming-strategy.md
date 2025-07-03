# Resource Naming Strategy for Multi-Instance Deployments

This document describes strategies for avoiding Kubernetes resource name collisions when managing multiple libvirt instances.

## The Problem

When managing VMs with the same name across multiple libvirt hosts, Kubernetes resource names can collide:

```yaml
# Both of these would try to create a K8s resource named "webserver"
- Host1: VM named "webserver" 
- Host2: VM named "webserver"
```

## Solution: Instance-Aware Naming

### Automatic Naming in Compositions

The multi-instance composition (`kvm-multiinstance-composition.yaml`) automatically prefixes Kubernetes resource names with the provider config name:

```yaml
# Input
metadata:
  name: webserver
spec:
  providerConfigRef: libvirt-host1

# Result
Kubernetes resource: libvirt-host1-webserver
Libvirt VM name: webserver
```

### Benefits

1. **No Collisions**: Resources on different hosts have unique K8s names
2. **Clear Ownership**: Resource names indicate which libvirt instance they belong to
3. **Original Names Preserved**: Libvirt resources keep their original names
4. **Namespace Support**: Combined with K8s namespaces for additional isolation

## Naming Strategies

### 1. Provider Config Prefix (Recommended)

Prefixes resources with the full provider config name:
- Pattern: `{providerConfig}-{resourceName}`
- Example: `libvirt-host1-webserver`

### 2. Host Prefix

Extracts hostname from provider config:
- Pattern: `{hostname}-{resourceName}`
- Example: `host1-webserver` (from `libvirt-host1`)

### 3. Manual Naming

For direct resource creation (not using compositions):
```yaml
# Manually include instance identifier
metadata:
  name: host1-webserver
  labels:
    libvirt.nourspeed.io/instance: host1
```

## Implementation Details

### Composition Patches

The composition uses patches to implement naming:

```yaml
patches:
  # K8s resource name with prefix
  - type: CombineFromComposite
    combine:
      variables:
        - fromFieldPath: spec.providerConfigRef
        - fromFieldPath: metadata.name
      strategy: string
      string:
        fmt: "%s-%s"
    toFieldPath: metadata.name
    
  # Original name for libvirt
  - fromFieldPath: spec.name
    toFieldPath: spec.forProvider.name
```

### Labels and Annotations

Resources are labeled for easy filtering:

```yaml
metadata:
  labels:
    libvirt.nourspeed.io/instance: libvirt-host1
  annotations:
    libvirt.nourspeed.io/original-name: webserver
```

## Best Practices

### 1. Use Consistent Naming

Choose a naming convention and stick to it:
- `{environment}-{role}`: `prod-webserver`, `dev-database`
- `{team}-{app}`: `platform-nginx`, `data-postgres`

### 2. Leverage Namespaces

Combine with Kubernetes namespaces for additional isolation:
```bash
# Team A's webserver on host1
kubectl -n team-a apply -f webserver.yaml

# Team B's webserver on host2  
kubectl -n team-b apply -f webserver.yaml
```

### 3. Use Compositions

Always use the multi-instance composition for automatic naming:
```yaml
apiVersion: nourspeed.io/v1alpha1
kind: XKvm
metadata:
  name: myapp  # Simple name - composition handles prefixing
spec:
  providerConfigRef: libvirt-prod-01
```

### 4. Query by Instance

Find all resources on a specific libvirt instance:
```bash
kubectl get domains,volumes,pools -l libvirt.nourspeed.io/instance=libvirt-host1
```

## Migration Guide

### From Single to Multi-Instance

1. **Update Compositions**: Switch to `kvm-multiinstance-composition.yaml`
2. **Update Claims**: No changes needed - composition handles naming
3. **Update Queries**: Use labels to filter by instance

### Handling Existing Resources

For resources created before implementing naming strategy:

1. Add instance labels manually:
   ```bash
   kubectl label domain/my-vm libvirt.nourspeed.io/instance=host1
   ```

2. Consider recreating with new naming:
   ```bash
   kubectl delete domain/my-vm
   kubectl apply -f vm-with-naming.yaml
   ```

## Limitations

- **String-based References**: Volume paths and network names are strings, not K8s references
- **External Resources**: Resources created outside K8s won't follow naming conventions
- **Name Length**: K8s names limited to 63 characters - long names may be truncated

## Future Enhancements

- **Configurable Strategy**: Provider-level naming strategy configuration
- **Automatic Migration**: Tools to rename existing resources
- **Validation**: Webhook warnings for resources without instance identification