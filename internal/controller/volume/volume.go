/*
Copyright 2025 Ross Golder
*/

package volume

import (
	"context"
	"fmt"
	"strings"

	"github.com/digitalocean/go-libvirt"
	"github.com/pkg/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/rossigee/provider-libvirt/apis/v1alpha1"
	"github.com/rossigee/provider-libvirt/internal/clients"
)

const (
	errNotVolume      = "managed resource is not a Volume custom resource"
	errTrackPCUsage   = "cannot track ProviderConfig usage"
	errGetPC          = "cannot get ProviderConfig"
	errGetCreds       = "cannot get credentials"
	errNewClient      = "cannot create new libvirt client"
	errCreateVolume   = "cannot create volume"
	errDeleteVolume   = "cannot delete volume"
	errDescribeVolume = "cannot describe volume"
	errUpdateVolume   = "cannot update volume"
)

// Setup adds a controller that reconciles Volume managed resources.
func Setup(mgr ctrl.Manager, l logging.Logger) error {
	name := managed.ControllerName(v1alpha1.VolumeGroupKind.String())

	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.VolumeGroupVersionKind),
		managed.WithExternalConnecter(&connector{
			kube:         mgr.GetClient(),
			usage:        resource.NewProviderConfigUsageTracker(mgr.GetClient(), &v1alpha1.ProviderConfigUsage{}),
			newServiceFn: clients.GetLibvirtClient,
		}),
		managed.WithLogger(l.WithValues("controller", name)),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		managed.WithConnectionPublishers(cps...))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 10,
		}).
		For(&v1alpha1.Volume{}).
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
	_, ok := mg.(*v1alpha1.Volume)
	if !ok {
		return nil, errors.New(errNotVolume)
	}

	if err := c.usage.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, errTrackPCUsage)
	}

	svc, err := c.newServiceFn(ctx, c.kube, mg)
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	return &external{service: svc, kube: c.kube}, nil
}

// An ExternalClient observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	service *clients.LibvirtClient
	kube    client.Client
}

