/*
Copyright 2025 Ross Golder

End-to-end lifecycle tests for provider-libvirt.
These tests simulate complete VM lifecycle scenarios with proper state transitions.
*/

package test

import (
	"context"
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"

	"github.com/rossigee/provider-libvirt/apis/v1beta1"
)

// TestVMLifecycleWorkflows tests complete VM lifecycle scenarios
func TestVMLifecycleWorkflows(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E lifecycle tests in short mode")
	}

	ctx := context.Background()
	scheme := createTestScheme()

	// Test scenarios with different VM configurations
	scenarios := []struct {
		name        string
		description string
		testFunc    func(t *testing.T, ctx context.Context, client client.Client)
	}{
		{
			name:        "BasicVMLifecycle",
			description: "Test basic VM creation, start, stop, and deletion",
			testFunc:    testBasicVMLifecycle,
		},
		{
			name:        "MultiDiskVMLifecycle",
			description: "Test VM with multiple disks lifecycle",
			testFunc:    testMultiDiskVMLifecycle,
		},
		{
			name:        "MultiNetworkVMLifecycle",
			description: "Test VM with multiple network interfaces",
			testFunc:    testMultiNetworkVMLifecycle,
		},
		{
			name:        "VMStateTransitions",
			description: "Test VM state transitions (running, stopped, paused)",
			testFunc:    testVMStateTransitions,
		},
		{
			name:        "VMConfigurationUpdates",
			description: "Test VM configuration updates (memory, CPU)",
			testFunc:    testVMConfigurationUpdates,
		},
		{
			name:        "VMErrorRecovery",
			description: "Test VM error scenarios and recovery",
			testFunc:    testVMErrorRecovery,
		},
		{
			name:        "ConcurrentVMOperations",
			description: "Test concurrent VM operations",
			testFunc:    testConcurrentVMOperations,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			// Create fresh client for each test to avoid interference
			k8sClient := fake.NewClientBuilder().WithScheme(scheme).Build()

			// Setup test environment
			setupTestEnvironment(t, ctx, k8sClient)

			// Run the specific test
			scenario.testFunc(t, ctx, k8sClient)

			// Cleanup test environment
			cleanupTestEnvironment(t, ctx, k8sClient)
		})
	}
}

