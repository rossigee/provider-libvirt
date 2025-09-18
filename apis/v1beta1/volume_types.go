/*
Copyright 2025 Ross Golder
*/

package v1beta1

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
type VolumeParameters struct {
	// Name of the volume
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Pool where the volume should be created
	// +kubebuilder:validation:Required
	Pool string `json:"pool"`

	// Size of the volume in bytes
	// +kubebuilder:validation:Required
	Size int64 `json:"size"`

	// Format of the volume (e.g., "qcow2", "raw")
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="qcow2"
	Format string `json:"format,omitempty"`

	// Source volume to clone from
	// +kubebuilder:validation:Optional
	Source string `json:"source,omitempty"`

	// Base volume for backing store
	// +kubebuilder:validation:Optional
	BaseVolumePool string `json:"baseVolumePool,omitempty"`

	// Base volume name for backing store
	// +kubebuilder:validation:Optional
	BaseVolumeName string `json:"baseVolumeName,omitempty"`
}

// VolumeStatus defines the observed state of Volume
type VolumeStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          VolumeObservation `json:"atProvider,omitempty"`
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

func init() {
	SchemeBuilder.Register(&Volume{}, &VolumeList{})
}