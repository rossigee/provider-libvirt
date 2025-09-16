/*
Copyright 2025 Ross Golder
*/

package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/digitalocean/go-libvirt"
	"github.com/digitalocean/go-libvirt/socket/dialers"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

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
	configRef := mg.GetProviderConfigReference()
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
			pcNamespace = "default"
		} else {
			pcNamespace = "crossplane-system"
		}
	}
	
	// Use the namespace where we found the ProviderConfig
	// pcNamespace is already set from the successful Get() call above

	// Track usage of this provider config (v2 compatible)
	pcu := &v1alpha1.ProviderConfigUsage{}
	pcu.SetName(mg.GetName() + "-" + configRef.Name)
	
	// ProviderConfigUsage must be in the same namespace as the ProviderConfig
	// Ensure we have a valid namespace
	if pcNamespace == "" {
		return nil, errors.New("ProviderConfig namespace is empty - cannot create ProviderConfigUsage - DEBUG: this should not happen in v0.3.1")
	}

	// DEBUG: Log the namespace we're setting
	fmt.Printf("DEBUG v0.3.1: Setting ProviderConfigUsage namespace to: '%s' (len=%d)\n", pcNamespace, len(pcNamespace))
	pcu.SetNamespace(pcNamespace)

	// DEBUG: Verify the namespace was set correctly
	setNamespace := pcu.GetNamespace()
	fmt.Printf("DEBUG v0.3.1: ProviderConfigUsage namespace after SetNamespace: '%s' (len=%d)\n", setNamespace, len(setNamespace))

	if setNamespace == "" {
		return nil, errors.New("ProviderConfigUsage SetNamespace failed - namespace is still empty after setting")
	}
	pcu.ProviderConfigReference = xpv1.Reference{Name: configRef.Name}
	pcu.ResourceReference = xpv1.TypedReference{
		APIVersion: mg.GetObjectKind().GroupVersionKind().GroupVersion().String(),
		Kind:       mg.GetObjectKind().GroupVersionKind().Kind,
		Name:       mg.GetName(),
		UID:        mg.GetUID(),
	}
	
	if err := kube.Create(ctx, pcu); err != nil && !kerrors.IsAlreadyExists(err) {
		return nil, errors.Wrap(err, errTrackUsage)
	}

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

	return &LibvirtClient{
		Libvirt: libvirt.NewWithDialer(dialers.NewAlreadyConnected(conn)),
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

	return &LibvirtClient{
		Libvirt: libvirt.NewWithDialer(dialers.NewAlreadyConnected(conn)),
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