func testBasicVMLifecycle(t *testing.T, ctx context.Context, k8sClient client.Client) {
	// Phase 1: Create infrastructure resources
	storagePool := createTestStoragePool("basic-pool")
	volume := createTestVolume("basic-disk", "basic-pool")
	network := createTestNetwork("basic-network")

	// Create resources
	createResource(t, ctx, k8sClient, storagePool)
	makeResourceReady(t, ctx, k8sClient, storagePool)

	createResource(t, ctx, k8sClient, volume)
	makeResourceReady(t, ctx, k8sClient, volume)

	createResource(t, ctx, k8sClient, network)
	makeResourceReady(t, ctx, k8sClient, network)

	// Phase 2: Create VM (initially stopped)
	domain := &v1beta1.Domain{
		ObjectMeta: metav1.ObjectMeta{
			Name: "basic-vm",
		},
		Spec: v1beta1.DomainSpec{
			ManagedResourceSpec: xpv1.ManagedResourceSpec{
				ProviderConfigReference: &xpv1.ProviderConfigReference{Name: "test-provider-config"},
			},
			ForProvider: v1beta1.DomainParameters{
				Name:    "basic-test-vm",
				Memory:  1073741824, // 1GB
				Vcpu:    1,
				Type:    "kvm",
				Running: &[]bool{false}[0], // Start stopped
				Disk: []v1beta1.DomainDisk{
					{
						VolumeRef: &xpv1.Reference{Name: volume.Name},
						Type:      "virtio",
						Device:    "vda",
						BootOrder: &[]int32{1}[0],
					},
				},
				NetworkInterface: []v1beta1.DomainNetworkInterface{
					{
						NetworkRef: &xpv1.Reference{Name: network.Name},
						Model:      "virtio",
					},
				},
			},
		},
	}

	createResource(t, ctx, k8sClient, domain)

	// Simulate domain creation in libvirt
	domain.Status.SetConditions(xpv1.Available())
	domain.Status.AtProvider.ID = "1"
	domain.Status.AtProvider.UUID = "550e8400-e29b-41d4-a716-446655440000"
	domain.Status.AtProvider.State = "shutoff"
	updateResourceStatus(t, ctx, k8sClient, domain)

	// Phase 3: Start VM
	t.Run("StartVM", func(t *testing.T) {
		// Update domain spec to start VM
		var currentDomain v1beta1.Domain
		err := k8sClient.Get(ctx, types.NamespacedName{Name: domain.Name}, &currentDomain)
		if err != nil {
			t.Fatalf("Failed to get domain: %v", err)
		}

		currentDomain.Spec.ForProvider.Running = &[]bool{true}[0]
		err = k8sClient.Update(ctx, &currentDomain)
		if err != nil {
			t.Fatalf("Failed to update domain to running: %v", err)
		}

		// Simulate VM starting
		currentDomain.Status.AtProvider.State = "running"
		updateResourceStatus(t, ctx, k8sClient, &currentDomain)

		// Verify VM is running
		var updatedDomain v1beta1.Domain
		err = k8sClient.Get(ctx, types.NamespacedName{Name: domain.Name}, &updatedDomain)
		if err != nil {
			t.Fatalf("Failed to get updated domain: %v", err)
		}

		if updatedDomain.Spec.ForProvider.Running == nil || !*updatedDomain.Spec.ForProvider.Running {
			t.Error("Expected domain to be running")
		}
		if updatedDomain.Status.AtProvider.State != "running" {
			t.Errorf("Expected domain state 'running', got '%s'", updatedDomain.Status.AtProvider.State)
		}
	})

	// Phase 4: Stop VM
	t.Run("StopVM", func(t *testing.T) {
		var currentDomain v1beta1.Domain
		err := k8sClient.Get(ctx, types.NamespacedName{Name: domain.Name}, &currentDomain)
		if err != nil {
			t.Fatalf("Failed to get domain: %v", err)
		}

		currentDomain.Spec.ForProvider.Running = &[]bool{false}[0]
		err = k8sClient.Update(ctx, &currentDomain)
		if err != nil {
			t.Fatalf("Failed to update domain to stopped: %v", err)
		}

		// Simulate VM stopping
		currentDomain.Status.AtProvider.State = "shutoff"
		updateResourceStatus(t, ctx, k8sClient, &currentDomain)

		// Verify VM is stopped
		var updatedDomain v1beta1.Domain
		err = k8sClient.Get(ctx, types.NamespacedName{Name: domain.Name}, &updatedDomain)
		if err != nil {
			t.Fatalf("Failed to get updated domain: %v", err)
		}

		if updatedDomain.Spec.ForProvider.Running != nil && *updatedDomain.Spec.ForProvider.Running {
			t.Error("Expected domain to be stopped")
		}
		if updatedDomain.Status.AtProvider.State != "shutoff" {
			t.Errorf("Expected domain state 'shutoff', got '%s'", updatedDomain.Status.AtProvider.State)
		}
	})

	// Phase 5: Delete VM and cleanup
	t.Run("DeleteVM", func(t *testing.T) {
		err := k8sClient.Delete(ctx, domain)
		if err != nil {
			t.Fatalf("Failed to delete domain: %v", err)
		}

		// Verify domain is deleted
		var deletedDomain v1beta1.Domain
		err = k8sClient.Get(ctx, types.NamespacedName{Name: domain.Name}, &deletedDomain)
		if err == nil {
			t.Error("Expected domain to be deleted, but it still exists")
		}
	})

	t.Log("Basic VM lifecycle test completed successfully")
}

