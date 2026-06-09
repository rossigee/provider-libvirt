/*
Copyright 2025 Ross Golder
*/

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
)

// NodeDeviceSpec defines the desired state of NodeDevice
type NodeDeviceSpec struct {
	xpv1.ManagedResourceSpec `json:",inline"`
	ForProvider       NodeDeviceParameters `json:"forProvider"`
}

// NodeDeviceParameters are the configurable fields of a NodeDevice.
type NodeDeviceParameters struct {
	// Type of the node device (pci, usb, scsi, storage, net, mdev)
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=pci;usb;scsi;storage;net;mdev
	Type string `json:"type"`

	// Name of the node device (for existing devices)
	// +kubebuilder:validation:Optional
	Name string `json:"name,omitempty"`

	// PCI address for PCI devices
	// +kubebuilder:validation:Optional
	PCIAddress *PCIAddress `json:"pciAddress,omitempty"`

	// USB address for USB devices
	// +kubebuilder:validation:Optional
	USBAddress *USBAddress `json:"usbAddress,omitempty"`

	// SCSI address for SCSI devices
	// +kubebuilder:validation:Optional
	SCSIAddress *SCSIAddress `json:"scsiAddress,omitempty"`

	// Driver to bind/unbind the device
	// +kubebuilder:validation:Optional
	Driver string `json:"driver,omitempty"`

	// Detached determines if device should be detached from host
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	Detached bool `json:"detached,omitempty"`

	// Persistent determines if device definition is persistent
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	Persistent bool `json:"persistent,omitempty"`

	// Autostart determines if device starts automatically
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	Autostart bool `json:"autostart,omitempty"`

	// MediatedDevice configuration for mdev devices
	// +kubebuilder:validation:Optional
	MediatedDevice *MediatedDevice `json:"mediatedDevice,omitempty"`

	// USBDevice configuration for USB devices
	// +kubebuilder:validation:Optional
	USBDevice *USBDevice `json:"usbDevice,omitempty"`

	// SCSIDevice configuration for SCSI devices
	// +kubebuilder:validation:Optional
	SCSIDevice *SCSIDevice `json:"scsiDevice,omitempty"`
}

// PCIAddress represents a PCI device address
type PCIAddress struct {
	// Domain of the PCI address
	// +kubebuilder:validation:Required
	Domain string `json:"domain"`

	// Bus of the PCI address
	// +kubebuilder:validation:Required
	Bus string `json:"bus"`

	// Slot of the PCI address
	// +kubebuilder:validation:Required
	Slot string `json:"slot"`

	// Function of the PCI address
	// +kubebuilder:validation:Required
	Function string `json:"function"`
}

// USBAddress represents a USB device address
type USBAddress struct {
	// Bus number of the USB device
	// +kubebuilder:validation:Required
	Bus string `json:"bus"`

	// Device number of the USB device
	// +kubebuilder:validation:Required
	Device string `json:"device"`
}

// SCSIAddress represents a SCSI device address
type SCSIAddress struct {
	// Host of the SCSI address
	// +kubebuilder:validation:Required
	Host string `json:"host"`

	// Bus of the SCSI address
	// +kubebuilder:validation:Required
	Bus string `json:"bus"`

	// Target of the SCSI address
	// +kubebuilder:validation:Required
	Target string `json:"target"`

	// LUN of the SCSI address
	// +kubebuilder:validation:Required
	LUN string `json:"lun"`
}

// MediatedDevice represents mediated device configuration
type MediatedDevice struct {
	// Parent device for the mediated device
	// +kubebuilder:validation:Required
	Parent string `json:"parent"`

	// Type of the mediated device
	// +kubebuilder:validation:Required
	Type string `json:"type"`

	// UUID for the mediated device (optional, will be generated if not provided)
	// +kubebuilder:validation:Optional
	UUID string `json:"uuid,omitempty"`
}

