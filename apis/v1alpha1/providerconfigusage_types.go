/*
Copyright 2025 Ross Golder
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="CONFIG-NAME",type="string",JSONPath=".providerConfigRef.name"
// +kubebuilder:printcolumn:name="RESOURCE-KIND",type="string",JSONPath=".resourceRef.kind"
// +kubebuilder:printcolumn:name="RESOURCE-NAME",type="string",JSONPath=".resourceRef.name"

// A ProviderConfigUsage indicates that a resource is using a ProviderConfig.
type ProviderConfigUsage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	xpv1.ProviderConfigUsage `json:",inline"`
}


// +kubebuilder:object:root=true

// ProviderConfigUsageList contains a list of ProviderConfigUsage
type ProviderConfigUsageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProviderConfigUsage `json:"items"`
}

// ProviderConfigUsageGroupKind is the GroupKind for ProviderConfigUsage
var ProviderConfigUsageGroupKind = schema.GroupKind{
	Group: Group,
	Kind:  "ProviderConfigUsage",
}

// ProviderConfigUsageGroupVersionKind is the GroupVersionKind for ProviderConfigUsage
var ProviderConfigUsageGroupVersionKind = schema.GroupVersionKind{
	Group:   Group,
	Version: Version,
	Kind:    "ProviderConfigUsage",
}

// ProviderConfigUsageKind is the kind for ProviderConfigUsage
const ProviderConfigUsageKind = "ProviderConfigUsage"

func init() {
	SchemeBuilder.Register(&ProviderConfigUsage{}, &ProviderConfigUsageList{})
}