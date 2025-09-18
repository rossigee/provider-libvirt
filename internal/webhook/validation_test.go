package webhook

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
	"github.com/rossigee/provider-libvirt/apis/v1alpha1"
)

func TestValidateResourceName(t *testing.T) {
	cases := map[string]struct {
		name     string
		wantErrs int
	}{
		"ValidName": {
			name:     "test-resource",
			wantErrs: 0,
		},
		"EmptyName": {
			name:     "",
			wantErrs: 1,
		},
		"TooLongName": {
			name:     "test-" + string(make([]byte, 250)), // 255+ chars
			wantErrs: 2, // Both length and regex validation fail
		},
		"InvalidChars": {
			name:     "Test_Resource!",
			wantErrs: 1,
		},
		"ValidSingleChar": {
			name:     "a",
			wantErrs: 0,
		},
		"ValidWithNumbers": {
			name:     "test-123",
			wantErrs: 0,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			errs := validateResourceName(tc.name, field.NewPath("metadata", "name"))
			if len(errs) != tc.wantErrs {
				t.Errorf("validateResourceName() = %d errors, want %d", len(errs), tc.wantErrs)
			}
		})
	}
}

func TestValidateMemorySize(t *testing.T) {
	cases := map[string]struct {
		memory   int64
		wantErrs int
	}{
		"ValidMemory": {
			memory:   512 * 1024 * 1024, // 512MB
			wantErrs: 0,
		},
		"ZeroMemory": {
			memory:   0,
			wantErrs: 2, // Both greater than 0 and minimum 128MB fail
		},
		"NegativeMemory": {
			memory:   -1,
			wantErrs: 2, // Both greater than 0 and minimum 128MB fail
		},
		"TooSmallMemory": {
			memory:   64 * 1024 * 1024, // 64MB < 128MB minimum
			wantErrs: 1,
		},
		"TooLargeMemory": {
			memory:   2 * 1024 * 1024 * 1024 * 1024, // 2TB > 1TB maximum
			wantErrs: 1,
		},
		"MinimumMemory": {
			memory:   128 * 1024 * 1024, // 128MB exactly
			wantErrs: 0,
		},
		"MaximumMemory": {
			memory:   1024 * 1024 * 1024 * 1024, // 1TB exactly
			wantErrs: 0,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			errs := validateMemorySize(tc.memory, field.NewPath("spec", "memory"))
			if len(errs) != tc.wantErrs {
				t.Errorf("validateMemorySize() = %d errors, want %d", len(errs), tc.wantErrs)
			}
		})
	}
}

func TestValidateVcpuCount(t *testing.T) {
	cases := map[string]struct {
		vcpu     int
		wantErrs int
	}{
		"ValidVcpu": {
			vcpu:     4,
			wantErrs: 0,
		},
		"ZeroVcpu": {
			vcpu:     0,
			wantErrs: 1,
		},
		"NegativeVcpu": {
			vcpu:     -1,
			wantErrs: 1,
		},
		"TooManyVcpu": {
			vcpu:     512,
			wantErrs: 1,
		},
		"MinimumVcpu": {
			vcpu:     1,
			wantErrs: 0,
		},
		"MaximumVcpu": {
			vcpu:     256,
			wantErrs: 0,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			errs := validateVcpuCount(tc.vcpu, field.NewPath("spec", "vcpu"))
			if len(errs) != tc.wantErrs {
				t.Errorf("validateVcpuCount() = %d errors, want %d", len(errs), tc.wantErrs)
			}
		})
	}
}

func TestValidateIPAddress(t *testing.T) {
	cases := map[string]struct {
		ip       string
		wantErrs int
	}{
		"ValidIPv4": {
			ip:       "192.168.1.1",
			wantErrs: 0,
		},
		"ValidIPv6": {
			ip:       "2001:db8::1",
			wantErrs: 0,
		},
		"EmptyIP": {
			ip:       "",
			wantErrs: 0, // Empty is optional
		},
		"InvalidIP": {
			ip:       "256.256.256.256",
			wantErrs: 1,
		},
		"InvalidFormat": {
			ip:       "not-an-ip",
			wantErrs: 1,
		},
		"Localhost": {
			ip:       "127.0.0.1",
			wantErrs: 0,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			errs := validateIPAddress(tc.ip, field.NewPath("spec", "ip"))
			if len(errs) != tc.wantErrs {
				t.Errorf("validateIPAddress() = %d errors, want %d", len(errs), tc.wantErrs)
			}
		})
	}
}

func TestValidatePortRange(t *testing.T) {
	cases := map[string]struct {
		portRange string
		wantErrs  int
	}{
		"ValidSinglePort": {
			portRange: "80",
			wantErrs:  0,
		},
		"ValidPortRange": {
			portRange: "8000-8010",
			wantErrs:  0,
		},
		"EmptyPortRange": {
			portRange: "",
			wantErrs:  0, // Empty is optional
		},
		"InvalidPortNumber": {
			portRange: "abc",
			wantErrs:  1,
		},
		"PortTooLow": {
			portRange: "0",
			wantErrs:  1,
		},
		"PortTooHigh": {
			portRange: "70000",
			wantErrs:  1,
		},
		"InvalidRange": {
			portRange: "9000-8000", // start > end
			wantErrs:  1,
		},
		"TooManyDashes": {
			portRange: "8000-8010-8020",
			wantErrs:  1,
		},
		"ValidEdgePorts": {
			portRange: "1-65535",
			wantErrs:  0,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			errs := validatePortRange(tc.portRange, field.NewPath("spec", "ports"))
			if len(errs) != tc.wantErrs {
				t.Errorf("validatePortRange() = %d errors, want %d", len(errs), tc.wantErrs)
			}
		})
	}
}

