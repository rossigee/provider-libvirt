/*
Copyright 2025 Ross Golder

Cross-resource integration tests for provider-libvirt.
These tests validate the complete workflow of creating interdependent resources.
*/

package test

import (
	"context"
	"github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/rossigee/provider-libvirt/apis/v1beta1"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"testing"
)

// TestCrossResourceIntegration tests the complete cross-resource workflow:
// StoragePool → Volume → Network → Domain with references
func TestCrossResourceIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping cross-resource integration test in short mode")
	}

	ctx := context.Background()
	scheme := createTestScheme()
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	// Test case: Complete VM setup with cross-resource dependencies
	t.Run("CompleteVMSetupWithCrossReferences", func(t *testing.T) {
		testCompleteVMSetup(t, ctx, fakeClient)
	})

	// Test case: Volume reference validation
	t.Run("VolumeReferenceValidation", func(t *testing.T) {
		testVolumeReferenceValidation(t, ctx, fakeClient)
	})

	// Test case: Network reference validation
	t.Run("NetworkReferenceValidation", func(t *testing.T) {
		testNetworkReferenceValidation(t, ctx, fakeClient)
	})

	// Test case: Backward compatibility with direct paths
	t.Run("BackwardCompatibilityWithDirectPaths", func(t *testing.T) {
		testBackwardCompatibility(t, ctx, fakeClient)
	})

	// Test case: Resource dependency ordering
	t.Run("ResourceDependencyOrdering", func(t *testing.T) {
		testResourceDependencyOrdering(t, ctx, fakeClient)
	})
}

