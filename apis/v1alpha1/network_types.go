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

	// Mode of the network (nat, bridge, isolated, routed, open)
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=nat;bridge;isolated;routed;open
	Mode string `json:"mode"`

	// Bridge settings for the network
	// +kubebuilder:validation:Optional
	Bridge *NetworkBridge `json:"bridge,omitempty"`

	// Domain name for the network
	// +kubebuilder:validation:Optional
	Domain *NetworkDomain `json:"domain,omitempty"`

	// IP configuration for the network
	// +kubebuilder:validation:Optional
	IP *NetworkIP `json:"ip,omitempty"`

	// DHCP configuration for the network
	// +kubebuilder:validation:Optional
	DHCP *NetworkDHCP `json:"dhcp,omitempty"`

	// DNS configuration for the network
	// +kubebuilder:validation:Optional
	DNS *NetworkDNS `json:"dns,omitempty"`

	// Forward settings for NAT/routed networks
	// +kubebuilder:validation:Optional
	Forward *NetworkForward `json:"forward,omitempty"`

	// AutoStart determines if network starts automatically
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	AutoStart *bool `json:"autoStart,omitempty"`
}

// NetworkBridge represents bridge configuration
type NetworkBridge struct {
	// Name of the bridge interface
	// +kubebuilder:validation:Optional
	Name string `json:"name,omitempty"`

	// STP (Spanning Tree Protocol) configuration
	// +kubebuilder:validation:Optional
	STP *NetworkSTP `json:"stp,omitempty"`

	// Delay for bridge forwarding
	// +kubebuilder:validation:Optional
	Delay *int32 `json:"delay,omitempty"`

	// MAC address of the bridge
	// +kubebuilder:validation:Optional
	MAC string `json:"mac,omitempty"`
}

// NetworkSTP represents Spanning Tree Protocol settings
type NetworkSTP struct {
	// Enabled determines if STP is enabled
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	Enabled *bool `json:"enabled,omitempty"`

	// Priority for STP
	// +kubebuilder:validation:Optional
	Priority *int32 `json:"priority,omitempty"`

	// Forward delay in seconds
	// +kubebuilder:validation:Optional
	ForwardDelay *int32 `json:"forwardDelay,omitempty"`

	// Hello time in seconds
	// +kubebuilder:validation:Optional
	HelloTime *int32 `json:"helloTime,omitempty"`

	// Max age in seconds
	// +kubebuilder:validation:Optional
	MaxAge *int32 `json:"maxAge,omitempty"`
}

// NetworkDomain represents domain configuration
type NetworkDomain struct {
	// Name of the domain
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// LocalOnly determines if domain is local only
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	LocalOnly *bool `json:"localOnly,omitempty"`
}

// NetworkIP represents IP configuration
type NetworkIP struct {
	// Address of the network (CIDR notation)
	// +kubebuilder:validation:Required
	Address string `json:"address"`

	// Netmask for the network (alternative to CIDR)
	// +kubebuilder:validation:Optional
	Netmask string `json:"netmask,omitempty"`

	// Prefix length for the network (alternative to CIDR)
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=32
	Prefix *int32 `json:"prefix,omitempty"`

	// Family of the IP address (ipv4, ipv6)
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=ipv4;ipv6
	// +kubebuilder:default="ipv4"
	Family string `json:"family,omitempty"`

	// Local determines if this is a local-only address
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	Local *bool `json:"local,omitempty"`
}

// NetworkDHCP represents DHCP configuration
type NetworkDHCP struct {
	// Enabled determines if DHCP is enabled
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	Enabled *bool `json:"enabled,omitempty"`

	// Range of IP addresses for DHCP
	// +kubebuilder:validation:Optional
	Range *NetworkDHCPRange `json:"range,omitempty"`

	// Static host assignments
	// +kubebuilder:validation:Optional
	Hosts []NetworkDHCPHost `json:"hosts,omitempty"`

	// Bootp settings for network boot
	// +kubebuilder:validation:Optional
	Bootp *NetworkBootp `json:"bootp,omitempty"`
}

