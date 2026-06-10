package domain

import (
	"testing"

	"github.com/rossigee/provider-libvirt/apis/v1beta1"
)

// Phase 1 API Tests - WWN, Console Target, Boot Devices, Machine Type

func TestGenerateDomainXMLWithDiskWWN(t *testing.T) {
	ext := &external{client: nil}
	cr := testDomain(func(d *v1beta1.Domain) {
		d.Spec.ForProvider.Disk = []v1beta1.DomainDisk{
			{
				File:   "/var/lib/libvirt/images/disk.qcow2",
				Device: "vda",
				WWN:    "6001405abc1234567890",
			},
		}
	})

	xml := ext.generateDomainXML(cr)
	if !contains(xml, "6001405abc1234567890") {
		t.Error("Domain XML should contain disk WWN")
	}
	if !contains(xml, "<wwn>") {
		t.Error("Domain XML should have WWN element")
	}
}

func TestGenerateDomainXMLWithoutDiskWWN(t *testing.T) {
	ext := &external{client: nil}
	cr := testDomain(func(d *v1beta1.Domain) {
		d.Spec.ForProvider.Disk = []v1beta1.DomainDisk{
			{
				File:   "/var/lib/libvirt/images/disk.qcow2",
				Device: "vda",
			},
		}
	})

	xml := ext.generateDomainXML(cr)
	// Should not have WWN element when not specified
	diskSection := extractDiskSection(xml)
	if contains(diskSection, "<wwn>") {
		t.Error("Domain XML should not have WWN element when not specified")
	}
}

func TestGenerateDomainXMLWithConsoleTarget(t *testing.T) {
	ext := &external{client: nil}
	cr := testDomain(func(d *v1beta1.Domain) {
		d.Spec.ForProvider.Console = []v1beta1.DomainConsole{
			{
				Type:   "pty",
				Target: "virtio",
			},
		}
	})

	xml := ext.generateDomainXML(cr)
	if !contains(xml, `<target type='virtio'/>`) {
		t.Error("Domain XML should contain console target")
	}
}

func TestGenerateDomainXMLWithDefaultConsoleTarget(t *testing.T) {
	ext := &external{client: nil}
	cr := testDomain(func(d *v1beta1.Domain) {
		d.Spec.ForProvider.Console = []v1beta1.DomainConsole{
			{
				Type: "pty",
			},
		}
	})

	xml := ext.generateDomainXML(cr)
	if !contains(xml, `<target type='virtio'/>`) {
		t.Error("Domain XML should have default virtio console target")
	}
}

func TestGenerateDomainXMLWithBootDevices(t *testing.T) {
	ext := &external{client: nil}
	cr := testDomain(func(d *v1beta1.Domain) {
		d.Spec.ForProvider.Boot = []string{"disk", "network"}
	})

	xml := ext.generateDomainXML(cr)
	if !contains(xml, `<boot dev='disk'/>`) {
		t.Error("Domain XML should contain disk boot device")
	}
	if !contains(xml, `<boot dev='network'/>`) {
		t.Error("Domain XML should contain network boot device")
	}
}

func TestGenerateDomainXMLWithoutBootDevices(t *testing.T) {
	ext := &external{client: nil}
	cr := testDomain()

	xml := ext.generateDomainXML(cr)
	// Should not have boot element when not specified
	osSection := extractOSSection(xml)
	if contains(osSection, "<boot") {
		t.Error("Domain XML should not have boot element when not specified")
	}
}

func TestGenerateDomainXMLWithMachineType(t *testing.T) {
	ext := &external{client: nil}
	cr := testDomain(func(d *v1beta1.Domain) {
		d.Spec.ForProvider.Machine = "q35"
	})

	xml := ext.generateDomainXML(cr)
	if !contains(xml, `machine='q35'`) {
		t.Error("Domain XML should contain machine type")
	}
}

func TestGenerateDomainXMLWithoutMachineType(t *testing.T) {
	ext := &external{client: nil}
	cr := testDomain()

	xml := ext.generateDomainXML(cr)
	// Should not have machine attribute when not specified
	osType := extractOSType(xml)
	if contains(osType, "machine=") {
		t.Error("Domain XML should not have machine attribute when not specified")
	}
}

func TestGenerateDomainXMLBootDeviceOrder(t *testing.T) {
	ext := &external{client: nil}
	cr := testDomain(func(d *v1beta1.Domain) {
		d.Spec.ForProvider.Boot = []string{"network", "disk", "cdrom"}
	})

	xml := ext.generateDomainXML(cr)
	networkPos := findPosition(xml, `<boot dev='network'/>`)
	diskPos := findPosition(xml, `<boot dev='disk'/>`)
	cdromPos := findPosition(xml, `<boot dev='cdrom'/>`)

	if networkPos == -1 || diskPos == -1 || cdromPos == -1 {
		t.Error("All boot devices should be present")
	}
	if networkPos > diskPos || diskPos > cdromPos {
		t.Error("Boot devices should be in specified order")
	}
}

func TestGenerateDomainXMLMultipleDiskWWN(t *testing.T) {
	ext := &external{client: nil}
	cr := testDomain(func(d *v1beta1.Domain) {
		d.Spec.ForProvider.Disk = []v1beta1.DomainDisk{
			{
				File:   "/var/lib/libvirt/images/disk1.qcow2",
				Device: "vda",
				WWN:    "6001405abc1234567890",
			},
			{
				File:   "/var/lib/libvirt/images/disk2.qcow2",
				Device: "vdb",
				WWN:    "6001405def9876543210",
			},
		}
	})

	xml := ext.generateDomainXML(cr)
	if !contains(xml, "6001405abc1234567890") {
		t.Error("Domain XML should contain first disk WWN")
	}
	if !contains(xml, "6001405def9876543210") {
		t.Error("Domain XML should contain second disk WWN")
	}
}

func TestGenerateDomainXMLMachineTypeVariations(t *testing.T) {
	machineTypes := []string{"pc", "q35", "pseries", "arm-genericv7-machine"}

	for _, machineType := range machineTypes {
		ext := &external{client: nil}
		cr := testDomain(func(d *v1beta1.Domain) {
			d.Spec.ForProvider.Machine = machineType
		})

		xml := ext.generateDomainXML(cr)
		if !contains(xml, machineType) {
			t.Errorf("Domain XML should contain machine type: %s", machineType)
		}
	}
}

// Helper functions

func findPosition(s, substr string) int {
	for i := 0; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func extractDiskSection(xml string) string {
	start := findPosition(xml, "<disk")
	if start == -1 {
		return ""
	}
	end := findPosition(xml[start:], "</disk>")
	if end == -1 {
		return ""
	}
	return xml[start : start+end+7]
}

func extractOSSection(xml string) string {
	start := findPosition(xml, "<os>")
	if start == -1 {
		return ""
	}
	end := findPosition(xml[start:], "</os>")
	if end == -1 {
		return ""
	}
	return xml[start : start+end+5]
}

func extractOSType(xml string) string {
	start := findPosition(xml, "<type")
	if start == -1 {
		return ""
	}
	end := findPosition(xml[start:], ">")
	if end == -1 {
		return ""
	}
	return xml[start : start+end+1]
}
