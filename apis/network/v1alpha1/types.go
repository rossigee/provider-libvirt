/*
Copyright 2025 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
)

// NetworkDHCPRange defines a DHCP address range within a network.
type NetworkDHCPRange struct {
	// Start is the first IP address in the DHCP range.
	Start string `json:"start"`
	// End is the last IP address in the DHCP range.
	End string `json:"end"`
}

// NetworkParameters are the configurable fields of a libvirt virtual network.
type NetworkParameters struct {
	// Name of the virtual network.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Mode is the network forwarding mode (nat, route, bridge, none, open).
	// +kubebuilder:validation:Enum=nat;route;bridge;none;open
	// +kubebuilder:default=nat
	// +optional
	Mode *string `json:"mode,omitempty"`

	// Bridge is the host bridge interface name to use.
	// +optional
	Bridge *string `json:"bridge,omitempty"`

	// Domain is the DNS domain name for the network.
	// +optional
	Domain *string `json:"domain,omitempty"`

	// Addresses is the list of CIDR subnets for this network (e.g. ["192.168.100.0/24"]).
	// +optional
	Addresses []string `json:"addresses,omitempty"`

	// DHCPRanges defines DHCP address ranges. If empty, DHCP is disabled.
	// +optional
	DHCPRanges []NetworkDHCPRange `json:"dhcpRanges,omitempty"`

	// Autostart controls whether the network is started on host boot.
	// +kubebuilder:default=true
	// +optional
	Autostart *bool `json:"autostart,omitempty"`
}

// NetworkObservation holds the observed state of a libvirt virtual network.
type NetworkObservation struct {
	// UUID is the libvirt-assigned UUID of the network.
	UUID string `json:"uuid,omitempty"`

	// BridgeName is the actual bridge interface name assigned by libvirt.
	BridgeName string `json:"bridgeName,omitempty"`

	// Active indicates whether the network is currently active.
	Active bool `json:"active,omitempty"`
}

// NetworkSpec defines the desired state of a Network.
type NetworkSpec struct {
	xpv1.ManagedResourceSpec `json:",inline"`
	ForProvider              NetworkParameters `json:"forProvider"`
}

// NetworkStatus defines the observed state of a Network.
type NetworkStatus struct {
	xpv1.ManagedResourceStatus `json:",inline"`
	AtProvider                 NetworkObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// Network is a managed resource representing a libvirt virtual network.
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="MODE",type="string",JSONPath=".spec.forProvider.mode"
// +kubebuilder:printcolumn:name="BRIDGE",type="string",JSONPath=".status.atProvider.bridgeName"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Cluster,categories={crossplane,provider,libvirt}
type Network struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NetworkSpec   `json:"spec"`
	Status NetworkStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NetworkList contains a list of Network.
type NetworkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Network `json:"items"`
}
