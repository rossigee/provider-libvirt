/*
Copyright 2025 Ross Golder
*/

package clients

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"

	"github.com/rossigee/provider-libvirt/apis/v1alpha1"
)

func TestGetLibvirtClient(t *testing.T) {
	type args struct {
		ctx  context.Context
		kube client.Client
		mg   resource.Managed
	}

	tests := []struct {
		name    string
		args    args
		want    *LibvirtClient
		wantErr bool
		errMsg  string
	}{
		{
			name: "NoProviderConfigRef",
			args: args{
				ctx:  context.Background(),
				kube: fake.NewClientBuilder().Build(),
				mg: &v1alpha1.Domain{
					Spec: v1alpha1.DomainSpec{
						ResourceSpec: xpv1.ResourceSpec{
							// No ProviderConfigReference
						},
					},
				},
			},
			wantErr: true,
			errMsg:  errNoProviderConfig,
		},
		{
			name: "ProviderConfigNotFound",
			args: args{
				ctx:  context.Background(),
				kube: fake.NewClientBuilder().Build(),
				mg: &v1alpha1.Domain{
					Spec: v1alpha1.DomainSpec{
						ResourceSpec: xpv1.ResourceSpec{
							ProviderConfigReference: &xpv1.Reference{
								Name: "test-provider-config",
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  errGetProviderConfig,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetLibvirtClient(tt.args.ctx, tt.args.kube, tt.args.mg)
			
			if tt.wantErr {
				if err == nil {
					t.Errorf("GetLibvirtClient() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if tt.errMsg != "" && !errors.Is(err, errors.New(tt.errMsg)) && 
				   !contains(err.Error(), tt.errMsg) {
					t.Errorf("GetLibvirtClient() error = %v, want error containing %v", err, tt.errMsg)
				}
				return
			}
			
			if err != nil {
				t.Errorf("GetLibvirtClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if got == nil {
				t.Errorf("GetLibvirtClient() = nil, want non-nil client")
			}
		})
	}
}

func TestGetLibvirtClient_WithValidConfig(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1alpha1.SchemeBuilder.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	// Create test credentials
	creds := map[string]string{
		keyURI: "qemu+tcp://localhost:16509/system",
	}
	credsJSON, _ := json.Marshal(creds)

	// Create provider config
	pc := &v1alpha1.ProviderConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-provider-config",
			Namespace: "crossplane-system",
		},
		Spec: v1alpha1.ProviderConfigSpec{
			Credentials: v1alpha1.ProviderCredentials{
				Source: xpv1.CredentialsSourceSecret,
				CommonCredentialSelectors: xpv1.CommonCredentialSelectors{
					SecretRef: &xpv1.SecretKeySelector{
						SecretReference: xpv1.SecretReference{
							Name:      "test-secret",
							Namespace: "test-namespace",
						},
						Key: "credentials",
					},
				},
			},
		},
	}

	// Create secret with credentials
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "test-namespace",
		},
		Data: map[string][]byte{
			"credentials": credsJSON,
		},
	}

	kube := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(pc, secret).
		Build()

	mg := &v1alpha1.Domain{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-domain",
		},
		Spec: v1alpha1.DomainSpec{
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: &xpv1.Reference{
					Name: "test-provider-config",
				},
			},
		},
	}

	// Note: This test will fail connecting to libvirt since we don't have a mock server
	// In real scenarios, you'd want to mock the network connection
	_, err := GetLibvirtClient(context.Background(), kube, mg)
	
	// We expect a connection error since we're not mocking the network
	if err == nil {
		t.Error("Expected connection error since libvirt server is not running")
	}
	
	// The error should be about connection, not about missing config
	if contains(err.Error(), errNoProviderConfig) || 
	   contains(err.Error(), errGetProviderConfig) ||
	   contains(err.Error(), errExtractCredentials) {
		t.Errorf("Unexpected error type: %v", err)
	}
}

func TestConnectToLibvirt_URIParsing(t *testing.T) {
	tests := []struct {
		name    string
		uri     string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "InvalidURI",
			uri:     "not-a-valid-uri",
			wantErr: true,
			errMsg:  "unsupported libvirt URI scheme",
		},
		{
			name:    "UnsupportedScheme",
			uri:     "http://localhost/test",
			wantErr: true,
			errMsg:  "unsupported libvirt URI scheme",
		},
		{
			name: "ValidTCPURI",
			uri:  "qemu+tcp://localhost:16509/system",
			// This will fail with connection error, but URI parsing should succeed
			wantErr: true, // Connection will fail, but that's expected
		},
		{
			name: "ValidUnixURI", 
			uri:  "qemu+unix:///system",
			// This will fail with connection error since socket doesn't exist
			wantErr: true, // Connection will fail, but that's expected
		},
		{
			name: "ValidSSHURI",
			uri:  "qemu+ssh://user@host/system",
			// This will fail with connection error, but URI parsing should succeed
			wantErr: true, // Connection will fail, but that's expected
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := connectToLibvirt(tt.uri)
			
			if !tt.wantErr && err != nil {
				t.Errorf("connectToLibvirt() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if tt.wantErr && err == nil {
				t.Errorf("connectToLibvirt() error = nil, wantErr %v", tt.wantErr)
				return
			}
			
			if tt.errMsg != "" && err != nil && !contains(err.Error(), tt.errMsg) {
				t.Errorf("connectToLibvirt() error = %v, want error containing %v", err, tt.errMsg)
			}
		})
	}
}