func TestValidateMACAddress(t *testing.T) {
	cases := map[string]struct {
		mac      string
		wantErrs int
	}{
		"ValidMAC": {
			mac:      "aa:bb:cc:dd:ee:ff",
			wantErrs: 0,
		},
		"ValidMACUppercase": {
			mac:      "AA:BB:CC:DD:EE:FF",
			wantErrs: 0,
		},
		"ValidMACMixed": {
			mac:      "Aa:Bb:Cc:Dd:Ee:Ff",
			wantErrs: 0,
		},
		"EmptyMAC": {
			mac:      "",
			wantErrs: 0, // Empty is optional
		},
		"InvalidMACFormat": {
			mac:      "aa-bb-cc-dd-ee",
			wantErrs: 1,
		},
		"InvalidMACChars": {
			mac:      "gg:hh:ii:jj:kk:ll",
			wantErrs: 1,
		},
		"MACTooShort": {
			mac:      "aa:bb:cc:dd:ee",
			wantErrs: 1,
		},
		"MACTooLong": {
			mac:      "aa:bb:cc:dd:ee:ff:gg",
			wantErrs: 1,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			errs := validateMACAddress(tc.mac, field.NewPath("spec", "mac"))
			if len(errs) != tc.wantErrs {
				t.Errorf("validateMACAddress() = %d errors, want %d", len(errs), tc.wantErrs)
			}
		})
	}
}

func TestValidateFilePath(t *testing.T) {
	cases := map[string]struct {
		path     string
		wantErrs int
	}{
		"ValidPath": {
			path:     "/var/lib/libvirt/images/test.qcow2",
			wantErrs: 0,
		},
		"EmptyPath": {
			path:     "",
			wantErrs: 1,
		},
		"RelativePath": {
			path:     "relative/path",
			wantErrs: 1,
		},
		"PathWithParentDir": {
			path:     "/var/lib/../etc/passwd",
			wantErrs: 2, // Both parent dir and dangerous pattern validation fail
		},
		"PathWithCurrentDir": {
			path:     "/var/lib/./images",
			wantErrs: 1,
		},
		"PathWithDoubleSlash": {
			path:     "/var//lib/images",
			wantErrs: 1,
		},
		"RootPath": {
			path:     "/",
			wantErrs: 0,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			errs := validateFilePath(tc.path, field.NewPath("spec", "path"))
			if len(errs) != tc.wantErrs {
				t.Errorf("validateFilePath() = %d errors, want %d", len(errs), tc.wantErrs)
			}
		})
	}
}

// Test webhook interface implementations
func TestValidationWebhookMethods(t *testing.T) {
	webhook := &ValidationWebhook{
		Client: &test.MockClient{},
	}

	domain := &v1alpha1.Domain{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-domain",
			Namespace: "default",
		},
		Spec: v1alpha1.DomainSpec{
			ForProvider: v1alpha1.DomainParameters{
				Name:   "test-domain",
				Memory: 512 * 1024 * 1024, // 512MB
				Vcpu:   2,
			},
		},
	}

	// Test ValidateCreate
	warnings, err := webhook.ValidateCreate(context.Background(), domain)
	if err != nil {
		t.Errorf("ValidateCreate() failed: %v", err)
	}
	_ = warnings // Ignore warnings for test

	// Test ValidateUpdate
	warnings, err = webhook.ValidateUpdate(context.Background(), domain, domain)
	if err != nil {
		t.Errorf("ValidateUpdate() failed: %v", err)
	}
	_ = warnings // Ignore warnings for test

	// Test ValidateDelete
	warnings, err = webhook.ValidateDelete(context.Background(), domain)
	if err != nil {
		t.Errorf("ValidateDelete() failed: %v", err)
	}
	_ = warnings // Ignore warnings for test
}

func TestValidateObjectSwitch(t *testing.T) {
	webhook := &ValidationWebhook{
		Client: &test.MockClient{},
	}

	// Test with different resource types
	testCases := []struct {
		name   string
		obj    runtime.Object
		oldObj runtime.Object
	}{
		{
			name: "Domain",
			obj: &v1alpha1.Domain{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
			},
		},
		{
			name: "Volume",
			obj: &v1alpha1.Volume{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
			},
		},
		{
			name: "Network",
			obj: &v1alpha1.Network{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
			},
		},
		{
			name: "StoragePool",
			obj: &v1alpha1.StoragePool{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
			},
		},
		{
			name: "Secret",
			obj: &v1alpha1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			warnings, err := webhook.validateObject(context.Background(), tc.obj, tc.oldObj)
			// We expect validation to pass or fail gracefully, not crash
			_ = warnings // Ignore warnings for test
			_ = err // Allow validation errors, just ensure no panic
		})
	}
}