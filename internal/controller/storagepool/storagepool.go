/*
Copyright 2025 Ross Golder
*/

package storagepool

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"libvirt.org/go/libvirt"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"

	"github.com/rossigee/provider-libvirt/apis/v1beta1"
	"github.com/rossigee/provider-libvirt/internal/clients"
)

// StoragePoolClient defines libvirt operations needed for storage pools
type StoragePoolClient interface {
	StoragePoolLookupByName(name string) (*libvirt.StoragePool, error)
	StoragePoolDefineXML(xml string, flags uint32) (*libvirt.StoragePool, error)
	StoragePoolCreate(sp *libvirt.StoragePool, flags uint32) error
	StoragePoolDestroy(sp *libvirt.StoragePool) error
	StoragePoolUndefine(sp *libvirt.StoragePool) error
	StoragePoolIsActive(sp *libvirt.StoragePool) (bool, error)
	StoragePoolIsPersistent(sp *libvirt.StoragePool) (bool, error)
	StoragePoolGetAutostart(sp *libvirt.StoragePool) (bool, error)
	StoragePoolSetAutostart(sp *libvirt.StoragePool, autostart bool) error
	StoragePoolGetInfo(sp *libvirt.StoragePool) (*libvirt.StoragePoolInfo, error)
	StoragePoolBuild(sp *libvirt.StoragePool, flags uint32) error
}

// Setup adds a controller that reconciles StoragePool managed resources.
func Setup(mgr ctrl.Manager, l logging.Logger) error {
	name := managed.ControllerName(v1beta1.StoragePoolGroupKind.String())

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1beta1.StoragePoolGroupVersionKind),
		managed.WithExternalConnecter(&connector{
			kube:         mgr.GetClient(),
			usage:        resource.TrackerFn(func(ctx context.Context, mg resource.Managed) error { return nil }),
			newServiceFn: clients.GetLibvirtClient,
		}),
		managed.WithLogger(l.WithValues("controller", name)),
		managed.WithPollInterval(clients.DefaultPollInterval),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1beta1.StoragePool{}).
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
	client StoragePoolClient
}

func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1beta1.StoragePool)
	if !ok {
		return managed.ExternalObservation{}, errors.New("managed resource is not a StoragePool custom resource")
	}

	// Lookup storage pool by name
	pool, err := c.client.StoragePoolLookupByName(cr.Spec.ForProvider.Name)
	if err != nil {
		if clients.IsNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, err
	}

	// Get pool state
	active, err := c.client.StoragePoolIsActive(pool)
	if err != nil {
		return managed.ExternalObservation{}, err
	}

	state := "inactive"
	if active {
		state = "active"
	}
	cr.Status.AtProvider.State = state
	cr.Status.AtProvider.Active = active

	// Get persistence status
	persistent, err := c.client.StoragePoolIsPersistent(pool)
	if err != nil {
		return managed.ExternalObservation{}, err
	}
	cr.Status.AtProvider.Persistent = persistent

	// Get autostart status
	autostart, err := c.client.StoragePoolGetAutostart(pool)
	if err != nil {
		return managed.ExternalObservation{}, err
	}
	cr.Status.AtProvider.AutoStart = autostart

	// Get pool info
	info, err := c.client.StoragePoolGetInfo(pool)
	if err != nil {
		return managed.ExternalObservation{}, err
	}

	cr.Status.AtProvider.Capacity = int64(info.Capacity)
	cr.Status.AtProvider.Allocation = int64(info.Allocation)
	cr.Status.AtProvider.Available = int64(info.Available)

	cr.Status.SetConditions(xpv1.Available())

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: true,
	}, nil
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1beta1.StoragePool)
	if !ok {
		return managed.ExternalCreation{}, errors.New("managed resource is not a StoragePool custom resource")
	}

	cr.Status.SetConditions(xpv1.Creating())

	xml := c.generatePoolXML(cr)

	pool, err := c.client.StoragePoolDefineXML(xml, 0)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, "cannot define storage pool")
	}

	// Build pool if needed
	if err := c.client.StoragePoolBuild(pool, 0); err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, "cannot build storage pool")
	}

	// Start pool (default)
	if err := c.client.StoragePoolCreate(pool, 0); err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, "cannot start storage pool")
	}

	// Set autostart if specified
	if cr.Spec.ForProvider.Autostart != nil && *cr.Spec.ForProvider.Autostart {
		if err := c.client.StoragePoolSetAutostart(pool, true); err != nil {
			return managed.ExternalCreation{}, errors.Wrap(err, "cannot set autostart")
		}
	}

	cr.Status.AtProvider.State = "active"
	cr.Status.AtProvider.Active = true

	return managed.ExternalCreation{}, nil
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1beta1.StoragePool)
	if !ok {
		return managed.ExternalUpdate{}, errors.New("managed resource is not a StoragePool custom resource")
	}

	// Lookup existing pool
	pool, err := c.client.StoragePoolLookupByName(cr.Spec.ForProvider.Name)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, "cannot find storage pool")
	}

	// Update autostart
	autostart := cr.Spec.ForProvider.Autostart != nil && *cr.Spec.ForProvider.Autostart
	if err := c.client.StoragePoolSetAutostart(pool, autostart); err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, "cannot set autostart")
	}

	return managed.ExternalUpdate{}, nil
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*v1beta1.StoragePool)
	if !ok {
		return managed.ExternalDelete{}, errors.New("managed resource is not a StoragePool custom resource")
	}

	cr.Status.SetConditions(xpv1.Deleting())

	pool, err := c.client.StoragePoolLookupByName(cr.Spec.ForProvider.Name)
	if err != nil {
		if clients.IsNotFound(err) {
			return managed.ExternalDelete{}, nil
		}
		return managed.ExternalDelete{}, errors.Wrap(err, "cannot find storage pool")
	}

	// Stop pool if active
	active, err := c.client.StoragePoolIsActive(pool)
	if err != nil {
		return managed.ExternalDelete{}, err
	}

	if active {
		if err := c.client.StoragePoolDestroy(pool); err != nil {
			return managed.ExternalDelete{}, errors.Wrap(err, "cannot destroy storage pool")
		}
	}

	// Undefine pool
	if err := c.client.StoragePoolUndefine(pool); err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, "cannot undefine storage pool")
	}

	return managed.ExternalDelete{}, nil
}

func (c *external) Disconnect(_ context.Context) error {
	return nil
}

// generatePoolXML creates libvirt storage pool XML definition
func (c *external) generatePoolXML(cr *v1beta1.StoragePool) string {
	params := cr.Spec.ForProvider

	poolType := "dir"
	if params.Type != "" {
		poolType = params.Type
	}

	path := "/var/lib/libvirt/images"
	if params.Target != nil && params.Target.Path != "" {
		path = params.Target.Path
	}

	xml := fmt.Sprintf(`<pool type='%s'>
  <name>%s</name>
  <target>
    <path>%s</path>
  </target>
</pool>`,
		poolType,
		params.Name,
		path)

	return xml
}