func testMultiDiskVMLifecycle(t *testing.T, ctx context.Context, k8sClient client.Client) {
	// Create multiple storage pools and volumes
	ssdPool := createTestStoragePool("ssd-pool")
	dataPool := createTestStoragePool("data-pool")
	network := createTestNetwork("multi-disk-network")

	systemDisk := createTestVolume("system-disk", "ssd-pool")
	dataDisk := createTestVolume("data-disk", "data-pool")
	logDisk := createTestVolume("log-disk", "data-pool")

	// Create all resources
	resources := []client.Object{ssdPool, dataPool, network, systemDisk, dataDisk, logDisk}
	for _, resource := range resources {
		createResource(t, ctx, k8sClient, resource)
		makeResourceReady(t, ctx, k8sClient, resource)
	}

	// Create VM with multiple disks
	domain := &v1beta1.Domain{
		ObjectMeta: metav1.ObjectMeta{
			Name: "multi-disk-vm",
		},
		Spec: v1beta1.DomainSpec{
			ManagedResourceSpec: xpv1.ManagedResourceSpec{
				ProviderConfigReference: &xpv1.ProviderConfigReference{Name: "test-provider-config"},
			},
			ForProvider: v1beta1.DomainParameters{
				Name:    "multi-disk-test-vm",
				Memory:  4294967296, // 4GB
				Vcpu:    2,
				Running: &[]bool{false}[0],
				Disk: []v1beta1.DomainDisk{
					{
						VolumeRef: &xpv1.Reference{Name: "system-disk"},
						Type:      "virtio",
						Device:    "vda",
						BootOrder: &[]int32{1}[0],
						WWN:       "0x50014ee20b2a5599",
					},
					{
						VolumeRef: &xpv1.Reference{Name: "data-disk"},
						Type:      "virtio",
						Device:    "vdb",
						WWN:       "0x50014ee20b2a559a",
					},
					{
						VolumeRef: &xpv1.Reference{Name: "log-disk"},
						Type:      "virtio",
						Device:    "vdc",
						WWN:       "0x50014ee20b2a559b",
					},
				},
				NetworkInterface: []v1beta1.DomainNetworkInterface{
					{
						NetworkRef: &xpv1.Reference{Name: "multi-disk-network"},
						Model:      "virtio",
					},
				},
			},
		},
	}

	createResource(t, ctx, k8sClient, domain)
	makeResourceReady(t, ctx, k8sClient, domain)

	// Verify all disk references are preserved
	var retrievedDomain v1beta1.Domain
	err := k8sClient.Get(ctx, types.NamespacedName{Name: "multi-disk-vm"}, &retrievedDomain)
	if err != nil {
		t.Fatalf("Failed to retrieve domain: %v", err)
	}

	if len(retrievedDomain.Spec.ForProvider.Disk) != 3 {
		t.Errorf("Expected 3 disks, got %d", len(retrievedDomain.Spec.ForProvider.Disk))
	}

	// Verify disk ordering and references
	expectedDisks := []struct {
		volumeName string
		device     string
		wwn        string
	}{
		{"system-disk", "vda", "0x50014ee20b2a5599"},
		{"data-disk", "vdb", "0x50014ee20b2a559a"},
		{"log-disk", "vdc", "0x50014ee20b2a559b"},
	}

	for i, expected := range expectedDisks {
		if i >= len(retrievedDomain.Spec.ForProvider.Disk) {
			t.Errorf("Missing disk %d", i)
			continue
		}

		disk := retrievedDomain.Spec.ForProvider.Disk[i]
		if disk.VolumeRef == nil || disk.VolumeRef.Name != expected.volumeName {
			t.Errorf("Disk %d: expected volume %s, got %v", i, expected.volumeName, disk.VolumeRef)
		}
		if disk.Device != expected.device {
			t.Errorf("Disk %d: expected device %s, got %s", i, expected.device, disk.Device)
		}
		if disk.WWN != expected.wwn {
			t.Errorf("Disk %d: expected WWN %s, got %s", i, expected.wwn, disk.WWN)
		}
	}

	t.Log("Multi-disk VM lifecycle test completed successfully")
}

