/*
Copyright 2025 Ross Golder
*/

package clients

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/digitalocean/go-libvirt"
	"github.com/digitalocean/go-libvirt/socket/dialers"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"

	"github.com/rossigee/provider-libvirt/apis/v1alpha1"
)

// Note: The go-libvirt library automatically includes storage operations
// as part of the main Libvirt interface. We don't need a separate interface.

const (
	keyURI = "uri"

	// Default controller settings
	DefaultPollInterval             = 60 * time.Second
	DefaultMaxConcurrentReconciles  = 10

	// error messages
	errNoProviderConfig     = "no providerConfigRef provided"
	errGetProviderConfig    = "cannot get referenced ProviderConfig"
	errTrackUsage           = "cannot track ProviderConfig usage"
	errExtractCredentials   = "cannot extract credentials"
	errUnmarshalCredentials = "cannot unmarshal libvirt credentials as JSON"
	errParseURI             = "cannot parse libvirt URI"
	errConnectLibvirt       = "cannot connect to libvirt"
)

// LibvirtCredentials represents libvirt connection credentials
type LibvirtCredentials map[string]string

// LibvirtClient wraps go-libvirt connection with additional metadata
type LibvirtClient struct {
	*libvirt.Libvirt
	conn net.Conn
	uri  string
}

// Close closes the libvirt connection
func (c *LibvirtClient) Close() error {
	if c.Libvirt != nil {
		if err := c.Disconnect(); err != nil {
			return err
		}
	}
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// GetLibvirtClient establishes a connection to libvirt using provider configuration
func GetLibvirtClient(ctx context.Context, kube client.Client, mg resource.Managed) (*LibvirtClient, error) {
	// Get provider config reference from the managed resource's ResourceSpec
	var configRef *xpv1.Reference

	// Type assert to extract the ProviderConfigReference from the managed resource
	switch mr := mg.(type) {
	case interface{ GetProviderConfigReference() *xpv1.Reference }:
		configRef = mr.GetProviderConfigReference()
	default:
		return nil, errors.New(errGetProviderConfig)
	}

	if configRef == nil {
		return nil, errors.New(errNoProviderConfig)
	}

	// First try to find the ProviderConfig in the managed resource's namespace
	pc := &v1alpha1.ProviderConfig{}
	pcNamespace := mg.GetNamespace()
	if pcNamespace == "" {
		pcNamespace = "crossplane-system" // Default for cluster-scoped resources
	}

	err := kube.Get(ctx, types.NamespacedName{Name: configRef.Name, Namespace: pcNamespace}, pc)
	if err != nil {
		// If not found, try crossplane-system namespace first (most common location)
		err = kube.Get(ctx, types.NamespacedName{Name: configRef.Name, Namespace: "crossplane-system"}, pc)
		if err != nil {
			// Finally try default namespace for backward compatibility
			err = kube.Get(ctx, types.NamespacedName{Name: configRef.Name, Namespace: "default"}, pc)
			if err != nil {
				return nil, errors.Wrap(err, errGetProviderConfig)
			}
		}
	}

	// ProviderConfigUsage tracking is handled by the controller framework
	// via c.usage.Track(ctx, mg) call in the controller's Connect method

	// Extract credentials
	data, err := resource.CommonCredentialExtractor(ctx, pc.Spec.Credentials.Source, kube, pc.Spec.Credentials.CommonCredentialSelectors)
	if err != nil {
		return nil, errors.Wrap(err, errExtractCredentials)
	}

	creds := map[string]string{}
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, errors.Wrap(err, errUnmarshalCredentials)
	}

	uri, ok := creds[keyURI]
	if !ok {
		return nil, errors.New("libvirt URI not found in credentials")
	}

	// Parse and connect to libvirt
	conn, err := connectToLibvirt(uri)
	if err != nil {
		return nil, errors.Wrap(err, errConnectLibvirt)
	}

	libvirtClient := libvirt.NewWithDialer(dialers.NewAlreadyConnected(conn))

	// Connect to libvirt daemon
	if err := libvirtClient.Connect(); err != nil {
		_ = conn.Close() // Ignore error during cleanup
		return nil, errors.Wrap(err, "cannot connect to libvirt daemon")
	}

	// Perform authentication if required
	authTypes, err := libvirtClient.AuthList()
	if err != nil {
		_ = conn.Close() // Ignore error during cleanup
		return nil, errors.Wrap(err, "cannot get authentication types")
	}

	// If authentication is required, perform it
	if len(authTypes) > 0 {
		// For TLS connections, usually no additional auth is needed beyond certificates
		// but some setups might require Polkit or SASL
		for _, authType := range authTypes {
			if authType == 2 { // AuthTypePolkit (usually for local connections)
				_, err := libvirtClient.AuthPolkit()
				if err != nil {
					_ = conn.Close() // Ignore error during cleanup
					return nil, errors.Wrap(err, "polkit authentication failed")
				}
				break
			}
			// For TLS, usually just the certificate validation is sufficient
			// and no additional authentication procedure is needed
		}
	}

	return &LibvirtClient{
		Libvirt: libvirtClient,
		conn:    conn,
		uri:     uri,
	}, nil
}

