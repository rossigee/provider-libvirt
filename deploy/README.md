# Provider-Libvirt v0.2.1 Deployment

This directory contains deployment manifests and testing scripts for provider-libvirt v0.2.1, which includes the new **NodeDevice** resource for hardware device passthrough.

## 🎯 New Features in v0.2.1

### NodeDevice Resource
- **GPU Passthrough**: Direct GPU assignment to VMs for ML/AI workloads
- **vGPU Support**: NVIDIA GRID mediated device creation for multi-tenant GPU sharing
- **USB Device Management**: USB device passthrough for security tokens, cameras, etc.
- **SCSI Device Support**: Storage device passthrough
- **PCI Device Control**: Generic PCI device management with VFIO driver support

## 📦 Quick Deployment

### Prerequisites
- Kubernetes cluster with Crossplane installed
- kubectl configured
- libvirt host accessible via SSH or network

### 1. Deploy Provider
```bash
./deploy-test.sh
```

### 2. Configure Connection
Edit the libvirt connection URI in the secret:
```bash
# Example connection URIs:
# Local: qemu:///system
# SSH: qemu+ssh://user@host/system
# TCP: qemu+tcp://host:16509/system

kubectl patch secret libvirt-credentials -n crossplane-system \
  --patch='{"data":{"uri":"'$(echo -n "qemu+ssh://user@host/system" | base64)'"}}'
```

### 3. Test NodeDevice Resources
```bash
./test-nodedevice.sh
```

## 🔧 Manual Testing

### Check CRDs
```bash
kubectl get crd | grep nodedevice
# Expected: nodedevices.nodedevice.libvirt.crossplane.io
```

### Deploy Example Resources
```bash
kubectl apply -f nodedevice-test.yaml
```

### Monitor Resources
```bash
# Check NodeDevice status
kubectl get nodedevices -w

# Monitor provider logs
kubectl logs -n crossplane-system -l pkg.crossplane.io/provider=provider-libvirt -f

# Describe specific resource
kubectl describe nodedevice gpu-passthrough-test
```

## 📋 Example Use Cases

### 1. GPU Passthrough for ML Training
```yaml
apiVersion: nodedevice.libvirt.crossplane.io/v1alpha1
kind: NodeDevice
metadata:
  name: ml-gpu
spec:
  forProvider:
    name: pci_0000_01_00_0
    type: pci
    pciAddress:
      domain: "0x0000"
      bus: "0x01"
      slot: "0x00"
      function: "0x0"
    detached: true
    driver: vfio-pci
  providerConfigRef:
    name: default
```

### 2. Multi-tenant vGPU
```yaml
apiVersion: nodedevice.libvirt.crossplane.io/v1alpha1
kind: NodeDevice
metadata:
  name: vgpu-tenant-1
spec:
  forProvider:
    type: mdev
    mediatedDevice:
      parent: pci_0000_02_00_0
      type: nvidia-63
    persistent: true
  providerConfigRef:
    name: default
```

### 3. USB Security Token
```yaml
apiVersion: nodedevice.libvirt.crossplane.io/v1alpha1
kind: NodeDevice
metadata:
  name: security-token
spec:
  forProvider:
    type: usb
    usbDevice:
      vendorID: "20a0"
      productID: "4108"
    detached: false
  providerConfigRef:
    name: default
```

## 🚨 Requirements

### Host Configuration
- **IOMMU**: Enable Intel VT-d or AMD-Vi in BIOS
- **VFIO**: Load vfio-pci kernel module
- **Device Isolation**: Isolate devices using kernel parameters

### Kernel Parameters Example
```bash
# Intel
intel_iommu=on vfio-pci.ids=10de:1b80

# AMD
amd_iommu=on vfio-pci.ids=10de:1b80
```

### libvirt Host Permissions
Ensure the connection user has proper permissions:
```bash
# Add user to libvirt group
sudo usermod -a -G libvirt username

# Or configure SASL authentication for network access
```

## 🔍 Troubleshooting

### Provider Not Starting
```bash
kubectl describe provider provider-libvirt
kubectl logs -n crossplane-system -l pkg.crossplane.io/provider=provider-libvirt
```

### Connection Issues
```bash
# Test libvirt connection manually
virsh -c qemu+ssh://user@host/system list

# Check secret
kubectl get secret libvirt-credentials -n crossplane-system -o yaml
```

### Device Not Found
```bash
# List available devices on host
virsh nodedev-list --tree

# Check specific device
virsh nodedev-dumpxml pci_0000_01_00_0
```

### VFIO Issues
```bash
# Check IOMMU groups
find /sys/kernel/iommu_groups -name "devices" -exec ls -la {} \;

# Verify VFIO driver
lsmod | grep vfio
```

## 📊 Monitoring

### Resource Status
```bash
kubectl get nodedevices -o custom-columns="NAME:.metadata.name,TYPE:.spec.forProvider.type,DETACHED:.spec.forProvider.detached,STATUS:.status.conditions[?(@.type=='Ready')].status"
```

### Provider Health
```bash
kubectl get providers provider-libvirt -o yaml | grep -A 10 conditions
```

## 🧹 Cleanup

```bash
# Remove test resources
kubectl delete -f nodedevice-test.yaml

# Remove provider
kubectl delete provider provider-libvirt

# Remove runtime config
kubectl delete deploymentruntimeconfig provider-libvirt-runtime-config

# Remove credentials
kubectl delete secret libvirt-credentials -n crossplane-system
```

## 🔗 Related Resources

- [Crossplane Documentation](https://crossplane.io/docs/)
- [libvirt Documentation](https://libvirt.org/docs.html)
- [VFIO Documentation](https://www.kernel.org/doc/html/latest/driver-api/vfio.html)
- [NVIDIA GRID vGPU](https://docs.nvidia.com/grid/)

## 🎉 Enterprise Features

The NodeDevice implementation provides enterprise-grade hardware passthrough capabilities:

- **Security**: Proper device isolation and validation
- **Observability**: Comprehensive status reporting and connection details
- **Flexibility**: Support for multiple device types and configurations
- **Reliability**: Robust error handling and state management
- **Scalability**: Efficient resource management for multiple devices