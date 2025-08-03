/*
Copyright 2025 Ross Golder
*/

package storagepool

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
	errNotStoragePool      = "managed resource is not a StoragePool custom resource"
	errTrackPCUsage        = "cannot track ProviderConfig usage"
	errGetPC               = "cannot get ProviderConfig"
	errGetCreds            = "cannot get credentials"
	errNewClient           = "cannot create new libvirt client"
	errCreateStoragePool   = "cannot create storage pool"
	errDeleteStoragePool   = "cannot delete storage pool"
	errDescribeStoragePool = "cannot describe storage pool"
	errUpdateStoragePool   = "cannot update storage pool"
	errStartStoragePool    = "cannot start storage pool"
	errStopStoragePool     = "cannot stop storage pool"
)

// Setup adds a controller that reconciles StoragePool managed resources.
func Setup(mgr ctrl.Manager, l logging.Logger) error {
	name := managed.ControllerName(v1alpha1.StoragePoolGroupKind.String())

	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.StoragePoolGroupVersionKind),
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
		For(&v1alpha1.StoragePool{}).
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
	_, ok := mg.(*v1alpha1.StoragePool)
	if !ok {
		return nil, errors.New(errNotStoragePool)
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
	cr, ok := mg.(*v1alpha1.StoragePool)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotStoragePool)
	}

	// Get external name (storage pool name in libvirt)
	poolName := meta.GetExternalName(cr)
	if poolName == "" {
		poolName = cr.Spec.ForProvider.Name
		meta.SetExternalName(cr, poolName)
	}

	// Look up storage pool by name
	pool, err := c.service.StoragePoolLookupByName(poolName)
	if err != nil {
		// Check if error is "storage pool not found"
		if isStoragePoolNotFound(err) {
			return managed.ExternalObservation{
				ResourceExists: false,
			}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, errDescribeStoragePool)
	}

	// Get storage pool state
	active, err := c.service.StoragePoolIsActive(pool)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errDescribeStoragePool)
	}

	persistent, err := c.service.StoragePoolIsPersistent(pool)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errDescribeStoragePool)
	}

	autoStart, err := c.service.StoragePoolGetAutostart(pool)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errDescribeStoragePool)
	}

	// Get storage pool info
	state, capacity, allocation, available, err := c.service.StoragePoolGetInfo(pool)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errDescribeStoragePool)
	}

	// Note: Could get XML for detailed parsing if needed
	// xml, err := c.service.StoragePoolGetXMLDesc(pool, 0)
	// if err != nil {
	//	return managed.ExternalObservation{}, errors.Wrap(err, errDescribeStoragePool)
	// }

	// Get volumes in the pool if active
	var volumes []v1alpha1.StoragePoolVolume
	if active == 1 {
		libvirtVolumes, _, err := c.service.StoragePoolListAllVolumes(pool, 0, 0)
		if err == nil {
			volumes = convertStoragePoolVolumes(libvirtVolumes, c.service)
		}
	}

	// Update status
	cr.Status.AtProvider.UUID = fmt.Sprintf("%x", pool.UUID)
	cr.Status.AtProvider.Active = active == 1
	cr.Status.AtProvider.Persistent = persistent == 1
	cr.Status.AtProvider.AutoStart = autoStart == 1
	cr.Status.AtProvider.State = storagePoolStateToString(libvirt.StoragePoolState(state))
	cr.Status.AtProvider.Capacity = int64(capacity)
	cr.Status.AtProvider.Allocation = int64(allocation)
	cr.Status.AtProvider.Available = int64(available)
	cr.Status.AtProvider.VolumeCount = int32(len(volumes))
	cr.Status.AtProvider.Volumes = volumes

	// Set Ready condition
	cr.SetConditions(xpv1.Available())

	return managed.ExternalObservation{
		ResourceExists:          true,
		ResourceUpToDate:        isStoragePoolUpToDate(cr, int(active), int(autoStart)),
		ConnectionDetails:       getStoragePoolConnectionDetails(cr, pool),
		ResourceLateInitialized: false,
	}, nil
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.StoragePool)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotStoragePool)
	}

	cr.SetConditions(xpv1.Creating())

	// Generate storage pool XML from spec
	xml, err := generateStoragePoolXML(cr)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateStoragePool)
	}

	// Define storage pool (create persistent configuration)
	pool, err := c.service.StoragePoolDefineXML(xml, 0)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateStoragePool)
	}

	// Set external name
	meta.SetExternalName(cr, cr.Spec.ForProvider.Name)

	// Set autostart if requested
	autoStart := cr.Spec.ForProvider.AutoStart
	if autoStart == nil || *autoStart {
		err = c.service.StoragePoolSetAutostart(pool, 1)
		if err != nil {
			return managed.ExternalCreation{}, errors.Wrap(err, errCreateStoragePool)
		}
	}

	// Build storage pool if needed (for some pool types)
	if needsBuilding(cr.Spec.ForProvider.Type) {
		err = c.service.StoragePoolBuild(pool, 0)
		if err != nil {
			return managed.ExternalCreation{}, errors.Wrap(err, errCreateStoragePool)
		}
	}

	// Start storage pool if autostart is enabled
	if autoStart == nil || *autoStart {
		err = c.service.StoragePoolCreate(pool, 0)
		if err != nil {
			return managed.ExternalCreation{}, errors.Wrap(err, errStartStoragePool)
		}
	}

	return managed.ExternalCreation{}, nil
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.StoragePool)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotStoragePool)
	}

	// Storage pool updates are limited in libvirt
	// Most changes require recreation of the pool
	// We can handle autostart changes and start/stop operations

	poolName := meta.GetExternalName(cr)
	pool, err := c.service.StoragePoolLookupByName(poolName)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateStoragePool)
	}

	// Check if autostart setting needs to be updated
	currentAutoStart, err := c.service.StoragePoolGetAutostart(pool)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateStoragePool)
	}

	desiredAutoStart := cr.Spec.ForProvider.AutoStart
	if desiredAutoStart != nil {
		if (*desiredAutoStart && currentAutoStart == 0) || (!*desiredAutoStart && currentAutoStart == 1) {
			newAutoStart := 0
			if *desiredAutoStart {
				newAutoStart = 1
			}
			err = c.service.StoragePoolSetAutostart(pool, int32(newAutoStart))
			if err != nil {
				return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateStoragePool)
			}
		}
	}

	// Check if storage pool should be started/stopped based on autostart
	active, err := c.service.StoragePoolIsActive(pool)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateStoragePool)
	}

	if desiredAutoStart != nil && *desiredAutoStart && active == 0 {
		// Should be running but isn't
		err = c.service.StoragePoolCreate(pool, 0)
		if err != nil {
			return managed.ExternalUpdate{}, errors.Wrap(err, errStartStoragePool)
		}
	} else if desiredAutoStart != nil && !*desiredAutoStart && active == 1 {
		// Should be stopped but is running
		err = c.service.StoragePoolDestroy(pool)
		if err != nil {
			return managed.ExternalUpdate{}, errors.Wrap(err, errStopStoragePool)
		}
	}

	return managed.ExternalUpdate{}, nil
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha1.StoragePool)
	if !ok {
		return errors.New(errNotStoragePool)
	}

	cr.SetConditions(xpv1.Deleting())

	poolName := meta.GetExternalName(cr)
	pool, err := c.service.StoragePoolLookupByName(poolName)
	if err != nil {
		if isStoragePoolNotFound(err) {
			return nil // Already deleted
		}
		return errors.Wrap(err, errDeleteStoragePool)
	}

	// Stop storage pool if it's running
	active, err := c.service.StoragePoolIsActive(pool)
	if err != nil {
		return errors.Wrap(err, errDeleteStoragePool)
	}

	if active == 1 {
		err = c.service.StoragePoolDestroy(pool)
		if err != nil {
			return errors.Wrap(err, errStopStoragePool)
		}
	}

	// Undefine storage pool (delete persistent configuration)
	err = c.service.StoragePoolUndefine(pool)
	if err != nil {
		return errors.Wrap(err, errDeleteStoragePool)
	}

	return nil
}

