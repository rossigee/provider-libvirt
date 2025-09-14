/*
Copyright 2025 Ross Golder
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

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

	// Device type: pci, usb, scsi, mdev, net, storage
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

	// Device management settings - libvirt manages device lifecycle
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
	// PCI domain (usually 0x0000)
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^0x[0-9a-fA-F]{4}$`
	Domain string `json:"domain"`

	// PCI bus
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^0x[0-9a-fA-F]{2}$`
	Bus string `json:"bus"`

	// PCI slot
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^0x[0-9a-fA-F]{2}$`
	Slot string `json:"slot"`

	// PCI function
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^0x[0-9a-fA-F]$`
	Function string `json:"function"`
}

// USBDevice represents USB device identification
type USBDevice struct {
	// Vendor ID (hexadecimal)
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Pattern=`^0x[0-9a-fA-F]{4}$`
	VendorID string `json:"vendorId,omitempty"`

	// Product ID (hexadecimal)
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Pattern=`^0x[0-9a-fA-F]{4}$`
	ProductID string `json:"productId,omitempty"`

	// USB bus number
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=255
	Bus int32 `json:"bus,omitempty"`

	// USB device number
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=127
	Device int32 `json:"device,omitempty"`
}

// SCSIDevice represents SCSI device identification
type SCSIDevice struct {
	// SCSI adapter name
	// +kubebuilder:validation:Required
	Adapter string `json:"adapter"`

	// SCSI bus
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=0
	Bus int32 `json:"bus"`

	// SCSI target
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=0
	Target int32 `json:"target"`

	// SCSI unit/LUN
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=0
	Unit int32 `json:"unit"`
}

// MediatedDevice represents mediated device (mdev) configuration
type MediatedDevice struct {
	// Parent device name (must be an existing PCI device)
	// +kubebuilder:validation:Required
	Parent string `json:"parent"`

	// Mediated device type (e.g., "nvidia-63", "i915-GVTg_V5_4")
	// +kubebuilder:validation:Required
	Type string `json:"type"`

	// UUID for the mediated device (auto-generated if not specified)
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Pattern=`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`
	UUID string `json:"uuid,omitempty"`

	// Attributes for mdev creation
	// +kubebuilder:validation:Optional
	Attributes map[string]string `json:"attributes,omitempty"`
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

	// Parent device (for dependent devices like mdev)
	Parent string `json:"parent,omitempty"`

	// Driver currently bound to device
	Driver string `json:"driver,omitempty"`

	// Device capabilities detected by libvirt
	Capabilities []NodeDeviceCapability `json:"capabilities,omitempty"`

	// IOMMU group information (for PCI devices)
	IOMMUGroup *IOMMUGroup `json:"iommuGroup,omitempty"`

	// NUMA node affinity
	NUMANode int32 `json:"numaNode,omitempty"`

	// Device path in sysfs
	SysfsPath string `json:"sysfsPath,omitempty"`

	// Product information
	Product *DeviceProduct `json:"product,omitempty"`

	// Vendor information
	Vendor *DeviceVendor `json:"vendor,omitempty"`

	// Available mediated device types (for mdev-capable devices)
	MdevTypes []MdevType `json:"mdevTypes,omitempty"`
}

// NodeDeviceCapability represents device capability information
type NodeDeviceCapability struct {
	// Capability type (e.g., "pci", "usb", "scsi")
	Type string `json:"type"`

	// Additional capability-specific data
	Data map[string]string `json:"data,omitempty"`
}

// IOMMUGroup represents IOMMU grouping information for PCI devices
type IOMMUGroup struct {
	// IOMMU group number
	Number int32 `json:"number"`

	// Other devices in the same IOMMU group
	Devices []string `json:"devices,omitempty"`
}

// DeviceProduct represents device product information
type DeviceProduct struct {
	// Product ID (hexadecimal)
	ID string `json:"id"`

	// Product name/description
	Name string `json:"name"`
}

// DeviceVendor represents device vendor information
type DeviceVendor struct {
	// Vendor ID (hexadecimal)
	ID string `json:"id"`

	// Vendor name
	Name string `json:"name"`
}

// MdevType represents available mediated device types
type MdevType struct {
	// Mediated device type identifier
	ID string `json:"id"`

	// Human-readable name
	Name string `json:"name"`

	// Description of capabilities
	Description string `json:"description,omitempty"`

	// Available instances
	AvailableInstances int32 `json:"availableInstances"`

	// Device API type
	DeviceAPI string `json:"deviceApi,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="TYPE",type="string",JSONPath=".spec.forProvider.type"
// +kubebuilder:printcolumn:name="STATE",type="string",JSONPath=".status.atProvider.state"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// A NodeDevice is a libvirt host hardware device for passthrough.
type NodeDevice struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodeDeviceSpec   `json:"spec"`
	Status NodeDeviceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NodeDeviceList contains a list of NodeDevice
type NodeDeviceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodeDevice `json:"items"`
}

// NodeDeviceGroupKind is the GroupKind for NodeDevice
var NodeDeviceGroupKind = schema.GroupKind{
	Group: Group,
	Kind:  "NodeDevice",
}

// NodeDeviceGroupVersionKind is the GroupVersionKind for NodeDevice
var NodeDeviceGroupVersionKind = schema.GroupVersionKind{
	Group:   Group,
	Version: Version,
	Kind:    "NodeDevice",
}

// NodeDeviceKind is the kind for NodeDevice
const NodeDeviceKind = "NodeDevice"

func init() {
	SchemeBuilder.Register(&NodeDevice{}, &NodeDeviceList{})
}