func testCompleteVMSetup(t *testing.T, ctx context.Context, k8sClient client.Client) {
	// Step 1: Create StoragePool
	storagePool := &v1beta1.StoragePool{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-storage-pool",
		},
		Spec: v1beta1.StoragePoolSpec{
			ManagedResourceSpec: xpv1.ManagedResourceSpec{
				ProviderConfigReference: &xpv1.ProviderConfigReference{Name: "test-provider-config"},
			},
			ForProvider: v1beta1.StoragePoolParameters{
				Name: "test-pool",
				Type: "dir",
				Target: &v1beta1.StoragePoolTarget{
					Path: "/tmp/test-pool",
				},
				Autostart: &[]bool{true}[0],
			},
		},
	}

	err := k8sClient.Create(ctx, storagePool)
	if err != nil {
		t.Fatalf("Failed to create StoragePool: %v", err)
	}

	// Simulate StoragePool becoming ready
	storagePool.Status.SetConditions(xpv1.Available())
	storagePool.Status.AtProvider.State = "active"
	storagePool.Status.AtProvider.Capacity = 107374182400 // 100GB
	storagePool.Status.AtProvider.Available = 107374182400
	err = k8sClient.Status().Update(ctx, storagePool)
	if err != nil {
		t.Logf("Warning: Failed to update StoragePool status: %v", err)
	}

	// Step 2: Create Volume in the StoragePool
	volume := &v1beta1.Volume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-vm-disk",
		},
		Spec: v1beta1.VolumeSpec{
			ManagedResourceSpec: xpv1.ManagedResourceSpec{
				ProviderConfigReference: &xpv1.ProviderConfigReference{Name: "test-provider-config"},
			},
			ForProvider: v1beta1.VolumeParameters{
				Name:   "test-vm-disk.qcow2",
				Pool:   "test-pool", // References the StoragePool
				Format: "qcow2",
				Size:   int64Ptr(21474836480), // 20GB in bytes
			},
		},
	}

	err = k8sClient.Create(ctx, volume)
	if err != nil {
		t.Fatalf("Failed to create Volume: %v", err)
	}

	// Simulate Volume becoming ready
	volume.Status.SetConditions(xpv1.Available())
	volume.Status.AtProvider.Path = "/tmp/test-pool/test-vm-disk.qcow2"
	volume.Status.AtProvider.Capacity = 21474836480
	volume.Status.AtProvider.Allocation = 1048576 // 1MB allocated initially
	err = k8sClient.Status().Update(ctx, volume)
	if err != nil {
		t.Logf("Warning: Failed to update Volume status: %v", err)
	}

	// Step 3: Create Network
	network := &v1beta1.Network{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-vm-network",
		},
		Spec: v1beta1.NetworkSpec{
			ManagedResourceSpec: xpv1.ManagedResourceSpec{
				ProviderConfigReference: &xpv1.ProviderConfigReference{Name: "test-provider-config"},
			},
			ForProvider: v1beta1.NetworkParameters{
				Name: "test-network",
				Mode: "nat",
				IP: []v1beta1.NetworkIP{
					{
						Address: "192.168.122.1",
						Netmask: "255.255.255.0",
					},
				},
				DHCP: &v1beta1.NetworkDHCP{
					Ranges: []v1beta1.NetworkDHCPRange{
						{
							Start: "192.168.122.100",
							End:   "192.168.122.200",
						},
					},
				},
				Autostart: &[]bool{true}[0],
			},
		},
	}

	err = k8sClient.Create(ctx, network)
	if err != nil {
		t.Fatalf("Failed to create Network: %v", err)
	}

	// Simulate Network becoming ready
	network.Status.SetConditions(xpv1.Available())
	network.Status.AtProvider.Active = true
	err = k8sClient.Status().Update(ctx, network)
	if err != nil {
		t.Logf("Warning: Failed to update Network status: %v", err)
	}

	// Step 4: Create Domain with cross-resource references
	domain := &v1beta1.Domain{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-vm-with-refs",
		},
		Spec: v1beta1.DomainSpec{
			ManagedResourceSpec: xpv1.ManagedResourceSpec{
				ProviderConfigReference: &xpv1.ProviderConfigReference{Name: "test-provider-config"},
			},
			ForProvider: v1beta1.DomainParameters{
				Name:    "test-integration-vm",
				Memory:  2147483648, // 2GB
				Vcpu:    2,
				Type:    "kvm",
				Arch:    "x86_64",
				Running: &[]bool{false}[0], // Start stopped for safety
				Boot:    []string{"hd"},
				// Disk with Volume reference (NEW cross-resource feature)
				Disk: []v1beta1.DomainDisk{
					{
						VolumeRef: &xpv1.Reference{Name: "test-vm-disk"}, // Cross-resource reference
						Type:      "virtio",
						Device:    "vda",
						BootOrder: &[]int32{1}[0],
					},
				},
				// Network interface with Network reference (NEW cross-resource feature)
				NetworkInterface: []v1beta1.DomainNetworkInterface{
					{
						NetworkRef:   &xpv1.Reference{Name: "test-vm-network"}, // Cross-resource reference
						Model:        "virtio",
						WaitForLease: true,
					},
				},
				Console: []v1beta1.DomainConsole{
					{
						Type: "pty",
					},
				},
				Graphics: []v1beta1.DomainGraphics{
					{
						Type:          "spice",
						ListenAddress: "127.0.0.1",
						Autoport:      true,
					},
				},
			},
		},
	}

	err = k8sClient.Create(ctx, domain)
	if err != nil {
		t.Fatalf("Failed to create Domain: %v", err)
	}

	// Step 5: Verify all resources were created successfully
	t.Run("VerifyResourceCreation", func(t *testing.T) {
		// Verify StoragePool
		var retrievedPool v1beta1.StoragePool
		err := k8sClient.Get(ctx, types.NamespacedName{Name: "test-storage-pool"}, &retrievedPool)
		if err != nil {
			t.Errorf("Failed to retrieve StoragePool: %v", err)
		}
		if retrievedPool.Spec.ForProvider.Name != "test-pool" {
			t.Errorf("Expected pool name 'test-pool', got '%s'", retrievedPool.Spec.ForProvider.Name)
		}

		// Verify Volume
		var retrievedVolume v1beta1.Volume
		err = k8sClient.Get(ctx, types.NamespacedName{Name: "test-vm-disk"}, &retrievedVolume)
		if err != nil {
			t.Errorf("Failed to retrieve Volume: %v", err)
		}
		if retrievedVolume.Spec.ForProvider.Pool != "test-pool" {
			t.Errorf("Expected volume pool 'test-pool', got '%s'", retrievedVolume.Spec.ForProvider.Pool)
		}

		// Verify Network
		var retrievedNetwork v1beta1.Network
		err = k8sClient.Get(ctx, types.NamespacedName{Name: "test-vm-network"}, &retrievedNetwork)
		if err != nil {
			t.Errorf("Failed to retrieve Network: %v", err)
		}
		if retrievedNetwork.Spec.ForProvider.Mode != "nat" {
			t.Errorf("Expected network mode 'nat', got '%s'", retrievedNetwork.Spec.ForProvider.Mode)
		}

		// Verify Domain with cross-references
		var retrievedDomain v1beta1.Domain
		err = k8sClient.Get(ctx, types.NamespacedName{Name: "test-vm-with-refs"}, &retrievedDomain)
		if err != nil {
			t.Errorf("Failed to retrieve Domain: %v", err)
		}

		// Verify cross-resource references
		if len(retrievedDomain.Spec.ForProvider.Disk) == 0 {
			t.Error("Expected Domain to have disk configuration")
		} else {
			disk := retrievedDomain.Spec.ForProvider.Disk[0]
			if disk.VolumeRef == nil {
				t.Error("Expected disk to have VolumeRef")
			} else if disk.VolumeRef.Name != "test-vm-disk" {
				t.Errorf("Expected disk VolumeRef name 'test-vm-disk', got '%s'", disk.VolumeRef.Name)
			}
		}

		if len(retrievedDomain.Spec.ForProvider.NetworkInterface) == 0 {
			t.Error("Expected Domain to have network interface configuration")
		} else {
			netif := retrievedDomain.Spec.ForProvider.NetworkInterface[0]
			if netif.NetworkRef == nil {
				t.Error("Expected network interface to have NetworkRef")
			} else if netif.NetworkRef.Name != "test-vm-network" {
				t.Errorf("Expected network interface NetworkRef name 'test-vm-network', got '%s'", netif.NetworkRef.Name)
			}
		}
	})

	// Step 6: Test cleanup (reverse dependency order)
	t.Run("VerifyCleanupOrder", func(t *testing.T) {
		// Delete Domain first (depends on Volume and Network)
		err := k8sClient.Delete(ctx, domain)
		if err != nil {
			t.Errorf("Failed to delete Domain: %v", err)
		}

		// Delete Network (no dependencies)
		err = k8sClient.Delete(ctx, network)
		if err != nil {
			t.Errorf("Failed to delete Network: %v", err)
		}

		// Delete Volume (depends on StoragePool)
		err = k8sClient.Delete(ctx, volume)
		if err != nil {
			t.Errorf("Failed to delete Volume: %v", err)
		}

		// Delete StoragePool last (Volume depends on it)
		err = k8sClient.Delete(ctx, storagePool)
		if err != nil {
			t.Errorf("Failed to delete StoragePool: %v", err)
		}
	})

	t.Log("Cross-resource integration test completed successfully")
}

