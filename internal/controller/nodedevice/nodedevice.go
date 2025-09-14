/*
Copyright 2025 Ross Golder
*/

package nodedevice

import (
	"context"
	"fmt"
	"strings"

	"github.com/digitalocean/go-libvirt"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/rossigee/provider-libvirt/apis/v1alpha1"
	"github.com/rossigee/provider-libvirt/internal/clients"
)

const (
	errNotNodeDevice      = "managed resource is not a NodeDevice custom resource"
	errTrackPCUsage       = "cannot track ProviderConfig usage"
	errGetPC              = "cannot get ProviderConfig"
	errGetCreds           = "cannot get credentials"
	errNewClient          = "cannot create new Service"
	errNodeDeviceLookup   = "cannot lookup node device"
	errNodeDeviceDetach   = "cannot detach node device"
	errNodeDeviceReattach = "cannot reattach node device"
	errNodeDeviceCreate   = "cannot create node device"
	errNodeDeviceDestroy  = "cannot destroy node device"
	errNodeDeviceDefine   = "cannot define node device"
	errNodeDeviceUndefine = "cannot undefine node device"
	errParseXML           = "cannot parse device XML"
	errGenerateXML        = "cannot generate device XML"
)

// NodeDeviceService defines the interface for node device operations
type NodeDeviceService interface {
	// Device discovery and management
	ConnectListAllNodeDevices(needResults int32, flags uint32) ([]libvirt.NodeDevice, uint32, error)
	NodeDeviceLookupByName(name string) (libvirt.NodeDevice, error)
	NodeDeviceGetXMLDesc(name string, flags uint32) (string, error)

	// Device lifecycle
	NodeDeviceCreateXML(xml string, flags uint32) (libvirt.NodeDevice, error)
	NodeDeviceDestroy(name string) error
	NodeDeviceDefineXML(xml string, flags uint32) (libvirt.NodeDevice, error)
	NodeDeviceUndefine(name string, flags uint32) error

	// Device attachment management
	NodeDeviceDetachFlags(name string, driverName libvirt.OptString, flags uint32) error
	NodeDeviceReAttach(name string) error

	// Device state management
	NodeDeviceSetAutostart(name string, autostart int32) error
	NodeDeviceGetAutostart(name string) (int32, error)

	// Connection management
	Disconnect() error
}

// Setup adds a controller that reconciles NodeDevice managed resources.
func Setup(mgr ctrl.Manager, l logging.Logger) error {
	name := managed.ControllerName(v1alpha1.NodeDeviceGroupKind.String())

	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.NodeDeviceGroupVersionKind),
		managed.WithExternalConnecter(&connector{
			kube:         mgr.GetClient(),
			usage:        resource.NewProviderConfigUsageTracker(mgr.GetClient(), &v1alpha1.ProviderConfigUsage{}),
			newServiceFn: clients.GetLibvirtClient}),
		managed.WithLogger(l.WithValues("controller", name)),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		managed.WithConnectionPublishers(cps...))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.NodeDevice{}).
		Complete(r)
}

// A connector is expected to produce an ExternalClient when its Connect method
// is called.
type connector struct {
	kube         client.Client
	usage        resource.Tracker
	newServiceFn func(ctx context.Context, kube client.Client, mg resource.Managed) (*clients.LibvirtClient, error)
}

// Connect typically produces an ExternalClient by:
// 1. Tracking that the managed resource is using a ProviderConfig.
// 2. Getting the managed resource's ProviderConfig.
// 3. Getting the credentials specified by the ProviderConfig.
// 4. Using the credentials to form a client.
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*v1alpha1.NodeDevice)
	if !ok {
		return nil, errors.New(errNotNodeDevice)
	}

	if err := c.usage.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, errTrackPCUsage)
	}

	pc := &v1alpha1.ProviderConfig{}
	if err := c.kube.Get(ctx, types.NamespacedName{Name: cr.GetProviderConfigReference().Name}, pc); err != nil {
		return nil, errors.Wrap(err, errGetPC)
	}

	client, err := c.newServiceFn(ctx, c.kube, mg)
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	return &external{service: client.Libvirt}, nil
}


