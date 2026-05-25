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

// PoolParameters are the configurable fields of a libvirt storage pool.
type PoolParameters struct {
	// Name of the storage pool.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Type of the storage pool (dir, logical, zfs, iscsi, etc.).
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=dir;logical;zfs;iscsi;iscsi-direct;disk;mpath;rbd;sheepdog;gluster;scsi;multipath;vstorage
	Type string `json:"type"`

	// Path is the target directory for dir-type pools.
	// +optional
	Path *string `json:"path,omitempty"`
}

// PoolObservation holds the observed state of a libvirt storage pool.
type PoolObservation struct {
	// UUID is the libvirt-assigned UUID of the pool.
	UUID string `json:"uuid,omitempty"`

	// State is the current pool state (inactive, building, running, degraded, inaccessible).
	State string `json:"state,omitempty"`

	// Capacity is the total capacity of the pool in bytes.
	Capacity uint64 `json:"capacity,omitempty"`

	// Allocation is the currently allocated space in bytes.
	Allocation uint64 `json:"allocation,omitempty"`

	// Available is the available space in bytes.
	Available uint64 `json:"available,omitempty"`
}

// PoolSpec defines the desired state of a Pool.
type PoolSpec struct {
	xpv1.ManagedResourceSpec `json:",inline"`
	ForProvider              PoolParameters `json:"forProvider"`
}

// PoolStatus defines the observed state of a Pool.
type PoolStatus struct {
	xpv1.ManagedResourceStatus `json:",inline"`
	AtProvider                 PoolObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// Pool is a managed resource representing a libvirt storage pool.
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="TYPE",type="string",JSONPath=".spec.forProvider.type"
// +kubebuilder:printcolumn:name="STATE",type="string",JSONPath=".status.atProvider.state"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Cluster,categories={crossplane,provider,libvirt}
type Pool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PoolSpec   `json:"spec"`
	Status PoolStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PoolList contains a list of Pool.
type PoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Pool `json:"items"`
}
