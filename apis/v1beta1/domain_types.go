/*
Copyright 2025 Ross Golder
*/

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
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
}

// DomainDisk represents a disk device
type DomainDisk struct {
	// Device type (e.g., "disk", "cdrom")
	// +kubebuilder:validation:Required
	Device string `json:"device"`

	// Source for the disk
	// +kubebuilder:validation:Required
	Source string `json:"source"`

	// Target device name
	// +kubebuilder:validation:Required
	Target string `json:"target"`

	// Bus type (e.g., "virtio", "ide", "scsi")
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="virtio"
	Bus string `json:"bus,omitempty"`
}

// DomainNetworkInterface represents a network interface
type DomainNetworkInterface struct {
	// Network name
	// +kubebuilder:validation:Required
	Network string `json:"network"`

	// MAC address
	// +kubebuilder:validation:Optional
	Mac string `json:"mac,omitempty"`

	// Model type
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="virtio"
	Model string `json:"model,omitempty"`
}

// DomainStatus defines the observed state of Domain
type DomainStatus struct {
	xpv1.ResourceStatus `json:",inline"`
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

func init() {
	SchemeBuilder.Register(&Domain{}, &DomainList{})
}