// An ExternalClient observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	service NodeDeviceService
}

func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.NodeDevice)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotNodeDevice)
	}

	// Try to find the device by name
	deviceName := getDeviceName(cr)
	if deviceName == "" {
		// For discovery mode, we need to find the device by criteria
		device, err := c.findDeviceByCriteria(cr)
		if err != nil {
			if clients.IsNotFound(err) {
				return managed.ExternalObservation{
					ResourceExists: false,
				}, nil
			}
			return managed.ExternalObservation{}, errors.Wrap(err, errNodeDeviceLookup)
		}
		deviceName = device.Name
	}

	device, err := c.service.NodeDeviceLookupByName(deviceName)
	if err != nil {
		if clients.IsNotFound(err) {
			return managed.ExternalObservation{
				ResourceExists: false,
			}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, errNodeDeviceLookup)
	}

	// Get device XML for detailed information
	xmlDesc, err := c.service.NodeDeviceGetXMLDesc(device.Name, 0)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errNodeDeviceLookup)
	}

	// Parse XML to get device information
	deviceInfo, err := c.parseDeviceXML(xmlDesc)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errParseXML)
	}

	// Update status
	cr.Status.AtProvider = *deviceInfo

	// Check if device is up-to-date
	upToDate := c.isUpToDate(cr, deviceInfo)

	// Set resource as ready
	cr.Status.SetConditions(xpv1.Available())

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: upToDate,
		ConnectionDetails: managed.ConnectionDetails{
			"device_name": []byte(deviceInfo.Name),
			"device_type": []byte(cr.Spec.ForProvider.Type),
		},
	}, nil
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.NodeDevice)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotNodeDevice)
	}

	cr.Status.SetConditions(xpv1.Creating())

	// Handle different device types
	switch cr.Spec.ForProvider.Type {
	case "mdev":
		return c.createMediatedDevice(ctx, cr)
	case "pci", "usb", "scsi", "net", "storage":
		// For existing devices, we just need to manage their state
		return c.manageExistingDevice(ctx, cr)
	default:
		return managed.ExternalCreation{}, errors.Errorf("unsupported device type: %s", cr.Spec.ForProvider.Type)
	}
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.NodeDevice)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotNodeDevice)
	}

	deviceName := getDeviceName(cr)
	device, err := c.service.NodeDeviceLookupByName(deviceName)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errNodeDeviceLookup)
	}

	// Handle detachment state changes
	if cr.Spec.ForProvider.Detached {
		if err := c.detachDevice(device, cr.Spec.ForProvider.Driver); err != nil {
			return managed.ExternalUpdate{}, err
		}
	} else {
		if err := c.reattachDevice(device); err != nil {
			return managed.ExternalUpdate{}, err
		}
	}

	// Handle autostart changes
	if cr.Spec.ForProvider.Autostart {
		if err := c.service.NodeDeviceSetAutostart(device.Name, 1); err != nil {
			return managed.ExternalUpdate{}, errors.Wrap(err, "cannot set autostart")
		}
	} else {
		if err := c.service.NodeDeviceSetAutostart(device.Name, 0); err != nil {
			return managed.ExternalUpdate{}, errors.Wrap(err, "cannot unset autostart")
		}
	}

	return managed.ExternalUpdate{}, nil
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*v1alpha1.NodeDevice)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotNodeDevice)
	}

	cr.Status.SetConditions(xpv1.Deleting())

	deviceName := getDeviceName(cr)
	device, err := c.service.NodeDeviceLookupByName(deviceName)
	if err != nil {
		if clients.IsNotFound(err) {
			return managed.ExternalDelete{}, nil
		}
		return managed.ExternalDelete{}, errors.Wrap(err, errNodeDeviceLookup)
	}

	// For mediated devices, destroy the device
	if cr.Spec.ForProvider.Type == "mdev" {
		if err := c.service.NodeDeviceDestroy(device.Name); err != nil {
			return managed.ExternalDelete{}, errors.Wrap(err, errNodeDeviceDestroy)
		}
	}

	// Reattach device if it was detached
	if cr.Spec.ForProvider.Detached {
		if err := c.reattachDevice(device); err != nil {
			return managed.ExternalDelete{}, err
		}
	}

	// Undefine persistent devices
	if cr.Spec.ForProvider.Persistent && cr.Spec.ForProvider.Type == "mdev" {
		if err := c.service.NodeDeviceUndefine(device.Name, 0); err != nil {
			return managed.ExternalDelete{}, errors.Wrap(err, errNodeDeviceUndefine)
		}
	}

	return managed.ExternalDelete{}, nil
}

