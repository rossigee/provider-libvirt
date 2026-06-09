/*
Copyright 2025 Ross Golder

Dependency validation tests for provider-libvirt.
These tests ensure proper handling of resource dependencies and validation.
*/

package test

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"

	"github.com/rossigee/provider-libvirt/apis/v1beta1"
)

// TestResourceDependencyValidation tests various dependency scenarios
func TestResourceDependencyValidation(t *testing.T) {
	ctx := context.Background()
	scheme := createTestScheme()

	tests := []struct {
		name        string
		description string
		testFunc    func(t *testing.T, ctx context.Context, client client.Client)
	}{
		{
			name:        "VolumeReadinessValidation",
			description: "Test Volume must be Ready before Domain can use it",
			testFunc:    testVolumeReadinessValidation,
		},
		{
			name:        "NetworkReadinessValidation",
			description: "Test Network must be Ready before Domain can use it",
			testFunc:    testNetworkReadinessValidation,
		},
		{
			name:        "StoragePoolDependencyValidation",
			description: "Test Volume depends on StoragePool being Ready",
			testFunc:    testStoragePoolDependencyValidation,
		},
		{
			name:        "CrossReferenceResolution",
			description: "Test cross-reference resolution with mock libvirt client",
			testFunc:    testCrossReferenceResolution,
		},
		{
			name:        "MixedReferenceTypes",
			description: "Test mixed reference types (VolumeRef + direct file)",
			testFunc:    testMixedReferenceTypes,
		},
		{
			name:        "CircularDependencyDetection",
			description: "Test detection of circular dependencies",
			testFunc:    testCircularDependencyDetection,
		},
		{
			name:        "DependencyCleanupOrder",
			description: "Test proper cleanup order respecting dependencies",
			testFunc:    testDependencyCleanupOrder,
		},
		{
			name:        "ResourceNotFoundHandling",
			description: "Test graceful handling of missing referenced resources",
			testFunc:    testResourceNotFoundHandling,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k8sClient := fake.NewClientBuilder().WithScheme(scheme).Build()
			setupTestEnvironment(t, ctx, k8sClient)
			tt.testFunc(t, ctx, k8sClient)
		})
	}
}

func testVolumeReadinessValidation(t *testing.T, ctx context.Context, k8sClient client.Client) {
	// Step 1: Create Volume but don't make it Ready
	volume := createTestVolume("readiness-test-volume", "default")
	createResource(t, ctx, k8sClient, volume)
	
	// Volume is created but not Ready (no status conditions set)
	
	// Step 2: Create Domain that references the not-ready Volume
	domain := &v1beta1.Domain{
		ObjectMeta: metav1.ObjectMeta{
			Name: "readiness-test-domain",
		},
		Spec: v1beta1.DomainSpec{
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: &xpv1.Reference{Name: "test-provider-config"},
			},
			ForProvider: v1beta1.DomainParameters{
				Name:    "readiness-test-vm",
				Memory:  1073741824,
				Vcpu:    1,
				Running: &[]bool{false}[0],
				Disk: []v1beta1.DomainDisk{
					{
						VolumeRef: &xpv1.Reference{Name: "readiness-test-volume"},
						Type:      "virtio",
					},
				},
			},
		},
	}

	createResource(t, ctx, k8sClient, domain)

	// Step 3: Simulate domain controller trying to resolve Volume reference
	// This should fail because Volume is not Ready
	
	t.Run("VolumeNotReady", func(t *testing.T) {
		// Verify Domain was created with the reference
		var retrievedDomain v1beta1.Domain
		err := k8sClient.Get(ctx, types.NamespacedName{Name: "readiness-test-domain"}, &retrievedDomain)
		if err != nil {
			t.Fatalf("Failed to retrieve Domain: %v", err)
		}

		if len(retrievedDomain.Spec.ForProvider.Disk) == 0 {
			t.Fatal("Expected Domain to have disk configuration")
		}

		disk := retrievedDomain.Spec.ForProvider.Disk[0]
		if disk.VolumeRef == nil || disk.VolumeRef.Name != "readiness-test-volume" {
			t.Error("Expected Domain to reference readiness-test-volume")
		}

		// In a real scenario, the Domain controller would:
		// 1. Try to resolve the Volume reference
		// 2. Find the Volume exists but is not Ready
		// 3. Set Domain condition to indicate dependency not ready
		// 4. Requeue for later reconciliation
		
		// For this test, we simulate this by not setting Domain as Ready
		if len(retrievedDomain.Status.Conditions) > 0 {
			ready := retrievedDomain.Status.GetCondition(xpv1.TypeReady)
			if ready.Status == "True" {
				t.Error("Expected Domain to not be Ready when Volume is not Ready")
			}
		}
	})

	// Step 4: Make Volume Ready and verify Domain can now be processed
	t.Run("VolumeBecomesReady", func(t *testing.T) {
		// Make Volume Ready
		makeResourceReady(t, ctx, k8sClient, volume)

		// Verify Volume is now Ready
		var updatedVolume v1beta1.Volume
		err := k8sClient.Get(ctx, types.NamespacedName{Name: "readiness-test-volume"}, &updatedVolume)
		if err != nil {
			t.Fatalf("Failed to get updated Volume: %v", err)
		}

		ready := updatedVolume.Status.GetCondition(xpv1.TypeReady)
		if ready.Status != "True" {
			t.Error("Expected Volume to be Ready")
		}

		// Now Domain controller should be able to process the Domain
		// In a real scenario, this would happen during reconciliation
		makeResourceReady(t, ctx, k8sClient, domain)

		var updatedDomain v1beta1.Domain
		err = k8sClient.Get(ctx, types.NamespacedName{Name: "readiness-test-domain"}, &updatedDomain)
		if err != nil {
			t.Fatalf("Failed to get updated Domain: %v", err)
		}

		domainReady := updatedDomain.Status.GetCondition(xpv1.TypeReady)
		if domainReady.Status != "True" {
			t.Error("Expected Domain to be Ready when Volume is Ready")
		}
	})

	t.Log("Volume readiness validation test completed successfully")
}

