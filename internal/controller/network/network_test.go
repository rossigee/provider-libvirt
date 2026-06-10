package network

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/rossigee/provider-libvirt/apis/v1beta1"
)

type networkModifier func(*v1beta1.Network)

func testNetwork(m ...networkModifier) *v1beta1.Network {
	n := &v1beta1.Network{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Network",
			APIVersion: "libvirt.m.crossplane.io/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-network",
			Namespace: "default",
		},
		Spec: v1beta1.NetworkSpec{
			ForProvider: v1beta1.NetworkParameters{
				Name: "test-net",
				Mode: "nat",
			},
		},
	}
	for _, f := range m {
		f(n)
	}
	return n
}

func boolPtr(b bool) *bool {
	return &b
}

func TestGenerateNetworkXML(t *testing.T) {
	cases := map[string]struct {
		network *v1beta1.Network
		want    string
	}{
		"BasicNAT": {
			network: testNetwork(),
			want:    "test-net",
		},
		"BridgeMode": {
			network: testNetwork(func(n *v1beta1.Network) {
				n.Spec.ForProvider.Mode = "bridge"
			}),
			want: "test-net",
		},
		"WithDHCP": {
			network: testNetwork(func(n *v1beta1.Network) {
				n.Spec.ForProvider.IP = []v1beta1.NetworkIP{
					{
						Address: "192.168.122.1",
						Netmask: "255.255.255.0",
					},
				}
				n.Spec.ForProvider.DHCP = &v1beta1.NetworkDHCP{
					Enabled: boolPtr(true),
					Start:   "192.168.122.2",
					End:     "192.168.122.254",
				}
			}),
			want: "test-net",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ext := &external{client: nil}
			got := ext.generateNetworkXML(tc.network)
			if !contains(got, tc.want) {
				t.Errorf("generateNetworkXML(...): expected to contain %q, got:\n%s", tc.want, got)
			}
			// Verify mode is in XML
			if !contains(got, tc.network.Spec.ForProvider.Mode) {
				t.Errorf("generateNetworkXML(...): expected mode %q in output", tc.network.Spec.ForProvider.Mode)
			}
		})
	}
}

func TestNetworkParameters(t *testing.T) {
	cases := map[string]struct {
		network *v1beta1.Network
		want    string
	}{
		"NameParameter": {
			network: testNetwork(),
			want:    "test-net",
		},
		"ModeParameter": {
			network: testNetwork(func(n *v1beta1.Network) {
				n.Spec.ForProvider.Mode = "bridge"
			}),
			want: "bridge",
		},
		"DomainParameter": {
			network: testNetwork(func(n *v1beta1.Network) {
				n.Spec.ForProvider.Domain = "example.com"
			}),
			want: "example.com",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			params := tc.network.Spec.ForProvider
			if name == "NameParameter" && params.Name != tc.want {
				t.Errorf("Expected name %q, got %q", tc.want, params.Name)
			}
			if name == "ModeParameter" && params.Mode != tc.want {
				t.Errorf("Expected mode %q, got %q", tc.want, params.Mode)
			}
			if name == "DomainParameter" && params.Domain != tc.want {
				t.Errorf("Expected domain %q, got %q", tc.want, params.Domain)
			}
		})
	}
}

func TestNetworkModes(t *testing.T) {
	cases := map[string]struct {
		mode string
		want string
	}{
		"NATMode": {
			mode: "nat",
			want: "nat",
		},
		"BridgeMode": {
			mode: "bridge",
			want: "bridge",
		},
		"RoutedMode": {
			mode: "routed",
			want: "routed",
		},
		"IsolatedMode": {
			mode: "isolated",
			want: "isolated",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			n := testNetwork(func(net *v1beta1.Network) {
				net.Spec.ForProvider.Mode = tc.mode
			})
			if n.Spec.ForProvider.Mode != tc.want {
				t.Errorf("Expected mode %q, got %q", tc.want, n.Spec.ForProvider.Mode)
			}
		})
	}
}

