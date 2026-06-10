package network

import (
	"testing"

	"github.com/rossigee/provider-libvirt/apis/v1beta1"
)

// Error path and validation tests for network controller

func TestGenerateNetworkXMLEmptyName(t *testing.T) {
	ext := &external{client: nil}
	network := testNetwork(func(n *v1beta1.Network) {
		n.Spec.ForProvider.Name = ""
	})

	xml := ext.generateNetworkXML(network)
	if len(xml) == 0 {
		t.Error("Expected XML to be generated even with empty name")
	}
}

func TestGenerateNetworkXMLEmptyMode(t *testing.T) {
	ext := &external{client: nil}
	network := testNetwork(func(n *v1beta1.Network) {
		n.Spec.ForProvider.Mode = ""
	})

	xml := ext.generateNetworkXML(network)
	// Should still generate valid XML
	if !contains(xml, "network") {
		t.Error("Expected valid network XML")
	}
}

func TestGenerateNetworkXMLNoIP(t *testing.T) {
	ext := &external{client: nil}
	network := testNetwork(func(n *v1beta1.Network) {
		n.Spec.ForProvider.IP = nil
	})

	xml := ext.generateNetworkXML(network)
	// Should still generate XML without IP block
	if !contains(xml, "network") {
		t.Error("Expected network XML without IP configuration")
	}
}

func TestGenerateNetworkXMLNoDHCP(t *testing.T) {
	ext := &external{client: nil}
	network := testNetwork(func(n *v1beta1.Network) {
		n.Spec.ForProvider.DHCP = nil
	})

	xml := ext.generateNetworkXML(network)
	// Should generate XML without DHCP
	if !contains(xml, "network") {
		t.Error("Expected network XML without DHCP")
	}
}

func TestGenerateNetworkXMLMultipleIPs(t *testing.T) {
	ext := &external{client: nil}
	network := testNetwork(func(n *v1beta1.Network) {
		n.Spec.ForProvider.IP = []v1beta1.NetworkIP{
			{Address: "192.168.1.1", Netmask: "255.255.255.0"},
			{Address: "10.0.0.1", Netmask: "255.255.255.0"},
			{Address: "172.16.0.1", Netmask: "255.255.255.0"},
		}
	})

	xml := ext.generateNetworkXML(network)
	// All IPs should be represented
	if !contains(xml, "192.168.1.1") || !contains(xml, "10.0.0.1") || !contains(xml, "172.16.0.1") {
		t.Error("Expected all IP addresses in XML")
	}
}

func TestGenerateNetworkXMLSpecialCharacters(t *testing.T) {
	ext := &external{client: nil}
	network := testNetwork(func(n *v1beta1.Network) {
		n.Spec.ForProvider.Name = "test-net-123"
		n.Spec.ForProvider.Domain = "test.example.com"
	})

	xml := ext.generateNetworkXML(network)
	if !contains(xml, "test-net-123") {
		t.Error("Expected network name with special characters in XML")
	}
}

func TestGenerateNetworkXMLLargeIPRange(t *testing.T) {
	ext := &external{client: nil}
	network := testNetwork(func(n *v1beta1.Network) {
		n.Spec.ForProvider.IP = []v1beta1.NetworkIP{
			{Address: "192.168.1.1", Netmask: "255.255.0.0"},
		}
		n.Spec.ForProvider.DHCP = &v1beta1.NetworkDHCP{
			Enabled: boolPtr(true),
			Start:   "192.168.1.10",
			End:     "192.168.255.254",
		}
	})

	xml := ext.generateNetworkXML(network)
	if !contains(xml, "192.168.1.10") || !contains(xml, "192.168.255.254") {
		t.Error("Expected full DHCP range in XML")
	}
}

func TestGenerateNetworkXMLDHCPDisabled(t *testing.T) {
	ext := &external{client: nil}
	enabled := false
	network := testNetwork(func(n *v1beta1.Network) {
		n.Spec.ForProvider.IP = []v1beta1.NetworkIP{
			{Address: "192.168.1.1", Netmask: "255.255.255.0"},
		}
		n.Spec.ForProvider.DHCP = &v1beta1.NetworkDHCP{
			Enabled: &enabled,
			Start:   "192.168.1.10",
			End:     "192.168.1.254",
		}
	})

	xml := ext.generateNetworkXML(network)
	// Should generate XML but potentially without DHCP configuration
	if !contains(xml, "network") {
		t.Error("Expected valid network XML even with disabled DHCP")
	}
}

func TestGenerateNetworkXMLConsistency(t *testing.T) {
	ext := &external{client: nil}
	network := testNetwork()

	// Same network should produce same XML twice
	xml1 := ext.generateNetworkXML(network)
	xml2 := ext.generateNetworkXML(network)

	if xml1 != xml2 {
		t.Error("Same network should produce identical XML")
	}
}

func TestGenerateNetworkXMLDifferentNetworks(t *testing.T) {
	ext := &external{client: nil}
	net1 := testNetwork(func(n *v1beta1.Network) {
		n.Spec.ForProvider.Mode = "nat"
	})
	net2 := testNetwork(func(n *v1beta1.Network) {
		n.Spec.ForProvider.Mode = "bridge"
	})

	xml1 := ext.generateNetworkXML(net1)
	xml2 := ext.generateNetworkXML(net2)

	if xml1 == xml2 {
		t.Error("Different network modes should produce different XML")
	}
}

func TestGenerateNetworkXMLBridgeNameHandling(t *testing.T) {
	ext := &external{client: nil}
	bridge := "virbr-custom"
	network := testNetwork(func(n *v1beta1.Network) {
		n.Spec.ForProvider.Mode = "bridge"
		n.Spec.ForProvider.Bridge = &v1beta1.NetworkBridge{
			Name: bridge,
		}
	})

	xml := ext.generateNetworkXML(network)
	if !contains(xml, bridge) {
		t.Errorf("Expected bridge name %s in XML", bridge)
	}
}