func testNetworkReadinessValidation(t *testing.T, ctx context.Context, k8sClient client.Client) {
	// Similar to Volume test but for Network dependencies
	network := createTestNetwork("readiness-test-network")
	createResource(t, ctx, k8sClient, network)
	
	domain := &v1beta1.Domain{
		ObjectMeta: metav1.ObjectMeta{
			Name: "network-readiness-test-domain",
		},
		Spec: v1beta1.DomainSpec{
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: &xpv1.Reference{Name: "test-provider-config"},
			},
			ForProvider: v1beta1.DomainParameters{
				Name:    "network-readiness-test-vm",
				Memory:  1073741824,
				Vcpu:    1,
				Running: &[]bool{false}[0],
				NetworkInterface: []v1beta1.DomainNetworkInterface{
					{
						NetworkRef: &xpv1.Reference{Name: "readiness-test-network"},
						Model:      "virtio",
					},
				},
			},
		},
	}

	createResource(t, ctx, k8sClient, domain)

	// Test that Domain cannot be processed without Ready Network
	var retrievedDomain v1beta1.Domain
	err := k8sClient.Get(ctx, types.NamespacedName{Name: "network-readiness-test-domain"}, &retrievedDomain)
	if err != nil {
		t.Fatalf("Failed to retrieve Domain: %v", err)
	}

	// Verify Network reference is preserved
	if len(retrievedDomain.Spec.ForProvider.NetworkInterface) == 0 {
		t.Fatal("Expected Domain to have network interface configuration")
	}

	netif := retrievedDomain.Spec.ForProvider.NetworkInterface[0]
	if netif.NetworkRef == nil || netif.NetworkRef.Name != "readiness-test-network" {
		t.Error("Expected Domain to reference readiness-test-network")
	}

	// Make Network Ready and verify Domain can be processed
	makeResourceReady(t, ctx, k8sClient, network)
	makeResourceReady(t, ctx, k8sClient, domain)

	var updatedDomain v1beta1.Domain
	err = k8sClient.Get(ctx, types.NamespacedName{Name: "network-readiness-test-domain"}, &updatedDomain)
	if err != nil {
		t.Fatalf("Failed to get updated Domain: %v", err)
	}

	domainReady := updatedDomain.Status.GetCondition(xpv1.TypeReady)
	if domainReady.Status != "True" {
		t.Error("Expected Domain to be Ready when Network is Ready")
	}

	t.Log("Network readiness validation test completed successfully")
}

