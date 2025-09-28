# NodeDevice Resource Design

## Overview
NodeDevice resource enables enterprise-grade hardware device passthrough in libvirt environments. Critical for GPU compute, high-performance networking, and specialized hardware access.

## Resource Structure

```go
// NodeDeviceSpec defines the desired state of NodeDevice
type NodeDeviceSpec struct {
    xpv1.ResourceSpec `json:",inline"`
    ForProvider       NodeDeviceParameters `json:"forProvider"`
}

// NodeDeviceParameters are the configurable fields of a NodeDevice.
type NodeDeviceParameters struct {
    // Device name (auto-discovered or user-specified)
    // +kubebuilder:validation:Optional
    Name string `json:"name,omitempty"`

    // Device type: pci, usb, scsi, mdev
    // +kubebuilder:validation:Required
    // +kubebuilder:validation:Enum=pci;usb;scsi;mdev;net;storage
    Type string `json:"type"`

    // Device address for PCI devices
    // +kubebuilder:validation:Optional
    PCIAddress *PCIAddress `json:"pciAddress,omitempty"`

    // USB device identification
    // +kubebuilder:validation:Optional
    USBDevice *USBDevice `json:"usbDevice,omitempty"`

    // SCSI device identification
    // +kubebuilder:validation:Optional
    SCSIDevice *SCSIDevice `json:"scsiDevice,omitempty"`

    // Mediated device configuration
    // +kubebuilder:validation:Optional
    MediatedDevice *MediatedDevice `json:"mediatedDevice,omitempty"`

    // Device management settings
    // +kubebuilder:validation:Optional
    // +kubebuilder:default=true
    Managed bool `json:"managed,omitempty"`

    // Detach device from host driver for passthrough
    // +kubebuilder:validation:Optional
    // +kubebuilder:default=false
    Detached bool `json:"detached,omitempty"`

    // Driver to use for detached device (e.g., "vfio-pci")
    // +kubebuilder:validation:Optional
    Driver string `json:"driver,omitempty"`

    // Autostart device on host boot
    // +kubebuilder:validation:Optional
    // +kubebuilder:default=false
    Autostart bool `json:"autostart,omitempty"`

    // Persistent device definition
    // +kubebuilder:validation:Optional
    // +kubebuilder:default=true
    Persistent bool `json:"persistent,omitempty"`
}

// PCIAddress represents PCI device addressing
type PCIAddress struct {
    // PCI domain
    // +kubebuilder:validation:Required
    Domain string `json:"domain"`

    // PCI bus
    // +kubebuilder:validation:Required
    Bus string `json:"bus"`

    // PCI slot
    // +kubebuilder:validation:Required
    Slot string `json:"slot"`

    // PCI function
    // +kubebuilder:validation:Required
    Function string `json:"function"`
}

// USBDevice represents USB device identification
type USBDevice struct {
    // Vendor ID
    // +kubebuilder:validation:Optional
    VendorID string `json:"vendorId,omitempty"`

    // Product ID
    // +kubebuilder:validation:Optional
    ProductID string `json:"productId,omitempty"`

    // USB bus number
    // +kubebuilder:validation:Optional
    Bus int32 `json:"bus,omitempty"`

    // USB device number
    // +kubebuilder:validation:Optional
    Device int32 `json:"device,omitempty"`
}

// SCSIDevice represents SCSI device identification
type SCSIDevice struct {
    // SCSI adapter name
    // +kubebuilder:validation:Required
    Adapter string `json:"adapter"`

    // SCSI bus
    // +kubebuilder:validation:Required
    Bus int32 `json:"bus"`

    // SCSI target
    // +kubebuilder:validation:Required
    Target int32 `json:"target"`

    // SCSI unit/LUN
    // +kubebuilder:validation:Required
    Unit int32 `json:"unit"`
}

// MediatedDevice represents mediated device (mdev) configuration
type MediatedDevice struct {
    // Parent device name
    // +kubebuilder:validation:Required
    Parent string `json:"parent"`

    // Mediated device type (e.g., "nvidia-63")
    // +kubebuilder:validation:Required
    Type string `json:"type"`

    // UUID for the mediated device
    // +kubebuilder:validation:Optional
    UUID string `json:"uuid,omitempty"`
}

// NodeDeviceStatus defines the observed state of NodeDevice
type NodeDeviceStatus struct {
    xpv1.ResourceStatus `json:",inline"`
    AtProvider          NodeDeviceObservation `json:"atProvider,omitempty"`
}

// NodeDeviceObservation are the observable fields of a NodeDevice.
type NodeDeviceObservation struct {
    // Device name as known to libvirt
    Name string `json:"name,omitempty"`

    // Device state: active, inactive, detached
    State string `json:"state,omitempty"`

    // Parent device (for dependent devices)
    Parent string `json:"parent,omitempty"`

    // Driver currently bound to device
    Driver string `json:"driver,omitempty"`

    // Device capabilities
    Capabilities []NodeDeviceCapability `json:"capabilities,omitempty"`

    // IOMMU group information (for PCI devices)
    IOMMUGroup *IOMMUGroup `json:"iommuGroup,omitempty"`

    // NUMA node affinity
    NUMANode int32 `json:"numaNode,omitempty"`

    // Device path in sysfs
    SysfsPath string `json:"sysfsPath,omitempty"`
}

// NodeDeviceCapability represents device capability information
type NodeDeviceCapability struct {
    // Capability type
    Type string `json:"type"`

    // Additional capability data (JSON)
    Data map[string]string `json:"data,omitempty"`
}

// IOMMUGroup represents IOMMU grouping information
type IOMMUGroup struct {
    // IOMMU group number
    Number int32 `json:"number"`

    // Other devices in the same IOMMU group
    Devices []string `json:"devices,omitempty"`
}
```

