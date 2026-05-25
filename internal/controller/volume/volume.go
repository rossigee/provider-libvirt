/*
Copyright 2025 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package volume

import (
	"context"
	"fmt"

	"github.com/digitalocean/go-libvirt"
	"github.com/pkg/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"

	v1beta1 "github.com/rossigee/provider-libvirt/apis/v1beta1"
	volumev1alpha1 "github.com/rossigee/provider-libvirt/apis/volume/v1alpha1"
	"github.com/rossigee/provider-libvirt/internal/clients"
)

const (
	errNotVolume = "managed resource is not a Volume"
	errObserve   = "cannot observe volume"
	errCreate    = "cannot create volume"
	errDelete    = "cannot delete volume"
)

// Setup adds a controller that reconciles Volume managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(volumev1alpha1.VolumeGroupKind)

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(volumev1alpha1.VolumeGroupVersionKind),
		managed.WithExternalConnector(&connector{kube: mgr.GetClient()}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithPollInterval(o.PollInterval),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorder(name))),
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&volumev1alpha1.Volume{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

type connector struct{ kube client.Client }

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*volumev1alpha1.Volume)
	if !ok {
		return nil, errors.New(errNotVolume)
	}
	pcRef := cr.GetProviderConfigReference()
	if pcRef == nil {
		return nil, errors.New("no providerConfigRef")
	}
	pc := &v1beta1.ProviderConfig{}
	if err := c.kube.Get(ctx, client.ObjectKey{Name: pcRef.Name}, pc); err != nil {
		return nil, errors.Wrap(err, "cannot get ProviderConfig")
	}
	lc, err := clients.GetClient(ctx, c.kube, pc)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get libvirt client")
	}
	return &external{client: lc}, nil
}

type external struct{ client *clients.Client }

func (e *external) Observe(_ context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*volumev1alpha1.Volume)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotVolume)
	}

	lv := e.client.Libvirt()
	pool, err := lv.StoragePoolLookupByName(cr.Spec.ForProvider.Pool)
	if err != nil {
		if isNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, errObserve)
	}

	vol, err := lv.StorageVolLookupByName(pool, cr.Spec.ForProvider.Name)
	if err != nil {
		if isNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, errObserve)
	}

	_, capacity, allocation, err := lv.StorageVolGetInfo(vol)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errObserve)
	}

	cr.Status.AtProvider.Key = vol.Key
	cr.Status.AtProvider.Capacity = capacity
	cr.Status.AtProvider.Allocation = allocation
	cr.SetConditions(xpv1.Available())
	meta.SetExternalName(cr, vol.Key)

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: true,
	}, nil
}

func (e *external) Create(_ context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*volumev1alpha1.Volume)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotVolume)
	}

	cr.SetConditions(xpv1.Creating())
	lv := e.client.Libvirt()

	pool, err := lv.StoragePoolLookupByName(cr.Spec.ForProvider.Pool)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, "cannot find pool")
	}

	xml := buildVolumeXML(cr.Spec.ForProvider)
	vol, err := lv.StorageVolCreateXML(pool, xml, 0)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreate)
	}

	meta.SetExternalName(cr, vol.Key)
	return managed.ExternalCreation{}, nil
}

func (e *external) Update(_ context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	_, ok := mg.(*volumev1alpha1.Volume)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotVolume)
	}
	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(_ context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*volumev1alpha1.Volume)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotVolume)
	}

	cr.SetConditions(xpv1.Deleting())
	lv := e.client.Libvirt()

	pool, err := lv.StoragePoolLookupByName(cr.Spec.ForProvider.Pool)
	if err != nil {
		if isNotFound(err) {
			return managed.ExternalDelete{}, nil
		}
		return managed.ExternalDelete{}, errors.Wrap(err, errDelete)
	}

	vol, err := lv.StorageVolLookupByName(pool, cr.Spec.ForProvider.Name)
	if err != nil {
		if isNotFound(err) {
			return managed.ExternalDelete{}, nil
		}
		return managed.ExternalDelete{}, errors.Wrap(err, errDelete)
	}

	if err := lv.StorageVolDelete(vol, libvirt.StorageVolDeleteNormal); err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, "cannot delete volume")
	}
	return managed.ExternalDelete{}, nil
}

func (e *external) Disconnect(_ context.Context) error {
	e.client.Close()
	return nil
}

func buildVolumeXML(p volumev1alpha1.VolumeParameters) string {
	format := "qcow2"
	if p.Format != nil {
		format = *p.Format
	}
	capacity := uint64(0)
	if p.Capacity != nil {
		capacity = *p.Capacity
	}
	backing := ""
	if p.BaseVolumeID != nil {
		backing = fmt.Sprintf(`  <backingStore><path>%s</path><format type="%s"/></backingStore>`, *p.BaseVolumeID, format)
	}
	return fmt.Sprintf(`<volume>
  <name>%s</name>
  <capacity unit="bytes">%d</capacity>
  <target><format type="%s"/></target>
%s
</volume>`, p.Name, capacity, format, backing)
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	lverr, ok := err.(libvirt.Error)
	if !ok {
		return false
	}
	return lverr.Code == uint32(libvirt.ErrNoStoragePool) ||
		lverr.Code == uint32(libvirt.ErrNoStorageVol) ||
		lverr.Code == uint32(libvirt.ErrNoNetwork) ||
		lverr.Code == uint32(libvirt.ErrNoDomain)
}
