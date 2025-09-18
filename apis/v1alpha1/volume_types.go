/*
Copyright 2025 Ross Golder
*/

package v1alpha1

//go:generate go run -tags generate sigs.k8s.io/controller-tools/cmd/controller-gen object:headerFile="../../hack/boilerplate.go.txt" paths="./..."

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
)

// VolumeSpec defines the desired state of Volume
type VolumeSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       VolumeParameters `json:"forProvider"`
}

// VolumeParameters are the configurable fields of a Volume.
// +kubebuilder:validation:XValidation:rule="has(self.capacity) || has(self.size)",message="Either capacity or size must be specified"
type VolumeParameters struct {
	// Name of the volume
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Storage pool name where the volume will be created
	// +kubebuilder:validation:Required
	Pool string `json:"pool"`

	// Capacity of the volume in bytes (deprecated - use Size instead)
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=1048576
	Capacity *int64 `json:"capacity,omitempty"`

	// Size of the volume using human-readable format (e.g., "100G", "1T", "512M")
	// Takes precedence over Capacity if both are specified
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?[KMGTPE]?[iB]?$`
	Size string `json:"size,omitempty"`

	// Format of the volume (qcow2, raw, vmdk, etc.)
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="qcow2"
	Format string `json:"format,omitempty"`

	// Allocation of the volume in bytes (for sparse files)
	// +kubebuilder:validation:Optional
	Allocation *int64 `json:"allocation,omitempty"`

	// Source volume to clone from
	// +kubebuilder:validation:Optional
	Source *VolumeSource `json:"source,omitempty"`

	// Base volume for backing store (for qcow2)
	// +kubebuilder:validation:Optional
	BackingStore *VolumeBackingStore `json:"backingStore,omitempty"`

	// Encryption settings for the volume
	// +kubebuilder:validation:Optional
	Encryption *VolumeEncryption `json:"encryption,omitempty"`

	// Target configuration for the volume
	// +kubebuilder:validation:Optional
	Target *VolumeTarget `json:"target,omitempty"`
}

// VolumeSource represents the source for volume creation
type VolumeSource struct {
	// Volume name to clone from
	// +kubebuilder:validation:Optional
	Volume string `json:"volume,omitempty"`

	// Pool name where the source volume resides
	// +kubebuilder:validation:Optional
	Pool string `json:"pool,omitempty"`

	// URL to download volume from
	// +kubebuilder:validation:Optional
	URL string `json:"url,omitempty"`

	// File path to copy from (for file-based pools)
	// +kubebuilder:validation:Optional
	File string `json:"file,omitempty"`
}

// VolumeBackingStore represents backing store configuration
type VolumeBackingStore struct {
	// Path to the backing store file
	// +kubebuilder:validation:Required
	Path string `json:"path"`

	// Format of the backing store
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="qcow2"
	Format string `json:"format,omitempty"`
}

// VolumeEncryption represents volume encryption settings
type VolumeEncryption struct {
	// Encryption format (luks, qcow)
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=luks;qcow
	Format string `json:"format"`

	// SecretRef references a Secret resource for encryption
	// +kubebuilder:validation:Optional
	SecretRef *xpv1.Reference `json:"secretRef,omitempty"`

	// Secret containing encryption key (legacy - use SecretRef instead)
	// +kubebuilder:validation:Optional
	Secret *VolumeEncryptionSecret `json:"secret,omitempty"`
}

// VolumeEncryptionSecret represents encryption secret reference (legacy)
type VolumeEncryptionSecret struct {
	// Type of secret (passphrase, aes)
	// +kubebuilder:validation:Required
	Type string `json:"type"`

	// UUID of the secret
	// +kubebuilder:validation:Optional
	UUID string `json:"uuid,omitempty"`

	// Usage type for the secret
	// +kubebuilder:validation:Optional
	Usage string `json:"usage,omitempty"`
}

// VolumeTarget represents target device configuration
type VolumeTarget struct {
	// Path where the volume will be accessible
	// +kubebuilder:validation:Optional
	Path string `json:"path,omitempty"`

	// Format for the target
	// +kubebuilder:validation:Optional
	Format string `json:"format,omitempty"`

	// Permissions for the volume
	// +kubebuilder:validation:Optional
	Permissions *VolumePermissions `json:"permissions,omitempty"`

	// Timestamps configuration
	// +kubebuilder:validation:Optional
	Timestamps *VolumeTimestamps `json:"timestamps,omitempty"`

	// Compatibility settings
	// +kubebuilder:validation:Optional
	Compat string `json:"compat,omitempty"`

	// Features enabled for the volume
	// +kubebuilder:validation:Optional
	Features []string `json:"features,omitempty"`
}

// VolumePermissions represents volume permission settings
type VolumePermissions struct {
	// Owner of the volume
	// +kubebuilder:validation:Optional
	Owner *int32 `json:"owner,omitempty"`

	// Group of the volume
	// +kubebuilder:validation:Optional
	Group *int32 `json:"group,omitempty"`

	// Mode (permissions) of the volume
	// +kubebuilder:validation:Optional
	Mode *int32 `json:"mode,omitempty"`

	// SELinux label
	// +kubebuilder:validation:Optional
	Label string `json:"label,omitempty"`
}

// VolumeTimestamps represents timestamp configuration
type VolumeTimestamps struct {
	// Access time
	// +kubebuilder:validation:Optional
	Atime *metav1.Time `json:"atime,omitempty"`

	// Modification time
	// +kubebuilder:validation:Optional
	Mtime *metav1.Time `json:"mtime,omitempty"`

	// Creation time
	// +kubebuilder:validation:Optional
	Ctime *metav1.Time `json:"ctime,omitempty"`
}

// VolumeStatus defines the observed state of Volume
type VolumeStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          VolumeObservation `json:"atProvider,omitempty"`
}

// VolumeObservation are the observable fields of a Volume.
type VolumeObservation struct {
	// Key is the libvirt volume key
	Key string `json:"key,omitempty"`

	// Path is the full path to the volume
	Path string `json:"path,omitempty"`

	// Type of the volume (file, block, dir, network)
	Type string `json:"type,omitempty"`

	// Capacity of the volume in bytes
	Capacity int64 `json:"capacity,omitempty"`

	// Allocation of the volume in bytes
	Allocation int64 `json:"allocation,omitempty"`

	// Physical size of the volume in bytes
	Physical int64 `json:"physical,omitempty"`

	// Format of the volume
	Format string `json:"format,omitempty"`

	// Pool the volume belongs to
	Pool string `json:"pool,omitempty"`

	// CreationTime when the volume was created
	CreationTime *metav1.Time `json:"creationTime,omitempty"`

	// ModificationTime when the volume was last modified
	ModificationTime *metav1.Time `json:"modificationTime,omitempty"`

	// BackingStore information if present
	BackingStore *VolumeBackingStoreStatus `json:"backingStore,omitempty"`
}

// VolumeBackingStoreStatus represents observed backing store information
type VolumeBackingStoreStatus struct {
	// Path to the backing store
	Path string `json:"path,omitempty"`

	// Format of the backing store
	Format string `json:"format,omitempty"`

	// Capacity of the backing store
	Capacity int64 `json:"capacity,omitempty"`

	// Allocation of the backing store
	Allocation int64 `json:"allocation,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="POOL",type="string",JSONPath=".status.atProvider.pool"
// +kubebuilder:printcolumn:name="CAPACITY",type="string",JSONPath=".status.atProvider.capacity"
// +kubebuilder:printcolumn:name="FORMAT",type="string",JSONPath=".status.atProvider.format"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// A Volume is a libvirt storage volume.
type Volume struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VolumeSpec   `json:"spec"`
	Status VolumeStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// VolumeList contains a list of Volume
type VolumeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Volume `json:"items"`
}


// VolumeGroupKind is the GroupKind for Volume
var VolumeGroupKind = schema.GroupKind{
	Group: Group,
	Kind:  "Volume",
}

// VolumeGroupVersionKind is the GroupVersionKind for Volume
var VolumeGroupVersionKind = schema.GroupVersionKind{
	Group:   Group,
	Version: Version,
	Kind:    "Volume",
}

// VolumeKind is the kind for Volume
const VolumeKind = "Volume"

func init() {
	SchemeBuilder.Register(&Volume{}, &VolumeList{})
}