// Helper functions

func isStoragePoolNotFound(err error) bool {
	return strings.Contains(err.Error(), "not found") ||
		strings.Contains(err.Error(), "no storage pool")
}

func storagePoolStateToString(state libvirt.StoragePoolState) string {
	switch state {
	case libvirt.StoragePoolInactive:
		return "inactive"
	case libvirt.StoragePoolBuilding:
		return "building"
	case libvirt.StoragePoolRunning:
		return "active"
	case libvirt.StoragePoolDegraded:
		return "degraded"
	case libvirt.StoragePoolInaccessible:
		return "inaccessible"
	default:
		return "unknown"
	}
}

func isStoragePoolUpToDate(cr *v1alpha1.StoragePool, active int, autoStart int) bool {
	// Check if autostart matches desired state
	desiredAutoStart := cr.Spec.ForProvider.AutoStart
	if desiredAutoStart != nil {
		if (*desiredAutoStart && autoStart == 0) || (!*desiredAutoStart && autoStart == 1) {
			return false
		}
	}

	// Check if storage pool should be active based on autostart
	if desiredAutoStart != nil && *desiredAutoStart && active == 0 {
		return false
	} else if desiredAutoStart != nil && !*desiredAutoStart && active == 1 {
		return false
	}

	// Additional checks can be added here for other storage pool properties
	return true
}