// NetworkDHCPRange represents DHCP address range
type NetworkDHCPRange struct {
	// Start IP address of the range
	// +kubebuilder:validation:Required
	Start string `json:"start"`

	// End IP address of the range
	// +kubebuilder:validation:Required
	End string `json:"end"`
}

// NetworkDHCPHost represents static DHCP host assignment
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

	// ID for the host (alternative to MAC)
	// +kubebuilder:validation:Optional
	ID string `json:"id,omitempty"`
}

// NetworkBootp represents network boot configuration
type NetworkBootp struct {
	// File to serve for network boot
	// +kubebuilder:validation:Optional
	File string `json:"file,omitempty"`

	// Server for network boot
	// +kubebuilder:validation:Optional
	Server string `json:"server,omitempty"`
}

// NetworkDNS represents DNS configuration
type NetworkDNS struct {
	// Enable DNS forwarding
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	Enable *bool `json:"enable,omitempty"`

	// Forward DNS queries to these servers
	// +kubebuilder:validation:Optional
	Forwarders []NetworkDNSForwarder `json:"forwarders,omitempty"`

	// DNS hostnames
	// +kubebuilder:validation:Optional
	Hosts []NetworkDNSHost `json:"hosts,omitempty"`

	// SRV records
	// +kubebuilder:validation:Optional
	SRV []NetworkDNSSRV `json:"srv,omitempty"`

	// TXT records
	// +kubebuilder:validation:Optional
	TXT []NetworkDNSTXT `json:"txt,omitempty"`
}

// NetworkDNSForwarder represents DNS forwarder configuration
type NetworkDNSForwarder struct {
	// Domain to forward (empty for all)
	// +kubebuilder:validation:Optional
	Domain string `json:"domain,omitempty"`

	// Address of the DNS server
	// +kubebuilder:validation:Required
	Address string `json:"address"`

	// Port of the DNS server
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=53
	Port *int32 `json:"port,omitempty"`
}

// NetworkDNSHost represents DNS hostname record
type NetworkDNSHost struct {
	// IP address
	// +kubebuilder:validation:Required
	IP string `json:"ip"`

	// Hostname
	// +kubebuilder:validation:Required
	Hostname string `json:"hostname"`
}

// NetworkDNSSRV represents DNS SRV record
type NetworkDNSSRV struct {
	// Service name
	// +kubebuilder:validation:Required
	Service string `json:"service"`

	// Protocol (tcp, udp)
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=tcp;udp
	Protocol string `json:"protocol"`

	// Target hostname
	// +kubebuilder:validation:Required
	Target string `json:"target"`

	// Port number
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port"`

	// Priority
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=0
	Priority *int32 `json:"priority,omitempty"`

	// Weight
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=0
	Weight *int32 `json:"weight,omitempty"`

	// Domain
	// +kubebuilder:validation:Optional
	Domain string `json:"domain,omitempty"`
}

// NetworkDNSTXT represents DNS TXT record
type NetworkDNSTXT struct {
	// Name of the record
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Value of the TXT record
	// +kubebuilder:validation:Required
	Value string `json:"value"`
}

// NetworkForward represents network forwarding configuration
type NetworkForward struct {
	// Mode of forwarding (nat, route, open, bridge)
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=nat;route;open;bridge
	Mode string `json:"mode"`

	// Device to forward to
	// +kubebuilder:validation:Optional
	Dev string `json:"dev,omitempty"`

	// Managed determines if libvirt manages the interface
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	Managed *bool `json:"managed,omitempty"`

	// NAT configuration
	// +kubebuilder:validation:Optional
	NAT *NetworkNAT `json:"nat,omitempty"`

	// Interfaces to forward to
	// +kubebuilder:validation:Optional
	Interfaces []NetworkInterface `json:"interfaces,omitempty"`

	// Physical function for SR-IOV
	// +kubebuilder:validation:Optional
	PF *NetworkPF `json:"pf,omitempty"`
}