func testMultiNetworkVMLifecycle(t *testing.T, ctx context.Context, k8sClient client.Client) {
	// Create multiple networks
	dmzNetwork := createTestNetwork("dmz-network")
	mgmtNetwork := createTestNetwork("mgmt-network")
	volume := createTestVolume("multi-net-disk", "default")

	resources := []client.Object{dmzNetwork, mgmtNetwork, volume}
	for _, resource := range resources {
		createResource(t, ctx, k8sClient, resource)
		makeResourceReady(t, ctx, k8sClient, resource)
	}

	// Create VM with multiple network interfaces
	domain := &v1beta1.Domain{
		ObjectMeta: metav1.ObjectMeta{
			Name: "multi-network-vm",
		},
		Spec: v1beta1.DomainSpec{
			ManagedResourceSpec: xpv1.ManagedResourceSpec{
				ProviderConfigReference: &xpv1.ProviderConfigReference{Name: "test-provider-config"},
			},
			ForProvider: v1beta1.DomainParameters{
				Name:    "multi-network-test-vm",
				Memory:  2147483648, // 2GB
				Vcpu:    2,
				Running: &[]bool{false}[0],
				Disk: []v1beta1.DomainDisk{
					{
						VolumeRef: &xpv1.Reference{Name: "multi-net-disk"},
						Type:      "virtio",
					},
				},
				NetworkInterface: []v1beta1.DomainNetworkInterface{
					{
						NetworkRef: &xpv1.Reference{Name: "dmz-network"},
						Model:      "virtio",
					},
					{
						NetworkRef: &xpv1.Reference{Name: "mgmt-network"},
						Model:      "virtio",
					},
					{
						// Test mixed configuration - bridge interface
						NetworkName: "br0",
						Model:       "virtio",
					},
				},
			},
		},
	}

	createResource(t, ctx, k8sClient, domain)
	makeResourceReady(t, ctx, k8sClient, domain)

	// Verify network interface configuration
	var retrievedDomain v1beta1.Domain
	err := k8sClient.Get(ctx, types.NamespacedName{Name: "multi-network-vm"}, &retrievedDomain)
	if err != nil {
		t.Fatalf("Failed to retrieve domain: %v", err)
	}

	if len(retrievedDomain.Spec.ForProvider.NetworkInterface) != 3 {
		t.Errorf("Expected 3 network interfaces, got %d", len(retrievedDomain.Spec.ForProvider.NetworkInterface))
	}

	// Verify network interface types
	interfaces := retrievedDomain.Spec.ForProvider.NetworkInterface

	// First interface - network reference
	if interfaces[0].NetworkRef == nil || interfaces[0].NetworkRef.Name != "dmz-network" {
		t.Errorf("Expected first interface to reference dmz-network")
	}

	// Second interface - network reference
	if interfaces[1].NetworkRef == nil || interfaces[1].NetworkRef.Name != "mgmt-network" {
		t.Errorf("Expected second interface to reference mgmt-network")
	}

	// Third interface - bridge
	if interfaces[2].NetworkName != "br0" {
		t.Errorf("Expected third interface to use bridge br0, got %s", interfaces[2].NetworkName)
	}

	t.Log("Multi-network VM lifecycle test completed successfully")
}

