# HostInterface Resource Design

## Overview
HostInterface resource enables enterprise-grade physical network interface management on libvirt hypervisor hosts. Critical for network bonding, VLAN configuration, and advanced networking topologies.

## Resource Structure

```go
// HostInterfaceSpec defines the desired state of HostInterface
type HostInterfaceSpec struct {
    xpv1.ResourceSpec `json:",inline"`
    ForProvider       HostInterfaceParameters `json:"forProvider"`
}

// HostInterfaceParameters are the configurable fields of a HostInterface.
type HostInterfaceParameters struct {
    // Interface name
    // +kubebuilder:validation:Required
    Name string `json:"name"`

    // Interface type: ethernet, bond, bridge, vlan
    // +kubebuilder:validation:Required
    // +kubebuilder:validation:Enum=ethernet;bond;bridge;vlan;macvlan;macvtap
    Type string `json:"type"`

    // Start interface on creation
    // +kubebuilder:validation:Optional
    // +kubebuilder:default=true
    StartMode bool `json:"startMode,omitempty"`

    // Interface protocol configuration
    // +kubebuilder:validation:Optional
    Protocol *InterfaceProtocol `json:"protocol,omitempty"`

    // Bond configuration (for bond interfaces)
    // +kubebuilder:validation:Optional
    Bond *BondConfiguration `json:"bond,omitempty"`

    // Bridge configuration (for bridge interfaces)
    // +kubebuilder:validation:Optional
    Bridge *BridgeConfiguration `json:"bridge,omitempty"`

    // VLAN configuration (for VLAN interfaces)
    // +kubebuilder:validation:Optional
    VLAN *VLANConfiguration `json:"vlan,omitempty"`

    // MAC address (optional, auto-assigned if not specified)
    // +kubebuilder:validation:Optional
    MACAddress string `json:"macAddress,omitempty"`

    // MTU size
    // +kubebuilder:validation:Optional
    // +kubebuilder:default=1500
    // +kubebuilder:validation:Minimum=68
    // +kubebuilder:validation:Maximum=9000
    MTU int32 `json:"mtu,omitempty"`

    // Interface persistence across reboots
    // +kubebuilder:validation:Optional
    // +kubebuilder:default=true
    Persistent bool `json:"persistent,omitempty"`
}

// InterfaceProtocol defines network protocol configuration
type InterfaceProtocol struct {
    // IP address family: ipv4, ipv6, or both
    // +kubebuilder:validation:Optional
    // +kubebuilder:validation:Enum=ipv4;ipv6;dual
    // +kubebuilder:default="ipv4"
    Family string `json:"family,omitempty"`

    // IPv4 configuration
    // +kubebuilder:validation:Optional
    IPv4 *IPv4Configuration `json:"ipv4,omitempty"`

    // IPv6 configuration
    // +kubebuilder:validation:Optional
    IPv6 *IPv6Configuration `json:"ipv6,omitempty"`
}

// IPv4Configuration defines IPv4 network settings
type IPv4Configuration struct {
    // Configuration method: static, dhcp, none
    // +kubebuilder:validation:Required
    // +kubebuilder:validation:Enum=static;dhcp;none
    Method string `json:"method"`

    // Static IP address (for static method)
    // +kubebuilder:validation:Optional
    Address string `json:"address,omitempty"`

    // Network prefix length (for static method)
    // +kubebuilder:validation:Optional
    // +kubebuilder:validation:Minimum=1
    // +kubebuilder:validation:Maximum=32
    Prefix int32 `json:"prefix,omitempty"`

    // Gateway address (for static method)
    // +kubebuilder:validation:Optional
    Gateway string `json:"gateway,omitempty"`

    // DNS servers
    // +kubebuilder:validation:Optional
    DNS []string `json:"dns,omitempty"`
}

// IPv6Configuration defines IPv6 network settings
type IPv6Configuration struct {
    // Configuration method: static, dhcp, autoconf, none
    // +kubebuilder:validation:Required
    // +kubebuilder:validation:Enum=static;dhcp;autoconf;none
    Method string `json:"method"`

    // Static IP address (for static method)
    // +kubebuilder:validation:Optional
    Address string `json:"address,omitempty"`

    // Network prefix length (for static method)
    // +kubebuilder:validation:Optional
    // +kubebuilder:validation:Minimum=1
    // +kubebuilder:validation:Maximum=128
    Prefix int32 `json:"prefix,omitempty"`

    // Gateway address (for static method)
    // +kubebuilder:validation:Optional
    Gateway string `json:"gateway,omitempty"`

    // DNS servers
    // +kubebuilder:validation:Optional
    DNS []string `json:"dns,omitempty"`
}

// BondConfiguration defines network bonding settings
type BondConfiguration struct {
    // Bonding mode: active-backup, balance-rr, balance-xor, broadcast, 802.3ad, balance-tlb, balance-alb
    // +kubebuilder:validation:Required
    // +kubebuilder:validation:Enum=active-backup;balance-rr;balance-xor;broadcast;802.3ad;balance-tlb;balance-alb
    Mode string `json:"mode"`

    // Slave interfaces
    // +kubebuilder:validation:Required
    // +kubebuilder:validation:MinItems=2
    Slaves []string `json:"slaves"`

    // Primary interface (for active-backup mode)
    // +kubebuilder:validation:Optional
    Primary string `json:"primary,omitempty"`

    // MII monitoring interval (milliseconds)
    // +kubebuilder:validation:Optional
    // +kubebuilder:default=100
    MIIMonitorInterval int32 `json:"miiMonitorInterval,omitempty"`

    // Link failure threshold
    // +kubebuilder:validation:Optional
    // +kubebuilder:default=1
    UpDelay int32 `json:"upDelay,omitempty"`

    // Link recovery threshold
    // +kubebuilder:validation:Optional
    // +kubebuilder:default=1
    DownDelay int32 `json:"downDelay,omitempty"`

    // LACP rate (for 802.3ad mode): slow or fast
    // +kubebuilder:validation:Optional
    // +kubebuilder:validation:Enum=slow;fast
    LACPRate string `json:"lacpRate,omitempty"`
}

// BridgeConfiguration defines bridge settings
type BridgeConfiguration struct {
    // Enable STP
    // +kubebuilder:validation:Optional
    // +kubebuilder:default=true
    STP bool `json:"stp,omitempty"`

    // STP forward delay
    // +kubebuilder:validation:Optional
    // +kubebuilder:default=0
    ForwardDelay int32 `json:"forwardDelay,omitempty"`

    // Member interfaces
    // +kubebuilder:validation:Optional
    Members []string `json:"members,omitempty"`
}

// VLANConfiguration defines VLAN settings
type VLANConfiguration struct {
    // Parent interface
    // +kubebuilder:validation:Required
    Parent string `json:"parent"`

    // VLAN ID
    // +kubebuilder:validation:Required
    // +kubebuilder:validation:Minimum=1
    // +kubebuilder:validation:Maximum=4094
    ID int32 `json:"id"`

    // VLAN protocol: 802.1Q or 802.1ad
    // +kubebuilder:validation:Optional
    // +kubebuilder:validation:Enum=802.1Q;802.1ad
    // +kubebuilder:default="802.1Q"
    Protocol string `json:"protocol,omitempty"`
}

// HostInterfaceStatus defines the observed state of HostInterface
type HostInterfaceStatus struct {
    xpv1.ResourceStatus `json:",inline"`
    AtProvider          HostInterfaceObservation `json:"atProvider,omitempty"`
}

// HostInterfaceObservation are the observable fields of a HostInterface.
type HostInterfaceObservation struct {
    // Interface name
    Name string `json:"name,omitempty"`

    // Interface state: active, inactive
    State string `json:"state,omitempty"`

    // MAC address
    MACAddress string `json:"macAddress,omitempty"`

    // Interface type
    Type string `json:"type,omitempty"`

    // Current IP addresses
    IPAddresses []InterfaceAddress `json:"ipAddresses,omitempty"`

    // Interface statistics
    Statistics *InterfaceStatistics `json:"statistics,omitempty"`

    // Bond status (for bonded interfaces)
    BondStatus *BondStatus `json:"bondStatus,omitempty"`

    // Bridge status (for bridge interfaces)
    BridgeStatus *BridgeStatus `json:"bridgeStatus,omitempty"`

    // VLAN status (for VLAN interfaces)
    VLANStatus *VLANStatus `json:"vlanStatus,omitempty"`
}

// InterfaceAddress represents an IP address assigned to interface
type InterfaceAddress struct {
    // IP address
    Address string `json:"address"`

    // Prefix length
    Prefix int32 `json:"prefix"`

    // Address family: ipv4 or ipv6
    Family string `json:"family"`
}

// InterfaceStatistics represents interface traffic statistics
type InterfaceStatistics struct {
    // Received packets
    RxPackets int64 `json:"rxPackets"`

    // Transmitted packets
    TxPackets int64 `json:"txPackets"`

    // Received bytes
    RxBytes int64 `json:"rxBytes"`

    // Transmitted bytes
    TxBytes int64 `json:"txBytes"`

    // Receive errors
    RxErrors int64 `json:"rxErrors"`

    // Transmit errors
    TxErrors int64 `json:"txErrors"`
}

// BondStatus represents bonding interface status
type BondStatus struct {
    // Active bonding mode
    Mode string `json:"mode"`

    // Active slave interfaces
    ActiveSlaves []string `json:"activeSlaves"`

    // Backup slave interfaces
    BackupSlaves []string `json:"backupSlaves,omitempty"`

    // Primary interface
    Primary string `json:"primary,omitempty"`
}

// BridgeStatus represents bridge interface status
type BridgeStatus struct {
    // STP enabled
    STPEnabled bool `json:"stpEnabled"`

    // Bridge ID
    BridgeID string `json:"bridgeId"`

    // Member ports
    Ports []BridgePort `json:"ports,omitempty"`
}

// BridgePort represents a bridge port
type BridgePort struct {
    // Port name
    Name string `json:"name"`

    // Port state
    State string `json:"state"`

    // Port cost
    Cost int32 `json:"cost"`
}

// VLANStatus represents VLAN interface status
type VLANStatus struct {
    // VLAN ID
    ID int32 `json:"id"`

    // Parent interface
    Parent string `json:"parent"`

    // VLAN protocol
    Protocol string `json:"protocol"`
}
```

