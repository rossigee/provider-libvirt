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

// VolumeParameters are the configurable fields of a libvirt storage volume.
type VolumeParameters struct {
	// Name of the storage volume.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Pool is the name of the storage pool that contains this volume.
	// +kubebuilder:validation:Required
	Pool string `json:"pool"`

	// Format is the volume format (raw, qcow2, vmdk, etc.).
	// +kubebuilder:validation:Enum=raw;qcow2;qcow;vmdk;vpc;vdi
	// +kubebuilder:default=qcow2
	// +optional
	Format *string `json:"format,omitempty"`

	// Capacity is the volume capacity in bytes.
	// +optional
	Capacity *uint64 `json:"capacity,omitempty"`

	// BaseVolumeID is the UUID of a backing volume for copy-on-write images.
	// +optional
	BaseVolumeID *string `json:"baseVolumeId,omitempty"`

	// Source is a URL to download the volume contents from on creation.
	// +optional
	Source *string `json:"source,omitempty"`
}

// VolumeObservation holds the observed state of a libvirt storage volume.
type VolumeObservation struct {
	// Key is the unique storage volume key (pool/name path).
	Key string `json:"key,omitempty"`

	// Capacity is the actual volume capacity in bytes.
	Capacity uint64 `json:"capacity,omitempty"`

	// Allocation is the currently allocated disk space in bytes.
	Allocation uint64 `json:"allocation,omitempty"`
}

// VolumeSpec defines the desired state of a Volume.
type VolumeSpec struct {
	xpv1.ManagedResourceSpec `json:",inline"`
	ForProvider              VolumeParameters `json:"forProvider"`
}

// VolumeStatus defines the observed state of a Volume.
type VolumeStatus struct {
	xpv1.ManagedResourceStatus `json:",inline"`
	AtProvider                 VolumeObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// Volume is a managed resource representing a libvirt storage volume.
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="POOL",type="string",JSONPath=".spec.forProvider.pool"
// +kubebuilder:printcolumn:name="FORMAT",type="string",JSONPath=".spec.forProvider.format"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Cluster,categories={crossplane,provider,libvirt}
type Volume struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VolumeSpec   `json:"spec"`
	Status VolumeStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// VolumeList contains a list of Volume.
type VolumeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Volume `json:"items"`
}