// connectToLibvirt establishes connection based on URI scheme
func connectToLibvirt(uri string) (net.Conn, error) {
	parsedURI, err := url.Parse(uri)
	if err != nil {
		return nil, errors.Wrap(err, errParseURI)
	}

	switch parsedURI.Scheme {
	case "qemu+ssh":
		// For SSH connections, we need to handle the SSH tunnel
		// This is a simplified implementation - in production you'd want
		// proper SSH key management and connection pooling
		host := parsedURI.Host
		if !strings.Contains(host, ":") {
			host += ":22" // default SSH port
		}
		
		// For now, we'll use a simple TCP connection to the libvirt daemon
		// In production, this would go through SSH tunnel
		return net.Dial("tcp", host)
		
	case "qemu+tcp":
		// Direct TCP connection
		host := parsedURI.Host
		if !strings.Contains(host, ":") {
			host += ":16509" // default libvirt TCP port
		}
		return net.Dial("tcp", host)
		
	case "qemu+unix":
		// Unix socket connection
		path := parsedURI.Path
		if path == "" {
			path = "/var/run/libvirt/libvirt-sock"
		}
		return net.Dial("unix", path)

	case "qemu+tls":
		// TLS connection with client certificate authentication
		hostname := parsedURI.Hostname()
		host := parsedURI.Host
		if !strings.Contains(host, ":") {
			host += ":16514" // default libvirt TLS port
		}

		// Load client certificates for libvirt TLS authentication
		tlsConfig, err := loadLibvirtTLSConfig()
		if err != nil {
			return nil, errors.Wrap(err, "cannot load TLS certificates")
		}

		// Set the server name from the URI hostname for certificate validation
		tlsConfig.ServerName = hostname

		return tls.Dial("tcp", host, tlsConfig)

	default:
		return nil, fmt.Errorf("unsupported libvirt URI scheme: %s", parsedURI.Scheme)
	}
}

// NewLibvirtClient creates a new libvirt client from credentials
func NewLibvirtClient(creds LibvirtCredentials) (*LibvirtClient, error) {
	uri, ok := creds[keyURI]
	if !ok {
		return nil, errors.New("libvirt URI not found in credentials")
	}

	conn, err := connectToLibvirt(uri)
	if err != nil {
		return nil, errors.Wrap(err, errConnectLibvirt)
	}

	libvirtClient := libvirt.NewWithDialer(dialers.NewAlreadyConnected(conn))

	// Connect to libvirt daemon
	if err := libvirtClient.Connect(); err != nil {
		_ = conn.Close() // Ignore error during cleanup
		return nil, errors.Wrap(err, "cannot connect to libvirt daemon")
	}

	// Perform authentication if required
	authTypes, err := libvirtClient.AuthList()
	if err != nil {
		_ = conn.Close() // Ignore error during cleanup
		return nil, errors.Wrap(err, "cannot get authentication types")
	}

	// If authentication is required, perform it
	if len(authTypes) > 0 {
		// For TLS connections, usually no additional auth is needed beyond certificates
		// but some setups might require Polkit or SASL
		for _, authType := range authTypes {
			if authType == 2 { // AuthTypePolkit (usually for local connections)
				_, err := libvirtClient.AuthPolkit()
				if err != nil {
					_ = conn.Close() // Ignore error during cleanup
					return nil, errors.Wrap(err, "polkit authentication failed")
				}
				break
			}
			// For TLS, usually just the certificate validation is sufficient
			// and no additional authentication procedure is needed
		}
	}

	return &LibvirtClient{
		Libvirt: libvirtClient,
		conn:    conn,
		uri:     uri,
	}, nil
}

// GetCredentials extracts credentials from ProviderCredentials
func GetCredentials(ctx context.Context, kube client.Client, source xpv1.CredentialsSource, cs xpv1.CommonCredentialSelectors) (LibvirtCredentials, error) {
	data, err := resource.CommonCredentialExtractor(ctx, source, kube, cs)
	if err != nil {
		return nil, errors.Wrap(err, errExtractCredentials)
	}

	creds := LibvirtCredentials{}
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, errors.Wrap(err, errUnmarshalCredentials)
	}

	return creds, nil
}

// IsNotFound checks if an error indicates a resource was not found
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	// Check for common libvirt "not found" error patterns
	errStr := err.Error()
	return strings.Contains(errStr, "not found") ||
		strings.Contains(errStr, "does not exist") ||
		strings.Contains(errStr, "no such") ||
		strings.Contains(errStr, "Domain not found") ||
		strings.Contains(errStr, "Secret not found") ||
		strings.Contains(errStr, "Volume not found") ||
		strings.Contains(errStr, "Network not found") ||
		strings.Contains(errStr, "Pool not found")
}