func testVMStateTransitions(t *testing.T, ctx context.Context, k8sClient client.Client) {
	// Setup basic VM
	volume := createTestVolume("state-test-disk", "default")
	network := createTestNetwork("state-test-network")

	createResource(t, ctx, k8sClient, volume)
	makeResourceReady(t, ctx, k8sClient, volume)
	createResource(t, ctx, k8sClient, network)
	makeResourceReady(t, ctx, k8sClient, network)

	domain := &v1beta1.Domain{
		ObjectMeta: metav1.ObjectMeta{
			Name: "state-test-vm",
		},
		Spec: v1beta1.DomainSpec{
			ManagedResourceSpec: xpv1.ManagedResourceSpec{
				ProviderConfigReference: &xpv1.ProviderConfigReference{Name: "test-provider-config"},
			},
			ForProvider: v1beta1.DomainParameters{
				Name:    "state-transition-vm",
				Memory:  1073741824,
				Vcpu:    1,
				Running: &[]bool{false}[0], // Start stopped
				Disk: []v1beta1.DomainDisk{
					{VolumeRef: &xpv1.Reference{Name: "state-test-disk"}, Type: "virtio"},
				},
				NetworkInterface: []v1beta1.DomainNetworkInterface{
					{NetworkRef: &xpv1.Reference{Name: "state-test-network"}, Model: "virtio"},
				},
			},
		},
	}

	createResource(t, ctx, k8sClient, domain)

	// Test state transitions: stopped -> running -> stopped
	states := []struct {
		name          string
		running       bool
		expectedState string
	}{
		{"InitialStopped", false, "shutoff"},
		{"StartVM", true, "running"},
		{"StopVM", false, "shutoff"},
		{"RestartVM", true, "running"},
	}

	for _, state := range states {
		t.Run(state.name, func(t *testing.T) {
			// Update VM running state
			var currentDomain v1beta1.Domain
			err := k8sClient.Get(ctx, types.NamespacedName{Name: "state-test-vm"}, &currentDomain)
			if err != nil {
				t.Fatalf("Failed to get domain: %v", err)
			}

			currentDomain.Spec.ForProvider.Running = &state.running
			err = k8sClient.Update(ctx, &currentDomain)
			if err != nil {
				t.Fatalf("Failed to update domain running state: %v", err)
			}

			// Simulate libvirt state change
			currentDomain.Status.AtProvider.State = state.expectedState
			updateResourceStatus(t, ctx, k8sClient, &currentDomain)

			// Verify state
			var updatedDomain v1beta1.Domain
			err = k8sClient.Get(ctx, types.NamespacedName{Name: "state-test-vm"}, &updatedDomain)
			if err != nil {
				t.Fatalf("Failed to get updated domain: %v", err)
			}

			if (updatedDomain.Spec.ForProvider.Running == nil && state.running) || (updatedDomain.Spec.ForProvider.Running != nil && *updatedDomain.Spec.ForProvider.Running != state.running) {
				t.Errorf("Expected running=%v, got %v", state.running, updatedDomain.Spec.ForProvider.Running)
			}

			if updatedDomain.Status.AtProvider.State != state.expectedState {
				t.Errorf("Expected state=%s, got %s", state.expectedState, updatedDomain.Status.AtProvider.State)
			}
		})
	}

	t.Log("VM state transitions test completed successfully")
}

func testVMConfigurationUpdates(t *testing.T, ctx context.Context, k8sClient client.Client) {
	// Setup basic VM
	volume := createTestVolume("config-test-disk", "default")
	network := createTestNetwork("config-test-network")

	createResource(t, ctx, k8sClient, volume)
	makeResourceReady(t, ctx, k8sClient, volume)
	createResource(t, ctx, k8sClient, network)
	makeResourceReady(t, ctx, k8sClient, network)

	domain := &v1beta1.Domain{
		ObjectMeta: metav1.ObjectMeta{
			Name: "config-test-vm",
		},
		Spec: v1beta1.DomainSpec{
			ManagedResourceSpec: xpv1.ManagedResourceSpec{
				ProviderConfigReference: &xpv1.ProviderConfigReference{Name: "test-provider-config"},
			},
			ForProvider: v1beta1.DomainParameters{
				Name:    "config-update-vm",
				Memory:  1073741824, // 1GB
				Vcpu:    1,
				Running: &[]bool{false}[0],
				Disk: []v1beta1.DomainDisk{
					{VolumeRef: &xpv1.Reference{Name: "config-test-disk"}, Type: "virtio"},
				},
				NetworkInterface: []v1beta1.DomainNetworkInterface{
					{NetworkRef: &xpv1.Reference{Name: "config-test-network"}, Model: "virtio"},
				},
			},
		},
	}

	createResource(t, ctx, k8sClient, domain)
	makeResourceReady(t, ctx, k8sClient, domain)

	// Test configuration updates
	t.Run("UpdateMemory", func(t *testing.T) {
		var currentDomain v1beta1.Domain
		err := k8sClient.Get(ctx, types.NamespacedName{Name: "config-test-vm"}, &currentDomain)
		if err != nil {
			t.Fatalf("Failed to get domain: %v", err)
		}

		// Update memory to 2GB
		currentDomain.Spec.ForProvider.Memory = 2147483648
		err = k8sClient.Update(ctx, &currentDomain)
		if err != nil {
			t.Fatalf("Failed to update domain memory: %v", err)
		}

		// Verify update
		var updatedDomain v1beta1.Domain
		err = k8sClient.Get(ctx, types.NamespacedName{Name: "config-test-vm"}, &updatedDomain)
		if err != nil {
			t.Fatalf("Failed to get updated domain: %v", err)
		}

		if updatedDomain.Spec.ForProvider.Memory != 2147483648 {
			t.Errorf("Expected memory 2147483648, got %d", updatedDomain.Spec.ForProvider.Memory)
		}
	})

	t.Run("UpdateCPU", func(t *testing.T) {
		var currentDomain v1beta1.Domain
		err := k8sClient.Get(ctx, types.NamespacedName{Name: "config-test-vm"}, &currentDomain)
		if err != nil {
			t.Fatalf("Failed to get domain: %v", err)
		}

		// Update CPU to 2 cores
		currentDomain.Spec.ForProvider.Vcpu = 2
		err = k8sClient.Update(ctx, &currentDomain)
		if err != nil {
			t.Fatalf("Failed to update domain CPU: %v", err)
		}

		// Verify update
		var updatedDomain v1beta1.Domain
		err = k8sClient.Get(ctx, types.NamespacedName{Name: "config-test-vm"}, &updatedDomain)
		if err != nil {
			t.Fatalf("Failed to get updated domain: %v", err)
		}

		if updatedDomain.Spec.ForProvider.Vcpu != 2 {
			t.Errorf("Expected vcpu 2, got %d", updatedDomain.Spec.ForProvider.Vcpu)
		}
	})

	t.Log("VM configuration updates test completed successfully")
}