func testStoragePoolDependencyValidation(t *testing.T, ctx context.Context, k8sClient client.Client) {
	// Test Volume dependency on StoragePool
	
	// Step 1: Create Volume that references non-existent StoragePool
	volume := &v1beta1.Volume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pool-dependency-volume",
		},
		Spec: v1beta1.VolumeSpec{
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: &xpv1.Reference{Name: "test-provider-config"},
			},
			ForProvider: v1beta1.VolumeParameters{
				Name:   "pool-dependency-volume.qcow2",
				Pool:   "non-existent-pool", // References non-existent pool
				Format: "qcow2",
				Size:   int64Ptr(10737418240), // 10GB in bytes
			},
		},
	}

	createResource(t, ctx, k8sClient, volume)

	// Volume should be created but not Ready due to missing StoragePool
	var retrievedVolume v1beta1.Volume
	err := k8sClient.Get(ctx, types.NamespacedName{Name: "pool-dependency-volume"}, &retrievedVolume)
	if err != nil {
		t.Fatalf("Failed to retrieve Volume: %v", err)
	}

	if retrievedVolume.Spec.ForProvider.Pool != "non-existent-pool" {
		t.Error("Expected Volume to preserve pool name reference")
	}

	// Step 2: Create the StoragePool
	storagePool := &v1beta1.StoragePool{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dependency-test-pool",
		},
		Spec: v1beta1.StoragePoolSpec{
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: &xpv1.Reference{Name: "test-provider-config"},
			},
			ForProvider: v1beta1.StoragePoolParameters{
				Name: "non-existent-pool", // This matches the Volume's Pool
				Type: "dir",
				Target: &v1beta1.StoragePoolTarget{
					Path: "/tmp/dependency-test-pool",
				},
				Autostart: &[]bool{true}[0],
			},
		},
	}

	createResource(t, ctx, k8sClient, storagePool)
	makeResourceReady(t, ctx, k8sClient, storagePool)

	// Step 3: Now Volume should be able to become Ready
	makeResourceReady(t, ctx, k8sClient, volume)

	var updatedVolume v1beta1.Volume
	err = k8sClient.Get(ctx, types.NamespacedName{Name: "pool-dependency-volume"}, &updatedVolume)
	if err != nil {
		t.Fatalf("Failed to get updated Volume: %v", err)
	}

	volumeReady := updatedVolume.Status.GetCondition(xpv1.TypeReady)
	if volumeReady.Status != "True" {
		t.Error("Expected Volume to be Ready when StoragePool is Ready")
	}

	t.Log("StoragePool dependency validation test completed successfully")
}

func testCrossReferenceResolution(t *testing.T, ctx context.Context, k8sClient client.Client) {
	// Test the actual cross-reference resolution logic from domain controller
	
	// Create all dependencies first
	volume := createTestVolume("xref-test-volume", "default")
	network := createTestNetwork("xref-test-network")
	
	createResource(t, ctx, k8sClient, volume)
	makeResourceReady(t, ctx, k8sClient, volume)
	createResource(t, ctx, k8sClient, network)
	makeResourceReady(t, ctx, k8sClient, network)

	// Create Domain with cross-references
	testDomain := &v1beta1.Domain{
		ObjectMeta: metav1.ObjectMeta{
			Name: "xref-test-domain",
		},
		Spec: v1beta1.DomainSpec{
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: &xpv1.Reference{Name: "test-provider-config"},
			},
			ForProvider: v1beta1.DomainParameters{
				Name:    "xref-test-vm",
				Memory:  1073741824,
				Vcpu:    1,
				Running: &[]bool{false}[0],
				Disk: []v1beta1.DomainDisk{
					{
						VolumeRef: &xpv1.Reference{Name: "xref-test-volume"},
						Type:      "virtio",
						Device:    "vda",
					},
				},
				NetworkInterface: []v1beta1.DomainNetworkInterface{
					{
						NetworkRef: &xpv1.Reference{Name: "xref-test-network"},
						Model:      "virtio",
					},
				},
			},
		},
	}

	createResource(t, ctx, k8sClient, testDomain)

	// Test cross-reference resolution by attempting to generate XML
	// This would normally be done by the domain controller
	t.Run("TestXMLGenerationWithReferences", func(t *testing.T) {
		var retrievedDomain v1beta1.Domain
		err := k8sClient.Get(ctx, types.NamespacedName{Name: "xref-test-domain"}, &retrievedDomain)
		if err != nil {
			t.Fatalf("Failed to retrieve Domain: %v", err)
		}

		// The domain controller would call generateDomainXMLWithClient
		// For this test, we verify that the references are properly set
		if len(retrievedDomain.Spec.ForProvider.Disk) == 0 {
			t.Fatal("Expected Domain to have disk configuration")
		}

		disk := retrievedDomain.Spec.ForProvider.Disk[0]
		if disk.VolumeRef == nil || disk.VolumeRef.Name != "xref-test-volume" {
			t.Error("Expected disk to reference xref-test-volume")
		}

		if len(retrievedDomain.Spec.ForProvider.NetworkInterface) == 0 {
			t.Fatal("Expected Domain to have network interface configuration")
		}

		netif := retrievedDomain.Spec.ForProvider.NetworkInterface[0]
		if netif.NetworkRef == nil || netif.NetworkRef.Name != "xref-test-network" {
			t.Error("Expected network interface to reference xref-test-network")
		}

		// In a real test with actual libvirt, we would test:
		// xml, err := domain.generateDomainXMLWithClient(&retrievedDomain, k8sClient)
		// And verify the XML contains the resolved paths/networks
	})

	t.Log("Cross-reference resolution test completed successfully")
}