func getStoragePoolConnectionDetails(cr *v1alpha1.StoragePool, pool libvirt.StoragePool) managed.ConnectionDetails {
	cd := managed.ConnectionDetails{}
	cd["pool-uuid"] = []byte(fmt.Sprintf("%x", pool.UUID))
	cd["pool-name"] = []byte(pool.Name)
	cd["pool-type"] = []byte(cr.Spec.ForProvider.Type)
	if cr.Spec.ForProvider.Target != nil {
		cd["pool-path"] = []byte(cr.Spec.ForProvider.Target.Path)
	}
	return cd
}

func parseStoragePoolType(xml string) string {
	// This is a simplified XML parser
	// In production, you'd want a proper XML parser

	// Look for <pool type='...'>
	start := strings.Index(xml, "<pool type='")
	if start == -1 {
		return "unknown"
	}

	start += 12 // length of "<pool type='"
	end := strings.Index(xml[start:], "'")
	if end == -1 {
		return "unknown"
	}

	return xml[start : start+end]
}

func convertStoragePoolVolumes(libvirtVolumes []libvirt.StorageVol, service *clients.LibvirtClient) []v1alpha1.StoragePoolVolume {
	volumes := make([]v1alpha1.StoragePoolVolume, len(libvirtVolumes))
	for i, vol := range libvirtVolumes {
		// Get volume info
		volType, capacity, allocation, err := service.StorageVolGetInfo(vol)
		if err != nil {
			// Skip volumes with errors
			continue
		}

		// Get volume path
		path, err := service.StorageVolGetPath(vol)
		if err != nil {
			path = ""
		}

		volumes[i] = v1alpha1.StoragePoolVolume{
			Name:       vol.Name,
			Key:        vol.Key,
			Path:       path,
			Type:       storageVolTypeToString(libvirt.StorageVolType(volType)),
			Capacity:   int64(capacity),
			Allocation: int64(allocation),
		}
	}
	return volumes
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

func needsBuilding(poolType string) bool {
	// Some pool types need to be built before they can be used
	switch poolType {
	case "fs", "disk", "logical":
		return true
	default:
		return false
	}
}

func generateStoragePoolXML(cr *v1alpha1.StoragePool) (string, error) {
	spec := cr.Spec.ForProvider

	// Start building the storage pool XML
	xml := fmt.Sprintf("<pool type='%s'>\n  <name>%s</name>\n", spec.Type, spec.Name)

	// Add UUID if we're updating an existing pool
	if cr.Status.AtProvider.UUID != "" {
		xml += fmt.Sprintf("  <uuid>%s</uuid>\n", cr.Status.AtProvider.UUID)
	}

	// Add capacity if specified
	if spec.Capacity != nil {
		xml += fmt.Sprintf("  <capacity unit='bytes'>%d</capacity>\n", *spec.Capacity)
	}

	// Add target configuration
	if spec.Target != nil {
		xml += "  <target>\n"
		xml += fmt.Sprintf("    <path>%s</path>\n", spec.Target.Path)

		// Add target permissions
		if spec.Target.Permissions != nil {
			xml += "    <permissions>\n"
			if spec.Target.Permissions.Owner != nil {
				xml += fmt.Sprintf("      <owner>%d</owner>\n", *spec.Target.Permissions.Owner)
			}
			if spec.Target.Permissions.Group != nil {
				xml += fmt.Sprintf("      <group>%d</group>\n", *spec.Target.Permissions.Group)
			}
			if spec.Target.Permissions.Mode != nil {
				xml += fmt.Sprintf("      <mode>%04o</mode>\n", *spec.Target.Permissions.Mode)
			}
			if spec.Target.Permissions.Label != "" {
				xml += fmt.Sprintf("      <label>%s</label>\n", spec.Target.Permissions.Label)
			}
			xml += "    </permissions>\n"
		}

		// Add target encryption
		if spec.Target.Encryption != nil {
			xml += fmt.Sprintf("    <encryption format='%s'>\n", spec.Target.Encryption.Format)
			if spec.Target.Encryption.Secret != nil {
				secret := spec.Target.Encryption.Secret
				xml += fmt.Sprintf("      <secret type='%s'", secret.Type)
				if secret.UUID != "" {
					xml += fmt.Sprintf(" uuid='%s'", secret.UUID)
				}
				if secret.Usage != "" {
					xml += fmt.Sprintf(" usage='%s'", secret.Usage)
				}
				xml += "/>\n"
			}
			xml += "    </encryption>\n"
		}

		xml += "  </target>\n"
	}

	// Add source configuration
	if spec.Source != nil {
		xml += "  <source>\n"

		// Add host for network storage
		if spec.Source.Host != nil {
			xml += fmt.Sprintf("    <host name='%s'", spec.Source.Host.Name)
			if spec.Source.Host.Port != nil {
				xml += fmt.Sprintf(" port='%d'", *spec.Source.Host.Port)
			}
			xml += "/>\n"
		}

		// Add device for block storage
		if spec.Source.Device != nil {
			xml += fmt.Sprintf("    <device path='%s'", spec.Source.Device.Path)
			if spec.Source.Device.PartTable != "" {
				xml += fmt.Sprintf(" part_separator='%s'", spec.Source.Device.PartTable)
			}
			xml += "/>\n"

			// Add free extents for logical pools
			if spec.Source.Device.FreeExtents != nil {
				xml += fmt.Sprintf("    <freeExtent start='%d' end='%d'/>\n",
					spec.Source.Device.FreeExtents.Start, spec.Source.Device.FreeExtents.End)
			}
		}

		// Add directory
		if spec.Source.Dir != "" {
			xml += fmt.Sprintf("    <dir path='%s'/>\n", spec.Source.Dir)
		}

		// Add name
		if spec.Source.Name != "" {
			xml += fmt.Sprintf("    <name>%s</name>\n", spec.Source.Name)
		}

		// Add format
		if spec.Source.Format != nil {
			xml += fmt.Sprintf("    <format type='%s'", spec.Source.Format.Type)
			if spec.Source.Format.Vendor != "" {
				xml += fmt.Sprintf(" vendor='%s'", spec.Source.Format.Vendor)
			}
			xml += "/>\n"
		}

		// Add adapter for SCSI storage
		if spec.Source.Adapter != nil {
			xml += fmt.Sprintf("    <adapter type='%s'", spec.Source.Adapter.Type)
			if spec.Source.Adapter.Name != "" {
				xml += fmt.Sprintf(" name='%s'", spec.Source.Adapter.Name)
			}
			if spec.Source.Adapter.ParentWWN != "" {
				xml += fmt.Sprintf(" parent_wwn='%s'", spec.Source.Adapter.ParentWWN)
			}
			if spec.Source.Adapter.WWN != "" {
				xml += fmt.Sprintf(" wwn='%s'", spec.Source.Adapter.WWN)
			}
			if spec.Source.Adapter.WWPN != "" {
				xml += fmt.Sprintf(" wwpn='%s'", spec.Source.Adapter.WWPN)
			}
			if spec.Source.Adapter.WWNN != "" {
				xml += fmt.Sprintf(" wwnn='%s'", spec.Source.Adapter.WWNN)
			}
			xml += "/>\n"
		}

		// Add authentication
		if spec.Source.Auth != nil {
			xml += fmt.Sprintf("    <auth type='%s' username='%s'>\n",
				spec.Source.Auth.Type, spec.Source.Auth.Username)
			if spec.Source.Auth.Secret != nil {
				secret := spec.Source.Auth.Secret
				xml += fmt.Sprintf("      <secret type='%s'", secret.Type)
				if secret.UUID != "" {
					xml += fmt.Sprintf(" uuid='%s'", secret.UUID)
				}
				if secret.Usage != "" {
					xml += fmt.Sprintf(" usage='%s'", secret.Usage)
				}
				xml += "/>\n"
			}
			xml += "    </auth>\n"
		}

		// Add vendor and product
		if spec.Source.Vendor != "" {
			xml += fmt.Sprintf("    <vendor name='%s'/>\n", spec.Source.Vendor)
		}
		if spec.Source.Product != "" {
			xml += fmt.Sprintf("    <product name='%s'/>\n", spec.Source.Product)
		}

		xml += "  </source>\n"
	}

	xml += "</pool>"

	return xml, nil
}