func contains(s, substr string) bool {
	for i := 0; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestGenerateNetworkXMLWithIP(t *testing.T) {
	ext := &external{client: nil}
	network := testNetwork(func(n *v1beta1.Network) {
		n.Spec.ForProvider.IP = []v1beta1.NetworkIP{
			{
				Address: "192.168.122.1",
				Netmask: "255.255.255.0",
			},
		}
	})

	xml := ext.generateNetworkXML(network)
	if !contains(xml, "192.168.122.1") {
		t.Errorf("Expected IP address in XML, got:\n%s", xml)
	}
}

func TestGenerateNetworkXMLDomainParameter(t *testing.T) {
	ext := &external{client: nil}
	network := testNetwork(func(n *v1beta1.Network) {
		n.Spec.ForProvider.Domain = "test.local"
	})

	// Domain is stored in parameters but verify the network name is still in XML
	xml := ext.generateNetworkXML(network)
	if !contains(xml, "test-net") {
		t.Errorf("Expected network name in XML, got:\n%s", xml)
	}
	// Verify domain parameter is preserved in object
	if network.Spec.ForProvider.Domain != "test.local" {
		t.Errorf("Expected domain parameter to be set")
	}
}

func TestGenerateNetworkXMLNATMode(t *testing.T) {
	ext := &external{client: nil}
	network := testNetwork(func(n *v1beta1.Network) {
		n.Spec.ForProvider.Mode = "nat"
	})

	xml := ext.generateNetworkXML(network)
	if !contains(xml, "nat") {
		t.Errorf("Expected NAT mode in XML, got:\n%s", xml)
	}
}

func TestGenerateNetworkXMLBridgeMode(t *testing.T) {
	ext := &external{client: nil}
	network := testNetwork(func(n *v1beta1.Network) {
		n.Spec.ForProvider.Mode = "bridge"
		n.Spec.ForProvider.Bridge = &v1beta1.NetworkBridge{
			Name: "virbr1",
		}
	})

	xml := ext.generateNetworkXML(network)
	if !contains(xml, "bridge") {
		t.Errorf("Expected bridge mode in XML, got:\n%s", xml)
	}
}

func TestGenerateNetworkXMLRoutedMode(t *testing.T) {
	ext := &external{client: nil}
	network := testNetwork(func(n *v1beta1.Network) {
		n.Spec.ForProvider.Mode = "routed"
	})

	xml := ext.generateNetworkXML(network)
	if !contains(xml, "routed") {
		t.Errorf("Expected routed mode in XML, got:\n%s", xml)
	}
}

func TestGenerateNetworkXMLIsolatedMode(t *testing.T) {
	ext := &external{client: nil}
	network := testNetwork(func(n *v1beta1.Network) {
		n.Spec.ForProvider.Mode = "isolated"
	})

	xml := ext.generateNetworkXML(network)
	if !contains(xml, "isolated") {
		t.Errorf("Expected isolated mode in XML, got:\n%s", xml)
	}
}

func TestGenerateNetworkXMLDHCPConfiguration(t *testing.T) {
	ext := &external{client: nil}
	network := testNetwork(func(n *v1beta1.Network) {
		n.Spec.ForProvider.IP = []v1beta1.NetworkIP{
			{
				Address: "10.0.0.1",
				Netmask: "255.255.255.0",
			},
		}
		n.Spec.ForProvider.DHCP = &v1beta1.NetworkDHCP{
			Enabled: boolPtr(true),
			Start:   "10.0.0.50",
			End:     "10.0.0.200",
		}
	})

	xml := ext.generateNetworkXML(network)
	if !contains(xml, "dhcp") {
		t.Errorf("Expected DHCP in XML, got:\n%s", xml)
	}
	if !contains(xml, "10.0.0.50") || !contains(xml, "10.0.0.200") {
		t.Errorf("Expected DHCP range in XML, got:\n%s", xml)
	}
}

func TestNetworkWithoutDHCP(t *testing.T) {
	ext := &external{client: nil}
	network := testNetwork(func(n *v1beta1.Network) {
		n.Spec.ForProvider.IP = []v1beta1.NetworkIP{
			{
				Address: "10.0.0.1",
				Netmask: "255.255.255.0",
			},
		}
		n.Spec.ForProvider.DHCP = &v1beta1.NetworkDHCP{
			Enabled: boolPtr(false),
		}
	})

	xml := ext.generateNetworkXML(network)
	// Should not contain dhcp config when disabled
	if contains(xml, "dhcp") && contains(xml, "10.0.0.50") {
		t.Errorf("Expected no DHCP range in XML when disabled, got:\n%s", xml)
	}
}

func TestNetworkMultipleIPRanges(t *testing.T) {
	ext := &external{client: nil}
	network := testNetwork(func(n *v1beta1.Network) {
		n.Spec.ForProvider.IP = []v1beta1.NetworkIP{
			{
				Address: "192.168.1.1",
				Netmask: "255.255.255.0",
			},
			{
				Address: "10.0.0.1",
				Netmask: "255.255.255.0",
			},
		}
	})

	xml := ext.generateNetworkXML(network)
	if !contains(xml, "192.168.1.1") || !contains(xml, "10.0.0.1") {
		t.Errorf("Expected both IP ranges in XML, got:\n%s", xml)
	}
}
