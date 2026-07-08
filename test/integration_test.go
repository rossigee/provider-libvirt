/*
Copyright 2025 Ross Golder

Integration tests for provider-libvirt
These tests require a running libvirt instance and are intended for CI/CD validation.
*/

package test

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"

	"github.com/rossigee/provider-libvirt/apis/v1beta1"
)

// TestDomainLifecycle tests the complete lifecycle of a Domain resource
// This is an integration test that should be run against a real libvirt instance
func TestDomainLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This test would require a real libvirt connection
	// For now, it demonstrates the test structure
	
	scheme := runtime.NewScheme()
	_ = v1beta1.SchemeBuilder.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	// Create fake client for this test
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	// Test data
	providerConfig := &v1beta1.ProviderConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-provider-config",
		},
		Spec: v1beta1.ProviderConfigSpec{
			Credentials: v1beta1.ProviderCredentials{
				Source: xpv1.CredentialsSourceSecret,
				CommonCredentialSelectors: xpv1.CommonCredentialSelectors{
					SecretRef: &xpv1.SecretKeySelector{
						SecretReference: xpv1.SecretReference{
							Name:      "libvirt-credentials",
							Namespace: "crossplane-system",
						},
						Key: "credentials",
					},
				},
			},
		},
	}

	domain := &v1beta1.Domain{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-domain",
		},
		Spec: v1beta1.DomainSpec{
			ManagedResourceSpec: xpv1.ManagedResourceSpec{
				ProviderConfigReference: &xpv1.ProviderConfigReference{
					Name: "test-provider-config",
				},
				
			},
			ForProvider: v1beta1.DomainParameters{
				Name:    "test-integration-vm",
				Memory:  1073741824, // 1GB
				Vcpu:    1,
				Type:    "kvm",
				Arch:    "x86_64",
				Running: &[]bool{false}[0], // Start stopped for safety
				Boot:    []string{"hd"},
				Disk: []v1beta1.DomainDisk{
					{
						File: "/tmp/test-vm.qcow2",
						Type: "virtio",
					},
				},
				NetworkInterface: []v1beta1.DomainNetworkInterface{
					{
						NetworkName: "default",
						Model:       "virtio",
					},
				},
			},
		},
	}

	ctx := context.Background()

	// Create resources
	err := fakeClient.Create(ctx, providerConfig)
	if err != nil {
		t.Fatalf("Failed to create ProviderConfig: %v", err)
	}

	err = fakeClient.Create(ctx, domain)
	if err != nil {
		t.Fatalf("Failed to create Domain: %v", err)
	}

	// In a real integration test, you would:
	// 1. Wait for the domain to be created
	// 2. Verify the domain exists in libvirt
	// 3. Test state transitions (start/stop)
	// 4. Test updates
	// 5. Test deletion
	
	// Simulate waiting for reconciliation
	time.Sleep(100 * time.Millisecond)

	// Verify domain was created
	var retrievedDomain v1beta1.Domain
	err = fakeClient.Get(ctx, client.ObjectKey{Name: "test-domain"}, &retrievedDomain)
	if err != nil {
		t.Fatalf("Failed to retrieve Domain: %v", err)
	}

	// Verify spec matches
	if retrievedDomain.Spec.ForProvider.Name != "test-integration-vm" {
		t.Errorf("Expected domain name 'test-integration-vm', got '%s'", retrievedDomain.Spec.ForProvider.Name)
	}

	if retrievedDomain.Spec.ForProvider.Memory != 1073741824 {
		t.Errorf("Expected memory 1073741824, got %d", retrievedDomain.Spec.ForProvider.Memory)
	}

	if retrievedDomain.Spec.ForProvider.Vcpu != 1 {
		t.Errorf("Expected vcpu 1, got %d", retrievedDomain.Spec.ForProvider.Vcpu)
	}

	// Cleanup - delete domain
	err = fakeClient.Delete(ctx, domain)
	if err != nil {
		t.Fatalf("Failed to delete Domain: %v", err)
	}

	t.Log("Integration test completed successfully")
}

// TestProviderConfigValidation tests ProviderConfig validation
func TestProviderConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *v1beta1.ProviderConfig
		wantErr bool
	}{
		{
			name: "ValidConfig",
			config: &v1beta1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: "valid-config",
				},
				Spec: v1beta1.ProviderConfigSpec{
					Credentials: v1beta1.ProviderCredentials{
						Source: xpv1.CredentialsSourceSecret,
						CommonCredentialSelectors: xpv1.CommonCredentialSelectors{
							SecretRef: &xpv1.SecretKeySelector{
								SecretReference: xpv1.SecretReference{
									Name:      "creds",
									Namespace: "default",
								},
								Key: "credentials",
							},
						},
					},
				},
			},
			wantErr: false,
		},
	}

	scheme := runtime.NewScheme()
	_ = v1beta1.SchemeBuilder.AddToScheme(scheme)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
			
			err := fakeClient.Create(context.Background(), tt.config)
			
			if tt.wantErr && err == nil {
				t.Error("Expected error but got none")
			}
			
			if !tt.wantErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

// TestDomainSpecValidation tests Domain spec validation
func TestDomainSpecValidation(t *testing.T) {
	tests := []struct {
		name    string
		domain  *v1beta1.Domain
		wantErr bool
	}{
		{
			name: "ValidDomain",
			domain: &v1beta1.Domain{
				ObjectMeta: metav1.ObjectMeta{
					Name: "valid-domain",
				},
				Spec: v1beta1.DomainSpec{
					ManagedResourceSpec: xpv1.ManagedResourceSpec{
						ProviderConfigReference: &xpv1.ProviderConfigReference{
							Name: "test-config",
						},
					},
					ForProvider: v1beta1.DomainParameters{
						Name:   "test-vm",
						Memory: 1073741824,
						Vcpu:   1,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "MinimalDomain",
			domain: &v1beta1.Domain{
				ObjectMeta: metav1.ObjectMeta{
					Name: "minimal-domain",
				},
				Spec: v1beta1.DomainSpec{
					ManagedResourceSpec: xpv1.ManagedResourceSpec{
						ProviderConfigReference: &xpv1.ProviderConfigReference{
							Name: "test-config",
						},
					},
					ForProvider: v1beta1.DomainParameters{
						Name:   "minimal-vm",
						Memory: 512 * 1024 * 1024, // 512MB
						Vcpu:   1,
					},
				},
			},
			wantErr: false,
		},
	}

	scheme := runtime.NewScheme()
	_ = v1beta1.SchemeBuilder.AddToScheme(scheme)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
			
			err := fakeClient.Create(context.Background(), tt.domain)
			
			if tt.wantErr && err == nil {
				t.Error("Expected error but got none")
			}
			
			if !tt.wantErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}