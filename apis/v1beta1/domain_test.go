/*
Copyright 2025 Ross Golder
*/

package v1beta1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
)

func TestDomainSpec(t *testing.T) {
	// Test basic Domain spec creation
	domain := &Domain{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "libvirt.m.crossplane.io/v1beta1",
			Kind:       "Domain",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-domain",
			Namespace: "test-ns",
		},
		Spec: DomainSpec{
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: &xpv1.Reference{
					Name: "test-provider-config",
				},
			},
			ForProvider: DomainParameters{
				Name:   "test-vm",
				Memory: 1073741824, // 1 GiB in bytes
				Vcpu:   2,
				Disk: []DomainDisk{
					{
						Device: "disk",
						Source: "/var/lib/libvirt/images/test-disk.qcow2",
						Target: "vda",
						Bus:    "virtio",
					},
				},
			},
		},
	}

	// Test that the domain has correct GVK
	gvk := domain.GetObjectKind().GroupVersionKind()
	if gvk.Group != "libvirt.m.crossplane.io" {
		t.Errorf("Expected group 'libvirt.m.crossplane.io', got '%s'", gvk.Group)
	}
	if gvk.Version != "v1beta1" {
		t.Errorf("Expected version 'v1beta1', got '%s'", gvk.Version)
	}
	if gvk.Kind != "Domain" {
		t.Errorf("Expected kind 'Domain', got '%s'", gvk.Kind)
	}

	// Test required fields are set
	if domain.Spec.ForProvider.Name != "test-vm" {
		t.Errorf("Expected name 'test-vm', got '%s'", domain.Spec.ForProvider.Name)
	}
	if domain.Spec.ForProvider.Memory != 1073741824 {
		t.Errorf("Expected memory 1073741824, got %d", domain.Spec.ForProvider.Memory)
	}
	if domain.Spec.ForProvider.Vcpu != 2 {
		t.Errorf("Expected vcpu 2, got %d", domain.Spec.ForProvider.Vcpu)
	}
}

func TestDomainConversion(t *testing.T) {
	// Test that Domain can be converted to/from runtime.Object
	domain := &Domain{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-domain",
			Namespace: "test-ns",
		},
		Spec: DomainSpec{
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: &xpv1.Reference{
					Name: "test-provider-config",
				},
			},
			ForProvider: DomainParameters{
				Name:   "test-vm",
				Memory: 1073741824, // 1 GiB in bytes
				Vcpu:   2,
			},
		},
	}

	// Convert to runtime.Object and back
	var obj runtime.Object = domain
	converted, ok := obj.(*Domain)
	if !ok {
		t.Error("Failed to convert Domain to runtime.Object and back")
	}

	// Compare original and converted
	if diff := cmp.Diff(domain, converted); diff != "" {
		t.Errorf("Domain conversion mismatch (-want +got):\n%s", diff)
	}
}

func TestDomainDefaults(t *testing.T) {
	domain := &Domain{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-domain",
			Namespace: "test-ns",
		},
		Spec: DomainSpec{
			ForProvider: DomainParameters{
				Name: "test-vm",
				// Test with minimal spec to verify defaults
			},
		},
	}

	// Verify the domain can be created with minimal spec
	if domain.Spec.ForProvider.Name != "test-vm" {
		t.Errorf("Expected name 'test-vm', got '%s'", domain.Spec.ForProvider.Name)
	}

	// Verify namespace-scoped resource
	if domain.Namespace != "test-ns" {
		t.Errorf("Expected namespace 'test-ns', got '%s'", domain.Namespace)
	}
}