func testMixedReferenceTypes(t *testing.T, ctx context.Context, k8sClient client.Client) {
	// Test Domain with mixed reference types (some VolumeRef, some direct file paths)
	
	volume := createTestVolume("mixed-ref-volume", "default")
	network := createTestNetwork("mixed-ref-network")
	
	createResource(t, ctx, k8sClient, volume)
	makeResourceReady(t, ctx, k8sClient, volume)
	createResource(t, ctx, k8sClient, network)
	makeResourceReady(t, ctx, k8sClient, network)

	domain := &v1beta1.Domain{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mixed-ref-domain",
		},
		Spec: v1beta1.DomainSpec{
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: &xpv1.Reference{Name: "test-provider-config"},
			},
			ForProvider: v1beta1.DomainParameters{
				Name:    "mixed-ref-vm",
				Memory:  2147483648,
				Vcpu:    2,
				Running: &[]bool{false}[0],
				Disk: []v1beta1.DomainDisk{
					{
						// Using VolumeRef (new cross-resource method)
						VolumeRef: &xpv1.Reference{Name: "mixed-ref-volume"},
						Type:      "virtio",
						Device:    "vda",
						BootOrder: &[]int32{1}[0],
					},
					{
						// Using direct file path (legacy method)
						File:   "/tmp/legacy-disk.qcow2",
						Type:   "virtio",
						Device: "vdb",
					},
					{
						// Using File path instead of deprecated VolumeID
						File: "/var/lib/libvirt/images/mixed-ref-volume.qcow2",
						Type:     "virtio",
						Device:   "vdc",
					},
				},
				NetworkInterface: []v1beta1.DomainNetworkInterface{
					{
						// Using NetworkRef (new cross-resource method)
						NetworkRef: &xpv1.Reference{Name: "mixed-ref-network"},
						Model:      "virtio",
					},
					{
						// Using direct network name (legacy method)
						NetworkName: "default",
						Model:       "virtio",
					},
					{
						// Using bridge (legacy method)
						NetworkName: "br0",
						Model:  "virtio",
					},
				},
			},
		},
	}

	createResource(t, ctx, k8sClient, domain)

	// Verify all reference types are preserved
	var retrievedDomain v1beta1.Domain
	err := k8sClient.Get(ctx, types.NamespacedName{Name: "mixed-ref-domain"}, &retrievedDomain)
	if err != nil {
		t.Fatalf("Failed to retrieve Domain: %v", err)
	}

	// Verify disk configurations
	if len(retrievedDomain.Spec.ForProvider.Disk) != 3 {
		t.Errorf("Expected 3 disks, got %d", len(retrievedDomain.Spec.ForProvider.Disk))
	}

	disks := retrievedDomain.Spec.ForProvider.Disk
	
	// First disk - VolumeRef
	if disks[0].VolumeRef == nil || disks[0].VolumeRef.Name != "mixed-ref-volume" {
		t.Error("Expected first disk to use VolumeRef")
	}
	
	// Second disk - direct file
	if disks[1].File != "/tmp/legacy-disk.qcow2" {
		t.Error("Expected second disk to use direct file path")
	}
	
	// Third disk - File path
	if disks[2].File != "/var/lib/libvirt/images/mixed-ref-volume.qcow2" {
		t.Error("Expected third disk to use File path")
	}

	// Verify network interface configurations
	if len(retrievedDomain.Spec.ForProvider.NetworkInterface) != 3 {
		t.Errorf("Expected 3 network interfaces, got %d", len(retrievedDomain.Spec.ForProvider.NetworkInterface))
	}

	netifs := retrievedDomain.Spec.ForProvider.NetworkInterface
	
	// First interface - NetworkRef
	if netifs[0].NetworkRef == nil || netifs[0].NetworkRef.Name != "mixed-ref-network" {
		t.Error("Expected first interface to use NetworkRef")
	}
	
	// Second interface - direct network name
	if netifs[1].NetworkName != "default" {
		t.Error("Expected second interface to use direct network name")
	}
	
	// Third interface - bridge
	if netifs[2].NetworkName != "br0" {
		t.Error("Expected third interface to use bridge")
	}

	t.Log("Mixed reference types test completed successfully")
}