## Key Features

### 1. **Multi-Device Type Support**
- **PCI**: Graphics cards, network cards, storage controllers
- **USB**: Specialized hardware, security tokens, cameras
- **SCSI**: Storage devices, tape drives
- **mdev**: GPU virtualization, VFIO mediated devices

### 2. **Enterprise Management**
- **Automatic Detection**: Discover available devices by capability
- **Lifecycle Management**: Detach/reattach for passthrough
- **Persistent Configuration**: Survive host reboots
- **IOMMU Awareness**: Handle IOMMU group dependencies

### 3. **Integration Patterns**
```yaml
# GPU Passthrough Example
apiVersion: libvirt.m.crossplane.io/v1beta1
kind: NodeDevice
metadata:
  name: gpu-nvidia-rtx4090
spec:
  forProvider:
    type: pci
    pciAddress:
      domain: "0x0000"
      bus: "0x01"
      slot: "0x00"
      function: "0x0"
    detached: true
    driver: "vfio-pci"
    managed: true
    persistent: true
---
# Domain using the GPU
apiVersion: libvirt.m.crossplane.io/v1beta1
kind: Domain
metadata:
  name: ml-workstation
spec:
  forProvider:
    # ... other domain config
    hostDevices:
    - nodeDeviceRef:
        name: gpu-nvidia-rtx4090
      type: pci
```

### 4. **Mediated Device Support**
```yaml
# GPU vGPU Creation
apiVersion: libvirt.m.crossplane.io/v1beta1
kind: NodeDevice
metadata:
  name: vgpu-instance-1
spec:
  forProvider:
    type: mdev
    mediatedDevice:
      parent: "pci_0000_01_00_0"
      type: "nvidia-63"  # GRID vGPU profile
    persistent: true
    autostart: true
```

## Implementation Priority

1. **Phase 1**: PCI device support (GPUs, NICs)
2. **Phase 2**: USB device support
3. **Phase 3**: Mediated device support (vGPU)
4. **Phase 4**: SCSI device support

## Enterprise Value

- **🚀 Performance**: Direct hardware access for compute/storage/networking
- **🔧 Flexibility**: Support for specialized hardware requirements
- **📊 Monitoring**: Device status and capability reporting
- **🛡️ Safety**: IOMMU group validation, managed device lifecycle
- **🔄 Integration**: Seamless domain assignment through cross-resource references