func (c *external) Disconnect(ctx context.Context) error {
	return c.service.Disconnect()
}

// Helper functions

func getDeviceName(cr *v1alpha1.NodeDevice) string {
	if cr.Spec.ForProvider.Name != "" {
		return cr.Spec.ForProvider.Name
	}
	if cr.Status.AtProvider.Name != "" {
		return cr.Status.AtProvider.Name
	}
	return ""
}

func (c *external) findDeviceByCriteria(cr *v1alpha1.NodeDevice) (libvirt.NodeDevice, error) {
	devices, _, err := c.service.ConnectListAllNodeDevices(0, 0)
	if err != nil {
		return libvirt.NodeDevice{}, err
	}

	for _, device := range devices {
		xmlDesc, err := c.service.NodeDeviceGetXMLDesc(device.Name, 0)
		if err != nil {
			continue
		}

		if c.matchesDeviceCriteria(cr, xmlDesc) {
			return device, nil
		}
	}

	return libvirt.NodeDevice{}, errors.New("device not found")
}

func (c *external) matchesDeviceCriteria(cr *v1alpha1.NodeDevice, xmlDesc string) bool {
	deviceInfo, err := c.parseDeviceXML(xmlDesc)
	if err != nil {
		return false
	}

	switch cr.Spec.ForProvider.Type {
	case "pci":
		if cr.Spec.ForProvider.PCIAddress == nil {
			return false
		}
		return c.matchesPCIAddress(deviceInfo, cr.Spec.ForProvider.PCIAddress)
	case "usb":
		if cr.Spec.ForProvider.USBDevice == nil {
			return false
		}
		return c.matchesUSBDevice(deviceInfo, cr.Spec.ForProvider.USBDevice)
	case "scsi":
		if cr.Spec.ForProvider.SCSIDevice == nil {
			return false
		}
		return c.matchesSCSIDevice(deviceInfo, cr.Spec.ForProvider.SCSIDevice)
	}

	return false
}

func (c *external) matchesPCIAddress(deviceInfo *v1alpha1.NodeDeviceObservation, pciAddr *v1alpha1.PCIAddress) bool {
	// This would need to parse the device XML and match PCI address
	// For now, this is a placeholder implementation
	return false
}

func (c *external) matchesUSBDevice(deviceInfo *v1alpha1.NodeDeviceObservation, usbDev *v1alpha1.USBDevice) bool {
	// This would need to parse the device XML and match USB vendor/product IDs
	// For now, this is a placeholder implementation
	return false
}

func (c *external) matchesSCSIDevice(deviceInfo *v1alpha1.NodeDeviceObservation, scsiDev *v1alpha1.SCSIDevice) bool {
	// This would need to parse the device XML and match SCSI address
	// For now, this is a placeholder implementation
	return false
}