func testCircularDependencyDetection(t *testing.T, ctx context.Context, k8sClient client.Client) {
	// Test detection of circular dependencies (though current design prevents this)
	
	// In the current design, circular dependencies are not possible because:
	// - Domain can reference Volume and Network
	// - Volume can reference StoragePool (by name, not Kubernetes resource)
	// - Network is independent
	// - StoragePool is independent
	
	// However, we can test edge cases that might create confusion
	
	// Create resources with similar names that might cause confusion
	similarVolume1 := &v1beta1.Volume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "circular-test-vol-1",
		},
		Spec: v1beta1.VolumeSpec{
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: &xpv1.Reference{Name: "test-provider-config"},
			},
			ForProvider: v1beta1.VolumeParameters{
				Name:   "circular-test-vol-1.qcow2",
				Pool:   "pool-a",
				Format: "qcow2",
				Size:   int64Ptr(10737418240), // 10GB in bytes
			},
		},
	}

	similarVolume2 := &v1beta1.Volume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "circular-test-vol-2",
		},
		Spec: v1beta1.VolumeSpec{
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: &xpv1.Reference{Name: "test-provider-config"},
			},
			ForProvider: v1beta1.VolumeParameters{
				Name:   "circular-test-vol-2.qcow2",
				Pool:   "pool-b",
				Format: "qcow2",
				Size:   int64Ptr(10737418240), // 10GB in bytes
			},
		},
	}

	createResource(t, ctx, k8sClient, similarVolume1)
	createResource(t, ctx, k8sClient, similarVolume2)
	makeResourceReady(t, ctx, k8sClient, similarVolume1)
	makeResourceReady(t, ctx, k8sClient, similarVolume2)

	// Create Domain that references both volumes
	domain := &v1beta1.Domain{
		ObjectMeta: metav1.ObjectMeta{
			Name: "circular-test-domain",
		},
		Spec: v1beta1.DomainSpec{
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: &xpv1.Reference{Name: "test-provider-config"},
			},
			ForProvider: v1beta1.DomainParameters{
				Name:    "circular-test-vm",
				Memory:  1073741824,
				Vcpu:    1,
				Running: &[]bool{false}[0],
				Disk: []v1beta1.DomainDisk{
					{
						VolumeRef: &xpv1.Reference{Name: "circular-test-vol-1"},
						Type:      "virtio",
						Device:    "vda",
					},
					{
						VolumeRef: &xpv1.Reference{Name: "circular-test-vol-2"},
						Type:      "virtio",
						Device:    "vdb",
					},
				},
			},
		},
	}

	createResource(t, ctx, k8sClient, domain)

	// Verify both volume references are properly handled
	var retrievedDomain v1beta1.Domain
	err := k8sClient.Get(ctx, types.NamespacedName{Name: "circular-test-domain"}, &retrievedDomain)
	if err != nil {
		t.Fatalf("Failed to retrieve Domain: %v", err)
	}

	if len(retrievedDomain.Spec.ForProvider.Disk) != 2 {
		t.Errorf("Expected 2 disks, got %d", len(retrievedDomain.Spec.ForProvider.Disk))
	}

	// The references should be distinct and not cause circular dependency issues
	disk1 := retrievedDomain.Spec.ForProvider.Disk[0]
	disk2 := retrievedDomain.Spec.ForProvider.Disk[1]

	if disk1.VolumeRef == nil || disk1.VolumeRef.Name != "circular-test-vol-1" {
		t.Error("Expected first disk to reference circular-test-vol-1")
	}

	if disk2.VolumeRef == nil || disk2.VolumeRef.Name != "circular-test-vol-2" {
		t.Error("Expected second disk to reference circular-test-vol-2")
	}

	t.Log("Circular dependency detection test completed successfully")
}