// NetworkNAT represents NAT configuration
type NetworkNAT struct {
	// Port ranges for NAT
	// +kubebuilder:validation:Optional
	Ports []NetworkNATPort `json:"ports,omitempty"`

	// Addresses for NAT
	// +kubebuilder:validation:Optional
	Addresses []NetworkNATAddress `json:"addresses,omitempty"`
}

// NetworkNATPort represents NAT port configuration
type NetworkNATPort struct {
	// Start port of the range
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Start int32 `json:"start"`

	// End port of the range
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	End int32 `json:"end"`
}

// NetworkNATAddress represents NAT address configuration
type NetworkNATAddress struct {
	// Start address of the range
	// +kubebuilder:validation:Required
	Start string `json:"start"`

	// End address of the range
	// +kubebuilder:validation:Required
	End string `json:"end"`
}

// NetworkInterface represents interface for forwarding
type NetworkInterface struct {
	// Device name
	// +kubebuilder:validation:Required
	Dev string `json:"dev"`

	// Connections (for bridge mode)
	// +kubebuilder:validation:Optional
	Connections *int32 `json:"connections,omitempty"`
}

// NetworkPF represents physical function for SR-IOV
type NetworkPF struct {
	// Device name
	// +kubebuilder:validation:Required
	Dev string `json:"dev"`

	// VLAN configuration
	// +kubebuilder:validation:Optional
	VLAN *NetworkVLAN `json:"vlan,omitempty"`
}

// NetworkVLAN represents VLAN configuration
type NetworkVLAN struct {
	// VLAN ID
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=4094
	ID int32 `json:"id"`

	// Trunk configuration
	// +kubebuilder:validation:Optional
	Trunk *bool `json:"trunk,omitempty"`
}

// NetworkStatus defines the observed state of Network
type NetworkStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          NetworkObservation `json:"atProvider,omitempty"`
}

// NetworkObservation are the observable fields of a Network.
type NetworkObservation struct {
	// UUID is the libvirt network UUID
	UUID string `json:"uuid,omitempty"`

	// Active indicates if the network is currently active
	Active bool `json:"active,omitempty"`

	// Persistent indicates if the network is persistent
	Persistent bool `json:"persistent,omitempty"`

	// AutoStart indicates if the network will auto-start
	AutoStart bool `json:"autoStart,omitempty"`

	// Bridge name (for bridge mode networks)
	BridgeName string `json:"bridgeName,omitempty"`

	// Connected domains count
	ConnectedDomains int32 `json:"connectedDomains,omitempty"`

	// Lease information from DHCP
	Leases []NetworkLease `json:"leases,omitempty"`
}

// NetworkLease represents a DHCP lease
type NetworkLease struct {
	// MAC address of the lease
	MAC string `json:"mac,omitempty"`

	// IP address of the lease
	IP string `json:"ip,omitempty"`

	// Hostname of the lease
	Hostname string `json:"hostname,omitempty"`

	// Client ID of the lease
	ClientID string `json:"clientId,omitempty"`

	// Expiry time of the lease
	Expiry *metav1.Time `json:"expiry,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="MODE",type="string",JSONPath=".spec.forProvider.mode"
// +kubebuilder:printcolumn:name="ACTIVE",type="boolean",JSONPath=".status.atProvider.active"
// +kubebuilder:printcolumn:name="BRIDGE",type="string",JSONPath=".status.atProvider.bridgeName"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// A Network is a libvirt virtual network.
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


// NetworkGroupKind is the GroupKind for Network
var NetworkGroupKind = schema.GroupKind{
	Group: Group,
	Kind:  "Network",
}

// NetworkGroupVersionKind is the GroupVersionKind for Network
var NetworkGroupVersionKind = schema.GroupVersionKind{
	Group:   Group,
	Version: Version,
	Kind:    "Network",
}

// NetworkKind is the kind for Network
const NetworkKind = "Network"

func init() {
	SchemeBuilder.Register(&Network{}, &NetworkList{})
}