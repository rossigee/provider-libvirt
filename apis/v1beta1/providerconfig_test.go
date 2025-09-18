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

func TestProviderConfigSpec(t *testing.T) {
	// Test basic ProviderConfig spec creation
	pc := &ProviderConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "libvirt.m.crossplane.io/v1beta1",
			Kind:       "ProviderConfig",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-provider-config",
			Namespace: "test-ns",
		},
		Spec: ProviderConfigSpec{
			Credentials: ProviderCredentials{
				Source: xpv1.CredentialsSourceSecret,
				CommonCredentialSelectors: xpv1.CommonCredentialSelectors{
					SecretRef: &xpv1.SecretKeySelector{
						SecretReference: xpv1.SecretReference{
							Name:      "libvirt-creds",
							Namespace: "test-ns",
						},
						Key: "credentials",
					},
				},
			},
		},
	}

	// Test that the provider config has correct GVK
	gvk := pc.GetObjectKind().GroupVersionKind()
	if gvk.Group != "libvirt.m.crossplane.io" {
		t.Errorf("Expected group 'libvirt.m.crossplane.io', got '%s'", gvk.Group)
	}
	if gvk.Version != "v1beta1" {
		t.Errorf("Expected version 'v1beta1', got '%s'", gvk.Version)
	}
	if gvk.Kind != "ProviderConfig" {
		t.Errorf("Expected kind 'ProviderConfig', got '%s'", gvk.Kind)
	}

	// Test credentials configuration
	if pc.Spec.Credentials.Source != xpv1.CredentialsSourceSecret {
		t.Errorf("Expected credentials source 'Secret', got '%s'", pc.Spec.Credentials.Source)
	}
	if pc.Spec.Credentials.SecretRef == nil {
		t.Error("Expected secret ref to be set")
	}
	if pc.Spec.Credentials.SecretRef.Name != "libvirt-creds" {
		t.Errorf("Expected secret name 'libvirt-creds', got '%s'", pc.Spec.Credentials.SecretRef.Name)
	}
	if pc.Spec.Credentials.SecretRef.Key != "credentials" {
		t.Errorf("Expected secret key 'credentials', got '%s'", pc.Spec.Credentials.SecretRef.Key)
	}
}

func TestProviderConfigConversion(t *testing.T) {
	// Test that ProviderConfig can be converted to/from runtime.Object
	pc := &ProviderConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-provider-config",
			Namespace: "test-ns",
		},
		Spec: ProviderConfigSpec{
			Credentials: ProviderCredentials{
				Source: xpv1.CredentialsSourceSecret,
				CommonCredentialSelectors: xpv1.CommonCredentialSelectors{
					SecretRef: &xpv1.SecretKeySelector{
						SecretReference: xpv1.SecretReference{
							Name:      "libvirt-creds",
							Namespace: "test-ns",
						},
						Key: "credentials",
					},
				},
			},
		},
	}

	// Convert to runtime.Object and back
	var obj runtime.Object = pc
	converted, ok := obj.(*ProviderConfig)
	if !ok {
		t.Error("Failed to convert ProviderConfig to runtime.Object and back")
	}

	// Compare original and converted
	if diff := cmp.Diff(pc, converted); diff != "" {
		t.Errorf("ProviderConfig conversion mismatch (-want +got):\n%s", diff)
	}
}

func TestProviderConfigInjectedIdentity(t *testing.T) {
	pc := &ProviderConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-injected-config",
			Namespace: "test-ns",
		},
		Spec: ProviderConfigSpec{
			Credentials: ProviderCredentials{
				Source: xpv1.CredentialsSourceInjectedIdentity,
			},
		},
	}

	// Test injected identity credentials
	if pc.Spec.Credentials.Source != xpv1.CredentialsSourceInjectedIdentity {
		t.Errorf("Expected credentials source 'InjectedIdentity', got '%s'", pc.Spec.Credentials.Source)
	}

	// Verify namespace-scoped resource
	if pc.Namespace != "test-ns" {
		t.Errorf("Expected namespace 'test-ns', got '%s'", pc.Namespace)
	}
}