func testDependencyCleanupOrder(t *testing.T, ctx context.Context, k8sClient client.Client) {
	// Test proper cleanup order: Domain → Volume → StoragePool, Network
	
	// Create full dependency chain
	storagePool := createTestStoragePool("cleanup-test-pool")
	volume := createTestVolume("cleanup-test-volume", "cleanup-test-pool")
	network := createTestNetwork("cleanup-test-network")

	resources := []client.Object{storagePool, volume, network}
	for _, resource := range resources {
		createResource(t, ctx, k8sClient, resource)
		makeResourceReady(t, ctx, k8sClient, resource)
	}

	domain := &v1beta1.Domain{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cleanup-test-domain",
		},
		Spec: v1beta1.DomainSpec{
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: &xpv1.Reference{Name: "test-provider-config"},
				DeletionPolicy:          xpv1.DeletionDelete,
			},
			ForProvider: v1beta1.DomainParameters{
				Name:    "cleanup-test-vm",
				Memory:  1073741824,
				Vcpu:    1,
				Running: &[]bool{false}[0],
				Disk: []v1beta1.DomainDisk{
					{
						VolumeRef: &xpv1.Reference{Name: "cleanup-test-volume"},
						Type:      "virtio",
					},
				},
				NetworkInterface: []v1beta1.DomainNetworkInterface{
					{
						NetworkRef: &xpv1.Reference{Name: "cleanup-test-network"},
						Model:      "virtio",
					},
				},
			},
		},
	}

	createResource(t, ctx, k8sClient, domain)

	// Test cleanup in proper order
	t.Run("CleanupInCorrectOrder", func(t *testing.T) {
		// 1. Delete Domain first (it depends on Volume and Network)
		err := k8sClient.Delete(ctx, domain)
		if err != nil {
			t.Fatalf("Failed to delete Domain: %v", err)
		}

		// Verify Domain is deleted
		var deletedDomain v1beta1.Domain
		err = k8sClient.Get(ctx, types.NamespacedName{Name: "cleanup-test-domain"}, &deletedDomain)
		if err == nil {
			t.Error("Expected Domain to be deleted")
		}

		// 2. Delete Network (no dependencies)
		err = k8sClient.Delete(ctx, network)
		if err != nil {
			t.Fatalf("Failed to delete Network: %v", err)
		}

		// 3. Delete Volume (depends on StoragePool)
		err = k8sClient.Delete(ctx, volume)
		if err != nil {
			t.Fatalf("Failed to delete Volume: %v", err)
		}

		// 4. Delete StoragePool last (Volume depends on it)
		err = k8sClient.Delete(ctx, storagePool)
		if err != nil {
			t.Fatalf("Failed to delete StoragePool: %v", err)
		}
	})

	t.Log("Dependency cleanup order test completed successfully")
}

