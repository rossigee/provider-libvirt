/*
Copyright 2025 Ross Golder
*/

package v1beta1

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"github.com/crossplane/crossplane/apis/v2/core/v2"
)


// DomainSpec defines the desired state of Domain
type DomainSpec struct {
	xpv1.ManagedResourceSpec `json:",inline"`
	ForProvider       DomainParameters `json:"forProvider"`
}

// DomainParameters are the configurable fields of a Domain.
type DomainParameters struct {
	// Name of the domain
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Memory size in bytes
	// +kubebuilder:validation:Required
	Memory int64 `json:"memory"`

	// Number of virtual CPUs
	// +kubebuilder:validation:Required
	Vcpu int32 `json:"vcpu"`

	// Boot devices in order of preference
	// +kubebuilder:validation:Optional
	Boot []string `json:"boot,omitempty"`

	// Disk devices
	// +kubebuilder:validation:Optional
	Disk []DomainDisk `json:"disk,omitempty"`

	// Network interfaces
	// +kubebuilder:validation:Optional
	NetworkInterface []DomainNetworkInterface `json:"networkInterface,omitempty"`

	// Domain type (e.g., "kvm", "qemu")
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="kvm"
	Type string `json:"type,omitempty"`

	// Machine type
	// +kubebuilder:validation:Optional
	Machine string `json:"machine,omitempty"`

	// Architecture
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="x86_64"
	Arch string `json:"arch,omitempty"`

	// Running determines if domain should be running
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	Running *bool `json:"running,omitempty"`

	// Autostart determines if domain starts automatically
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	Autostart *bool `json:"autostart,omitempty"`

	// Console configuration for the domain
	// +kubebuilder:validation:Optional
	Console []DomainConsole `json:"console,omitempty"`

	// Graphics configuration for the domain
	// +kubebuilder:validation:Optional
	Graphics []DomainGraphics `json:"graphics,omitempty"`
}

// DomainDisk represents a disk device
type DomainDisk struct {
	// VolumeRef references a Volume resource (for cross-resource references)
	// +kubebuilder:validation:Optional
	VolumeRef *xpv1.Reference `json:"volumeRef,omitempty"`

	// File path for direct file access (legacy)
	// +kubebuilder:validation:Optional
	File string `json:"file,omitempty"`

	// Device name (e.g., "vda", "vdb")
	// +kubebuilder:validation:Optional
	Device string `json:"device,omitempty"`

	// Type of the disk interface (e.g., "virtio", "ide", "scsi")
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="virtio"
	Type string `json:"type,omitempty"`

	// BootOrder for this disk
	// +kubebuilder:validation:Optional
	BootOrder *int32 `json:"bootOrder,omitempty"`

	// Bus type (e.g., "virtio", "ide", "scsi")
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="virtio"
	Bus string `json:"bus,omitempty"`

	// WWN (World Wide Name) for the disk
	// +kubebuilder:validation:Optional
	WWN string `json:"wwn,omitempty"`
}

// DomainNetworkInterface represents a network interface
type DomainNetworkInterface struct {
	// NetworkRef references a Network resource (for cross-resource references)
	// +kubebuilder:validation:Optional
	NetworkRef *xpv1.Reference `json:"networkRef,omitempty"`

	// NetworkName for direct network name (legacy)
	// +kubebuilder:validation:Optional
	NetworkName string `json:"networkName,omitempty"`

	// MAC address
	// +kubebuilder:validation:Optional
	Mac string `json:"mac,omitempty"`

	// Model type
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="virtio"
	Model string `json:"model,omitempty"`

	// WaitForLease waits for IP lease from DHCP
	// +kubebuilder:validation:Optional
	WaitForLease bool `json:"waitForLease,omitempty"`

	// Vlan VLAN configuration for 802.1q VLAN tagging
	// +kubebuilder:validation:Optional
	Vlan *DomainInterfaceVlan `json:"vlan,omitempty"`
}

