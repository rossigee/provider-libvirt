package domain

import (
	"testing"

	"github.com/rossigee/provider-libvirt/apis/v1beta1"
)

// Phase 2 API Tests - WaitForLease

func TestGenerateDomainXMLWithWaitForLease(t *testing.T) {
	ext := &external{client: nil}
	cr := testDomain(func(d *v1beta1.Domain) {
		d.Spec.ForProvider.NetworkInterface = []v1beta1.DomainNetworkInterface{
			{
				NetworkName: "default",
				WaitForLease: true,
			},
		}
	})

	xml := ext.generateDomainXML(cr)
	if !contains(xml, `waitForLease='yes'`) {
		t.Error("Domain XML should contain waitForLease attribute when enabled")
	}
}

func TestGenerateDomainXMLWithoutWaitForLease(t *testing.T) {
	ext := &external{client: nil}
	cr := testDomain(func(d *v1beta1.Domain) {
		d.Spec.ForProvider.NetworkInterface = []v1beta1.DomainNetworkInterface{
			{
				NetworkName: "default",
				WaitForLease: false,
			},
		}
	})

	xml := ext.generateDomainXML(cr)
	if contains(xml, "waitForLease") {
		t.Error("Domain XML should not contain waitForLease attribute when disabled")
	}
}

func TestGenerateDomainXMLWaitForLeaseWithMAC(t *testing.T) {
	ext := &external{client: nil}
	cr := testDomain(func(d *v1beta1.Domain) {
		d.Spec.ForProvider.NetworkInterface = []v1beta1.DomainNetworkInterface{
			{
				NetworkName:  "default",
				Mac:          "52:54:00:12:34:56",
				WaitForLease: true,
			},
		}
	})

	xml := ext.generateDomainXML(cr)
	if !contains(xml, `waitForLease='yes'`) {
		t.Error("Domain XML should contain waitForLease with MAC address")
	}
	if !contains(xml, "52:54:00:12:34:56") {
		t.Error("Domain XML should contain MAC address")
	}
}

func TestGenerateDomainXMLMultipleInterfacesWaitForLease(t *testing.T) {
	ext := &external{client: nil}
	cr := testDomain(func(d *v1beta1.Domain) {
		d.Spec.ForProvider.NetworkInterface = []v1beta1.DomainNetworkInterface{
			{
				NetworkName:  "default",
				WaitForLease: true,
			},
			{
				NetworkName:  "internal",
				WaitForLease: false,
			},
		}
	})

	xml := ext.generateDomainXML(cr)
	// Should have waitForLease in first interface
	interfaceCount := 0
	i := 0
	for i < len(xml) {
		pos := findPosition(xml[i:], "<interface")
		if pos == -1 {
			break
		}
		interfaceCount++
		i += pos + 10
	}
	if interfaceCount < 2 {
		t.Error("Domain XML should have multiple network interfaces")
	}
}

func TestGenerateDomainXMLWaitForLeaseVariations(t *testing.T) {
	scenarios := []struct {
		name             string
		waitForLease     bool
		shouldContain    string
		shouldNotContain string
	}{
		{
			name:          "Wait for lease enabled",
			waitForLease:  true,
			shouldContain: "waitForLease='yes'",
		},
		{
			name:             "Wait for lease disabled",
			waitForLease:     false,
			shouldNotContain: "waitForLease",
		},
	}

	for _, scenario := range scenarios {
		ext := &external{client: nil}
		cr := testDomain(func(d *v1beta1.Domain) {
			d.Spec.ForProvider.NetworkInterface = []v1beta1.DomainNetworkInterface{
				{
					NetworkName:  "default",
					WaitForLease: scenario.waitForLease,
				},
			}
		})

		xml := ext.generateDomainXML(cr)

		if scenario.shouldContain != "" && !contains(xml, scenario.shouldContain) {
			t.Errorf("%s: XML should contain '%s'", scenario.name, scenario.shouldContain)
		}
		if scenario.shouldNotContain != "" && contains(xml, scenario.shouldNotContain) {
			t.Errorf("%s: XML should not contain '%s'", scenario.name, scenario.shouldNotContain)
		}
	}
}

func TestGenerateDomainXMLWaitForLeaseWithModel(t *testing.T) {
	ext := &external{client: nil}
	cr := testDomain(func(d *v1beta1.Domain) {
		d.Spec.ForProvider.NetworkInterface = []v1beta1.DomainNetworkInterface{
			{
				NetworkName:  "default",
				Model:        "virtio",
				WaitForLease: true,
			},
		}
	})

	xml := ext.generateDomainXML(cr)
	if !contains(xml, `waitForLease='yes'`) {
		t.Error("Domain XML should have waitForLease with model")
	}
	if !contains(xml, `type='virtio'`) {
		t.Error("Domain XML should have virtio model")
	}
}

func TestGenerateDomainXMLWaitForLeaseOrdering(t *testing.T) {
	ext := &external{client: nil}
	cr := testDomain(func(d *v1beta1.Domain) {
		d.Spec.ForProvider.NetworkInterface = []v1beta1.DomainNetworkInterface{
			{
				NetworkName:  "default",
				Mac:          "52:54:00:12:34:56",
				WaitForLease: true,
			},
		}
	})

	xml := ext.generateDomainXML(cr)

	// Check that waitForLease is in the source element
	if !contains(xml, `<source network='default' waitForLease='yes'/>`) {
		t.Error("waitForLease should be in source element")
	}
}