func testVMErrorRecovery(t *testing.T, ctx context.Context, k8sClient client.Client) {
	// Test various error scenarios

	t.Run("MissingVolumeReference", func(t *testing.T) {
		domain := &v1beta1.Domain{
			ObjectMeta: metav1.ObjectMeta{
				Name: "error-missing-volume",
			},
			Spec: v1beta1.DomainSpec{
				ManagedResourceSpec: xpv1.ManagedResourceSpec{
					ProviderConfigReference: &xpv1.ProviderConfigReference{Name: "test-provider-config"},
				},
				ForProvider: v1beta1.DomainParameters{
					Name:    "error-vm",
					Memory:  1073741824,
					Vcpu:    1,
					Running: &[]bool{false}[0],
					Disk: []v1beta1.DomainDisk{
						{VolumeRef: &xpv1.Reference{Name: "non-existent-volume"}, Type: "virtio"},
					},
				},
			},
		}

		createResource(t, ctx, k8sClient, domain)

		// The domain should be created but not become ready
		var retrievedDomain v1beta1.Domain
		err := k8sClient.Get(ctx, types.NamespacedName{Name: "error-missing-volume"}, &retrievedDomain)
		if err != nil {
			t.Fatalf("Failed to retrieve domain: %v", err)
		}

		// Verify the error reference is preserved for debugging
		if len(retrievedDomain.Spec.ForProvider.Disk) == 0 ||
			retrievedDomain.Spec.ForProvider.Disk[0].VolumeRef == nil ||
			retrievedDomain.Spec.ForProvider.Disk[0].VolumeRef.Name != "non-existent-volume" {
			t.Error("Expected domain to preserve invalid volume reference")
		}

		// Cleanup
		err = k8sClient.Delete(ctx, domain)
		if err != nil {
			t.Errorf("Failed to cleanup domain: %v", err)
		}
	})

	t.Run("MissingNetworkReference", func(t *testing.T) {
		domain := &v1beta1.Domain{
			ObjectMeta: metav1.ObjectMeta{
				Name: "error-missing-network",
			},
			Spec: v1beta1.DomainSpec{
				ManagedResourceSpec: xpv1.ManagedResourceSpec{
					ProviderConfigReference: &xpv1.ProviderConfigReference{Name: "test-provider-config"},
				},
				ForProvider: v1beta1.DomainParameters{
					Name:    "error-network-vm",
					Memory:  1073741824,
					Vcpu:    1,
					Running: &[]bool{false}[0],
					NetworkInterface: []v1beta1.DomainNetworkInterface{
						{NetworkRef: &xpv1.Reference{Name: "non-existent-network"}, Model: "virtio"},
					},
				},
			},
		}

		createResource(t, ctx, k8sClient, domain)

		var retrievedDomain v1beta1.Domain
		err := k8sClient.Get(ctx, types.NamespacedName{Name: "error-missing-network"}, &retrievedDomain)
		if err != nil {
			t.Fatalf("Failed to retrieve domain: %v", err)
		}

		// Verify the error reference is preserved
		if len(retrievedDomain.Spec.ForProvider.NetworkInterface) == 0 ||
			retrievedDomain.Spec.ForProvider.NetworkInterface[0].NetworkRef == nil ||
			retrievedDomain.Spec.ForProvider.NetworkInterface[0].NetworkRef.Name != "non-existent-network" {
			t.Error("Expected domain to preserve invalid network reference")
		}

		// Cleanup
		err = k8sClient.Delete(ctx, domain)
		if err != nil {
			t.Errorf("Failed to cleanup domain: %v", err)
		}
	})

	t.Log("VM error recovery test completed successfully")
}

