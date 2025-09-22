/*
Copyright 2025 Ross Golder
*/

package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

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

// LibvirtClient provides libvirt operations using virsh command-line interface
type LibvirtClient struct {
	*VirshClient
	uri string
}

// Close closes the libvirt connection
func (c *LibvirtClient) Close() error {
	if c.VirshClient != nil {
		return c.VirshClient.Close()
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

	// Create virsh client - no complex connection logic needed
	virshClient := NewVirshClient(uri)

	return &LibvirtClient{
		VirshClient: virshClient,
		uri:         uri,
	}, nil
}


// NewLibvirtClient creates a new libvirt client from credentials
func NewLibvirtClient(creds LibvirtCredentials) (*LibvirtClient, error) {
	uri, ok := creds[keyURI]
	if !ok {
		return nil, errors.New("libvirt URI not found in credentials")
	}

	// Create virsh client - no complex connection logic needed
	virshClient := NewVirshClient(uri)

	return &LibvirtClient{
		VirshClient: virshClient,
		uri:         uri,
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

