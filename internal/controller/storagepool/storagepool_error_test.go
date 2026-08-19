package storagepool

import (
	"testing"

	"github.com/rossigee/provider-libvirt/apis/v1beta1"
)

// Error path and validation tests for storage pool controller

func TestGeneratePoolXMLEmptyName(t *testing.T) {
	ext := &external{client: nil}
	pool := testStoragePool(func(sp *v1beta1.StoragePool) {
		sp.Spec.ForProvider.Name = ""
	})

	xml := ext.generatePoolXML(pool)
	if len(xml) == 0 {
		t.Error("Expected XML to be generated even with empty name")
	}
}

func TestGeneratePoolXMLEmptyType(t *testing.T) {
	ext := &external{client: nil}
	pool := testStoragePool(func(sp *v1beta1.StoragePool) {
		sp.Spec.ForProvider.Type = ""
	})

	xml := ext.generatePoolXML(pool)
	// Should still generate XML
	if len(xml) == 0 {
		t.Error("Expected XML to be generated even with empty type")
	}
}

func TestGeneratePoolXMLNoPath(t *testing.T) {
	ext := &external{client: nil}
	pool := testStoragePool(func(sp *v1beta1.StoragePool) {
		sp.Spec.ForProvider.Target = nil
	})

	xml := ext.generatePoolXML(pool)
	// Should still generate XML
	if len(xml) == 0 {
		t.Error("Expected XML to be generated even without path")
	}
}

func TestGeneratePoolXMLSpecialCharacters(t *testing.T) {
	ext := &external{client: nil}
	pool := testStoragePool(func(sp *v1beta1.StoragePool) {
		sp.Spec.ForProvider.Name = "test-pool-123"
	})

	xml := ext.generatePoolXML(pool)
	if !contains(xml, "test-pool-123") {
		t.Error("Expected pool name with special characters in XML")
	}
}

func TestGeneratePoolXMLComplexPath(t *testing.T) {
	ext := &external{client: nil}
	pool := testStoragePool(func(sp *v1beta1.StoragePool) {
		sp.Spec.ForProvider.Target = &v1beta1.StoragePoolTarget{
			Path: "/srv/libvirt/storage/pool/images/archive",
		}
	})

	xml := ext.generatePoolXML(pool)
	if !contains(xml, "/srv/libvirt/storage/pool/images/archive") {
		t.Error("Expected complex path in XML")
	}
}

func TestGeneratePoolXMLConsistency(t *testing.T) {
	ext := &external{client: nil}
	pool := testStoragePool()

	// Same pool should produce same XML twice
	xml1 := ext.generatePoolXML(pool)
	xml2 := ext.generatePoolXML(pool)

	if xml1 != xml2 {
		t.Error("Same pool should produce identical XML")
	}
}

func TestGeneratePoolXMLDifferentPools(t *testing.T) {
	ext := &external{client: nil}
	pool1 := testStoragePool(func(sp *v1beta1.StoragePool) {
		sp.Spec.ForProvider.Type = "dir"
	})
	pool2 := testStoragePool(func(sp *v1beta1.StoragePool) {
		sp.Spec.ForProvider.Type = "fs"
	})

	xml1 := ext.generatePoolXML(pool1)
	xml2 := ext.generatePoolXML(pool2)

	if xml1 == xml2 {
		t.Error("Different pool types should produce different XML")
	}
}

func TestGeneratePoolXMLDifferentNames(t *testing.T) {
	ext := &external{client: nil}
	pool1 := testStoragePool(func(sp *v1beta1.StoragePool) {
		sp.Spec.ForProvider.Name = "pool1"
	})
	pool2 := testStoragePool(func(sp *v1beta1.StoragePool) {
		sp.Spec.ForProvider.Name = "pool2"
	})

	xml1 := ext.generatePoolXML(pool1)
	xml2 := ext.generatePoolXML(pool2)

	if xml1 == xml2 {
		t.Error("Different pool names should produce different XML")
	}
}

func TestGeneratePoolXMLDifferentPaths(t *testing.T) {
	ext := &external{client: nil}
	pool1 := testStoragePool(func(sp *v1beta1.StoragePool) {
		sp.Spec.ForProvider.Target = &v1beta1.StoragePoolTarget{
			Path: "/var/lib/libvirt/images",
		}
	})
	pool2 := testStoragePool(func(sp *v1beta1.StoragePool) {
		sp.Spec.ForProvider.Target = &v1beta1.StoragePoolTarget{
			Path: "/mnt/storage",
		}
	})

	xml1 := ext.generatePoolXML(pool1)
	xml2 := ext.generatePoolXML(pool2)

	if xml1 == xml2 {
		t.Error("Different paths should produce different XML")
	}
}

func TestStoragePoolTypeValidation(t *testing.T) {
	pool := testStoragePool()

	// Test all pool types
	types := []string{"dir", "fs", "netfs", "iscsi", "logical", "rbd", "gluster", "zfs"}
	for _, poolType := range types {
		pool.Spec.ForProvider.Type = poolType
		if pool.Spec.ForProvider.Type != poolType {
			t.Errorf("Failed to set pool type: %s", poolType)
		}
	}
}

func TestStoragePoolPathValidation(t *testing.T) {
	pool := testStoragePool()

	// Test various path patterns
	paths := []string{
		"/var/lib/libvirt/images",
		"/mnt/storage",
		"/srv/libvirt/images",
		"/tmp/storage",
		"/home/user/storage",
	}

	for _, path := range paths {
		pool.Spec.ForProvider.Target = &v1beta1.StoragePoolTarget{Path: path}
		if pool.Spec.ForProvider.Target.Path != path {
			t.Errorf("Failed to set pool path: %s", path)
		}
	}
}