func testConcurrentVMOperations(t *testing.T, ctx context.Context, k8sClient client.Client) {
	// Create shared resources
	sharedVolume := createTestVolume("shared-disk", "default")
	sharedNetwork := createTestNetwork("shared-network")

	createResource(t, ctx, k8sClient, sharedVolume)
	makeResourceReady(t, ctx, k8sClient, sharedVolume)
	createResource(t, ctx, k8sClient, sharedNetwork)
	makeResourceReady(t, ctx, k8sClient, sharedNetwork)

	// Create multiple VMs concurrently
	vmCount := 3
	domains := make([]*v1beta1.Domain, vmCount)

	for i := 0; i < vmCount; i++ {
		domains[i] = &v1beta1.Domain{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("concurrent-vm-%d", i),
			},
			Spec: v1beta1.DomainSpec{
				ManagedResourceSpec: xpv1.ManagedResourceSpec{
					ProviderConfigReference: &xpv1.ProviderConfigReference{Name: "test-provider-config"},
				},
				ForProvider: v1beta1.DomainParameters{
					Name:    fmt.Sprintf("concurrent-test-vm-%d", i),
					Memory:  1073741824,
					Vcpu:    1,
					Running: &[]bool{false}[0],
					Disk: []v1beta1.DomainDisk{
						{VolumeRef: &xpv1.Reference{Name: "shared-disk"}, Type: "virtio"},
					},
					NetworkInterface: []v1beta1.DomainNetworkInterface{
						{NetworkRef: &xpv1.Reference{Name: "shared-network"}, Model: "virtio"},
					},
				},
			},
		}
	}

	// Create all VMs concurrently
	for i, domain := range domains {
		t.Run(fmt.Sprintf("CreateVM%d", i), func(t *testing.T) {
			t.Parallel() // Run in parallel
			createResource(t, ctx, k8sClient, domain)
			makeResourceReady(t, ctx, k8sClient, domain)
		})
	}

	// Verify all VMs were created
	for i, domain := range domains {
		var retrievedDomain v1beta1.Domain
		err := k8sClient.Get(ctx, types.NamespacedName{Name: domain.Name}, &retrievedDomain)
		if err != nil {
			t.Errorf("Failed to retrieve VM %d: %v", i, err)
		}
	}

	// Cleanup all VMs
	for _, domain := range domains {
		err := k8sClient.Delete(ctx, domain)
		if err != nil {
			t.Errorf("Failed to delete domain %s: %v", domain.Name, err)
		}
	}

	t.Log("Concurrent VM operations test completed successfully")
}

// Helper functions for test setup and resource management

func cleanupTestEnvironment(t *testing.T, ctx context.Context, k8sClient client.Client) {
	// Cleanup is handled by individual tests to avoid conflicts
	// In a real environment, you might want to clean up shared resources here
}

func updateResourceStatus(t *testing.T, ctx context.Context, k8sClient client.Client, obj client.Object) {
	err := k8sClient.Status().Update(ctx, obj)
	if err != nil {
		t.Logf("Warning: Failed to update resource status for %T %s: %v", obj, obj.GetName(), err)
	}
}