func TestLibvirtClient_Close(t *testing.T) {
	// Test with nil client
	client := &LibvirtClient{}
	err := client.Close()
	if err != nil {
		t.Errorf("Close() with nil client should not error, got: %v", err)
	}

	// Test with mock connection
	mockConn := &mockConn{}
	client = &LibvirtClient{
		conn: mockConn,
	}
	
	err = client.Close()
	if err != nil {
		t.Errorf("Close() with mock connection failed: %v", err)
	}
	
	if !mockConn.closed {
		t.Error("Close() should have closed the connection")
	}
}

// Helper functions and mocks

func contains(s, substr string) bool {
	return len(s) >= len(substr) && 
		   (s == substr || (len(s) > len(substr) && 
		   	containsSubstring(s, substr)))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// mockConn implements net.Conn for testing
type mockConn struct {
	closed bool
}

func (m *mockConn) Read(b []byte) (n int, err error) {
	return 0, errors.New("mock read error")
}

func (m *mockConn) Write(b []byte) (n int, err error) {
	return len(b), nil
}

func (m *mockConn) Close() error {
	m.closed = true
	return nil
}

func (m *mockConn) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 12345}
}

func (m *mockConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 16509}
}

func (m *mockConn) SetDeadline(deadline time.Time) error {
	return nil
}

func (m *mockConn) SetReadDeadline(deadline time.Time) error {
	return nil
}

func (m *mockConn) SetWriteDeadline(deadline time.Time) error {
	return nil
}

/*
func TestExtractCredentials(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1alpha1.SchemeBuilder.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	tests := []struct {
		name    string
		setup   func() (client.Client, *v1alpha1.Domain)
		wantErr bool
		errMsg  string
	}{
		{
			name: "MissingURIInCredentials",
			setup: func() (client.Client, *v1alpha1.Domain) {
				// Create credentials without URI
				creds := map[string]string{
					"username": "admin",
					"password": "secret",
				}
				credsJSON, _ := json.Marshal(creds)

				// Create provider config
				pc := &v1alpha1.ProviderConfig{
					Spec: v1alpha1.ProviderConfigSpec{
						Credentials: v1alpha1.ProviderCredentials{
							Source: xpv1.CredentialsSourceSecret,
							CommonCredentialSelectors: xpv1.CommonCredentialSelectors{
								SecretRef: &xpv1.SecretKeySelector{
									SecretReference: xpv1.SecretReference{
										Name:      "test-secret",
										Namespace: "test-namespace",
									},
									Key: "credentials",
								},
							},
						},
				},
			}
				pc.SetName("test-provider-config")

				// Create secret with credentials
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-secret",
						Namespace: "test-namespace",
					},
					Data: map[string][]byte{
						"credentials": credsJSON,
					},
				}

				kube := fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(pc, secret).
					Build()

				mg := &v1alpha1.Domain{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-domain",
						Namespace: "test-namespace",
					},
					Spec: v1alpha1.DomainSpec{
						ResourceSpec: xpv1.ResourceSpec{
							ProviderConfigReference: &xpv1.Reference{
								Name: "test-provider-config",
							},
						},
					},
				}

				return kube, mg
			},
			wantErr: true,
			errMsg:  "libvirt URI not found in credentials",
		},
		{
			name: "InvalidCredentialsJSON",
			setup: func() (client.Client, *v1alpha1.Domain) {
				// Create provider config
				pc := &v1alpha1.ProviderConfig{
					Spec: v1alpha1.ProviderConfigSpec{
						Credentials: v1alpha1.ProviderCredentials{
							Source: xpv1.CredentialsSourceSecret,
							CommonCredentialSelectors: xpv1.CommonCredentialSelectors{
								SecretRef: &xpv1.SecretKeySelector{
									SecretReference: xpv1.SecretReference{
										Name:      "test-secret",
										Namespace: "test-namespace",
									},
									Key: "credentials",
								},
							},
						},
				},
			}
				pc.SetName("test-provider-config")

				// Create secret with invalid JSON
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-secret",
						Namespace: "test-namespace",
					},
					Data: map[string][]byte{
						"credentials": []byte("invalid-json"),
					},
				}

				kube := fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(pc, secret).
					Build()

				mg := &v1alpha1.Domain{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-domain",
						Namespace: "test-namespace",
					},
					Spec: v1alpha1.DomainSpec{
						ResourceSpec: xpv1.ResourceSpec{
							ProviderConfigReference: &xpv1.Reference{
								Name: "test-provider-config",
							},
						},
					},
				}

				return kube, mg
			},
			wantErr: true,
			errMsg:  errUnmarshalCredentials,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kube, mg := tt.setup()
			_, err := GetLibvirtClient(context.Background(), kube, mg)
			
			if tt.wantErr {
				if err == nil {
					t.Errorf("GetLibvirtClient() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("GetLibvirtClient() error = %v, want error containing %v", err, tt.errMsg)
				}
				return
			}
			
			if err != nil {
				t.Errorf("GetLibvirtClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
*/

