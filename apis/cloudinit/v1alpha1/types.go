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

// DiskParameters are the configurable fields of a libvirt cloud-init ISO disk.
type DiskParameters struct {
	// Name of the cloud-init disk.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Pool is the name of the storage pool to create the disk in.
	// +kubebuilder:validation:Required
	Pool string `json:"pool"`

	// UserData is the cloud-init user-data content (#cloud-config YAML).
	// +optional
	UserData *string `json:"userData,omitempty"`

	// MetaData is the cloud-init meta-data content.
	// +optional
	MetaData *string `json:"metaData,omitempty"`

	// NetworkConfig is the cloud-init network configuration content.
	// +optional
	NetworkConfig *string `json:"networkConfig,omitempty"`
}

// DiskObservation holds the observed state of a cloud-init disk.
type DiskObservation struct {
	// ID is the libvirt storage volume key of the created ISO.
	ID string `json:"id,omitempty"`
}

// DiskSpec defines the desired state of a cloud-init Disk.
type DiskSpec struct {
	xpv1.ManagedResourceSpec `json:",inline"`
	ForProvider              DiskParameters `json:"forProvider"`
}

// DiskStatus defines the observed state of a cloud-init Disk.
type DiskStatus struct {
	xpv1.ManagedResourceStatus `json:",inline"`
	AtProvider                 DiskObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// Disk is a managed resource representing a libvirt cloud-init ISO disk.
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="POOL",type="string",JSONPath=".spec.forProvider.pool"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Cluster,categories={crossplane,provider,libvirt}
type Disk struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DiskSpec   `json:"spec"`
	Status DiskStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DiskList contains a list of Disk.
type DiskList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Disk `json:"items"`
}
