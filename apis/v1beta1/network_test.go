/*
Copyright 2025 Ross Golder
*/

package v1beta1

import (
	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"testing"
)

// Helper function for bool pointers
func boolPtr(b bool) *bool {
	return &b
}

func TestNetworkSpec(t *testing.T) {
	// Test basic Network spec creation
	network := &Network{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "libvirt.m.crossplane.io/v1beta1",
			Kind:       "Network",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-network",
			Namespace: "test-ns",
		},
		Spec: NetworkSpec{
			ManagedResourceSpec: xpv1.ManagedResourceSpec{
				ProviderConfigReference: &xpv1.ProviderConfigReference{
					Name: "test-provider-config",
				},
			},
			ForProvider: NetworkParameters{
				Name:      "test-net",
				Mode:      "nat",
				Bridge:    &NetworkBridge{Name: "virbr0"},
				Domain:    "test.local",
				Addresses: []string{"192.168.100.1/24"},
				DNS: &NetworkDNS{
					Enabled:    boolPtr(true),
					Forwarders: []string{"8.8.8.8", "8.8.4.4"},
				},
				DHCP: &NetworkDHCP{
					Enabled: boolPtr(true),
					Start:   "192.168.100.2",
					End:     "192.168.100.254",
				},
			},
		},
	}

	// Test that the network has correct GVK
	gvk := network.GetObjectKind().GroupVersionKind()
	if gvk.Group != "libvirt.m.crossplane.io" {
		t.Errorf("Expected group 'libvirt.m.crossplane.io', got '%s'", gvk.Group)
	}
	if gvk.Version != "v1beta1" {
		t.Errorf("Expected version 'v1beta1', got '%s'", gvk.Version)
	}
	if gvk.Kind != "Network" {
		t.Errorf("Expected kind 'Network', got '%s'", gvk.Kind)
	}

	// Test required fields are set
	if network.Spec.ForProvider.Name != "test-net" {
		t.Errorf("Expected name 'test-net', got '%s'", network.Spec.ForProvider.Name)
	}
	if network.Spec.ForProvider.Mode != "nat" {
		t.Errorf("Expected mode 'nat', got '%s'", network.Spec.ForProvider.Mode)
	}
	if network.Spec.ForProvider.Bridge == nil || network.Spec.ForProvider.Bridge.Name != "virbr0" {
		t.Errorf("Expected bridge name 'virbr0', got %v", network.Spec.ForProvider.Bridge)
	}
}

func TestNetworkConversion(t *testing.T) {
	// Test that Network can be converted to/from runtime.Object
	network := &Network{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-network",
			Namespace: "test-ns",
		},
		Spec: NetworkSpec{
			ManagedResourceSpec: xpv1.ManagedResourceSpec{
				ProviderConfigReference: &xpv1.ProviderConfigReference{
					Name: "test-provider-config",
				},
			},
			ForProvider: NetworkParameters{
				Name: "test-net",
				Mode: "nat",
			},
		},
	}

	// Convert to runtime.Object and back
	var obj runtime.Object = network
	converted, ok := obj.(*Network)
	if !ok {
		t.Error("Failed to convert Network to runtime.Object and back")
	}

	// Compare original and converted
	if diff := cmp.Diff(network, converted); diff != "" {
		t.Errorf("Network conversion mismatch (-want +got):n%s", diff)
	}
}

func TestNetworkDHCP(t *testing.T) {
	network := &Network{
		Spec: NetworkSpec{
			ForProvider: NetworkParameters{
				Name:      "test-dhcp-net",
				Mode:      "nat",
				Addresses: []string{"192.168.100.1/24"},
				DHCP: &NetworkDHCP{
					Enabled: boolPtr(true),
					Start:   "192.168.100.2",
					End:     "192.168.100.254",
				},
			},
		},
	}

	// Verify DHCP configuration
	if network.Spec.ForProvider.DHCP == nil {
		t.Error("Expected DHCP configuration to be set")
	}
	if network.Spec.ForProvider.DHCP.Start != "192.168.100.2" {
		t.Errorf("Expected DHCP start '192.168.100.2', got '%s'", network.Spec.ForProvider.DHCP.Start)
	}
	if network.Spec.ForProvider.DHCP.End != "192.168.100.254" {
		t.Errorf("Expected DHCP end '192.168.100.254', got '%s'", network.Spec.ForProvider.DHCP.End)
	}

	// Verify namespace-scoped resource
	if network.Namespace != "" {
		t.Errorf("Expected no namespace (cluster-scoped), got '%s'", network.Namespace)
	}
}