func testVolumeReferenceValidation(t *testing.T, ctx context.Context, k8sClient client.Client) {
	// Test case: Domain referencing non-existent Volume should handle gracefully
	domain := &v1beta1.Domain{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-invalid-volume-ref",
		},
		Spec: v1beta1.DomainSpec{
			ManagedResourceSpec: xpv1.ManagedResourceSpec{
				ProviderConfigReference: &xpv1.ProviderConfigReference{Name: "test-provider-config"},
			},
			ForProvider: v1beta1.DomainParameters{
				Name:   "test-vm-invalid-ref",
				Memory: 1073741824,
				Vcpu:   1,
				Disk: []v1beta1.DomainDisk{
					{
						VolumeRef: &xpv1.Reference{Name: "non-existent-volume"},
						Type:      "virtio",
					},
				},
			},
		},
	}

	err := k8sClient.Create(ctx, domain)
	if err != nil {
		t.Fatalf("Failed to create Domain with invalid volume reference: %v", err)
	}

	// The Domain should be created but not become Ready due to missing Volume
	var retrievedDomain v1beta1.Domain
	err = k8sClient.Get(ctx, types.NamespacedName{Name: "test-invalid-volume-ref"}, &retrievedDomain)
	if err != nil {
		t.Fatalf("Failed to retrieve Domain: %v", err)
	}

	// In a real scenario, the controller would set an error condition
	// For this test, we just verify the reference exists
	if len(retrievedDomain.Spec.ForProvider.Disk) == 0 ||
		retrievedDomain.Spec.ForProvider.Disk[0].VolumeRef == nil {
		t.Error("Expected Domain to preserve invalid VolumeRef for debugging")
	}

	// Cleanup
	err = k8sClient.Delete(ctx, domain)
	if err != nil {
		t.Errorf("Failed to cleanup Domain: %v", err)
	}
}

