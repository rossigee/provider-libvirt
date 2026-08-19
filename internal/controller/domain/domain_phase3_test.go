package domain

import (
	"context"

	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"

	"testing"

	"github.com/rossigee/provider-libvirt/apis/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// Phase 3 API Tests - Reference Resolution (VolumeRef/NetworkRef)

func TestResolveVolumeRef(t *testing.T) {
	// Create a test Volume resource
	vol := &v1beta1.Volume{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-volume",
			Namespace: "default",
		},
		Spec: v1beta1.VolumeSpec{
			ForProvider: v1beta1.VolumeParameters{
				Name: "test-disk",
				Pool: "default",
			},
		},
		Status: v1beta1.VolumeStatus{
			AtProvider: v1beta1.VolumeObservation{
				Path: "/var/lib/libvirt/images/test-disk.qcow2",
			},
		},
	}

	// Create fake client with the volume
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)
	kubeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(vol).Build()

	ext := &external{
		client: nil,
		kube:   kubeClient,
	}

	// Test successful resolution
	ref := &xpv1.Reference{Name: "test-volume"}
	path, err := ext.resolveVolumeRef(context.Background(), ref, "default")
	if err != nil {
		t.Errorf("resolveVolumeRef failed: %v", err)
	}
	if path != "/var/lib/libvirt/images/test-disk.qcow2" {
		t.Errorf("Expected path '/var/lib/libvirt/images/test-disk.qcow2', got '%s'", path)
	}
}

func TestResolveVolumeRefNotFound(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)
	kubeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	ext := &external{
		client: nil,
		kube:   kubeClient,
	}

	// Test with non-existent volume
	ref := &xpv1.Reference{Name: "non-existent"}
	_, err := ext.resolveVolumeRef(context.Background(), ref, "default")
	if err == nil {
		t.Error("resolveVolumeRef should fail for non-existent volume")
	}
}

func TestResolveVolumeRefNilReference(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)
	kubeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	ext := &external{
		client: nil,
		kube:   kubeClient,
	}

	// Test with nil reference
	path, err := ext.resolveVolumeRef(context.Background(), nil, "default")
	if err != nil {
		t.Errorf("resolveVolumeRef with nil reference should not error: %v", err)
	}
	if path != "" {
		t.Errorf("Expected empty path for nil reference, got '%s'", path)
	}
}

func TestResolveNetworkRef(t *testing.T) {
	// Create a test Network resource
	net := &v1beta1.Network{
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

	// Create fake client with the network
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)
	kubeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(net).Build()

	ext := &external{
		client: nil,
		kube:   kubeClient,
	}

	// Test successful resolution
	ref := &xpv1.Reference{Name: "test-network"}
	name, err := ext.resolveNetworkRef(context.Background(), ref, "default")
	if err != nil {
		t.Errorf("resolveNetworkRef failed: %v", err)
	}
	if name != "test-net" {
		t.Errorf("Expected name 'test-net', got '%s'", name)
	}
}

func TestResolveNetworkRefNotFound(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)
	kubeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	ext := &external{
		client: nil,
		kube:   kubeClient,
	}

	// Test with non-existent network
	ref := &xpv1.Reference{Name: "non-existent"}
	_, err := ext.resolveNetworkRef(context.Background(), ref, "default")
	if err == nil {
		t.Error("resolveNetworkRef should fail for non-existent network")
	}
}

func TestResolveDiskSourceWithVolumeRef(t *testing.T) {
	// Create a test Volume resource
	vol := &v1beta1.Volume{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-volume",
			Namespace: "default",
		},
		Spec: v1beta1.VolumeSpec{
			ForProvider: v1beta1.VolumeParameters{
				Name: "test-disk",
				Pool: "default",
			},
		},
		Status: v1beta1.VolumeStatus{
			AtProvider: v1beta1.VolumeObservation{
				Path: "/var/lib/libvirt/images/test-disk.qcow2",
			},
		},
	}

	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)
	kubeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(vol).Build()

	ext := &external{
		client: nil,
		kube:   kubeClient,
	}

	disk := v1beta1.DomainDisk{
		VolumeRef: &xpv1.Reference{Name: "test-volume"},
	}

	source, err := ext.resolveDiskSource(context.Background(), disk, "default")
	if err != nil {
		t.Errorf("resolveDiskSource failed: %v", err)
	}
	if source != "/var/lib/libvirt/images/test-disk.qcow2" {
		t.Errorf("Expected source '/var/lib/libvirt/images/test-disk.qcow2', got '%s'", source)
	}
}

