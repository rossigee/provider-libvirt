/*
Copyright 2025 Ross Golder
*/

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
)

// SecretSpec defines the desired state of Secret
type SecretSpec struct {
	xpv1.ManagedResourceSpec `json:",inline"`
	ForProvider              SecretParameters `json:"forProvider"`
}

// SecretParameters are the configurable fields of a Secret.
type SecretParameters struct {
	// Type of the secret (volume, ceph, iscsi, tls)
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=volume;ceph;iscsi;tls
	Type string `json:"type"`

	// Usage type for the secret (encryption, authentication)
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=encryption;authentication
	Usage string `json:"usage"`

	// Description of the secret
	// +kubebuilder:validation:Optional
	Description string `json:"description,omitempty"`

	// Data contains the secret data based on type
	// +kubebuilder:validation:Required
	Data SecretData `json:"data"`

	// Ephemeral determines if secret is temporary
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	Ephemeral *bool `json:"ephemeral,omitempty"`

	// Private determines if secret is private to this connection
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	Private *bool `json:"private,omitempty"`
}

// SecretData contains type-specific secret data
type SecretData struct {
	// Volume secret data
	// +kubebuilder:validation:Optional
	Volume *VolumeSecretData `json:"volume,omitempty"`

	// Ceph secret data
	// +kubebuilder:validation:Optional
	Ceph *CephSecretData `json:"ceph,omitempty"`

	// ISCSI secret data
	// +kubebuilder:validation:Optional
	ISCSI *ISCSISecretData `json:"iscsi,omitempty"`

	// TLS secret data
	// +kubebuilder:validation:Optional
	TLS *TLSSecretData `json:"tls,omitempty"`
}

// VolumeSecretData contains volume encryption secrets
type VolumeSecretData struct {
	// Passphrase for volume encryption
	// +kubebuilder:validation:Optional
	Passphrase *SecretReference `json:"passphrase,omitempty"`

	// AES key for volume encryption
	// +kubebuilder:validation:Optional
	AESKey *SecretReference `json:"aesKey,omitempty"`
}

// CephSecretData contains Ceph authentication secrets
type CephSecretData struct {
	// Key for Ceph authentication
	// +kubebuilder:validation:Required
	Key *SecretReference `json:"key"`
}

// ISCSISecretData contains ISCSI authentication secrets
type ISCSISecretData struct {
	// Password for ISCSI authentication
	// +kubebuilder:validation:Required
	Password *SecretReference `json:"password"`
}

// TLSSecretData contains TLS certificates and keys
type TLSSecretData struct {
	// Certificate for TLS
	// +kubebuilder:validation:Required
	Certificate *SecretReference `json:"certificate"`

	// Private key for TLS
	// +kubebuilder:validation:Optional
	PrivateKey *SecretReference `json:"privateKey,omitempty"`
}

// SecretReference references a Kubernetes Secret
type SecretReference struct {
	// Name of the secret
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Namespace of the secret
	// +kubebuilder:validation:Optional
	Namespace string `json:"namespace,omitempty"`

	// Key within the secret
	// +kubebuilder:validation:Required
	Key string `json:"key"`
}

// SecretStatus defines the observed state of Secret
type SecretStatus struct {
	xpv1.ManagedResourceStatus `json:",inline"`
	AtProvider                 SecretObservation `json:"atProvider,omitempty"`
}

// SecretObservation are the observable fields of a Secret.
type SecretObservation struct {
	// UUID of the secret in libvirt
	UUID string `json:"uuid,omitempty"`

	// Type of the secret
	Type string `json:"type,omitempty"`

	// Usage type of the secret
	UsageType string `json:"usageType,omitempty"`

	// Usage ID of the secret
	UsageID string `json:"usageId,omitempty"`

	// Description of the secret
	Description string `json:"description,omitempty"`
}

// A Secret is a managed resource that represents a libvirt secret.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,libvirt}
// +kubebuilder:object:root=true
type Secret struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SecretSpec   `json:"spec"`
	Status SecretStatus `json:"status,omitempty"`
}

// SecretList contains a list of Secret
// +kubebuilder:object:root=true
type SecretList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Secret `json:"items"`
}

// Secret type metadata.
var (
	SecretKind             = "Secret"
	SecretGroupKind        = schema.GroupKind{Group: Group, Kind: SecretKind}
	SecretKindAPIVersion   = SecretKind + "." + SchemeGroupVersion.String()
	SecretGroupVersionKind = SchemeGroupVersion.WithKind(SecretKind)
)

// GetCondition of this Secret.
func (mg *Secret) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	return mg.Status.GetCondition(ct)
}

// SetConditions of this Secret.
func (mg *Secret) SetConditions(c ...xpv1.Condition) {
	mg.Status.SetConditions(c...)
}

// GetManagementPolicies of this Secret.
func (mg *Secret) GetManagementPolicies() xpv1.ManagementPolicies {
	return mg.Spec.ManagementPolicies
}

// SetManagementPolicies of this Secret.
func (mg *Secret) SetManagementPolicies(p xpv1.ManagementPolicies) {
	mg.Spec.ManagementPolicies = p
}

func init() {
	SchemeBuilder.Register(&Secret{}, &SecretList{})
}