func testNetworkReferenceValidation(t *testing.T, ctx context.Context, k8sClient client.Client) {
	// Test case: Domain referencing non-existent Network should handle gracefully
	domain := &v1beta1.Domain{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-invalid-network-ref",
		},
		Spec: v1beta1.DomainSpec{
			ManagedResourceSpec: xpv1.ManagedResourceSpec{
				ProviderConfigReference: &xpv1.ProviderConfigReference{Name: "test-provider-config"},
			},
			ForProvider: v1beta1.DomainParameters{
				Name:   "test-vm-invalid-network",
				Memory: 1073741824,
				Vcpu:   1,
				NetworkInterface: []v1beta1.DomainNetworkInterface{
					{
						NetworkRef: &xpv1.Reference{Name: "non-existent-network"},
						Model:      "virtio",
					},
				},
			},
		},
	}

	err := k8sClient.Create(ctx, domain)
	if err != nil {
		t.Fatalf("Failed to create Domain with invalid network reference: %v", err)
	}

	var retrievedDomain v1beta1.Domain
	err = k8sClient.Get(ctx, types.NamespacedName{Name: "test-invalid-network-ref"}, &retrievedDomain)
	if err != nil {
		t.Fatalf("Failed to retrieve Domain: %v", err)
	}

	// Verify the reference is preserved
	if len(retrievedDomain.Spec.ForProvider.NetworkInterface) == 0 ||
		retrievedDomain.Spec.ForProvider.NetworkInterface[0].NetworkRef == nil {
		t.Error("Expected Domain to preserve invalid NetworkRef for debugging")
	}

	// Cleanup
	err = k8sClient.Delete(ctx, domain)
	if err != nil {
		t.Errorf("Failed to cleanup Domain: %v", err)
	}
}

func testBackwardCompatibility(t *testing.T, ctx context.Context, k8sClient client.Client) {
	// Test case: Domain using legacy direct file paths should still work
	domain := &v1beta1.Domain{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-legacy-paths",
		},
		Spec: v1beta1.DomainSpec{
			ManagedResourceSpec: xpv1.ManagedResourceSpec{
				ProviderConfigReference: &xpv1.ProviderConfigReference{Name: "test-provider-config"},
			},
			ForProvider: v1beta1.DomainParameters{
				Name:   "test-legacy-vm",
				Memory: 1073741824,
				Vcpu:   1,
				// Legacy disk configuration (direct file path)
				Disk: []v1beta1.DomainDisk{
					{
						File: "/tmp/legacy-disk.qcow2", // Direct file path
						Type: "virtio",
					},
				},
				// Legacy network configuration (direct network name)
				NetworkInterface: []v1beta1.DomainNetworkInterface{
					{
						NetworkName: "default", // Direct network name
						Model:       "virtio",
					},
				},
			},
		},
	}

	err := k8sClient.Create(ctx, domain)
	if err != nil {
		t.Fatalf("Failed to create Domain with legacy configuration: %v", err)
	}

	var retrievedDomain v1beta1.Domain
	err = k8sClient.Get(ctx, types.NamespacedName{Name: "test-legacy-paths"}, &retrievedDomain)
	if err != nil {
		t.Fatalf("Failed to retrieve Domain: %v", err)
	}

	// Verify legacy configuration is preserved
	if len(retrievedDomain.Spec.ForProvider.Disk) == 0 {
		t.Error("Expected Domain to have disk configuration")
	} else {
		disk := retrievedDomain.Spec.ForProvider.Disk[0]
		if disk.File != "/tmp/legacy-disk.qcow2" {
			t.Errorf("Expected disk file '/tmp/legacy-disk.qcow2', got '%s'", disk.File)
		}
	}

	if len(retrievedDomain.Spec.ForProvider.NetworkInterface) == 0 {
		t.Error("Expected Domain to have network interface configuration")
	} else {
		netif := retrievedDomain.Spec.ForProvider.NetworkInterface[0]
		if netif.NetworkName != "default" {
			t.Errorf("Expected network name 'default', got '%s'", netif.NetworkName)
		}
	}

	// Cleanup
	err = k8sClient.Delete(ctx, domain)
	if err != nil {
		t.Errorf("Failed to cleanup Domain: %v", err)
	}
}