func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.Volume)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotVolume)
	}

	// Get external name (volume name in libvirt)
	volumeName := meta.GetExternalName(cr)
	if volumeName == "" {
		volumeName = cr.Spec.ForProvider.Name
		meta.SetExternalName(cr, volumeName)
	}

	// Get storage pool
	poolName := cr.Spec.ForProvider.Pool
	pool, err := c.service.StoragePool(poolName)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errDescribeVolume)
	}

	// Look up volume in storage pool
	volume, err := c.service.StorageVolLookupByName(pool, volumeName)
	if err != nil {
		// Check if error is "volume not found"
		if isVolumeNotFound(err) {
			return managed.ExternalObservation{
				ResourceExists: false,
			}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, errDescribeVolume)
	}

	// Get volume info  
	volType, capacity, allocation, err := c.service.StorageVolGetInfo(volume)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errDescribeVolume)
	}

	// Get volume XML for detailed information
	xml, err := c.service.StorageVolGetXMLDesc(volume, 0)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errDescribeVolume)
	}

	// Parse XML to extract format and other details
	format, path := parseVolumeXML(xml)

	// Update status
	cr.Status.AtProvider.Key = volume.Key
	cr.Status.AtProvider.Path = path
	cr.Status.AtProvider.Type = storageVolTypeToString(libvirt.StorageVolType(volType))
	cr.Status.AtProvider.Capacity = int64(capacity)
	cr.Status.AtProvider.Allocation = int64(allocation)
	cr.Status.AtProvider.Format = format
	cr.Status.AtProvider.Pool = poolName

	// Set Ready condition
	cr.SetConditions(xpv1.Available())

	return managed.ExternalObservation{
		ResourceExists:          true,
		ResourceUpToDate:        isVolumeUpToDate(cr, capacity),
		ConnectionDetails:       getVolumeConnectionDetails(cr, volume, path),
		ResourceLateInitialized: false,
	}, nil
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.Volume)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotVolume)
	}

	cr.SetConditions(xpv1.Creating())

	// Get storage pool
	poolName := cr.Spec.ForProvider.Pool
	pool, err := c.service.StoragePool(poolName)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateVolume)
	}

	// Generate volume XML from spec
	xml, err := generateVolumeXML(cr)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateVolume)
	}

	// Create volume in storage pool
	volume, err := c.service.StorageVolCreateXML(pool, xml, libvirt.StorageVolCreatePreallocMetadata)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateVolume)
	}

	// Set external name
	meta.SetExternalName(cr, cr.Spec.ForProvider.Name)

	// If we have a source, handle cloning or copying
	if cr.Spec.ForProvider.Source != nil {
		err = c.handleVolumeSource(ctx, cr, pool, volume)
		if err != nil {
			// Clean up the created volume on error
			_ = c.service.StorageVolDelete(volume, 0)
			return managed.ExternalCreation{}, errors.Wrap(err, errCreateVolume)
		}
	}

	return managed.ExternalCreation{}, nil
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.Volume)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotVolume)
	}

	// Volume updates are limited in libvirt
	// Most changes require recreation
	// We can potentially resize volumes if supported
	
	volumeName := meta.GetExternalName(cr)
	poolName := cr.Spec.ForProvider.Pool
	
	pool, err := c.service.StoragePool(poolName)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateVolume)
	}

	volume, err := c.service.StorageVolLookupByName(pool, volumeName)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateVolume)
	}

	// Check if resize is needed
	_, capacity, _, err := c.service.StorageVolGetInfo(volume)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateVolume)
	}

	if int64(capacity) < cr.Spec.ForProvider.Capacity {
		// Attempt to resize volume
		err = c.service.StorageVolResize(volume, uint64(cr.Spec.ForProvider.Capacity), libvirt.StorageVolResizeAllocate)
		if err != nil {
			return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateVolume)
		}
	}

	return managed.ExternalUpdate{}, nil
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*v1alpha1.Volume)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotVolume)
	}

	cr.SetConditions(xpv1.Deleting())

	volumeName := meta.GetExternalName(cr)
	poolName := cr.Spec.ForProvider.Pool

	pool, err := c.service.StoragePool(poolName)
	if err != nil {
		if isPoolNotFound(err) {
			return managed.ExternalDelete{}, nil // Pool doesn't exist, volume is already gone
		}
		return managed.ExternalDelete{}, errors.Wrap(err, errDeleteVolume)
	}

	volume, err := c.service.StorageVolLookupByName(pool, volumeName)
	if err != nil {
		if isVolumeNotFound(err) {
			return managed.ExternalDelete{}, nil // Already deleted
		}
		return managed.ExternalDelete{}, errors.Wrap(err, errDeleteVolume)
	}

	// Delete volume
	err = c.service.StorageVolDelete(volume, libvirt.StorageVolDeleteNormal)
	if err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, errDeleteVolume)
	}

	return managed.ExternalDelete{}, nil
}

func (c *external) Disconnect(ctx context.Context) error {
	// Nothing to disconnect for libvirt client
	return nil
}

// Helper functions

func isVolumeNotFound(err error) bool {
	return strings.Contains(err.Error(), "not found") ||
		strings.Contains(err.Error(), "no storage vol")
}

func isPoolNotFound(err error) bool {
	return strings.Contains(err.Error(), "not found") ||
		strings.Contains(err.Error(), "no storage pool")
}

func storageVolTypeToString(volType libvirt.StorageVolType) string {
	switch volType {
	case libvirt.StorageVolFile:
		return "file"
	case libvirt.StorageVolBlock:
		return "block"
	case libvirt.StorageVolDir:
		return "dir"
	case libvirt.StorageVolNetwork:
		return "network"
	case libvirt.StorageVolNetdir:
		return "netdir"
	case libvirt.StorageVolPloop:
		return "ploop"
	default:
		return "unknown"
	}
}

func isVolumeUpToDate(cr *v1alpha1.Volume, capacity uint64) bool {
	// Check if capacity matches
	if int64(capacity) != cr.Spec.ForProvider.Capacity {
		return false
	}

	// Additional checks can be added here
	return true
}

func getVolumeConnectionDetails(cr *v1alpha1.Volume, volume libvirt.StorageVol, path string) managed.ConnectionDetails {
	cd := managed.ConnectionDetails{}
	cd["volume-key"] = []byte(volume.Key)
	cd["volume-name"] = []byte(volume.Name)
	cd["volume-path"] = []byte(path)
	cd["volume-pool"] = []byte(cr.Spec.ForProvider.Pool)
	return cd
}