type DomainInterfaceVlan struct {
	ID int32 `json:"id"`
	NativeMode string `json:"nativeMode,omitempty"`
}

// DomainConsole represents console configuration
type DomainConsole struct {
	// Type of console (e.g., "pty", "tcp")
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="pty"
	Type string `json:"type,omitempty"`

	// Target for console output
	// +kubebuilder:validation:Optional
	Target string `json:"target,omitempty"`
}

// DomainGraphics represents graphics configuration
type DomainGraphics struct {
	// Type of graphics (e.g., "vnc", "spice", "rdp")
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="vnc"
	Type string `json:"type,omitempty"`

	// Port for graphics connection
	// +kubebuilder:validation:Optional
	Port *int32 `json:"port,omitempty"`

	// Listen address for graphics
	// +kubebuilder:validation:Optional
	Listen string `json:"listen,omitempty"`

	// ListenAddress for graphics connection (alias for Listen)
	// +kubebuilder:validation:Optional
	ListenAddress string `json:"listenAddress,omitempty"`

	// Autoport automatically assigns port
	// +kubebuilder:validation:Optional
	Autoport bool `json:"autoport,omitempty"`
}

// DomainStatus defines the observed state of Domain
type DomainStatus struct {
	xpv1.ManagedResourceStatus `json:",inline"`
	AtProvider          DomainObservation `json:"atProvider,omitempty"`
}

// DomainObservation are the observable fields of a Domain.
type DomainObservation struct {
	// ID is the domain ID
	ID string `json:"id,omitempty"`

	// State of the domain
	State string `json:"state,omitempty"`

	// UUID of the domain
	UUID string `json:"uuid,omitempty"`

	// Disks attached to the domain. Empty disk list indicates a potential zombie.
	Disks []DiskInfo `json:"disks,omitempty"`
}

// DiskInfo describes a disk device attached to a domain.
type DiskInfo struct {
	// Device is the target device name (e.g., "sda", "vda")
	Device string `json:"device,omitempty"`

	// Type is the disk type (e.g., "disk", "cdrom")
	Type string `json:"type,omitempty"`

	// Source is the disk source (file path, volume name, etc.)
	Source string `json:"source,omitempty"`

	// BootOrder is the boot order if configured
	BootOrder *int32 `json:"bootOrder,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="STATE",type="string",JSONPath=".status.atProvider.state"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,libvirt}

// A Domain is a managed resource that represents a libvirt domain (VM).
type Domain struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DomainSpec   `json:"spec"`
	Status DomainStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DomainList contains a list of Domain
type DomainList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Domain `json:"items"`
}

// Domain type metadata.
var (
	DomainKind             = "Domain"
	DomainGroupKind        = schema.GroupKind{Group: Group, Kind: DomainKind}
	DomainKindAPIVersion   = DomainKind + "." + SchemeGroupVersion.String()
	DomainGroupVersionKind = SchemeGroupVersion.WithKind(DomainKind)
)


// GetCondition of this Domain.
func (mg *Domain) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	return mg.Status.GetCondition(ct)
}

// SetConditions of this Domain.
func (mg *Domain) SetConditions(c ...xpv1.Condition) {
	mg.Status.SetConditions(c...)
}


// GetManagementPolicies of this Resource.
func (mg *Domain) GetManagementPolicies() xpv1.ManagementPolicies {
	return mg.Spec.ManagementPolicies
}

// SetManagementPolicies of this Resource.
func (mg *Domain) SetManagementPolicies(p xpv1.ManagementPolicies) {
	mg.Spec.ManagementPolicies = p
}

// GetWriteConnectionSecretToReference of this Domain.
func (mg *Domain) GetWriteConnectionSecretToReference() *xpv1.LocalSecretReference {
	return mg.Spec.WriteConnectionSecretToReference
}

// SetWriteConnectionSecretToReference of this Domain.
func (mg *Domain) SetWriteConnectionSecretToReference(r *xpv1.LocalSecretReference) {
	mg.Spec.WriteConnectionSecretToReference = r
}
}