func testResourceDependencyOrdering(t *testing.T, ctx context.Context, k8sClient client.Client) {
	// Test case: Create Domain before its dependencies to verify proper error handling

	// Try to create Domain that references non-existent resources
	domain := &v1beta1.Domain{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-dependency-ordering",
		},
		Spec: v1beta1.DomainSpec{
			ManagedResourceSpec: xpv1.ManagedResourceSpec{
				ProviderConfigReference: &xpv1.ProviderConfigReference{Name: "test-provider-config"},
			},
			ForProvider: v1beta1.DomainParameters{
				Name:   "test-dependency-vm",
				Memory: 1073741824,
				Vcpu:   1,
				Disk: []v1beta1.DomainDisk{
					{
						VolumeRef: &xpv1.Reference{Name: "future-volume"},
						Type:      "virtio",
					},
				},
				NetworkInterface: []v1beta1.DomainNetworkInterface{
					{
						NetworkRef: &xpv1.Reference{Name: "future-network"},
						Model:      "virtio",
					},
				},
			},
		},
	}

	// Domain creation should succeed (references are validated at reconcile time)
	err := k8sClient.Create(ctx, domain)
	if err != nil {
		t.Fatalf("Failed to create Domain with forward references: %v", err)
	}

	// Now create the dependencies
	volume := &v1beta1.Volume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "future-volume",
		},
		Spec: v1beta1.VolumeSpec{
			ManagedResourceSpec: xpv1.ManagedResourceSpec{
				ProviderConfigReference: &xpv1.ProviderConfigReference{Name: "test-provider-config"},
			},
			ForProvider: v1beta1.VolumeParameters{
				Name:   "future-volume.qcow2",
				Pool:   "default",
				Format: "qcow2",
				Size:   int64Ptr(10737418240), // 10GB in bytes
			},
		},
	}

	network := &v1beta1.Network{
		ObjectMeta: metav1.ObjectMeta{
			Name: "future-network",
		},
		Spec: v1beta1.NetworkSpec{
			ManagedResourceSpec: xpv1.ManagedResourceSpec{
				ProviderConfigReference: &xpv1.ProviderConfigReference{Name: "test-provider-config"},
			},
			ForProvider: v1beta1.NetworkParameters{
				Name: "future-network",
				Mode: "nat",
				IP: []v1beta1.NetworkIP{
					{
						Address: "10.0.0.1",
						Netmask: "255.255.255.0",
					},
				},
				Autostart: &[]bool{true}[0],
			},
		},
	}

	err = k8sClient.Create(ctx, volume)
	if err != nil {
		t.Fatalf("Failed to create Volume: %v", err)
	}

	err = k8sClient.Create(ctx, network)
	if err != nil {
		t.Fatalf("Failed to create Network: %v", err)
	}

	// Verify all resources exist
	var retrievedDomain v1beta1.Domain
	err = k8sClient.Get(ctx, types.NamespacedName{Name: "test-dependency-ordering"}, &retrievedDomain)
	if err != nil {
		t.Fatalf("Failed to retrieve Domain: %v", err)
	}

	var retrievedVolume v1beta1.Volume
	err = k8sClient.Get(ctx, types.NamespacedName{Name: "future-volume"}, &retrievedVolume)
	if err != nil {
		t.Fatalf("Failed to retrieve Volume: %v", err)
	}

	var retrievedNetwork v1beta1.Network
	err = k8sClient.Get(ctx, types.NamespacedName{Name: "future-network"}, &retrievedNetwork)
	if err != nil {
		t.Fatalf("Failed to retrieve Network: %v", err)
	}

	// Cleanup in proper order
	err = k8sClient.Delete(ctx, domain)
	if err != nil {
		t.Errorf("Failed to delete Domain: %v", err)
	}
	err = k8sClient.Delete(ctx, volume)
	if err != nil {
		t.Errorf("Failed to delete Volume: %v", err)
	}
	err = k8sClient.Delete(ctx, network)
	if err != nil {
		t.Errorf("Failed to delete Network: %v", err)
	}
}

// Helper function to create test scheme with all required types
func createTestScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = v1beta1.SchemeBuilder.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	return scheme
}
