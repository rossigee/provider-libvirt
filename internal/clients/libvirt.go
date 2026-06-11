// +build cgo

/*
Copyright 2025 Ross Golder
*/

package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"libvirt.org/go/libvirt"

	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"

	"github.com/rossigee/provider-libvirt/apis/v1beta1"
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

// LibvirtOperations defines the interface for libvirt operations
type LibvirtOperations interface {
	DomainLookupByName(name string) (*libvirt.Domain, error)
	DomainDefineXML(xml string) (*libvirt.Domain, error)
	DomainCreate(d *libvirt.Domain) error
	DomainSetAutostart(d *libvirt.Domain, autostart int) error
	DomainShutdown(d *libvirt.Domain) error
	DomainDestroy(d *libvirt.Domain) error
	DomainUndefine(d *libvirt.Domain) error
	NetworkLookupByName(name string) (*libvirt.Network, error)
	NetworkDefineXML(xml string) (*libvirt.Network, error)
	NetworkCreate(n *libvirt.Network) error
	NetworkDestroy(n *libvirt.Network) error
	NetworkUndefine(n *libvirt.Network) error
	StoragePoolLookupByName(name string) (*libvirt.StoragePool, error)
	StoragePoolDefineXML(xml string, flags uint32) (*libvirt.StoragePool, error)
	StoragePoolCreate(sp *libvirt.StoragePool, flags uint32) error
	StoragePoolDestroy(sp *libvirt.StoragePool) error
	StoragePoolUndefine(sp *libvirt.StoragePool) error
	StorageVolLookupByPath(path string) (*libvirt.StorageVol, error)
	StorageVolCreateXML(pool *libvirt.StoragePool, xml string, flags uint32) (*libvirt.StorageVol, error)
	StorageVolDelete(vol *libvirt.StorageVol, flags uint32) error
	Close() error
}

// LibvirtClient wraps libvirt CGO connection with additional metadata
type LibvirtClient struct {
	*libvirt.Connect
	uri string
}

// Close closes the libvirt connection
func (c *LibvirtClient) Close() error {
	if c.Connect != nil {
		res, err := c.Connect.Close()
		if err != nil {
			return err
		}
		if res != 0 {
			return fmt.Errorf("libvirt close returned error code: %d", res)
		}
	}
	return nil
}

// GetLibvirtClient establishes a connection to libvirt using provider configuration
func GetLibvirtClient(ctx context.Context, kube client.Client, mg resource.Managed) (*LibvirtClient, error) {
	// Get provider config reference from the managed resource's ResourceSpec
	var configRef *xpv1.Reference

	// Use reflection to extract ProviderConfigReference from Spec field
	mgVal := reflect.ValueOf(mg)
	if mgVal.Kind() == reflect.Ptr {
		mgVal = mgVal.Elem()
	}

	specField := mgVal.FieldByName("Spec")
	if specField.IsValid() {
		pcrField := specField.FieldByName("ProviderConfigReference")
		if pcrField.IsValid() && !pcrField.IsNil() {
			// dereference the pointer to get the ProviderConfigReference
			pcrVal := pcrField.Interface().(*xpv1.ProviderConfigReference)
			if pcrVal != nil {
				configRef = &xpv1.Reference{Name: pcrVal.Name}
			}
		}
	}

	// Fallback: try interface-based approach
	if configRef == nil {
		if getter, ok := mg.(interface{ GetProviderConfigReference() *xpv1.Reference }); ok {
			configRef = getter.GetProviderConfigReference()
		}
	}

	if configRef == nil {
		return nil, errors.New(errNoProviderConfig)
	}

	// First try to find the ProviderConfig in the managed resource's namespace
	pc := &v1beta1.ProviderConfig{}
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

	// Connect to libvirt using CGO bindings
	// This uses the same connection logic as virsh and handles TLS automatically
	conn, err := libvirt.NewConnect(uri)
	if err != nil {
		return nil, errors.Wrap(err, "cannot connect to libvirt")
	}

	return &LibvirtClient{
		Connect: conn,
		uri:     uri,
	}, nil
}

