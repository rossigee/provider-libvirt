/*
Copyright 2025 Ross Golder

Test helper functions for provider-libvirt integration tests.
These functions provide common utilities for setting up test resources.
*/

package test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"

	"github.com/rossigee/provider-libvirt/apis/v1alpha1"
	"github.com/rossigee/provider-libvirt/internal/utils"
)

// createTestStoragePool creates a basic StoragePool for testing
func createTestStoragePool(name string) *v1alpha1.StoragePool {
	return &v1alpha1.StoragePool{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1alpha1.StoragePoolSpec{
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: &xpv1.Reference{Name: "test-provider-config"},
			},
			ForProvider: v1alpha1.StoragePoolParameters{
				Name: name,
				Type: "dir",
				Target: &v1alpha1.StoragePoolTarget{
					Path: "/tmp/" + name,
				},
				AutoStart: &[]bool{true}[0],
			},
		},
	}
}

// createTestVolume creates a basic Volume for testing
func createTestVolume(name, poolName string) *v1alpha1.Volume {
	return &v1alpha1.Volume{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1alpha1.VolumeSpec{
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: &xpv1.Reference{Name: "test-provider-config"},
			},
			ForProvider: v1alpha1.VolumeParameters{
				Name:   name + ".qcow2",
				Pool:   poolName,
				Format: "qcow2",
				Size:   "10G", // 10GB using human-readable format
			},
		},
	}
}

// createTestNetwork creates a basic Network for testing
func createTestNetwork(name string) *v1alpha1.Network {
	return &v1alpha1.Network{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1alpha1.NetworkSpec{
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: &xpv1.Reference{Name: "test-provider-config"},
			},
			ForProvider: v1alpha1.NetworkParameters{
				Name: name,
				Mode: "nat",
				IP: &v1alpha1.NetworkIP{
					Address: "192.168.122.1",
					Netmask: "255.255.255.0",
				},
				AutoStart: &[]bool{true}[0],
			},
		},
	}
}

// createResource creates a resource in the fake client
func createResource(t *testing.T, ctx context.Context, k8sClient client.Client, obj client.Object) {
	err := k8sClient.Create(ctx, obj)
	if err != nil {
		t.Fatalf("Failed to create resource %T: %v", obj, err)
	}
}

// makeResourceReady simulates a resource becoming ready by setting appropriate conditions
func makeResourceReady(t *testing.T, ctx context.Context, k8sClient client.Client, obj client.Object) {
	// Get the current object
	err := k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)
	if err != nil {
		t.Fatalf("Failed to get resource for ready update: %v", err)
	}

	// Set the resource as ready based on its type
	switch resource := obj.(type) {
	case *v1alpha1.StoragePool:
		resource.Status.SetConditions(xpv1.Available())
		resource.Status.AtProvider.State = "active"
		resource.Status.AtProvider.Capacity = 107374182400 // 100GB
		resource.Status.AtProvider.Available = 107374182400

	case *v1alpha1.Volume:
		resource.Status.SetConditions(xpv1.Available())
		resource.Status.AtProvider.Path = "/tmp/" + resource.Spec.ForProvider.Pool + "/" + resource.Spec.ForProvider.Name

		// Resolve capacity from Size or Capacity field
		if capacity, err := resolveCapacityFromParameters(resource.Spec.ForProvider); err == nil {
			resource.Status.AtProvider.Capacity = capacity
		} else {
			resource.Status.AtProvider.Capacity = 10000000000 // Default 10GB if parsing fails
		}

		resource.Status.AtProvider.Allocation = 1048576 // 1MB allocated initially
		resource.Status.AtProvider.Type = "file"
		resource.Status.AtProvider.Format = resource.Spec.ForProvider.Format

	case *v1alpha1.Network:
		resource.Status.SetConditions(xpv1.Available())
		resource.Status.AtProvider.Active = true
		resource.Status.AtProvider.Persistent = true
		resource.Status.AtProvider.AutoStart = true

	case *v1alpha1.Domain:
		resource.Status.SetConditions(xpv1.Available())
		resource.Status.AtProvider.State = "shut off" // Start in stopped state for safety
		resource.Status.AtProvider.ID = "123"
		resource.Status.AtProvider.UUID = "550e8400-e29b-41d4-a716-446655440000"

	default:
		t.Fatalf("Unknown resource type for makeResourceReady: %T", obj)
	}

	// Update the status
	err = k8sClient.Status().Update(ctx, obj)
	if err != nil {
		t.Logf("Warning: Failed to update resource status (this is expected in fake client): %v", err)
	}
}

// setupTestEnvironment performs common test environment setup
func setupTestEnvironment(t *testing.T, ctx context.Context, k8sClient client.Client) {
	// Create test provider config
	providerConfig := &v1alpha1.ProviderConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-provider-config",
		},
		Spec: v1alpha1.ProviderConfigSpec{
			Credentials: v1alpha1.ProviderCredentials{
				Source: xpv1.CredentialsSourceSecret,
				CommonCredentialSelectors: xpv1.CommonCredentialSelectors{
					SecretRef: &xpv1.SecretKeySelector{
						SecretReference: xpv1.SecretReference{
							Name:      "test-libvirt-creds",
							Namespace: "crossplane-system",
						},
						Key: "credentials",
					},
				},
			},
		},
	}

	// Create test credentials secret
	creds := map[string]string{
		"uri": "qemu+tcp://localhost:16509/system",
	}
	credsJSON, _ := json.Marshal(creds)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-libvirt-creds",
			Namespace: "crossplane-system",
		},
		Data: map[string][]byte{
			"credentials": credsJSON,
		},
	}

	err := k8sClient.Create(ctx, providerConfig)
	if err != nil {
		t.Logf("Provider config may already exist: %v", err)
	}

	err = k8sClient.Create(ctx, secret)
	if err != nil {
		t.Logf("Secret may already exist: %v", err)
	}

	t.Log("Test environment setup completed")
}

// resolveCapacityFromParameters resolves the volume capacity from either Size or Capacity fields
// Size takes precedence over Capacity if both are specified
func resolveCapacityFromParameters(spec v1alpha1.VolumeParameters) (int64, error) {
	if spec.Size != "" {
		// Parse human-readable size (e.g., "100G")
		capacity, err := utils.ParseSize(spec.Size)
		if err != nil {
			return 0, err
		}
		return capacity, nil
	}

	if spec.Capacity != nil {
		// Use legacy byte capacity
		return *spec.Capacity, nil
	}

	return 0, fmt.Errorf("either size or capacity must be specified")
}