func (c *external) parseDeviceXML(xmlDesc string) (*v1alpha1.NodeDeviceObservation, error) {
	// This is a simplified XML parser - in production, this would need
	// comprehensive XML parsing logic for different device types

	observation := &v1alpha1.NodeDeviceObservation{}

	// Extract basic device information from XML
	// This is a placeholder implementation
	if strings.Contains(xmlDesc, "<name>") {
		start := strings.Index(xmlDesc, "<name>") + 6
		end := strings.Index(xmlDesc[start:], "</name>")
		if end > 0 {
			observation.Name = xmlDesc[start : start+end]
		}
	}

	// Set default state
	observation.State = "active"

	return observation, nil
}

func (c *external) isUpToDate(cr *v1alpha1.NodeDevice, deviceInfo *v1alpha1.NodeDeviceObservation) bool {
	// Check if device state matches desired state
	if cr.Spec.ForProvider.Detached {
		return deviceInfo.State == "detached"
	}
	return deviceInfo.State == "active"
}

func (c *external) createMediatedDevice(ctx context.Context, cr *v1alpha1.NodeDevice) (managed.ExternalCreation, error) {
	if cr.Spec.ForProvider.MediatedDevice == nil {
		return managed.ExternalCreation{}, errors.New("mediated device configuration required for mdev type")
	}

	mdev := cr.Spec.ForProvider.MediatedDevice
	deviceUUID := mdev.UUID
	if deviceUUID == "" {
		deviceUUID = uuid.New().String()
	}

	// Generate mdev XML
	xmlDesc := c.generateMdevXML(mdev, deviceUUID)

	// Create the device
	var device libvirt.NodeDevice
	var err error

	if cr.Spec.ForProvider.Persistent {
		// Create persistent definition first
		device, err = c.service.NodeDeviceDefineXML(xmlDesc, 0)
		if err != nil {
			return managed.ExternalCreation{}, errors.Wrap(err, errNodeDeviceDefine)
		}

		// Device defined successfully
	} else {
		// Create transient device
		device, err = c.service.NodeDeviceCreateXML(xmlDesc, 0)
		if err != nil {
			return managed.ExternalCreation{}, errors.Wrap(err, errNodeDeviceCreate)
		}
	}

	// Set autostart if requested
	if cr.Spec.ForProvider.Autostart {
		if err := c.service.NodeDeviceSetAutostart(device.Name, 1); err != nil {
			return managed.ExternalCreation{}, errors.Wrap(err, "cannot set autostart")
		}
	}

	return managed.ExternalCreation{
		ConnectionDetails: managed.ConnectionDetails{
			"device_name": []byte(device.Name),
			"device_uuid": []byte(deviceUUID),
		},
	}, nil
}

func (c *external) manageExistingDevice(ctx context.Context, cr *v1alpha1.NodeDevice) (managed.ExternalCreation, error) {
	// For existing hardware devices, we discover and manage their state
	device, err := c.findDeviceByCriteria(cr)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errNodeDeviceLookup)
	}

	// Detach device if requested
	if cr.Spec.ForProvider.Detached {
		if err := c.detachDevice(device, cr.Spec.ForProvider.Driver); err != nil {
			return managed.ExternalCreation{}, err
		}
	}

	return managed.ExternalCreation{
		ConnectionDetails: managed.ConnectionDetails{
			"device_name": []byte(device.Name),
		},
	}, nil
}

func (c *external) detachDevice(device libvirt.NodeDevice, driver string) error {
	if driver == "" {
		driver = "vfio-pci" // Default driver for device passthrough
	}

	return c.service.NodeDeviceDetachFlags(device.Name, libvirt.OptString{driver}, 0)
}

func (c *external) reattachDevice(device libvirt.NodeDevice) error {
	return c.service.NodeDeviceReAttach(device.Name)
}

func (c *external) generateMdevXML(mdev *v1alpha1.MediatedDevice, deviceUUID string) string {
	// Generate basic mdev XML structure
	return fmt.Sprintf(`<device>
  <parent>%s</parent>
  <capability type='mdev'>
    <type id='%s'/>
    <uuid>%s</uuid>
  </capability>
</device>`, mdev.Parent, mdev.Type, deviceUUID)
}