// NewLibvirtClient creates a new libvirt client from credentials
func NewLibvirtClient(creds LibvirtCredentials) (*LibvirtClient, error) {
	uri, ok := creds[keyURI]
	if !ok {
		return nil, errors.New("libvirt URI not found in credentials")
	}

	// Connect to libvirt using CGO bindings
	// This uses the same connection logic as virsh and handles TLS automatically
	conn, err := libvirt.NewConnect(uri)
	if err != nil {
		return nil, errors.Wrap(err, "cannot connect to libvirt")
	}

	return &LibvirtClient{
		Connect: conn,
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

// Note: TLS configuration is now handled automatically by the libvirt CGO bindings
// The library uses standard libvirt configuration paths and environment variables

// Compatibility layer methods to maintain interface compatibility with controllers
// These methods wrap the CGO bindings to match the old go-libvirt interface

// DomainLookupByName looks up a domain by name
func (c *LibvirtClient) DomainLookupByName(name string) (*libvirt.Domain, error) {
	return c.Connect.LookupDomainByName(name)
}

// DomainGetState gets the state of a domain
func (c *LibvirtClient) DomainGetState(domain *libvirt.Domain, flags uint32) (libvirt.DomainState, int, error) {
	state, reason, err := domain.GetState()
	return libvirt.DomainState(state), reason, err
}

// DomainGetInfo gets information about a domain
func (c *LibvirtClient) DomainGetInfo(domain *libvirt.Domain) (libvirt.DomainState, uint64, uint64, uint16, uint64, error) {
	info, err := domain.GetInfo()
	if err != nil {
		return 0, 0, 0, 0, 0, err
	}
	return libvirt.DomainState(info.State), info.MaxMem, info.Memory, uint16(info.NrVirtCpu), info.CpuTime, nil
}

// DomainDefineXML defines a domain from XML
func (c *LibvirtClient) DomainDefineXML(xml string) (*libvirt.Domain, error) {
	return c.Connect.DomainDefineXML(xml)
}

// DomainCreate starts a domain
func (c *LibvirtClient) DomainCreate(domain *libvirt.Domain) error {
	return domain.Create()
}

// DomainShutdown gracefully shuts down a domain
func (c *LibvirtClient) DomainShutdown(domain *libvirt.Domain) error {
	return domain.Shutdown()
}

// DomainDestroy forcefully destroys a domain
func (c *LibvirtClient) DomainDestroy(domain *libvirt.Domain) error {
	return domain.Destroy()
}

// DomainUndefine undefines a domain
func (c *LibvirtClient) DomainUndefine(domain *libvirt.Domain) error {
	return domain.Undefine()
}

// DomainSetAutostart sets domain autostart behavior
func (c *LibvirtClient) DomainSetAutostart(domain *libvirt.Domain, autostart int) error {
	return domain.SetAutostart(autostart != 0)
}

// Network compatibility methods

// NetworkLookupByName looks up a network by name
func (c *LibvirtClient) NetworkLookupByName(name string) (*libvirt.Network, error) {
	return c.Connect.LookupNetworkByName(name)
}

// NetworkIsActive checks if a network is active
func (c *LibvirtClient) NetworkIsActive(network *libvirt.Network) (bool, error) {
	return network.IsActive()
}

// NetworkIsPersistent checks if a network is persistent
func (c *LibvirtClient) NetworkIsPersistent(network *libvirt.Network) (bool, error) {
	return network.IsPersistent()
}

// NetworkGetAutostart gets network autostart status
func (c *LibvirtClient) NetworkGetAutostart(network *libvirt.Network) (bool, error) {
	return network.GetAutostart()
}

// NetworkGetXMLDesc gets network XML description
func (c *LibvirtClient) NetworkGetXMLDesc(network *libvirt.Network, flags uint32) (string, error) {
	return network.GetXMLDesc(libvirt.NetworkXMLFlags(flags))
}


// NetworkCreate starts a network
func (c *LibvirtClient) NetworkCreate(network *libvirt.Network) error {
	return network.Create()
}

// NetworkDefineXML defines a network from XML
func (c *LibvirtClient) NetworkDefineXML(xml string) (*libvirt.Network, error) {
	return c.Connect.NetworkDefineXML(xml)
}

// NetworkDestroy forcefully destroys a network
func (c *LibvirtClient) NetworkDestroy(network *libvirt.Network) error {
	return network.Destroy()
}

// NetworkUndefine undefines a network
func (c *LibvirtClient) NetworkUndefine(network *libvirt.Network) error {
	return network.Undefine()
}

// StoragePool compatibility methods

// StoragePoolLookupByName looks up a storage pool by name
func (c *LibvirtClient) StoragePoolLookupByName(name string) (*libvirt.StoragePool, error) {
	return c.Connect.LookupStoragePoolByName(name)
}

// StoragePoolIsActive checks if a storage pool is active
func (c *LibvirtClient) StoragePoolIsActive(pool *libvirt.StoragePool) (bool, error) {
	return pool.IsActive()
}

// StoragePoolIsPersistent checks if a storage pool is persistent
func (c *LibvirtClient) StoragePoolIsPersistent(pool *libvirt.StoragePool) (bool, error) {
	return pool.IsPersistent()
}

// StoragePoolGetAutostart gets storage pool autostart status
func (c *LibvirtClient) StoragePoolGetAutostart(pool *libvirt.StoragePool) (bool, error) {
	return pool.GetAutostart()
}

// StoragePoolGetInfo gets storage pool info
func (c *LibvirtClient) StoragePoolGetInfo(pool *libvirt.StoragePool) (*libvirt.StoragePoolInfo, error) {
	return pool.GetInfo()
}

// StoragePoolListAllVolumes lists all volumes in a storage pool
func (c *LibvirtClient) StoragePoolListAllVolumes(pool *libvirt.StoragePool, flags uint32) ([]libvirt.StorageVol, error) {
	return pool.ListAllStorageVolumes(flags)
}


// StoragePoolBuild builds a storage pool
func (c *LibvirtClient) StoragePoolBuild(pool *libvirt.StoragePool, flags uint32) error {
	return pool.Build(libvirt.StoragePoolBuildFlags(flags))
}

// StoragePoolCreate starts a storage pool
func (c *LibvirtClient) StoragePoolCreate(pool *libvirt.StoragePool, flags uint32) error {
	return pool.Create(libvirt.StoragePoolCreateFlags(flags))
}

// StoragePoolDefineXML defines a storage pool from XML
func (c *LibvirtClient) StoragePoolDefineXML(xml string, flags uint32) (*libvirt.StoragePool, error) {
	return c.Connect.StoragePoolDefineXML(xml, libvirt.StoragePoolDefineFlags(flags))
}

// StoragePoolDestroy forcefully destroys a storage pool
func (c *LibvirtClient) StoragePoolDestroy(pool *libvirt.StoragePool) error {
	return pool.Destroy()
}

// StoragePoolUndefine undefines a storage pool
func (c *LibvirtClient) StoragePoolUndefine(pool *libvirt.StoragePool) error {
	return pool.Undefine()
}

// StorageVol compatibility methods

// StorageVolLookupByName looks up a storage volume by name
func (c *LibvirtClient) StorageVolLookupByName(pool *libvirt.StoragePool, name string) (*libvirt.StorageVol, error) {
	return pool.LookupStorageVolByName(name)
}

// StorageVolGetInfo gets storage volume info
func (c *LibvirtClient) StorageVolGetInfo(vol *libvirt.StorageVol) (*libvirt.StorageVolInfo, error) {
	return vol.GetInfo()
}

// StorageVolGetXMLDesc gets storage volume XML description
func (c *LibvirtClient) StorageVolGetXMLDesc(vol *libvirt.StorageVol, flags uint32) (string, error) {
	return vol.GetXMLDesc(flags)
}

// StorageVolCreateXML creates a storage volume from XML
func (c *LibvirtClient) StorageVolCreateXML(pool *libvirt.StoragePool, xml string, flags uint32) (*libvirt.StorageVol, error) {
	return pool.StorageVolCreateXML(xml, libvirt.StorageVolCreateFlags(flags))
}

// StorageVolDelete deletes a storage volume
func (c *LibvirtClient) StorageVolDelete(vol *libvirt.StorageVol, flags uint32) error {
	return vol.Delete(libvirt.StorageVolDeleteFlags(flags))
}

// Secret compatibility methods

// SecretLookupByUUID looks up a secret by UUID
func (c *LibvirtClient) SecretLookupByUUID(uuid string) (*libvirt.Secret, error) {
	uuidBytes, err := StringToUUID(uuid)
	if err != nil {
		return nil, err
	}
	return c.Connect.LookupSecretByUUID(uuidBytes[:])
}

// SecretSetValue sets secret value
func (c *LibvirtClient) SecretSetValue(secret *libvirt.Secret, value []byte, flags uint32) error {
	return secret.SetValue(value, flags)
}

// SecretGetValue gets secret value
func (c *LibvirtClient) SecretGetValue(secret *libvirt.Secret, flags uint32) ([]byte, error) {
	return secret.GetValue(flags)
}

// SecretDefineXML defines a secret from XML
func (c *LibvirtClient) SecretDefineXML(xml string, flags uint32) (*libvirt.Secret, error) {
	return c.Connect.SecretDefineXML(xml, libvirt.SecretDefineFlags(flags))
}

// SecretUndefine undefines a secret
func (c *LibvirtClient) SecretUndefine(secret *libvirt.Secret) error {
	return secret.Undefine()
}

// NodeDevice compatibility methods

// NodeDeviceLookupByName looks up a node device by name
// TODO: Implement when libvirt-go supports NodeDevice API
func (c *LibvirtClient) NodeDeviceLookupByName(name string) (libvirt.NodeDevice, error) {
	return libvirt.NodeDevice{}, errors.New("NodeDevice API not implemented in current libvirt-go version")
}

// NodeDeviceGetXMLDesc gets node device XML description
// TODO: Implement when libvirt-go supports NodeDevice API
func (c *LibvirtClient) NodeDeviceGetXMLDesc(name string, flags uint32) (string, error) {
	return "", errors.New("NodeDevice API not implemented in current libvirt-go version")
}

// Additional compatibility methods for missing client functions

// StorageVolCreateXMLFrom creates storage volume from another volume
func (c *LibvirtClient) StorageVolCreateXMLFrom(pool *libvirt.StoragePool, xml string, source *libvirt.StorageVol, flags uint32) (*libvirt.StorageVol, error) {
	return pool.StorageVolCreateXMLFrom(xml, source, libvirt.StorageVolCreateFlags(flags))
}

// ConnectListAllNodeDevices lists all node devices
// TODO: Implement when libvirt-go supports NodeDevice API
func (c *LibvirtClient) ConnectListAllNodeDevices(need int32, flags uint32) ([]libvirt.NodeDevice, uint32, error) {
	return nil, 0, errors.New("NodeDevice API not implemented in current libvirt-go version")
}

// NodeDeviceCreateXML creates a node device from XML
// TODO: Implement when libvirt-go supports NodeDevice API
func (c *LibvirtClient) NodeDeviceCreateXML(xml string, flags uint32) (libvirt.NodeDevice, error) {
	return libvirt.NodeDevice{}, errors.New("NodeDevice API not implemented in current libvirt-go version")
}

// NodeDeviceDefineXML defines a node device from XML
// TODO: Implement when libvirt-go supports NodeDevice API
func (c *LibvirtClient) NodeDeviceDefineXML(xml string, flags uint32) (libvirt.NodeDevice, error) {
	return libvirt.NodeDevice{}, errors.New("NodeDevice API not implemented in current libvirt-go version")
}

// NodeDeviceUndefine undefines a node device
// TODO: Implement when libvirt-go supports NodeDevice API
func (c *LibvirtClient) NodeDeviceUndefine(name string, flags uint32) error {
	return errors.New("NodeDevice API not implemented in current libvirt-go version")
}

// NodeDeviceDetachFlags detaches a node device
// TODO: Implement when libvirt-go supports NodeDevice API
func (c *LibvirtClient) NodeDeviceDetachFlags(name string, driverName string, flags uint32) error {
	return errors.New("NodeDevice API not implemented in current libvirt-go version")
}

// NodeDeviceReAttach reattaches a node device
// TODO: Implement when libvirt-go supports NodeDevice API
func (c *LibvirtClient) NodeDeviceReAttach(name string) error {
	return errors.New("NodeDevice API not implemented in current libvirt-go version")
}

// NodeDeviceSetAutostart sets node device autostart
// TODO: Implement when libvirt-go supports NodeDevice API
func (c *LibvirtClient) NodeDeviceSetAutostart(name string, autostart int32) error {
	return errors.New("NodeDevice API not implemented in current libvirt-go version")
}

// NodeDeviceDestroy destroys a node device
// TODO: Implement when libvirt-go supports NodeDevice API
func (c *LibvirtClient) NodeDeviceDestroy(name string) error {
	return errors.New("NodeDevice API not implemented in current libvirt-go version")
}

// NetworkSetAutostart sets network autostart
func (c *LibvirtClient) NetworkSetAutostart(network *libvirt.Network, autostart bool) error {
	return network.SetAutostart(autostart)
}

// StorageVolResize resizes a storage volume
func (c *LibvirtClient) StorageVolResize(volume *libvirt.StorageVol, capacity uint64, flags libvirt.StorageVolResizeFlags) error {
	return volume.Resize(capacity, flags)
}

// StoragePoolSetAutostart sets storage pool autostart
func (c *LibvirtClient) StoragePoolSetAutostart(pool *libvirt.StoragePool, autostart bool) error {
	return pool.SetAutostart(autostart)
}

// NodeDeviceGetAutostart gets node device autostart setting
// TODO: Implement when libvirt-go supports NodeDevice API
func (c *LibvirtClient) NodeDeviceGetAutostart(name string) (int32, error) {
	return 0, errors.New("NodeDevice API not implemented in current libvirt-go version")
}

// Disconnect closes the libvirt connection
func (c *LibvirtClient) Disconnect() error {
	return c.Close()
}