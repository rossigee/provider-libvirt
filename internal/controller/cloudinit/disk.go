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

// Package cloudinit implements the controller for cloud-init ISO disks.
package cloudinit

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"

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

	cloudinitv1alpha1 "github.com/rossigee/provider-libvirt/apis/cloudinit/v1alpha1"
	v1beta1 "github.com/rossigee/provider-libvirt/apis/v1beta1"
	"github.com/rossigee/provider-libvirt/internal/clients"
)

const (
	errNotDisk = "managed resource is not a cloudinit Disk"
)

// Setup adds a controller that reconciles cloud-init Disk managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(cloudinitv1alpha1.DiskGroupKind)

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(cloudinitv1alpha1.DiskGroupVersionKind),
		managed.WithExternalConnector(&connector{kube: mgr.GetClient()}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithPollInterval(o.PollInterval),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorder(name))),
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&cloudinitv1alpha1.Disk{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

type connector struct{ kube client.Client }

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*cloudinitv1alpha1.Disk)
	if !ok {
		return nil, errors.New(errNotDisk)
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
	cr, ok := mg.(*cloudinitv1alpha1.Disk)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotDisk)
	}

	lv := e.client.Libvirt()

	pool, err := lv.StoragePoolLookupByName(cr.Spec.ForProvider.Pool)
	if err != nil {
		if isNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, "cannot look up pool")
	}

	vol, err := lv.StorageVolLookupByName(pool, cr.Spec.ForProvider.Name)
	if err != nil {
		if isNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, "cannot look up volume")
	}

	cr.Status.AtProvider.ID = vol.Key
	cr.SetConditions(xpv1.Available())
	meta.SetExternalName(cr, vol.Key)

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: true,
	}, nil
}

func (e *external) Create(_ context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*cloudinitv1alpha1.Disk)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotDisk)
	}

	cr.SetConditions(xpv1.Creating())
	lv := e.client.Libvirt()

	pool, err := lv.StoragePoolLookupByName(cr.Spec.ForProvider.Pool)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, "cannot find pool")
	}

	// Build cloud-init ISO content as a raw tar that libvirt can use to
	// create a volume populated with cloud-init files.
	isoData, err := buildCloudInitISO(cr.Spec.ForProvider)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, "cannot build cloud-init data")
	}

	// Create volume definition XML.
	volXML := fmt.Sprintf(`<volume>
  <name>%s</name>
  <capacity unit="bytes">%d</capacity>
  <target>
    <format type="raw"/>
  </target>
</volume>`, cr.Spec.ForProvider.Name, len(isoData))

	vol, err := lv.StorageVolCreateXML(pool, volXML, 0)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, "cannot create volume")
	}

	// Upload cloud-init data to the volume.
	if err := lv.StorageVolUpload(vol, bytes.NewReader(isoData), 0, uint64(len(isoData)), 0); err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, "cannot upload cloud-init data")
	}

	meta.SetExternalName(cr, vol.Key)
	return managed.ExternalCreation{}, nil
}

func (e *external) Update(_ context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	_, ok := mg.(*cloudinitv1alpha1.Disk)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotDisk)
	}
	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(_ context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*cloudinitv1alpha1.Disk)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotDisk)
	}

	cr.SetConditions(xpv1.Deleting())
	lv := e.client.Libvirt()

	pool, err := lv.StoragePoolLookupByName(cr.Spec.ForProvider.Pool)
	if err != nil {
		if isNotFound(err) {
			return managed.ExternalDelete{}, nil
		}
		return managed.ExternalDelete{}, errors.Wrap(err, "cannot find pool")
	}

	vol, err := lv.StorageVolLookupByName(pool, cr.Spec.ForProvider.Name)
	if err != nil {
		if isNotFound(err) {
			return managed.ExternalDelete{}, nil
		}
		return managed.ExternalDelete{}, errors.Wrap(err, "cannot find volume")
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

// buildCloudInitISO constructs a minimal cloud-init data payload.
// It creates a tar archive containing user-data, meta-data, and optionally
// network-config files. The libvirt StorageVolUpload writes these bytes
// directly to the volume as a raw disk image. For a proper ISO the operator
// would need to invoke genisoimage/mkisofs inside the provider pod; as a
// pragmatic alternative we store the files in a tar so that the domain
// can use a cloud-init datasource that reads from the attached volume.
//
// NOTE: most cloud-init implementations expect a VFAT or ISO9660 filesystem.
// Until in-process ISO creation is implemented, operators should prefer
// an external mechanism (e.g. a ConfigMap + initContainer) to create the ISO
// and simply reference the resulting volume key in their Domain resources.
// This controller still manages the volume lifecycle.
func buildCloudInitISO(p cloudinitv1alpha1.DiskParameters) ([]byte, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	files := map[string]string{}

	if p.UserData != nil {
		files["user-data"] = *p.UserData
	} else {
		files["user-data"] = "#cloud-config\n"
	}

	if p.MetaData != nil {
		files["meta-data"] = *p.MetaData
	} else {
		files["meta-data"] = fmt.Sprintf("instance-id: %s\nlocal-hostname: %s\n", p.Name, p.Name)
	}

	if p.NetworkConfig != nil {
		files["network-config"] = *p.NetworkConfig
	}

	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		if _, err := io.WriteString(tw, content); err != nil {
			return nil, err
		}
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
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
