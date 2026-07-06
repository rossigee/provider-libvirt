/*
Copyright 2025 Ross Golder
*/

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
)

// VolumeSpec defines the desired state of Volume
type VolumeSpec struct {
	xpv1.ManagedResourceSpec `json:",inline"`
	ForProvider              VolumeParameters `json:"forProvider"`
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

	// Size of the volume in bytes
	// Takes precedence over Capacity if both are specified
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=1048576
	Size *int64 `json:"size,omitempty"`

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

	// Base volume pool for backing store
	// +kubebuilder:validation:Optional
	BaseVolumePool string `json:"baseVolumePool,omitempty"`

	// Base volume name for backing store
	// +kubebuilder:validation:Optional
	BaseVolumeName string `json:"baseVolumeName,omitempty"`
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

	// Compat settings for the volume
	// +kubebuilder:validation:Optional
	Compat string `json:"compat,omitempty"`

	// Features for the volume
	// +kubebuilder:validation:Optional
	Features *VolumeFeatures `json:"features,omitempty"`
}

// VolumePermissions represents permission settings
type VolumePermissions struct {
	// Mode of the volume (e.g., 0644)
	// +kubebuilder:validation:Optional
	Mode string `json:"mode,omitempty"`

	// Owner of the volume
	// +kubebuilder:validation:Optional
	Owner string `json:"owner,omitempty"`

	// Group of the volume
	// +kubebuilder:validation:Optional
	Group string `json:"group,omitempty"`

	// Label for SELinux
	// +kubebuilder:validation:Optional
	Label string `json:"label,omitempty"`
}

// VolumeTimestamps represents timestamp settings
type VolumeTimestamps struct {
	// Atime settings
	// +kubebuilder:validation:Optional
	Atime string `json:"atime,omitempty"`

	// Mtime settings
	// +kubebuilder:validation:Optional
	Mtime string `json:"mtime,omitempty"`
}

// VolumeFeatures represents volume features
type VolumeFeatures struct {
	// Lazy refcounts
	// +kubebuilder:validation:Optional
	LazyRefcounts *bool `json:"lazy_refcounts,omitempty"`
}

// VolumeStatus defines the observed state of Volume
type VolumeStatus struct {
	xpv1.ManagedResourceStatus `json:",inline"`
	AtProvider                 VolumeObservation `json:"atProvider,omitempty"`
}

// VolumeObservation are the observable fields of a Volume.
type VolumeObservation struct {
	// Path to the volume file
	Path string `json:"path,omitempty"`

	// Actual size of the volume
	Size int64 `json:"size,omitempty"`

	// Format of the volume
	Format string `json:"format,omitempty"`

	// Key of the volume
	Key string `json:"key,omitempty"`

	// Type of the volume
	Type string `json:"type,omitempty"`

	// Capacity of the volume
	Capacity int64 `json:"capacity,omitempty"`

	// Allocation of the volume
	Allocation int64 `json:"allocation,omitempty"`

	// Pool name where the volume resides
	Pool string `json:"pool,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="POOL",type="string",JSONPath=".spec.forProvider.pool"
// +kubebuilder:printcolumn:name="SIZE",type="integer",JSONPath=".spec.forProvider.size"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,libvirt}

// A Volume is a managed resource that represents a libvirt storage volume.
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

// Volume type metadata.
var (
	VolumeKind             = "Volume"
	VolumeGroupKind        = schema.GroupKind{Group: Group, Kind: VolumeKind}
	VolumeKindAPIVersion   = VolumeKind + "." + SchemeGroupVersion.String()
	VolumeGroupVersionKind = SchemeGroupVersion.WithKind(VolumeKind)
)

// GetCondition of this Volume.
func (mg *Volume) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	return mg.Status.GetCondition(ct)
}

// SetConditions of this Volume.
func (mg *Volume) SetConditions(c ...xpv1.Condition) {
	mg.Status.SetConditions(c...)
}

// GetManagementPolicies of this Resource.
func (mg *Volume) GetManagementPolicies() xpv1.ManagementPolicies {
	return mg.Spec.ManagementPolicies
}

// SetManagementPolicies of this Resource.
func (mg *Volume) SetManagementPolicies(p xpv1.ManagementPolicies) {
	mg.Spec.ManagementPolicies = p
}

// GetWriteConnectionSecretToReference of this Volume.
func (mg *Volume) GetWriteConnectionSecretToReference() *xpv1.LocalSecretReference {
	return mg.Spec.WriteConnectionSecretToReference
}

// SetWriteConnectionSecretToReference of this Volume.
func (mg *Volume) SetWriteConnectionSecretToReference(r *xpv1.LocalSecretReference) {
	mg.Spec.WriteConnectionSecretToReference = r
}

func init() {
	SchemeBuilder.Register(&Volume{}, &VolumeList{})
}
