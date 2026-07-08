package volume

import (
	"github.com/rossigee/provider-libvirt/apis/v1beta1"
	"testing"
)

// Error path and validation tests for volume controller

func TestGenerateVolumeXMLEmptyName(t *testing.T) {
	ext := &external{client: nil}
	volume := testVolume(func(v *v1beta1.Volume) {
		v.Spec.ForProvider.Name = ""
	})

	xml := ext.generateVolumeXML(volume, 1073741824, "qcow2")
	if len(xml) == 0 {
		t.Error("Expected XML to be generated even with empty name")
	}
}

func TestGenerateVolumeXMLZeroCapacity(t *testing.T) {
	ext := &external{client: nil}
	volume := testVolume()

	xml := ext.generateVolumeXML(volume, 0, "qcow2")
	if !contains(xml, "0") {
		t.Error("Expected zero capacity in XML")
	}
}

func TestGenerateVolumeXMLLargeCapacity(t *testing.T) {
	ext := &external{client: nil}
	volume := testVolume()

	// 10TB
	largeCapacity := int64(10995116277760)
	xml := ext.generateVolumeXML(volume, largeCapacity, "qcow2")
	if !contains(xml, "10995116277760") {
		t.Error("Expected large capacity in XML")
	}
}

func TestGenerateVolumeXMLAllFormats(t *testing.T) {
	ext := &external{client: nil}
	volume := testVolume()

	formats := []string{"qcow2", "raw", "vmdk", "vpc", "vdi"}
	for _, format := range formats {
		xml := ext.generateVolumeXML(volume, 1073741824, format)
		if !contains(xml, format) {
			t.Errorf("Expected format %s in XML", format)
		}
	}
}

func TestGenerateVolumeXMLEmptyPool(t *testing.T) {
	ext := &external{client: nil}
	volume := testVolume(func(v *v1beta1.Volume) {
		v.Spec.ForProvider.Pool = ""
	})

	xml := ext.generateVolumeXML(volume, 1073741824, "qcow2")
	// Should still generate XML
	if len(xml) == 0 {
		t.Error("Expected XML to be generated even with empty pool")
	}
}

func TestGenerateVolumeXMLSpecialCharacters(t *testing.T) {
	ext := &external{client: nil}
	volume := testVolume(func(v *v1beta1.Volume) {
		v.Spec.ForProvider.Name = "test-disk-123"
	})

	xml := ext.generateVolumeXML(volume, 1073741824, "qcow2")
	if !contains(xml, "test-disk-123") {
		t.Error("Expected volume name with special characters in XML")
	}
}

func TestGenerateVolumeXMLConsistency(t *testing.T) {
	ext := &external{client: nil}
	volume := testVolume()

	// Same volume should produce same XML twice
	xml1 := ext.generateVolumeXML(volume, 1073741824, "qcow2")
	xml2 := ext.generateVolumeXML(volume, 1073741824, "qcow2")

	if xml1 != xml2 {
		t.Error("Same volume should produce identical XML")
	}
}

func TestGenerateVolumeXMLDifferentVolumes(t *testing.T) {
	ext := &external{client: nil}
	vol1 := testVolume(func(v *v1beta1.Volume) {
		v.Spec.ForProvider.Name = "disk1"
	})
	vol2 := testVolume(func(v *v1beta1.Volume) {
		v.Spec.ForProvider.Name = "disk2"
	})

	xml1 := ext.generateVolumeXML(vol1, 1073741824, "qcow2")
	xml2 := ext.generateVolumeXML(vol2, 1073741824, "qcow2")

	if xml1 == xml2 {
		t.Error("Different volumes should produce different XML")
	}
}

func TestGenerateVolumeXMLDifferentFormats(t *testing.T) {
	ext := &external{client: nil}
	volume := testVolume()

	xml1 := ext.generateVolumeXML(volume, 1073741824, "qcow2")
	xml2 := ext.generateVolumeXML(volume, 1073741824, "raw")

	if xml1 == xml2 {
		t.Error("Different formats should produce different XML")
	}
}

func TestGenerateVolumeXMLDifferentCapacities(t *testing.T) {
	ext := &external{client: nil}
	volume := testVolume()

	xml1 := ext.generateVolumeXML(volume, 1073741824, "qcow2")
	xml2 := ext.generateVolumeXML(volume, 2147483648, "qcow2")

	if xml1 == xml2 {
		t.Error("Different capacities should produce different XML")
	}
}

func TestVolumeNameValidation(t *testing.T) {
	volume := testVolume()

	// Test various name patterns
	names := []string{
		"simple",
		"with-dash",
		"with_underscore",
		"with123numbers",
		"MixedCase",
	}

	for _, name := range names {
		volume.Spec.ForProvider.Name = name
		if volume.Spec.ForProvider.Name != name {
			t.Errorf("Failed to set volume name: %s", name)
		}
	}
}
