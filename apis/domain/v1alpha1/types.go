/*
Copyright 2025 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
)

// DomainDisk defines a disk device attached to a domain.
type DomainDisk struct {
	// VolumeID is the libvirt storage volume key (pool/name) to attach.
	// +optional
	VolumeID *string `json:"volumeId,omitempty"`

	// URL is a remote URL to use as the disk image (http, https, ftp, tftp).
	// +optional
	URL *string `json:"url,omitempty"`

	// Bus is the disk bus type (virtio, scsi, ide, sata).
	// +kubebuilder:validation:Enum=virtio;scsi;ide;sata
	// +kubebuilder:default=virtio
	// +optional
	Bus *string `json:"bus,omitempty"`
}

// DomainNetworkInterface defines a network interface attached to a domain.
type DomainNetworkInterface struct {
	// NetworkID is the libvirt network name to attach to.
	// +optional
	NetworkID *string `json:"networkId,omitempty"`

	// Bridge is the host bridge interface to attach to (for bridge mode).
	// +optional
	Bridge *string `json:"bridge,omitempty"`

	// MAC is the MAC address for this interface. Generated if omitted.
	// +optional
	MAC *string `json:"mac,omitempty"`

	// Model is the NIC model (virtio, e1000, rtl8139).
	// +kubebuilder:validation:Enum=virtio;e1000;rtl8139;vmxnet3
	// +kubebuilder:default=virtio
	// +optional
	Model *string `json:"model,omitempty"`
}

// DomainParameters are the configurable fields of a libvirt domain (virtual machine).
type DomainParameters struct {
	// Name of the domain.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Memory is the amount of memory in MiB.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=512
	// +optional
	Memory *int `json:"memory,omitempty"`

	// VCPU is the number of virtual CPUs.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	// +optional
	VCPU *int `json:"vcpu,omitempty"`

	// Arch is the machine architecture (x86_64, aarch64, etc.).
	// +kubebuilder:default=x86_64
	// +optional
	Arch *string `json:"arch,omitempty"`

	// Machine is the machine type (pc, q35, etc.).
	// +optional
	Machine *string `json:"machine,omitempty"`

	// Firmware is the path to the firmware binary (for UEFI: /usr/share/OVMF/OVMF_CODE.fd).
	// +optional
	Firmware *string `json:"firmware,omitempty"`

	// Cloudinit is the name of the cloud-init disk resource to attach.
	// +optional
	Cloudinit *string `json:"cloudinit,omitempty"`

	// Disks are the disk devices to attach.
	// +optional
	Disks []DomainDisk `json:"disks,omitempty"`

	// NetworkInterfaces are the network interfaces to attach.
	// +optional
	NetworkInterfaces []DomainNetworkInterface `json:"networkInterfaces,omitempty"`

	// Autostart controls whether the domain is started on host boot.
	// +kubebuilder:default=false
	// +optional
	Autostart *bool `json:"autostart,omitempty"`

	// Running controls whether the domain should be running (started) after creation.
	// +kubebuilder:default=true
	// +optional
	Running *bool `json:"running,omitempty"`
}

// DomainObservation holds the observed state of a libvirt domain.
type DomainObservation struct {
	// UUID is the libvirt-assigned UUID of the domain.
	UUID string `json:"uuid,omitempty"`

	// State is the current domain state (running, blocked, paused, shutdown, shutoff, crashed, pmsuspended).
	State string `json:"state,omitempty"`

	// ID is the libvirt runtime domain ID (only valid when running).
	ID int `json:"id,omitempty"`
}

// DomainSpec defines the desired state of a Domain.
type DomainSpec struct {
	xpv1.ManagedResourceSpec `json:",inline"`
	ForProvider              DomainParameters `json:"forProvider"`
}

// DomainStatus defines the observed state of a Domain.
type DomainStatus struct {
	xpv1.ManagedResourceStatus `json:",inline"`
	AtProvider                 DomainObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// Domain is a managed resource representing a libvirt domain (virtual machine).
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="STATE",type="string",JSONPath=".status.atProvider.state"
// +kubebuilder:printcolumn:name="VCPU",type="integer",JSONPath=".spec.forProvider.vcpu"
// +kubebuilder:printcolumn:name="MEMORY",type="integer",JSONPath=".spec.forProvider.memory"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Cluster,categories={crossplane,provider,libvirt}
type Domain struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DomainSpec   `json:"spec"`
	Status DomainStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DomainList contains a list of Domain.
type DomainList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Domain `json:"items"`
}
