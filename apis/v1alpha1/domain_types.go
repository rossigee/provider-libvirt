/*
Copyright 2025 Ross Golder
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// DomainSpec defines the desired state of Domain
type DomainSpec struct {
	xpv1.ResourceSpec `json:",inline"`
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

	// Console configuration
	// +kubebuilder:validation:Optional
	Console *DomainConsole `json:"console,omitempty"`

	// Graphics configuration
	// +kubebuilder:validation:Optional
	Graphics *DomainGraphics `json:"graphics,omitempty"`

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

	// Autostart domain on host boot
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	Autostart bool `json:"autostart,omitempty"`

	// Running state of the domain
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	Running bool `json:"running,omitempty"`
}

// DomainDisk represents a disk device
type DomainDisk struct {
	// Volume resource reference (preferred)
	// +kubebuilder:validation:Optional
	VolumeRef *xpv1.Reference `json:"volumeRef,omitempty"`

	// Volume source reference (deprecated - use volumeRef)
	// +kubebuilder:validation:Optional
	VolumeID string `json:"volumeId,omitempty"`

	// Block file for the disk (fallback when no volume reference)
	// +kubebuilder:validation:Optional
	File string `json:"file,omitempty"`

	// SCSI or virtio, etc.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="virtio"
	Type string `json:"type,omitempty"`

	// WWN (World Wide Name) for the disk
	// +kubebuilder:validation:Optional
	WWN string `json:"wwn,omitempty"`

	// Device target name (vda, vdb, etc.) - auto-assigned if not specified
	// +kubebuilder:validation:Optional
	Device string `json:"device,omitempty"`

	// Boot priority for this disk (1 = highest priority)
	// +kubebuilder:validation:Optional
	BootOrder *int32 `json:"bootOrder,omitempty"`
}

// DomainNetworkInterface represents a network interface
type DomainNetworkInterface struct {
	// Network resource reference (preferred)
	// +kubebuilder:validation:Optional
	NetworkRef *xpv1.Reference `json:"networkRef,omitempty"`

	// Network name or bridge (fallback when no network reference)
	// +kubebuilder:validation:Optional
	NetworkName string `json:"networkName,omitempty"`

	// Bridge name for bridged networking
	// +kubebuilder:validation:Optional
	Bridge string `json:"bridge,omitempty"`

	// MAC address (auto-generated if not specified)
	// +kubebuilder:validation:Optional
	MAC string `json:"mac,omitempty"`

	// Model type (e.g., "virtio")
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="virtio"
	Model string `json:"model,omitempty"`

	// Hostname for the interface
	// +kubebuilder:validation:Optional
	Hostname string `json:"hostname,omitempty"`

	// Wait for lease
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	WaitForLease bool `json:"waitForLease,omitempty"`

	// Interface device name (eth0, ens3, etc.) - auto-assigned if not specified
	// +kubebuilder:validation:Optional
	Device string `json:"device,omitempty"`
}

// DomainConsole represents console configuration
type DomainConsole struct {
	// Console type (e.g., "pty")
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="pty"
	Type string `json:"type,omitempty"`

	// Target port
	// +kubebuilder:validation:Optional
	TargetPort string `json:"targetPort,omitempty"`

	// Target type
	// +kubebuilder:validation:Optional
	TargetType string `json:"targetType,omitempty"`

	// Source path
	// +kubebuilder:validation:Optional
	SourcePath string `json:"sourcePath,omitempty"`
}

// DomainGraphics represents graphics configuration
type DomainGraphics struct {
	// Graphics type (e.g., "spice", "vnc")
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="spice"
	Type string `json:"type,omitempty"`

	// Listen type
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="address"
	ListenType string `json:"listenType,omitempty"`

	// Listen address
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="127.0.0.1"
	ListenAddress string `json:"listenAddress,omitempty"`

	// Autoport
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	Autoport bool `json:"autoport,omitempty"`
}

// DomainStatus defines the observed state of Domain
type DomainStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          DomainObservation `json:"atProvider,omitempty"`
}

// DomainObservation are the observable fields of a Domain.
type DomainObservation struct {
	// ID is the libvirt domain ID
	ID string `json:"id,omitempty"`

	// UUID is the libvirt domain UUID  
	UUID string `json:"uuid,omitempty"`

	// State of the domain (running, shutoff, etc.)
	State string `json:"state,omitempty"`

	// IP addresses assigned to network interfaces
	NetworkInterfaces []DomainNetworkInterfaceStatus `json:"networkInterfaces,omitempty"`
}

// DomainNetworkInterfaceStatus represents the status of a network interface
type DomainNetworkInterfaceStatus struct {
	// Name of the interface
	Name string `json:"name,omitempty"`

	// MAC address
	MAC string `json:"mac,omitempty"`

	// IP addresses
	Addresses []string `json:"addresses,omitempty"`

	// Hostname
	Hostname string `json:"hostname,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="STATE",type="string",JSONPath=".status.atProvider.state"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// A Domain is a libvirt virtual machine domain.
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




// DomainGroupKind is the GroupKind for Domain
var DomainGroupKind = schema.GroupKind{
	Group: Group,
	Kind:  "Domain",
}

// DomainGroupVersionKind is the GroupVersionKind for Domain
var DomainGroupVersionKind = schema.GroupVersionKind{
	Group:   Group,
	Version: Version,
	Kind:    "Domain",
}

// DomainKind is the kind for Domain
const DomainKind = "Domain"

func init() {
	SchemeBuilder.Register(&Domain{}, &DomainList{})
}