## Key Features

### 1. **Multi-Interface Type Support**
- **Ethernet**: Physical and virtual ethernet interfaces
- **Bond**: NIC teaming for redundancy and bandwidth aggregation
- **Bridge**: Software bridges for VM networking
- **VLAN**: Network segmentation and isolation

### 2. **Enterprise Networking**
- **High Availability**: Active-backup and LACP bonding
- **Performance**: Load balancing across multiple interfaces
- **Segmentation**: VLAN support for network isolation
- **Monitoring**: Interface statistics and status reporting

### 3. **Configuration Examples**

```yaml
# High-Availability Bond Interface
apiVersion: libvirt.crossplane.io/v1alpha1
kind: HostInterface
metadata:
  name: bond-primary
spec:
  forProvider:
    name: bond0
    type: bond
    bond:
      mode: active-backup
      slaves: ["eth0", "eth1"]
      primary: "eth0"
      miiMonitorInterval: 100
    protocol:
      family: ipv4
      ipv4:
        method: static
        address: "192.168.1.100"
        prefix: 24
        gateway: "192.168.1.1"
        dns: ["8.8.8.8", "8.8.4.4"]
---
# VLAN Interface for Tenant Isolation
apiVersion: libvirt.crossplane.io/v1alpha1
kind: HostInterface
metadata:
  name: vlan-tenant-100
spec:
  forProvider:
    name: vlan100
    type: vlan
    vlan:
      parent: bond0
      id: 100
    protocol:
      family: ipv4
      ipv4:
        method: dhcp
---
# Bridge for VM Networking
apiVersion: libvirt.crossplane.io/v1alpha1
kind: HostInterface
metadata:
  name: vm-bridge
spec:
  forProvider:
    name: virbr1
    type: bridge
    bridge:
      stp: true
      members: ["vlan100"]
    protocol:
      family: ipv4
      ipv4:
        method: none  # Bridge doesn't need IP
```

### 4. **Integration with Existing Resources**
```yaml
# Network using HostInterface bridge
apiVersion: libvirt.crossplane.io/v1alpha1
kind: Network
metadata:
  name: tenant-network
spec:
  forProvider:
    name: tenant-100
    forward:
      mode: bridge
      bridge: virbr1  # References the HostInterface bridge
```

## Implementation Phases

1. **Phase 1**: Basic ethernet interface management
2. **Phase 2**: Bond interface support (active-backup, 802.3ad)
3. **Phase 3**: Bridge interface management
4. **Phase 4**: VLAN interface support
5. **Phase 5**: Advanced features (macvlan, macvtap)

## Enterprise Value

- **🔧 Infrastructure Management**: Declarative host network configuration
- **📊 Monitoring**: Interface status and statistics
- **🛡️ High Availability**: Bond configuration for redundancy
- **🌐 Network Segmentation**: VLAN support for multi-tenancy
- **🔄 Integration**: Seamless integration with Network resources
- **⚡ Performance**: Load balancing and bandwidth aggregation