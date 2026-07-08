package network

import (
	"github.com/rossigee/provider-libvirt/apis/v1beta1"
	"testing"
)

// Phase 1 API Tests - Domain, Bridge STP Delay

func TestGenerateNetworkXMLWithDomain(t *testing.T) {
	ext := &external{client: nil}
	cr := testNetwork(func(n *v1beta1.Network) {
		n.Spec.ForProvider.Domain = "example.com"
	})

	xml := ext.generateNetworkXML(cr)
	if !contains(xml, `<domain name='example.com'/>`) {
		t.Error("Network XML should contain domain configuration")
	}
}

func TestGenerateNetworkXMLWithoutDomain(t *testing.T) {
	ext := &external{client: nil}
	cr := testNetwork(func(n *v1beta1.Network) {
		n.Spec.ForProvider.Domain = ""
	})

	xml := ext.generateNetworkXML(cr)
	if contains(xml, "<domain") {
		t.Error("Network XML should not have domain element when not specified")
	}
}

func TestGenerateNetworkXMLWithBridgeSTPDelay(t *testing.T) {
	ext := &external{client: nil}
	delay := int32(10)
	cr := testNetwork(func(n *v1beta1.Network) {
		n.Spec.ForProvider.Mode = "bridge"
		n.Spec.ForProvider.Bridge = &v1beta1.NetworkBridge{
			Name:     "br0",
			STPDelay: &delay,
		}
	})

	xml := ext.generateNetworkXML(cr)
	if !contains(xml, `delay='10'`) {
		t.Error("Network XML should contain STP delay value")
	}
}

func TestGenerateNetworkXMLWithBridgeSTPDelayZero(t *testing.T) {
	ext := &external{client: nil}
	delay := int32(0)
	cr := testNetwork(func(n *v1beta1.Network) {
		n.Spec.ForProvider.Mode = "bridge"
		n.Spec.ForProvider.Bridge = &v1beta1.NetworkBridge{
			Name:     "br0",
			STPDelay: &delay,
		}
	})

	xml := ext.generateNetworkXML(cr)
	if !contains(xml, `delay='0'`) {
		t.Error("Network XML should handle zero STP delay")
	}
}

func TestGenerateNetworkXMLWithDefaultBridgeSTPDelay(t *testing.T) {
	ext := &external{client: nil}
	cr := testNetwork(func(n *v1beta1.Network) {
		n.Spec.ForProvider.Mode = "bridge"
		n.Spec.ForProvider.Bridge = &v1beta1.NetworkBridge{
			Name: "br0",
		}
	})

	xml := ext.generateNetworkXML(cr)
	if !contains(xml, `delay='0'`) {
		t.Error("Network XML should default STP delay to 0")
	}
}

func TestGenerateNetworkXMLBridgeSTPDelayVariations(t *testing.T) {
	delays := []int32{0, 1, 5, 10, 30}

	for _, delayVal := range delays {
		ext := &external{client: nil}
		delayPtr := delayVal
		cr := testNetwork(func(n *v1beta1.Network) {
			n.Spec.ForProvider.Mode = "bridge"
			n.Spec.ForProvider.Bridge = &v1beta1.NetworkBridge{
				Name:     "br0",
				STPDelay: &delayPtr,
			}
		})

		xml := ext.generateNetworkXML(cr)
		if !containsPattern(xml, "delay='") {
			t.Errorf("Network XML should contain STP delay for value: %d", delayVal)
		}
	}
}

func TestGenerateNetworkXMLDomainAndDNS(t *testing.T) {
	ext := &external{client: nil}
	cr := testNetwork(func(n *v1beta1.Network) {
		n.Spec.ForProvider.Domain = "example.com"
		n.Spec.ForProvider.DNS = &v1beta1.NetworkDNS{
			Forwarders: []string{"8.8.8.8", "8.8.4.4"},
		}
	})

	xml := ext.generateNetworkXML(cr)
	if !contains(xml, `<domain name='example.com'/>`) {
		t.Error("Network XML should contain domain")
	}
	if !contains(xml, "<dns>") {
		t.Error("Network XML should contain DNS configuration")
	}
	if !contains(xml, "8.8.8.8") && !contains(xml, "8.8.4.4") {
		t.Error("Network XML should contain DNS forwarders")
	}
}

func TestGenerateNetworkXMLDomainPresence(t *testing.T) {
	ext := &external{client: nil}
	cr := testNetwork(func(n *v1beta1.Network) {
		n.Spec.ForProvider.Domain = "example.com"
	})

	xml := ext.generateNetworkXML(cr)
	domainPos := findStringPosition(xml, "<domain")
	bridgePos := findStringPosition(xml, "<bridge")

	if domainPos == -1 {
		t.Error("Network should have domain element")
	}
	if bridgePos == -1 {
		t.Error("Network should have bridge element")
	}
	// Both elements should be present
	if domainPos == -1 || bridgePos == -1 {
		t.Error("Network XML should contain both domain and bridge elements")
	}
}

func TestGenerateNetworkXMLNATModeSTPDelay(t *testing.T) {
	ext := &external{client: nil}
	cr := testNetwork(func(n *v1beta1.Network) {
		n.Spec.ForProvider.Mode = "nat"
		// Bridge is optional in NAT mode - should default to virbr-networkname
	})

	xml := ext.generateNetworkXML(cr)
	// NAT mode doesn't use bridge STPDelay, it auto-creates bridge
	if !contains(xml, "virbr-") {
		t.Error("NAT mode should auto-create bridge")
	}
}

// Helper functions

func containsPattern(s, pattern string) bool {
	return findStringPosition(s, pattern) != -1
}

func findStringPosition(s, substr string) int {
	for i := 0; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
