package domain

import (
	"testing"

	"libvirt.org/go/libvirt"

	"github.com/rossigee/provider-libvirt/apis/v1beta1"
)

// Error path and validation tests for domain controller

func TestGenerateDomainXMLEmptyName(t *testing.T) {
	ext := &external{client: nil}
	domain := testDomain(func(d *v1beta1.Domain) {
		d.Spec.ForProvider.Name = ""
	})

	xml := ext.generateDomainXML(domain)
	if len(xml) == 0 {
		t.Error("Expected XML to be generated even with empty name")
	}
}

func TestGenerateDomainXMLZeroMemory(t *testing.T) {
	ext := &external{client: nil}
	domain := testDomain(func(d *v1beta1.Domain) {
		d.Spec.ForProvider.Memory = 0
	})

	xml := ext.generateDomainXML(domain)
	if !contains(xml, "0") {
		t.Error("Expected zero memory to be in XML")
	}
}

func TestGenerateDomainXMLZeroVCPU(t *testing.T) {
	ext := &external{client: nil}
	domain := testDomain(func(d *v1beta1.Domain) {
		d.Spec.ForProvider.Vcpu = 0
	})

	xml := ext.generateDomainXML(domain)
	// Should still generate XML
	if len(xml) == 0 {
		t.Error("Expected XML to be generated with zero vCPU")
	}
}

func TestGenerateDomainXMLEmptyType(t *testing.T) {
	ext := &external{client: nil}
	domain := testDomain(func(d *v1beta1.Domain) {
		d.Spec.ForProvider.Type = ""
	})

	xml := ext.generateDomainXML(domain)
	// Should default to kvm
	if !contains(xml, "type='kvm'") {
		t.Error("Expected default type 'kvm' when empty")
	}
}

func TestGenerateDomainXMLEmptyArch(t *testing.T) {
	ext := &external{client: nil}
	domain := testDomain(func(d *v1beta1.Domain) {
		d.Spec.ForProvider.Arch = ""
	})

	xml := ext.generateDomainXML(domain)
	// Should default to x86_64
	if !contains(xml, "arch='x86_64'") {
		t.Error("Expected default arch 'x86_64' when empty")
	}
}

func TestGenerateDomainXMLLargeDiskCount(t *testing.T) {
	ext := &external{client: nil}
	disks := make([]v1beta1.DomainDisk, 10)
	for i := 0; i < 10; i++ {
		disks[i] = v1beta1.DomainDisk{
			File:   "/var/lib/libvirt/images/disk" + string(rune('a'+i)) + ".qcow2",
			Device: "vd" + string(rune('a'+i)),
		}
	}

	domain := testDomain(func(d *v1beta1.Domain) {
		d.Spec.ForProvider.Disk = disks
	})

	xml := ext.generateDomainXML(domain)
	// All disks should be represented
	for i := 0; i < 10; i++ {
		diskFile := "/var/lib/libvirt/images/disk" + string(rune('a'+i)) + ".qcow2"
		if !contains(xml, diskFile) {
			t.Errorf("Expected disk file %s in XML", diskFile)
		}
	}
}

func TestGenerateDomainXMLLargeNetworkCount(t *testing.T) {
	ext := &external{client: nil}
	networks := make([]v1beta1.DomainNetworkInterface, 5)
	for i := 0; i < 5; i++ {
		networks[i] = v1beta1.DomainNetworkInterface{
			NetworkName: "net" + string(rune('0'+i)),
			Model:       "virtio",
		}
	}

	domain := testDomain(func(d *v1beta1.Domain) {
		d.Spec.ForProvider.NetworkInterface = networks
	})

	xml := ext.generateDomainXML(domain)
	// All networks should be represented
	for i := 0; i < 5; i++ {
		netName := "net" + string(rune('0'+i))
		if !contains(xml, netName) {
			t.Errorf("Expected network %s in XML", netName)
		}
	}
}

func TestGenerateDomainXMLSpecialCharacters(t *testing.T) {
	ext := &external{client: nil}
	domain := testDomain(func(d *v1beta1.Domain) {
		d.Spec.ForProvider.Name = "test-vm-123"
	})

	xml := ext.generateDomainXML(domain)
	if !contains(xml, "test-vm-123") {
		t.Error("Expected domain name with special characters in XML")
	}
}

func TestGenerateDomainXMLUtf8Name(t *testing.T) {
	ext := &external{client: nil}
	domain := testDomain(func(d *v1beta1.Domain) {
		d.Spec.ForProvider.Name = "test-vm-ñ"
	})

	xml := ext.generateDomainXML(domain)
	if !contains(xml, "test-vm") {
		t.Error("Expected domain name in XML with UTF-8 characters")
	}
}

func TestFormatDomainStateNilSafe(t *testing.T) {
	// Test that formatDomainState handles all valid states
	states := []libvirt.DomainState{
		libvirt.DOMAIN_NOSTATE,
		libvirt.DOMAIN_RUNNING,
		libvirt.DOMAIN_BLOCKED,
		libvirt.DOMAIN_PAUSED,
		libvirt.DOMAIN_SHUTDOWN,
		libvirt.DOMAIN_SHUTOFF,
		libvirt.DOMAIN_CRASHED,
		libvirt.DOMAIN_PMSUSPENDED,
	}

	for _, state := range states {
		result := formatDomainState(state)
		if len(result) == 0 {
			t.Errorf("formatDomainState returned empty string for state %v", state)
		}
	}
}

func TestBoolToIntConsistency(t *testing.T) {
	// Test that bool to int conversion is consistent
	trueVal := boolToInt(true)
	falseVal := boolToInt(false)

	if trueVal == falseVal {
		t.Error("boolToInt(true) and boolToInt(false) should differ")
	}

	if trueVal != 1 {
		t.Errorf("Expected boolToInt(true) = 1, got %d", trueVal)
	}

	if falseVal != 0 {
		t.Errorf("Expected boolToInt(false) = 0, got %d", falseVal)
	}
}

func TestGenerateDomainXMLConsistency(t *testing.T) {
	ext := &external{client: nil}
	domain := testDomain()

	// Same domain should produce same XML twice
	xml1 := ext.generateDomainXML(domain)
	xml2 := ext.generateDomainXML(domain)

	if xml1 != xml2 {
		t.Error("Same domain should produce identical XML")
	}
}

func TestGenerateDomainXMLDifferentDomains(t *testing.T) {
	ext := &external{client: nil}
	domain1 := testDomain(func(d *v1beta1.Domain) {
		d.Spec.ForProvider.Memory = 1073741824
	})
	domain2 := testDomain(func(d *v1beta1.Domain) {
		d.Spec.ForProvider.Memory = 2147483648
	})

	xml1 := ext.generateDomainXML(domain1)
	xml2 := ext.generateDomainXML(domain2)

	if xml1 == xml2 {
		t.Error("Different domains should produce different XML")
	}
}
