/*
Copyright 2025 Ross Golder
*/

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
)

// NetworkSpec defines the desired state of Network
type NetworkSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       NetworkParameters `json:"forProvider"`
}

// NetworkParameters are the configurable fields of a Network.
type NetworkParameters struct {
	// Name of the network
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Mode of the network (e.g., "nat", "bridge", "isolated")
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="nat"
	Mode string `json:"mode,omitempty"`

	// Bridge name for bridge mode
	// +kubebuilder:validation:Optional
	Bridge string `json:"bridge,omitempty"`

	// Domain for the network
	// +kubebuilder:validation:Optional
	Domain string `json:"domain,omitempty"`

	// Addresses for the network
	// +kubebuilder:validation:Optional
	Addresses []string `json:"addresses,omitempty"`

	// DHCP configuration
	// +kubebuilder:validation:Optional
	DHCP *NetworkDHCP `json:"dhcp,omitempty"`

	// DNS configuration
	// +kubebuilder:validation:Optional
	DNS *NetworkDNS `json:"dns,omitempty"`

	// Autostart the network
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	Autostart *bool `json:"autostart,omitempty"`
}

// NetworkDHCP represents DHCP configuration
type NetworkDHCP struct {
	// Enabled indicates if DHCP is enabled
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	Enabled *bool `json:"enabled,omitempty"`

	// Start IP address for DHCP range
	// +kubebuilder:validation:Optional
	Start string `json:"start,omitempty"`

	// End IP address for DHCP range
	// +kubebuilder:validation:Optional
	End string `json:"end,omitempty"`
}

// NetworkDNS represents DNS configuration
type NetworkDNS struct {
	// Enabled indicates if DNS is enabled
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	Enabled *bool `json:"enabled,omitempty"`

	// Forwarders for DNS
	// +kubebuilder:validation:Optional
	Forwarders []string `json:"forwarders,omitempty"`
}

// NetworkStatus defines the observed state of Network
type NetworkStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          NetworkObservation `json:"atProvider,omitempty"`
}

// NetworkObservation are the observable fields of a Network.
type NetworkObservation struct {
	// UUID of the network
	UUID string `json:"uuid,omitempty"`

	// Active state of the network
	Active bool `json:"active,omitempty"`

	// Persistent state of the network
	Persistent bool `json:"persistent,omitempty"`

	// Bridge name
	Bridge string `json:"bridge,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="MODE",type="string",JSONPath=".spec.forProvider.mode"
// +kubebuilder:printcolumn:name="ACTIVE",type="boolean",JSONPath=".status.atProvider.active"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,libvirt}

// A Network is a managed resource that represents a libvirt network.
type Network struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NetworkSpec   `json:"spec"`
	Status NetworkStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NetworkList contains a list of Network
type NetworkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Network `json:"items"`
}

// Network type metadata.
var (
	NetworkKind             = "Network"
	NetworkGroupKind        = schema.GroupKind{Group: Group, Kind: NetworkKind}
	NetworkKindAPIVersion   = NetworkKind + "." + SchemeGroupVersion.String()
	NetworkGroupVersionKind = SchemeGroupVersion.WithKind(NetworkKind)
)

func init() {
	SchemeBuilder.Register(&Network{}, &NetworkList{})
}