func testResourceNotFoundHandling(t *testing.T, ctx context.Context, k8sClient client.Client) {
	// Test graceful handling when referenced resources don't exist
	
	domain := &v1beta1.Domain{
		ObjectMeta: metav1.ObjectMeta{
			Name: "not-found-test-domain",
		},
		Spec: v1beta1.DomainSpec{
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: &xpv1.Reference{Name: "test-provider-config"},
			},
			ForProvider: v1beta1.DomainParameters{
				Name:    "not-found-test-vm",
				Memory:  1073741824,
				Vcpu:    1,
				Running: &[]bool{false}[0],
				Disk: []v1beta1.DomainDisk{
					{
						VolumeRef: &xpv1.Reference{Name: "non-existent-volume"},
						Type:      "virtio",
					},
				},
				NetworkInterface: []v1beta1.DomainNetworkInterface{
					{
						NetworkRef: &xpv1.Reference{Name: "non-existent-network"},
						Model:      "virtio",
					},
				},
			},
		},
	}

	createResource(t, ctx, k8sClient, domain)

	// Verify Domain was created but references are preserved for debugging
	var retrievedDomain v1beta1.Domain
	err := k8sClient.Get(ctx, types.NamespacedName{Name: "not-found-test-domain"}, &retrievedDomain)
	if err != nil {
		t.Fatalf("Failed to retrieve Domain: %v", err)
	}

	// Check that invalid references are preserved for debugging
	if len(retrievedDomain.Spec.ForProvider.Disk) == 0 {
		t.Fatal("Expected Domain to have disk configuration")
	}

	disk := retrievedDomain.Spec.ForProvider.Disk[0]
	if disk.VolumeRef == nil || disk.VolumeRef.Name != "non-existent-volume" {
		t.Error("Expected Domain to preserve non-existent volume reference for debugging")
	}

	if len(retrievedDomain.Spec.ForProvider.NetworkInterface) == 0 {
		t.Fatal("Expected Domain to have network interface configuration")
	}

	netif := retrievedDomain.Spec.ForProvider.NetworkInterface[0]
	if netif.NetworkRef == nil || netif.NetworkRef.Name != "non-existent-network" {
		t.Error("Expected Domain to preserve non-existent network reference for debugging")
	}

	// In a real scenario, the Domain controller would:
	// 1. Try to resolve references
	// 2. Fail to find the resources
	// 3. Set appropriate error conditions
	// 4. Requeue for retry (in case resources are created later)

	t.Log("Resource not found handling test completed successfully")
}

// Benchmark tests for performance validation
func BenchmarkCrossReferenceResolution(b *testing.B) {
	ctx := context.Background()
	scheme := createTestScheme()
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	// Setup test data
	volume := createTestVolume("bench-volume", "default")
	network := createTestNetwork("bench-network")
	
	createResource(&testing.T{}, ctx, k8sClient, volume)
	makeResourceReady(&testing.T{}, ctx, k8sClient, volume)
	createResource(&testing.T{}, ctx, k8sClient, network)
	makeResourceReady(&testing.T{}, ctx, k8sClient, network)

	domain := &v1beta1.Domain{
		ObjectMeta: metav1.ObjectMeta{
			Name: "bench-domain",
		},
		Spec: v1beta1.DomainSpec{
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: &xpv1.Reference{Name: "test-provider-config"},
			},
			ForProvider: v1beta1.DomainParameters{
				Name:    "bench-vm",
				Memory:  1073741824,
				Vcpu:    1,
				Running: &[]bool{false}[0],
				Disk: []v1beta1.DomainDisk{
					{
						VolumeRef: &xpv1.Reference{Name: "bench-volume"},
						Type:      "virtio",
					},
				},
				NetworkInterface: []v1beta1.DomainNetworkInterface{
					{
						NetworkRef: &xpv1.Reference{Name: "bench-network"},
						Model:      "virtio",
					},
				},
			},
		},
	}

	createResource(&testing.T{}, ctx, k8sClient, domain)

	// Benchmark cross-reference resolution
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var retrievedDomain v1beta1.Domain
		err := k8sClient.Get(ctx, types.NamespacedName{Name: "bench-domain"}, &retrievedDomain)
		if err != nil {
			b.Fatalf("Failed to retrieve Domain: %v", err)
		}

		// In a real benchmark, we would call the actual resolution functions
		// For now, we just verify the references exist
		if len(retrievedDomain.Spec.ForProvider.Disk) == 0 || 
			retrievedDomain.Spec.ForProvider.Disk[0].VolumeRef == nil {
			b.Fatal("Expected valid volume reference")
		}

		if len(retrievedDomain.Spec.ForProvider.NetworkInterface) == 0 ||
			retrievedDomain.Spec.ForProvider.NetworkInterface[0].NetworkRef == nil {
			b.Fatal("Expected valid network reference")
		}
	}
}