func TestConnectToLibvirt_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		uri     string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "EmptyURI",
			uri:     "",
			wantErr: true,
			errMsg:  "unsupported libvirt URI scheme",
		},
		{
			name:    "TCPWithDefaultPort",
			uri:     "qemu+tcp://localhost/system",
			wantErr: true, // Connection will fail, but URI parsing should succeed
		},
		{
			name:    "TCPWithCustomPort",
			uri:     "qemu+tcp://localhost:12345/system",
			wantErr: true, // Connection will fail, but URI parsing should succeed
		},
		{
			name:    "SSHWithDefaultPort",
			uri:     "qemu+ssh://user@hostname/system",
			wantErr: true, // Connection will fail, but URI parsing should succeed
		},
		{
			name:    "SSHWithCustomPort",
			uri:     "qemu+ssh://user@hostname:2222/system",
			wantErr: true, // Connection will fail, but URI parsing should succeed
		},
		{
			name:    "UnixSocketDefault",
			uri:     "qemu+unix:///system",
			wantErr: true, // Connection will fail since socket doesn't exist
		},
		{
			name:    "UnixSocketCustomPath",
			uri:     "qemu+unix:///var/run/libvirt/custom-sock",
			wantErr: true, // Connection will fail since socket doesn't exist
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := connectToLibvirt(tt.uri)
			
			if !tt.wantErr && err != nil {
				t.Errorf("connectToLibvirt() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if tt.wantErr && err == nil {
				t.Errorf("connectToLibvirt() error = nil, wantErr %v", tt.wantErr)
				return
			}
			
			if tt.errMsg != "" && err != nil && !contains(err.Error(), tt.errMsg) {
				t.Errorf("connectToLibvirt() error = %v, want error containing %v", err, tt.errMsg)
			}
		})
	}
}

func TestLibvirtClient_CloseWithError(t *testing.T) {
	// Test with mock connection that returns error on close
	mockConn := &mockConnWithError{}
	client := &LibvirtClient{
		conn: mockConn,
	}
	
	err := client.Close()
	if err == nil {
		t.Error("Close() should return error from connection close")
	}
	
	if !mockConn.closed {
		t.Error("Close() should have attempted to close the connection")
	}
}

// Additional mock types for enhanced testing

// mockConnWithError implements net.Conn and returns error on close
type mockConnWithError struct {
	closed bool
}

func (m *mockConnWithError) Read(b []byte) (n int, err error) {
	return 0, errors.New("mock read error")
}

func (m *mockConnWithError) Write(b []byte) (n int, err error) {
	return len(b), nil
}

func (m *mockConnWithError) Close() error {
	m.closed = true
	return errors.New("mock close error")
}

func (m *mockConnWithError) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 12345}
}

func (m *mockConnWithError) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 16509}
}

func (m *mockConnWithError) SetDeadline(deadline time.Time) error {
	return nil
}

func (m *mockConnWithError) SetReadDeadline(deadline time.Time) error {
	return nil
}

func (m *mockConnWithError) SetWriteDeadline(deadline time.Time) error {
	return nil
}