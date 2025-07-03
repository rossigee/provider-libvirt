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
   kubectl apply -f ../compositions/kvm-composition.yaml
   ```

3. Create VMs on different hosts:
   ```bash
   kubectl apply -f vm-on-host1.yaml
   kubectl apply -f vm-on-host2.yaml
   ```

## Key Features

- **Configurable storage paths**: Each host can have different storage locations
- **Network flexibility**: Specify different networks per host (default, br0, etc.)
- **Provider isolation**: Each VM explicitly targets a specific libvirt host
- **Resource parameterization**: Memory, CPU, and other specs are configurable

## Best Practices

1. Use descriptive ProviderConfig names that indicate the target host
2. Configure storage paths to match the actual libvirt host filesystem
3. Ensure network names exist on the target libvirt hosts
4. Use resource naming that includes the target host for clarity