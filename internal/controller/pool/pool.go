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

package pool

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

	poolv1alpha1 "github.com/rossigee/provider-libvirt/apis/pool/v1alpha1"
	v1beta1 "github.com/rossigee/provider-libvirt/apis/v1beta1"
	"github.com/rossigee/provider-libvirt/internal/clients"
)

const (
	errNotPool   = "managed resource is not a Pool"
	errGetPC     = "cannot get ProviderConfig"
	errGetClient = "cannot get libvirt client"
	errObserve   = "cannot observe pool"
	errCreate    = "cannot create pool"
	errDelete    = "cannot delete pool"
)

// Setup adds a controller that reconciles Pool managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(poolv1alpha1.PoolGroupKind)

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(poolv1alpha1.PoolGroupVersionKind),
		managed.WithExternalConnector(&connector{kube: mgr.GetClient()}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithPollInterval(o.PollInterval),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorder(name))),
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&poolv1alpha1.Pool{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

type connector struct {
	kube client.Client
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*poolv1alpha1.Pool)
	if !ok {
		return nil, errors.New(errNotPool)
	}

	pcRef := cr.GetProviderConfigReference()
	if pcRef == nil {
		return nil, errors.New(errGetPC)
	}

	pc := &v1beta1.ProviderConfig{}
	if err := c.kube.Get(ctx, client.ObjectKey{Name: pcRef.Name}, pc); err != nil {
		return nil, errors.Wrap(err, errGetPC)
	}

	lc, err := clients.GetClient(ctx, c.kube, pc)
	if err != nil {
		return nil, errors.Wrap(err, errGetClient)
	}

	return &external{client: lc, kube: c.kube}, nil
}

type external struct {
	client *clients.Client
	kube   client.Client
}

func (e *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*poolv1alpha1.Pool)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotPool)
	}

	lv := e.client.Libvirt()
	poolName := cr.Spec.ForProvider.Name

	pool, err := lv.StoragePoolLookupByName(poolName)
	if err != nil {
		if isNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, errObserve)
	}

	// Fetch pool info
	active, err := lv.StoragePoolIsActive(pool)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errObserve)
	}

	_, capacity, allocation, available, err := lv.StoragePoolGetInfo(pool)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errObserve)
	}

	cr.Status.AtProvider.UUID = formatUUID(pool.UUID)
	cr.Status.AtProvider.Capacity = capacity
	cr.Status.AtProvider.Allocation = allocation
	cr.Status.AtProvider.Available = available
	if active == 1 {
		cr.Status.AtProvider.State = "running"
	} else {
		cr.Status.AtProvider.State = "inactive"
	}

	cr.SetConditions(xpv1.Available())
	meta.SetExternalName(cr, poolName)

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: true, // Pool type/path are immutable after creation
	}, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*poolv1alpha1.Pool)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotPool)
	}

	cr.SetConditions(xpv1.Creating())
	lv := e.client.Libvirt()

	xml := buildPoolXML(cr.Spec.ForProvider)

	pool, err := lv.StoragePoolDefineXML(xml, 0)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreate)
	}

	if err := lv.StoragePoolBuild(pool, 0); err != nil {
		// Non-fatal for some pool types; continue
		ctrl.LoggerFrom(ctx).V(4).Info("StoragePoolBuild warning (may be ok)", "error", err)
	}

	if err := lv.StoragePoolCreate(pool, 0); err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, "cannot start pool after creation")
	}

	if err := lv.StoragePoolSetAutostart(pool, 1); err != nil {
		ctrl.LoggerFrom(ctx).V(4).Info("StoragePoolSetAutostart warning", "error", err)
	}

	meta.SetExternalName(cr, cr.Spec.ForProvider.Name)
	return managed.ExternalCreation{}, nil
}

func (e *external) Update(_ context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	// Storage pool type and path are immutable. No-op.
	_, ok := mg.(*poolv1alpha1.Pool)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotPool)
	}
	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*poolv1alpha1.Pool)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotPool)
	}

	cr.SetConditions(xpv1.Deleting())
	lv := e.client.Libvirt()

	pool, err := lv.StoragePoolLookupByName(cr.Spec.ForProvider.Name)
	if err != nil {
		if isNotFound(err) {
			return managed.ExternalDelete{}, nil
		}
		return managed.ExternalDelete{}, errors.Wrap(err, errDelete)
	}

	active, _ := lv.StoragePoolIsActive(pool)
	if active == 1 {
		if err := lv.StoragePoolDestroy(pool); err != nil {
			return managed.ExternalDelete{}, errors.Wrap(err, "cannot destroy pool")
		}
	}

	if err := lv.StoragePoolDelete(pool, 0); err != nil {
		ctrl.LoggerFrom(ctx).V(4).Info("StoragePoolDelete warning (may be ok for some pool types)", "error", err)
	}

	if err := lv.StoragePoolUndefine(pool); err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, "cannot undefine pool")
	}

	return managed.ExternalDelete{}, nil
}

func (e *external) Disconnect(_ context.Context) error {
	e.client.Close()
	return nil
}

func buildPoolXML(p poolv1alpha1.PoolParameters) string {
	path := ""
	if p.Path != nil {
		path = fmt.Sprintf("  <target><path>%s</path></target>\n", *p.Path)
	}
	return fmt.Sprintf(`<pool type="%s">
  <name>%s</name>
%s</pool>`, p.Type, p.Name, path)
}

func formatUUID(uuid [16]byte) string {
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16])
}

// isNotFound returns true when the libvirt error indicates the object does not exist.
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
