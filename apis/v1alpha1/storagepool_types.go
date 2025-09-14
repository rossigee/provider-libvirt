/*
Copyright 2025 Ross Golder
*/

package v1alpha1

//go:generate go run -tags generate sigs.k8s.io/controller-tools/cmd/controller-gen object:headerFile="../../hack/boilerplate.go.txt" paths="./..."

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// StoragePoolSpec defines the desired state of StoragePool
type StoragePoolSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       StoragePoolParameters `json:"forProvider"`
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

	// AutoStart determines if pool starts automatically
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	AutoStart *bool `json:"autoStart,omitempty"`

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

	// Encryption settings for the target
	// +kubebuilder:validation:Optional
	Encryption *StoragePoolEncryption `json:"encryption,omitempty"`
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

	// Adapter for SCSI storage
	// +kubebuilder:validation:Optional
	Adapter *StoragePoolAdapter `json:"adapter,omitempty"`

	// Name of the source (for various pool types)
	// +kubebuilder:validation:Optional
	Name string `json:"name,omitempty"`

	// Format for the source
	// +kubebuilder:validation:Optional
	Format *StoragePoolFormat `json:"format,omitempty"`

	// Authentication for network storage
	// +kubebuilder:validation:Optional
	Auth *StoragePoolAuth `json:"auth,omitempty"`

	// Vendor information
	// +kubebuilder:validation:Optional
	Vendor string `json:"vendor,omitempty"`

	// Product information
	// +kubebuilder:validation:Optional
	Product string `json:"product,omitempty"`
}

// StoragePoolHost represents host configuration for network storage
type StoragePoolHost struct {
	// Name or IP address of the host
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Port number
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port *int32 `json:"port,omitempty"`
}

// StoragePoolDevice represents device configuration for block storage
type StoragePoolDevice struct {
	// Path to the device
	// +kubebuilder:validation:Required
	Path string `json:"path"`

	// Part table type (for disk pools)
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=dos;gpt;mac;bsd;pc98;sun;lvm2
	PartTable string `json:"partTable,omitempty"`

	// Free extents information (for logical pools)
	// +kubebuilder:validation:Optional
	FreeExtents *StoragePoolFreeExtents `json:"freeExtents,omitempty"`
}

// StoragePoolAdapter represents SCSI adapter configuration
type StoragePoolAdapter struct {
	// Type of adapter (scsi_host, fc_host)
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=scsi_host;fc_host
	Type string `json:"type"`

	// Name of the adapter
	// +kubebuilder:validation:Optional
	Name string `json:"name,omitempty"`

	// Parent WWN for FC adapters
	// +kubebuilder:validation:Optional
	ParentWWN string `json:"parentWwn,omitempty"`

	// Parent fabric WWN for FC adapters
	// +kubebuilder:validation:Optional
	ParentFabricWWN string `json:"parentFabricWwn,omitempty"`

	// WWN for FC adapters
	// +kubebuilder:validation:Optional
	WWN string `json:"wwn,omitempty"`

	// WWPN for FC adapters
	// +kubebuilder:validation:Optional
	WWPN string `json:"wwpn,omitempty"`

	// WWNN for FC adapters
	// +kubebuilder:validation:Optional
	WWNN string `json:"wwnn,omitempty"`
}

// StoragePoolFormat represents format configuration
type StoragePoolFormat struct {
	// Type of format
	// +kubebuilder:validation:Required
	Type string `json:"type"`

	// Vendor for the format
	// +kubebuilder:validation:Optional
	Vendor string `json:"vendor,omitempty"`
}

// StoragePoolAuth represents authentication configuration
type StoragePoolAuth struct {
	// Type of authentication (chap, ceph)
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=chap;ceph
	Type string `json:"type"`

	// Username for authentication
	// +kubebuilder:validation:Required
	Username string `json:"username"`

	// Secret reference for authentication
	// +kubebuilder:validation:Required
	Secret *StoragePoolAuthSecret `json:"secret"`
}

// StoragePoolAuthSecret represents authentication secret reference
type StoragePoolAuthSecret struct {
	// Type of secret (iscsi, ceph, usage)
	// +kubebuilder:validation:Required
	Type string `json:"type"`

	// UUID of the secret
	// +kubebuilder:validation:Optional
	UUID string `json:"uuid,omitempty"`

	// Usage ID for the secret
	// +kubebuilder:validation:Optional
	Usage string `json:"usage,omitempty"`
}