func parseVolumeXML(xml string) (format, path string) {
	// This is a simplified XML parser
	// In production, you'd want a proper XML parser
	
	// Extract format
	if strings.Contains(xml, "<format type='qcow2'") {
		format = "qcow2"
	} else if strings.Contains(xml, "<format type='raw'") {
		format = "raw"
	} else if strings.Contains(xml, "<format type='vmdk'") {
		format = "vmdk"
	} else {
		format = "unknown"
	}

	// Extract path - look for <path>...</path>
	start := strings.Index(xml, "<path>")
	if start != -1 {
		start += 6 // length of "<path>"
		end := strings.Index(xml[start:], "</path>")
		if end != -1 {
			path = xml[start : start+end]
		}
	}

	return format, path
}

func generateVolumeXML(cr *v1alpha1.Volume) (string, error) {
	spec := cr.Spec.ForProvider
	
	// Set defaults
	format := spec.Format
	if format == "" {
		format = "qcow2"
	}

	allocation := spec.Allocation
	if allocation == nil {
		// Default allocation to 10% of capacity for sparse files
		defaultAllocation := spec.Capacity / 10
		allocation = &defaultAllocation
	}

	xml := fmt.Sprintf(`<volume type='file'>
  <name>%s</name>
  <capacity unit='bytes'>%d</capacity>
  <allocation unit='bytes'>%d</allocation>
  <target>
    <format type='%s'/>`, 
		spec.Name, spec.Capacity, *allocation, format)

	// Add target permissions if specified
	if spec.Target != nil && spec.Target.Permissions != nil {
		perm := spec.Target.Permissions
		xml += "\n    <permissions>"
		if perm.Owner != nil {
			xml += fmt.Sprintf("\n      <owner>%d</owner>", *perm.Owner)
		}
		if perm.Group != nil {
			xml += fmt.Sprintf("\n      <group>%d</group>", *perm.Group)
		}
		if perm.Mode != nil {
			xml += fmt.Sprintf("\n      <mode>%04o</mode>", *perm.Mode)
		}
		if perm.Label != "" {
			xml += fmt.Sprintf("\n      <label>%s</label>", perm.Label)
		}
		xml += "\n    </permissions>"
	}

	xml += "\n  </target>"

	// Add backing store if specified
	if spec.BackingStore != nil {
		backingFormat := spec.BackingStore.Format
		if backingFormat == "" {
			backingFormat = "qcow2"
		}
		xml += fmt.Sprintf(`
  <backingStore>
    <path>%s</path>
    <format type='%s'/>
  </backingStore>`, spec.BackingStore.Path, backingFormat)
	}

	// Add encryption if specified
	if spec.Encryption != nil {
		xml += fmt.Sprintf(`
  <encryption format='%s'>`, spec.Encryption.Format)
		if spec.Encryption.Secret != nil {
			secret := spec.Encryption.Secret
			xml += fmt.Sprintf(`
    <secret type='%s'`, secret.Type)
			if secret.UUID != "" {
				xml += fmt.Sprintf(` uuid='%s'`, secret.UUID)
			}
			if secret.Usage != "" {
				xml += fmt.Sprintf(` usage='%s'`, secret.Usage)
			}
			xml += "/>"
		}
		xml += "\n  </encryption>"
	}

	xml += "\n</volume>"

	return xml, nil
}

func (c *external) handleVolumeSource(ctx context.Context, cr *v1alpha1.Volume, pool libvirt.StoragePool, volume libvirt.StorageVol) error {
	source := cr.Spec.ForProvider.Source

	if source.Volume != "" {
		// Clone from another volume
		sourcePool := pool
		if source.Pool != "" {
			var err error
			sourcePool, err = c.service.StoragePool(source.Pool)
			if err != nil {
				return errors.Wrap(err, "cannot find source pool")
			}
		}

		sourceVolume, err := c.service.StorageVolLookupByName(sourcePool, source.Volume)
		if err != nil {
			return errors.Wrap(err, "cannot find source volume")
		}

		// Create XML for cloning
		xml := fmt.Sprintf(`<volume type='file'>
  <name>%s</name>
  <target>
    <format type='%s'/>
  </target>
</volume>`, cr.Spec.ForProvider.Name, cr.Spec.ForProvider.Format)

		// Create clone (this replaces the volume we created)
		_, err = c.service.StorageVolCreateXMLFrom(pool, xml, sourceVolume, libvirt.StorageVolCreatePreallocMetadata)
		if err != nil {
			return errors.Wrap(err, "cannot clone volume")
		}
	}

	// TODO: Handle URL and File sources
	// This would involve downloading or copying files

	return nil
}