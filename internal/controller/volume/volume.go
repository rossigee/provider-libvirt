/*
Copyright 2025 Ross Golder
*/

package volume

import (
	"context"
	"fmt"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/pkg/errors"
	"github.com/rossigee/provider-libvirt/apis/v1beta1"
	"github.com/rossigee/provider-libvirt/internal/clients"
	"libvirt.org/go/libvirt"
	"sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// VolumeClient defines libvirt operations needed for volumes
type VolumeClient interface {
	StorageVolLookupByName(pool *libvirt.StoragePool, name string) (*libvirt.StorageVol, error)
	StorageVolCreateXML(pool *libvirt.StoragePool, xml string, flags uint32) (*libvirt.StorageVol, error)
	StorageVolDelete(vol *libvirt.StorageVol, flags uint32) error
	StorageVolGetInfo(vol *libvirt.StorageVol) (*libvirt.StorageVolInfo, error)
	StorageVolResize(volume *libvirt.StorageVol, capacity uint64, flags libvirt.StorageVolResizeFlags) error
	StoragePoolLookupByName(name string) (*libvirt.StoragePool, error)
}

// Setup adds a controller that reconciles Volume managed resources.
func Setup(mgr ctrl.Manager, l logging.Logger) error {
	name := managed.ControllerName(v1beta1.VolumeGroupKind.String())

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1beta1.VolumeGroupVersionKind),
		managed.WithExternalConnector(&connector{
			kube:         mgr.GetClient(),
			usage:        resource.TrackerFn(func(ctx context.Context, mg resource.Managed) error { return nil }),
			newServiceFn: clients.GetLibvirtClient,
		}),
		managed.WithLogger(l.WithValues("controller", name)),
		managed.WithPollInterval(clients.DefaultPollInterval),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorder(name))))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1beta1.Volume{}).
		Complete(r)
}

type connector struct {
	kube         client.Client
	usage        resource.Tracker
	newServiceFn func(ctx context.Context, kube client.Client, mg resource.Managed) (*clients.LibvirtClient, error)
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	if err := c.usage.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, "cannot track provider config usage")
	}

	libvirtClient, err := c.newServiceFn(ctx, c.kube, mg)
	if err != nil {
		// Connection errors are handled by GetLibvirtClient with backoff logic
		// Return as-is so managed reconciler can handle requeue
		return nil, err
	}

	return &external{client: libvirtClient}, nil
}

type external struct {
	client VolumeClient
}

func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1beta1.Volume)
	if !ok {
		return managed.ExternalObservation{}, errors.New("managed resource is not a Volume custom resource")
	}

	// Lookup storage pool
	pool, err := c.client.StoragePoolLookupByName(cr.Spec.ForProvider.Pool)
	if err != nil {
		if clients.IsNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, err
	}

	// Lookup volume in pool
	vol, err := c.client.StorageVolLookupByName(pool, cr.Spec.ForProvider.Name)
	if err != nil {
		if clients.IsNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, err
	}

	// Get volume info
	info, err := c.client.StorageVolGetInfo(vol)
	if err != nil {
		return managed.ExternalObservation{}, err
	}

	// Update status
	path, err := vol.GetPath()
	if err != nil {
		return managed.ExternalObservation{}, err
	}

	cr.Status.AtProvider.Path = path
	cr.Status.AtProvider.Capacity = int64(info.Capacity)
	cr.Status.AtProvider.Allocation = int64(info.Allocation)

	cr.Status.SetConditions(xpv1.Available())

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: true,
	}, nil
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1beta1.Volume)
	if !ok {
		return managed.ExternalCreation{}, errors.New("managed resource is not a Volume custom resource")
	}

	cr.Status.SetConditions(xpv1.Creating())

	// Lookup storage pool
	pool, err := c.client.StoragePoolLookupByName(cr.Spec.ForProvider.Pool)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, "cannot find storage pool")
	}

	format := "qcow2"
	if cr.Spec.ForProvider.Format != "" {
		format = cr.Spec.ForProvider.Format
	}

	capacity := int64(0)
	if cr.Spec.ForProvider.Size != nil {
		capacity = *cr.Spec.ForProvider.Size
	} else if cr.Spec.ForProvider.Capacity != nil {
		capacity = *cr.Spec.ForProvider.Capacity
	}

	xml := c.generateVolumeXML(cr, capacity, format)

	vol, err := c.client.StorageVolCreateXML(pool, xml, 0)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, "cannot create volume")
	}

	path, err := vol.GetPath()
	if err != nil {
		return managed.ExternalCreation{}, err
	}

	cr.Status.AtProvider.Path = path
	cr.Status.AtProvider.Capacity = capacity

	return managed.ExternalCreation{}, nil
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1beta1.Volume)
	if !ok {
		return managed.ExternalUpdate{}, errors.New("managed resource is not a Volume custom resource")
	}

	// Lookup storage pool and volume
	pool, err := c.client.StoragePoolLookupByName(cr.Spec.ForProvider.Pool)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, "cannot find storage pool")
	}

	vol, err := c.client.StorageVolLookupByName(pool, cr.Spec.ForProvider.Name)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, "cannot find volume")
	}

	// Get current capacity
	info, err := c.client.StorageVolGetInfo(vol)
	if err != nil {
		return managed.ExternalUpdate{}, err
	}

	// Determine new capacity
	newCapacity := int64(0)
	if cr.Spec.ForProvider.Size != nil {
		newCapacity = *cr.Spec.ForProvider.Size
	} else if cr.Spec.ForProvider.Capacity != nil {
		newCapacity = *cr.Spec.ForProvider.Capacity
	}

	// Only resize if new capacity is larger
	if newCapacity > int64(info.Capacity) {
		if err := c.client.StorageVolResize(vol, uint64(newCapacity), 0); err != nil {
			return managed.ExternalUpdate{}, errors.Wrap(err, "cannot resize volume")
		}
	}

	return managed.ExternalUpdate{}, nil
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*v1beta1.Volume)
	if !ok {
		return managed.ExternalDelete{}, errors.New("managed resource is not a Volume custom resource")
	}

	cr.Status.SetConditions(xpv1.Deleting())

	pool, err := c.client.StoragePoolLookupByName(cr.Spec.ForProvider.Pool)
	if err != nil {
		if clients.IsNotFound(err) {
			return managed.ExternalDelete{}, nil
		}
		return managed.ExternalDelete{}, errors.Wrap(err, "cannot find storage pool")
	}

	vol, err := c.client.StorageVolLookupByName(pool, cr.Spec.ForProvider.Name)
	if err != nil {
		if clients.IsNotFound(err) {
			return managed.ExternalDelete{}, nil
		}
		return managed.ExternalDelete{}, errors.Wrap(err, "cannot find volume")
	}

	if err := c.client.StorageVolDelete(vol, 0); err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, "cannot delete volume")
	}

	return managed.ExternalDelete{}, nil
}

func (c *external) Disconnect(_ context.Context) error {
	return nil
}

// generateVolumeXML creates libvirt volume XML definition
func (c *external) generateVolumeXML(cr *v1beta1.Volume, capacity int64, format string) string {
	xml := fmt.Sprintf(`<volume type='file'>
  <name>%s</name>
  <capacity unit='bytes'>%d</capacity>
  <target>
    <format type='%s'/>
  </target>
</volume>`,
		cr.Spec.ForProvider.Name,
		capacity,
		format)

	return xml
}