// StoragePoolFreeExtents represents free extents information
type StoragePoolFreeExtents struct {
	// Start sector
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=0
	Start int64 `json:"start"`

	// End sector
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=0
	End int64 `json:"end"`
}

// StoragePoolPermissions represents permission settings
type StoragePoolPermissions struct {
	// Owner of the storage pool
	// +kubebuilder:validation:Optional
	Owner *int32 `json:"owner,omitempty"`

	// Group of the storage pool
	// +kubebuilder:validation:Optional
	Group *int32 `json:"group,omitempty"`

	// Mode (permissions) of the storage pool
	// +kubebuilder:validation:Optional
	Mode *int32 `json:"mode,omitempty"`

	// SELinux label
	// +kubebuilder:validation:Optional
	Label string `json:"label,omitempty"`
}

// StoragePoolEncryption represents encryption settings
type StoragePoolEncryption struct {
	// Encryption format (luks)
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=luks
	Format string `json:"format"`

	// Secret containing encryption key
	// +kubebuilder:validation:Required
	Secret *StoragePoolEncryptionSecret `json:"secret"`
}

// StoragePoolEncryptionSecret represents encryption secret reference
type StoragePoolEncryptionSecret struct {
	// Type of secret (passphrase)
	// +kubebuilder:validation:Required
	Type string `json:"type"`

	// UUID of the secret
	// +kubebuilder:validation:Optional
	UUID string `json:"uuid,omitempty"`

	// Usage type for the secret
	// +kubebuilder:validation:Optional
	Usage string `json:"usage,omitempty"`
}

// StoragePoolStatus defines the observed state of StoragePool
type StoragePoolStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          StoragePoolObservation `json:"atProvider,omitempty"`
}

// StoragePoolObservation are the observable fields of a StoragePool.
type StoragePoolObservation struct {
	// UUID is the libvirt storage pool UUID
	UUID string `json:"uuid,omitempty"`

	// State of the storage pool (inactive, active)
	State string `json:"state,omitempty"`

	// Active indicates if the storage pool is currently active
	Active bool `json:"active,omitempty"`

	// Persistent indicates if the storage pool is persistent
	Persistent bool `json:"persistent,omitempty"`

	// AutoStart indicates if the storage pool will auto-start
	AutoStart bool `json:"autoStart,omitempty"`

	// Capacity of the storage pool in bytes
	Capacity int64 `json:"capacity,omitempty"`

	// Allocation of the storage pool in bytes
	Allocation int64 `json:"allocation,omitempty"`

	// Available space in the storage pool in bytes
	Available int64 `json:"available,omitempty"`

	// VolumeCount is the number of volumes in the pool
	VolumeCount int32 `json:"volumeCount,omitempty"`

	// Volumes in the storage pool
	Volumes []StoragePoolVolume `json:"volumes,omitempty"`
}

// StoragePoolVolume represents a volume in the storage pool
type StoragePoolVolume struct {
	// Name of the volume
	Name string `json:"name,omitempty"`

	// Key of the volume
	Key string `json:"key,omitempty"`

	// Path of the volume
	Path string `json:"path,omitempty"`

	// Type of the volume
	Type string `json:"type,omitempty"`

	// Capacity of the volume in bytes
	Capacity int64 `json:"capacity,omitempty"`

	// Allocation of the volume in bytes
	Allocation int64 `json:"allocation,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="TYPE",type="string",JSONPath=".spec.forProvider.type"
// +kubebuilder:printcolumn:name="STATE",type="string",JSONPath=".status.atProvider.state"
// +kubebuilder:printcolumn:name="CAPACITY",type="string",JSONPath=".status.atProvider.capacity"
// +kubebuilder:printcolumn:name="AVAILABLE",type="string",JSONPath=".status.atProvider.available"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// A StoragePool is a libvirt storage pool.
type StoragePool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StoragePoolSpec   `json:"spec"`
	Status StoragePoolStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// StoragePoolList contains a list of StoragePool
type StoragePoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []StoragePool `json:"items"`
}


// StoragePoolGroupKind is the GroupKind for StoragePool
var StoragePoolGroupKind = schema.GroupKind{
	Group: Group,
	Kind:  "StoragePool",
}

// StoragePoolGroupVersionKind is the GroupVersionKind for StoragePool
var StoragePoolGroupVersionKind = schema.GroupVersionKind{
	Group:   Group,
	Version: Version,
	Kind:    "StoragePool",
}

// StoragePoolKind is the kind for StoragePool
const StoragePoolKind = "StoragePool"

func init() {
	SchemeBuilder.Register(&StoragePool{}, &StoragePoolList{})
}