// USBDevice represents USB device configuration
type USBDevice struct {
	// Vendor ID of the USB device
	// +kubebuilder:validation:Optional
	Vendor string `json:"vendor,omitempty"`

	// Product ID of the USB device
	// +kubebuilder:validation:Optional
	Product string `json:"product,omitempty"`

	// Bus number for the USB device
	// +kubebuilder:validation:Optional
	Bus string `json:"bus,omitempty"`

	// Device number for the USB device
	// +kubebuilder:validation:Optional
	Device string `json:"device,omitempty"`
}

// SCSIDevice represents SCSI device configuration
type SCSIDevice struct {
	// Host of the SCSI device
	// +kubebuilder:validation:Optional
	Host string `json:"host,omitempty"`

	// Bus of the SCSI device
	// +kubebuilder:validation:Optional
	Bus string `json:"bus,omitempty"`

	// Target of the SCSI device
	// +kubebuilder:validation:Optional
	Target string `json:"target,omitempty"`

	// LUN of the SCSI device
	// +kubebuilder:validation:Optional
	LUN string `json:"lun,omitempty"`
}

// NodeDeviceStatus defines the observed state of NodeDevice
type NodeDeviceStatus struct {
	xpv1.ManagedResourceStatus `json:",inline"`
	AtProvider          NodeDeviceObservation `json:"atProvider,omitempty"`
}

// NodeDeviceObservation are the observable fields of a NodeDevice.
type NodeDeviceObservation struct {
	// Name of the node device in libvirt
	Name string `json:"name,omitempty"`

	// Type of the node device
	Type string `json:"type,omitempty"`

	// Active status of the node device
	Active bool `json:"active,omitempty"`

	// State of the node device
	State string `json:"state,omitempty"`

	// Driver currently bound to the device
	Driver string `json:"driver,omitempty"`

	// Parent device name
	Parent string `json:"parent,omitempty"`

	// Capability of the device
	Capability string `json:"capability,omitempty"`

	// Path of the device
	Path string `json:"path,omitempty"`

	// PCI address (for PCI devices)
	PCIAddress *PCIAddress `json:"pciAddress,omitempty"`

	// USB address (for USB devices)
	USBAddress *USBAddress `json:"usbAddress,omitempty"`

	// SCSI address (for SCSI devices)
	SCSIAddress *SCSIAddress `json:"scsiAddress,omitempty"`
}

// A NodeDevice is a managed resource that represents a libvirt node device.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,libvirt}
// +kubebuilder:object:root=true
type NodeDevice struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodeDeviceSpec   `json:"spec"`
	Status NodeDeviceStatus `json:"status,omitempty"`
}

// NodeDeviceList contains a list of NodeDevice
// +kubebuilder:object:root=true
type NodeDeviceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodeDevice `json:"items"`
}

// NodeDevice type metadata.
var (
	NodeDeviceKind             = "NodeDevice"
	NodeDeviceGroupKind        = schema.GroupKind{Group: Group, Kind: NodeDeviceKind}
	NodeDeviceKindAPIVersion   = NodeDeviceKind + "." + SchemeGroupVersion.String()
	NodeDeviceGroupVersionKind = SchemeGroupVersion.WithKind(NodeDeviceKind)
)


// GetCondition of this NodeDevice.
func (mg *NodeDevice) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	return mg.Status.GetCondition(ct)
}

// SetConditions of this NodeDevice.
func (mg *NodeDevice) SetConditions(c ...xpv1.Condition) {
	mg.Status.SetConditions(c...)
}


// GetManagementPolicies of this Resource.
func (mg *NodeDevice) GetManagementPolicies() xpv1.ManagementPolicies {
	return mg.Spec.ManagementPolicies
}

// SetManagementPolicies of this Resource.
func (mg *NodeDevice) SetManagementPolicies(p xpv1.ManagementPolicies) {
	mg.Spec.ManagementPolicies = p
}
func init() {
	SchemeBuilder.Register(&NodeDevice{}, &NodeDeviceList{})
}