// UUIDToString converts a libvirt UUID byte array to string format
func UUIDToString(uuid [16]byte) string {
	return fmt.Sprintf("%02x%02x%02x%02x-%02x%02x-%02x%02x-%02x%02x-%02x%02x%02x%02x%02x%02x",
		uuid[0], uuid[1], uuid[2], uuid[3],
		uuid[4], uuid[5],
		uuid[6], uuid[7],
		uuid[8], uuid[9],
		uuid[10], uuid[11], uuid[12], uuid[13], uuid[14], uuid[15])
}

// StringToUUID converts a string UUID to libvirt UUID byte array
func StringToUUID(uuidStr string) ([16]byte, error) {
	var uuid [16]byte
	
	// Remove hyphens from UUID string
	cleanUUID := strings.ReplaceAll(uuidStr, "-", "")
	
	if len(cleanUUID) != 32 {
		return uuid, fmt.Errorf("invalid UUID length: expected 32 characters, got %d", len(cleanUUID))
	}
	
	// Convert hex string to bytes
	for i := 0; i < 16; i++ {
		hexByte := cleanUUID[i*2 : i*2+2]
		val, err := fmt.Sscanf(hexByte, "%02x", &uuid[i])
		if err != nil || val != 1 {
			return uuid, fmt.Errorf("invalid hex in UUID: %s", hexByte)
		}
	}
	
	return uuid, nil
}

// loadLibvirtTLSConfig loads TLS configuration for libvirt connections
// This function searches for certificates in standard locations and supports
// both Kubernetes mounted secrets and local development configurations
func loadLibvirtTLSConfig() (*tls.Config, error) {
	// Define possible certificate locations in order of preference
	certPaths := []struct {
		cert   string
		key    string
		ca     string
		desc   string
	}{
		{
			// Kubernetes mounted secret path (production)
			cert: "/etc/libvirt-tls/tls.crt",
			key:  "/etc/libvirt-tls/tls.key",
			ca:   "/etc/libvirt-tls/ca.crt",
			desc: "Kubernetes mounted secret",
		},
		{
			// Alternative Kubernetes secret path
			cert: "/var/run/secrets/libvirt-tls/clientcert.pem",
			key:  "/var/run/secrets/libvirt-tls/clientkey.pem",
			ca:   "/var/run/secrets/libvirt-tls/cacert.pem",
			desc: "Alternative Kubernetes secret",
		},
		{
			// User-specific libvirt configuration (development)
			cert: filepath.Join(os.Getenv("HOME"), ".config/libvirt/timewarp-rossg.crt"),
			key:  filepath.Join(os.Getenv("HOME"), ".config/libvirt/timewarp-rossg.key"),
			ca:   filepath.Join(os.Getenv("HOME"), ".config/libvirt/timewarp-rossg-ca.crt"),
			desc: "User configuration directory",
		},
		{
			// Standard libvirt PKI paths
			cert: "/etc/pki/libvirt/clientcert.pem",
			key:  "/etc/pki/libvirt/private/clientkey.pem",
			ca:   "/etc/pki/CA/cacert.pem",
			desc: "System PKI directory",
		},
	}

	var lastErr error

	// Try each certificate location
	for _, paths := range certPaths {
		// Check if all required files exist
		if !fileExists(paths.cert) || !fileExists(paths.key) || !fileExists(paths.ca) {
			lastErr = fmt.Errorf("%s: missing certificate files (cert: %s, key: %s, ca: %s)",
				paths.desc, paths.cert, paths.key, paths.ca)
			continue
		}

		// Load client certificate and key
		clientCert, err := tls.LoadX509KeyPair(paths.cert, paths.key)
		if err != nil {
			lastErr = errors.Wrapf(err, "%s: failed to load client certificate", paths.desc)
			continue
		}

		// Load CA certificate
		caCert, err := os.ReadFile(paths.ca)
		if err != nil {
			lastErr = errors.Wrapf(err, "%s: failed to read CA certificate", paths.desc)
			continue
		}

		// Create CA certificate pool
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			lastErr = fmt.Errorf("%s: failed to parse CA certificate", paths.desc)
			continue
		}

		// Create TLS configuration
		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{clientCert},
			RootCAs:      caCertPool,
			// ServerName will be set dynamically based on the connection URI
			MinVersion:   tls.VersionTLS12,
		}

		fmt.Printf("Successfully loaded TLS certificates from %s\n", paths.desc)
		return tlsConfig, nil
	}

	// If we get here, none of the certificate locations worked
	if lastErr != nil {
		return nil, errors.Wrap(lastErr, "failed to load TLS certificates from any location")
	}

	return nil, errors.New("no TLS certificates found in any standard location")
}

// fileExists checks if a file exists and is readable
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}