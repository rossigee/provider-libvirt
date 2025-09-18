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

// SecretSpec defines the desired state of Secret
type SecretSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       SecretParameters `json:"forProvider"`
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

// SecretData represents different types of secret data
type SecretData struct {
	// Volume encryption secrets
	// +kubebuilder:validation:Optional
	Volume *VolumeSecretData `json:"volume,omitempty"`

	// Ceph authentication secrets
	// +kubebuilder:validation:Optional
	Ceph *CephSecretData `json:"ceph,omitempty"`

	// iSCSI authentication secrets
	// +kubebuilder:validation:Optional
	ISCSI *ISCSISecretData `json:"iscsi,omitempty"`

	// TLS certificate secrets
	// +kubebuilder:validation:Optional
	TLS *TLSSecretData `json:"tls,omitempty"`
}

// VolumeSecretData represents volume encryption secret data
type VolumeSecretData struct {
	// Passphrase for volume encryption
	// +kubebuilder:validation:Optional
	Passphrase *SecretReference `json:"passphrase,omitempty"`

	// AES key for volume encryption (base64 encoded)
	// +kubebuilder:validation:Optional
	AESKey *SecretReference `json:"aesKey,omitempty"`

	// Format of the encryption (luks, qcow)
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=luks;qcow
	// +kubebuilder:default="luks"
	Format string `json:"format,omitempty"`
}

// CephSecretData represents Ceph authentication secret data
type CephSecretData struct {
	// Username for Ceph authentication
	// +kubebuilder:validation:Required
	Username string `json:"username"`

	// Key for Ceph authentication
	// +kubebuilder:validation:Required
	Key *SecretReference `json:"key"`

	// Monitor addresses
	// +kubebuilder:validation:Optional
	Monitors []string `json:"monitors,omitempty"`
}

// ISCSISecretData represents iSCSI authentication secret data
type ISCSISecretData struct {
	// Username for iSCSI CHAP authentication
	// +kubebuilder:validation:Required
	Username string `json:"username"`

	// Password for iSCSI CHAP authentication
	// +kubebuilder:validation:Required
	Password *SecretReference `json:"password"`

	// Target name
	// +kubebuilder:validation:Optional
	Target string `json:"target,omitempty"`
}

// TLSSecretData represents TLS certificate secret data
type TLSSecretData struct {
	// Certificate in PEM format
	// +kubebuilder:validation:Required
	Certificate *SecretReference `json:"certificate"`

	// Private key in PEM format
	// +kubebuilder:validation:Optional
	PrivateKey *SecretReference `json:"privateKey,omitempty"`

	// CA certificate in PEM format
	// +kubebuilder:validation:Optional
	CACertificate *SecretReference `json:"caCertificate,omitempty"`

	// Certificate chain in PEM format
	// +kubebuilder:validation:Optional
	CertificateChain *SecretReference `json:"certificateChain,omitempty"`
}

// SecretReference represents a reference to a Kubernetes secret
type SecretReference struct {
	// Name of the Kubernetes secret
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Namespace of the Kubernetes secret
	// +kubebuilder:validation:Optional
	Namespace string `json:"namespace,omitempty"`

	// Key within the secret
	// +kubebuilder:validation:Required
	Key string `json:"key"`
}

// SecretStatus defines the observed state of Secret
type SecretStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          SecretObservation `json:"atProvider,omitempty"`
}

// SecretObservation are the observable fields of a Secret.
type SecretObservation struct {
	// UUID is the libvirt secret UUID
	UUID string `json:"uuid,omitempty"`

	// UsageType is the secret usage type as seen by libvirt
	UsageType string `json:"usageType,omitempty"`

	// UsageID is the secret usage identifier
	UsageID string `json:"usageId,omitempty"`

	// Type is the secret type as seen by libvirt
	Type string `json:"type,omitempty"`

	// Description of the secret
	Description string `json:"description,omitempty"`

	// CreationTime when the secret was created
	CreationTime *metav1.Time `json:"creationTime,omitempty"`

	// LastModified when the secret was last modified
	LastModified *metav1.Time `json:"lastModified,omitempty"`

	// Ephemeral indicates if the secret is ephemeral
	Ephemeral bool `json:"ephemeral,omitempty"`

	// Private indicates if the secret is private
	Private bool `json:"private,omitempty"`

	// References to resources using this secret
	UsedBy []SecretUsageReference `json:"usedBy,omitempty"`
}

// SecretUsageReference represents a resource using this secret
type SecretUsageReference struct {
	// Type of resource using the secret
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=Domain;Volume;Network;StoragePool
	Type string `json:"type"`

	// Name of the resource using the secret
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Namespace of the resource using the secret
	// +kubebuilder:validation:Optional
	Namespace string `json:"namespace,omitempty"`

	// Purpose of the secret usage (encryption, authentication, etc.)
	// +kubebuilder:validation:Optional
	Purpose string `json:"purpose,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="TYPE",type="string",JSONPath=".spec.forProvider.type"
// +kubebuilder:printcolumn:name="USAGE",type="string",JSONPath=".spec.forProvider.usage"
// +kubebuilder:printcolumn:name="UUID",type="string",JSONPath=".status.atProvider.uuid"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// A Secret is a libvirt secret for secure credential management.
type Secret struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SecretSpec   `json:"spec"`
	Status SecretStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SecretList contains a list of Secret
type SecretList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Secret `json:"items"`
}


// SecretGroupKind is the GroupKind for Secret
var SecretGroupKind = schema.GroupKind{
	Group: Group,
	Kind:  "Secret",
}

// SecretGroupVersionKind is the GroupVersionKind for Secret
var SecretGroupVersionKind = schema.GroupVersionKind{
	Group:   Group,
	Version: Version,
	Kind:    "Secret",
}

// SecretKind is the kind for Secret
const SecretKind = "Secret"

func init() {
	SchemeBuilder.Register(&Secret{}, &SecretList{})
}