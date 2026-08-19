/*
Copyright 2025 Ross Golder
*/

package v1beta1

import (
	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// StoragePoolSpec defines the desired state of StoragePool
type StoragePoolSpec struct {
	xpv1.ManagedResourceSpec `json:",inline"`
	ForProvider              StoragePoolParameters `json:"forProvider"`
}

// StoragePoolParameters are the configurable fields of a StoragePool.
type StoragePoolParameters struct {
	// Name of the storage pool
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Type of the storage pool (dir, fs, netfs, iscsi, scsi, mpath, disk, logical, zfs, rbd, gluster)
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=dir;fs;netfs;iscsi;scsi;mpath;disk;logical;zfs;rbd;gluster;sheepdog;vstorage
	Type string `json:"type"`

	// Target configuration for the storage pool
	// +kubebuilder:validation:Required
	Target *StoragePoolTarget `json:"target"`

	// Source configuration for the storage pool
	// +kubebuilder:validation:Optional
	Source *StoragePoolSource `json:"source,omitempty"`

	// Capacity of the storage pool in bytes (for supported pool types)
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=0
	Capacity *int64 `json:"capacity,omitempty"`

	// Autostart determines if pool starts automatically
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	Autostart *bool `json:"autostart,omitempty"`

	// Permissions for the storage pool
	// +kubebuilder:validation:Optional
	Permissions *StoragePoolPermissions `json:"permissions,omitempty"`
}

// StoragePoolTarget represents target configuration
type StoragePoolTarget struct {
	// Path for the storage pool target
	// +kubebuilder:validation:Required
	Path string `json:"path"`

	// Permissions for the target
	// +kubebuilder:validation:Optional
	Permissions *StoragePoolPermissions `json:"permissions,omitempty"`
}

// StoragePoolSource represents source configuration
type StoragePoolSource struct {
	// Host for network-based storage (netfs, iscsi, rbd, gluster)
	// +kubebuilder:validation:Optional
	Host *StoragePoolHost `json:"host,omitempty"`

	// Device for block-based storage (disk, logical)
	// +kubebuilder:validation:Optional
	Device *StoragePoolDevice `json:"device,omitempty"`

	// Directory for directory-based storage
	// +kubebuilder:validation:Optional
	Dir string `json:"dir,omitempty"`

	// Name of the source (for various pool types)
	// +kubebuilder:validation:Optional
	Name string `json:"name,omitempty"`

	// Format for the source
	// +kubebuilder:validation:Optional
	Format *StoragePoolFormat `json:"format,omitempty"`
}

// StoragePoolHost represents host configuration
type StoragePoolHost struct {
	// Name or IP address of the host
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Port number for the connection
	// +kubebuilder:validation:Optional
	Port *int32 `json:"port,omitempty"`
}

// StoragePoolDevice represents device configuration
type StoragePoolDevice struct {
	// Path to the device
	// +kubebuilder:validation:Required
	Path string `json:"path"`
}

// StoragePoolFormat represents format configuration
type StoragePoolFormat struct {
	// Type of the format
	// +kubebuilder:validation:Required
	Type string `json:"type"`
}

// StoragePoolPermissions represents permission settings
type StoragePoolPermissions struct {
	// Mode of the pool (e.g., 0755)
	// +kubebuilder:validation:Optional
	Mode string `json:"mode,omitempty"`

	// Owner of the pool
	// +kubebuilder:validation:Optional
	Owner *string `json:"owner,omitempty"`

	// Group of the pool
	// +kubebuilder:validation:Optional
	Group *string `json:"group,omitempty"`

	// Label for SELinux
	// +kubebuilder:validation:Optional
	Label string `json:"label,omitempty"`
}

// StoragePoolStatus defines the observed state of StoragePool
type StoragePoolStatus struct {
	xpv1.ManagedResourceStatus `json:",inline"`
	AtProvider                 StoragePoolObservation `json:"atProvider,omitempty"`
}

// StoragePoolObservation are the observable fields of a StoragePool.
type StoragePoolObservation struct {
	// UUID of the storage pool in libvirt
	UUID string `json:"uuid,omitempty"`

	// Name of the storage pool
	Name string `json:"name,omitempty"`

	// State of the storage pool
	State string `json:"state,omitempty"`

	// Active status of the storage pool
	Active bool `json:"active,omitempty"`

	// Persistent status of the storage pool
	Persistent bool `json:"persistent,omitempty"`

	// AutoStart status of the storage pool
	AutoStart bool `json:"autoStart,omitempty"`

	// Capacity of the storage pool in bytes
	Capacity int64 `json:"capacity,omitempty"`

	// Allocation of the storage pool in bytes
	Allocation int64 `json:"allocation,omitempty"`

	// Available space in the storage pool in bytes
	Available int64 `json:"available,omitempty"`

	// Number of volumes in the storage pool
	VolumeCount int `json:"volumeCount,omitempty"`

	// Volumes in the storage pool
	Volumes []StoragePoolVolume `json:"volumes,omitempty"`
}

// StoragePoolVolume represents a volume in a storage pool
type StoragePoolVolume struct {
	// Name of the volume
	Name string `json:"name,omitempty"`

	// Type of the volume
	Type string `json:"type,omitempty"`

	// Key of the volume
	Key string `json:"key,omitempty"`

	// Path to the volume
	Path string `json:"path,omitempty"`

	// Capacity of the volume
	Capacity int64 `json:"capacity,omitempty"`

	// Allocation of the volume
	Allocation int64 `json:"allocation,omitempty"`
}

// A StoragePool is a managed resource that represents a libvirt storage pool.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,libvirt}
// +kubebuilder:object:root=true
type StoragePool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StoragePoolSpec   `json:"spec"`
	Status StoragePoolStatus `json:"status,omitempty"`
}

// StoragePoolList contains a list of StoragePool
// +kubebuilder:object:root=true
type StoragePoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []StoragePool `json:"items"`
}

// StoragePool type metadata.
var (
	StoragePoolKind             = "StoragePool"
	StoragePoolGroupKind        = schema.GroupKind{Group: Group, Kind: StoragePoolKind}
	StoragePoolKindAPIVersion   = StoragePoolKind + "." + SchemeGroupVersion.String()
	StoragePoolGroupVersionKind = SchemeGroupVersion.WithKind(StoragePoolKind)
)

// GetCondition of this StoragePool.
func (mg *StoragePool) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	return mg.Status.GetCondition(ct)
}

// SetConditions of this StoragePool.
func (mg *StoragePool) SetConditions(c ...xpv1.Condition) {
	mg.Status.SetConditions(c...)
}

// GetManagementPolicies of this Resource.
func (mg *StoragePool) GetManagementPolicies() xpv1.ManagementPolicies {
	return mg.Spec.ManagementPolicies
}

// SetManagementPolicies of this Resource.
func (mg *StoragePool) SetManagementPolicies(p xpv1.ManagementPolicies) {
	mg.Spec.ManagementPolicies = p
}

// GetWriteConnectionSecretToReference of this StoragePool.
func (mg *StoragePool) GetWriteConnectionSecretToReference() *xpv1.LocalSecretReference {
	return mg.Spec.WriteConnectionSecretToReference
}

// SetWriteConnectionSecretToReference of this StoragePool.
func (mg *StoragePool) SetWriteConnectionSecretToReference(r *xpv1.LocalSecretReference) {
	mg.Spec.WriteConnectionSecretToReference = r
}
