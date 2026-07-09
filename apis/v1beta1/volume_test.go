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

// Helper function to create pointer to int64
func int64Ptr(i int64) *int64 {
	return &i
}

func TestVolumeSpec(t *testing.T) {
	// Test basic Volume spec creation
	volume := &Volume{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "libvirt.m.crossplane.io/v1beta1",
			Kind:       "Volume",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-volume",
			Namespace: "test-ns",
		},
		Spec: VolumeSpec{
			ManagedResourceSpec: xpv1.ManagedResourceSpec{
				ProviderConfigReference: &xpv1.ProviderConfigReference{
					Name: "test-provider-config",
				},
			},
			ForProvider: VolumeParameters{
				Name:   "test-disk",
				Pool:   "default",
				Size:   int64Ptr(10737418240), // 10 GiB in bytes
				Format: "qcow2",
			},
		},
	}

	// Test that the volume has correct GVK
	gvk := volume.GetObjectKind().GroupVersionKind()
	if gvk.Group != "libvirt.m.crossplane.io" {
		t.Errorf("Expected group 'libvirt.m.crossplane.io', got '%s'", gvk.Group)
	}
	if gvk.Version != "v1beta1" {
		t.Errorf("Expected version 'v1beta1', got '%s'", gvk.Version)
	}
	if gvk.Kind != "Volume" {
		t.Errorf("Expected kind 'Volume', got '%s'", gvk.Kind)
	}

	// Test required fields are set
	if volume.Spec.ForProvider.Name != "test-disk" {
		t.Errorf("Expected name 'test-disk', got '%s'", volume.Spec.ForProvider.Name)
	}
	if volume.Spec.ForProvider.Pool != "default" {
		t.Errorf("Expected pool 'default', got '%s'", volume.Spec.ForProvider.Pool)
	}
	if volume.Spec.ForProvider.Size == nil || *volume.Spec.ForProvider.Size != 10737418240 {
		t.Errorf("Expected size 10737418240, got %v", volume.Spec.ForProvider.Size)
	}
	if volume.Spec.ForProvider.Format != "qcow2" {
		t.Errorf("Expected format 'qcow2', got '%s'", volume.Spec.ForProvider.Format)
	}
}

func TestVolumeConversion(t *testing.T) {
	// Test that Volume can be converted to/from runtime.Object
	volume := &Volume{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-volume",
			Namespace: "test-ns",
		},
		Spec: VolumeSpec{
			ManagedResourceSpec: xpv1.ManagedResourceSpec{
				ProviderConfigReference: &xpv1.ProviderConfigReference{
					Name: "test-provider-config",
				},
			},
			ForProvider: VolumeParameters{
				Name:   "test-disk",
				Pool:   "default",
				Size:   int64Ptr(10737418240), // 10 GiB in bytes
				Format: "qcow2",
			},
		},
	}

	// Convert to runtime.Object and back
	var obj runtime.Object = volume
	converted, ok := obj.(*Volume)
	if !ok {
		t.Error("Failed to convert Volume to runtime.Object and back")
	}

	// Compare original and converted
	if diff := cmp.Diff(volume, converted); diff != "" {
		t.Errorf("Volume conversion mismatch (-want +got):n%s", diff)
	}
}

func TestVolumeWithBackingStore(t *testing.T) {
	volume := &Volume{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-volume",
			Namespace: "test-ns",
		},
		Spec: VolumeSpec{
			ForProvider: VolumeParameters{
				Name:           "test-disk",
				Pool:           "default",
				Size:           int64Ptr(10737418240), // 10 GiB in bytes
				Format:         "qcow2",
				BaseVolumePool: "default",
				BaseVolumeName: "base-disk",
			},
		},
	}

	// Verify backing store configuration
	if volume.Spec.ForProvider.BaseVolumePool != "default" {
		t.Errorf("Expected base volume pool 'default', got '%s'", volume.Spec.ForProvider.BaseVolumePool)
	}
	if volume.Spec.ForProvider.BaseVolumeName != "base-disk" {
		t.Errorf("Expected base volume name 'base-disk', got '%s'", volume.Spec.ForProvider.BaseVolumeName)
	}

	// Verify namespace-scoped resource
	if volume.Namespace != "test-ns" {
		t.Errorf("Expected namespace 'test-ns', got '%s'", volume.Namespace)
	}
}
