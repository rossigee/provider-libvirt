/*
Copyright 2025 Ross Golder
*/

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
)

// NetworkSpec defines the desired state of Network
type NetworkSpec struct {
	xpv1.ManagedResourceSpec `json:",inline"`
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

	// Bridge configuration for bridge mode
	// +kubebuilder:validation:Optional
	Bridge *NetworkBridge `json:"bridge,omitempty"`

	// IP configuration for the network
	// +kubebuilder:validation:Optional
	IP []NetworkIP `json:"ip,omitempty"`

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

	// Ranges for DHCP (multiple ranges supported)
	// +kubebuilder:validation:Optional
	Ranges []NetworkDHCPRange `json:"ranges,omitempty"`
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

// NetworkBridge represents bridge configuration
type NetworkBridge struct {
	// Name of the bridge
	// +kubebuilder:validation:Optional
	Name string `json:"name,omitempty"`

	// STP (Spanning Tree Protocol) delay
	// +kubebuilder:validation:Optional
	STPDelay *int32 `json:"stpDelay,omitempty"`
}

// NetworkIP represents IP configuration
type NetworkIP struct {
	// Address of the network
	// +kubebuilder:validation:Required
	Address string `json:"address"`

	// Netmask for the network
	// +kubebuilder:validation:Optional
	Netmask string `json:"netmask,omitempty"`

	// DHCP configuration for this IP
	// +kubebuilder:validation:Optional
	DHCP *NetworkDHCPRange `json:"dhcp,omitempty"`
}

// NetworkDHCPRange represents DHCP range configuration
type NetworkDHCPRange struct {
	// Start IP address for DHCP range
	// +kubebuilder:validation:Required
	Start string `json:"start"`

	// End IP address for DHCP range
	// +kubebuilder:validation:Required
	End string `json:"end"`

	// DHCP hosts with fixed IP assignments
	// +kubebuilder:validation:Optional
	Hosts []NetworkDHCPHost `json:"hosts,omitempty"`
}

// NetworkDHCPHost represents a DHCP host configuration
type NetworkDHCPHost struct {
	// MAC address of the host
	// +kubebuilder:validation:Required
	MAC string `json:"mac"`

	// Name of the host
	// +kubebuilder:validation:Optional
	Name string `json:"name,omitempty"`

	// IP address to assign to the host
	// +kubebuilder:validation:Required
	IP string `json:"ip"`
}

// NetworkDomain represents domain configuration
type NetworkDomain struct {
	// Name of the domain
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Local domain only
	// +kubebuilder:validation:Optional
	LocalOnly *bool `json:"localOnly,omitempty"`
}

// NetworkBootp represents BOOTP configuration
type NetworkBootp struct {
	// File to serve via BOOTP
	// +kubebuilder:validation:Optional
	File string `json:"file,omitempty"`

	// Server for BOOTP
	// +kubebuilder:validation:Optional
	Server string `json:"server,omitempty"`
}

// NetworkStatus defines the observed state of Network
type NetworkStatus struct {
	xpv1.ManagedResourceStatus `json:",inline"`
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

	// Autostart state of the network
	Autostart bool `json:"autostart,omitempty"`
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


// GetCondition of this Network.
func (mg *Network) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	return mg.Status.GetCondition(ct)
}

// SetConditions of this Network.
func (mg *Network) SetConditions(c ...xpv1.Condition) {
	mg.Status.SetConditions(c...)
}


// GetManagementPolicies of this Resource.
func (mg *Network) GetManagementPolicies() xpv1.ManagementPolicies {
	return mg.Spec.ManagementPolicies
}

// SetManagementPolicies of this Resource.
func (mg *Network) SetManagementPolicies(p xpv1.ManagementPolicies) {
	mg.Spec.ManagementPolicies = p
}

// GetWriteConnectionSecretToReference of this Network.
func (mg *Network) GetWriteConnectionSecretToReference() *xpv1.LocalSecretReference {
	return mg.Spec.WriteConnectionSecretToReference
}

// SetWriteConnectionSecretToReference of this Network.
func (mg *Network) SetWriteConnectionSecretToReference(r *xpv1.LocalSecretReference) {
	mg.Spec.WriteConnectionSecretToReference = r
}

func init() {
	SchemeBuilder.Register(&Network{}, &NetworkList{})
}