func TestResolveDiskSourceWithFileOnly(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)
	kubeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	ext := &external{
		client: nil,
		kube:   kubeClient,
	}

	disk := v1beta1.DomainDisk{
		File: "/var/lib/libvirt/images/direct-disk.qcow2",
	}

	source, err := ext.resolveDiskSource(context.Background(), disk, "default")
	if err != nil {
		t.Errorf("resolveDiskSource with File should not fail: %v", err)
	}
	if source != "/var/lib/libvirt/images/direct-disk.qcow2" {
		t.Errorf("Expected source '/var/lib/libvirt/images/direct-disk.qcow2', got '%s'", source)
	}
}

func TestResolveDiskSourceVolumeRefPreferred(t *testing.T) {
	// Create a test Volume resource
	vol := &v1beta1.Volume{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-volume",
			Namespace: "default",
		},
		Spec: v1beta1.VolumeSpec{
			ForProvider: v1beta1.VolumeParameters{
				Name: "test-disk",
				Pool: "default",
			},
		},
		Status: v1beta1.VolumeStatus{
			AtProvider: v1beta1.VolumeObservation{
				Path: "/var/lib/libvirt/images/test-disk.qcow2",
			},
		},
	}

	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)
	kubeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(vol).Build()

	ext := &external{
		client: nil,
		kube:   kubeClient,
	}

	// Both VolumeRef and File specified - VolumeRef should take precedence
	disk := v1beta1.DomainDisk{
		VolumeRef: &xpv1.Reference{Name: "test-volume"},
		File:      "/var/lib/libvirt/images/fallback.qcow2",
	}

	source, err := ext.resolveDiskSource(context.Background(), disk, "default")
	if err != nil {
		t.Errorf("resolveDiskSource failed: %v", err)
	}
	// Should use VolumeRef path, not File path
	if source != "/var/lib/libvirt/images/test-disk.qcow2" {
		t.Errorf("Expected VolumeRef path, got '%s'", source)
	}
}

func TestResolveNetworkSourceWithNetworkRef(t *testing.T) {
	// Create a test Network resource
	net := &v1beta1.Network{
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

	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)
	kubeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(net).Build()

	ext := &external{
		client: nil,
		kube:   kubeClient,
	}

	ni := v1beta1.DomainNetworkInterface{
		NetworkRef: &xpv1.Reference{Name: "test-network"},
	}

	name, err := ext.resolveNetworkSource(context.Background(), ni, "default")
	if err != nil {
		t.Errorf("resolveNetworkSource failed: %v", err)
	}
	if name != "test-net" {
		t.Errorf("Expected name 'test-net', got '%s'", name)
	}
}

func TestResolveNetworkSourceWithNameOnly(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)
	kubeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	ext := &external{
		client: nil,
		kube:   kubeClient,
	}

	ni := v1beta1.DomainNetworkInterface{
		NetworkName: "default",
	}

	name, err := ext.resolveNetworkSource(context.Background(), ni, "default")
	if err != nil {
		t.Errorf("resolveNetworkSource with NetworkName should not fail: %v", err)
	}
	if name != "default" {
		t.Errorf("Expected name 'default', got '%s'", name)
	}
}

func TestResolveNetworkSourceNetworkRefPreferred(t *testing.T) {
	// Create a test Network resource
	net := &v1beta1.Network{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-network",
			Namespace: "default",
		},
		Spec: v1beta1.NetworkSpec{
			ForProvider: v1beta1.NetworkParameters{
				Name: "resolved-net",
				Mode: "nat",
			},
		},
	}

	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)
	kubeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(net).Build()

	ext := &external{
		client: nil,
		kube:   kubeClient,
	}

	// Both NetworkRef and NetworkName specified - NetworkRef should take precedence
	ni := v1beta1.DomainNetworkInterface{
		NetworkRef:  &xpv1.Reference{Name: "test-network"},
		NetworkName: "fallback-network",
	}

	name, err := ext.resolveNetworkSource(context.Background(), ni, "default")
	if err != nil {
		t.Errorf("resolveNetworkSource failed: %v", err)
	}
	// Should use NetworkRef name, not NetworkName
	if name != "resolved-net" {
		t.Errorf("Expected NetworkRef name 'resolved-net', got '%s'", name)
	}
}

func TestResolveReferenceWithNilKubeClient(t *testing.T) {
	ext := &external{
		client: nil,
		kube:   nil, // No kube client
	}

	// Should handle gracefully when kube client is nil
	ref := &xpv1.Reference{Name: "test-volume"}
	path, err := ext.resolveVolumeRef(context.Background(), ref, "default")
	if err != nil {
		t.Errorf("resolveVolumeRef with nil kube client should not error: %v", err)
	}
	if path != "" {
		t.Errorf("Expected empty path with nil kube client, got '%s'", path)
	}
}
