/*
Copyright 2025 Ross Golder
*/

package volume

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/pkg/errors"

	"github.com/rossigee/provider-libvirt/apis/v1beta1"
	"github.com/rossigee/provider-libvirt/internal/clients"
	"libvirt.org/go/libvirt"
	ctrl "sigs.k8s.io/controller-runtime"
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
	NewStream(flags libvirt.StreamFlags) (*libvirt.Stream, error)
	Close() error
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
			log:          l.WithValues("controller", name),
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
	log          logging.Logger
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	if err := c.usage.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, "cannot track provider config usage")
	}

	c.log.Debug("Volume Connect called", "name", mg.GetName(), "namespace", mg.GetNamespace())

	libvirtClient, err := c.newServiceFn(ctx, c.kube, mg)
	if err != nil {
		// Connection errors are handled by GetLibvirtClient with backoff logic
		// Return as-is so managed reconciler can handle requeue
		return nil, err
	}

	return &external{client: libvirtClient, log: c.log}, nil
}

type external struct {
	client VolumeClient
	log    logging.Logger
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

	// Refresh pool to ensure volume list is up-to-date
	if refreshErr := pool.Refresh(0); refreshErr != nil {
		c.log.Debug("Observe: failed to refresh pool", "pool", cr.Spec.ForProvider.Pool, "error", refreshErr.Error())
	}

	// Lookup volume in pool
	var vol *libvirt.StorageVol
	vol, err = c.client.StorageVolLookupByName(pool, cr.Spec.ForProvider.Name)
	if err != nil {
		if clients.IsNotFound(err) {
			c.log.Debug("Observe: volume not found, will be created", "name", cr.Spec.ForProvider.Name, "pool", cr.Spec.ForProvider.Pool)
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		c.log.Debug("Observe: StorageVolLookupByName failed", "name", cr.Spec.ForProvider.Name, "pool", cr.Spec.ForProvider.Pool, "error", err.Error())
		return managed.ExternalObservation{}, err
	}

	c.log.Debug("Observe: volume found", "name", cr.Spec.ForProvider.Name, "pool", cr.Spec.ForProvider.Pool)

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

	needsPopulation := false
	if cr.Spec.ForProvider.Source != nil && cr.Spec.ForProvider.Source.URL != "" {
		if int64(info.Allocation) < 8192 {
			c.log.Info("Volume exists but is empty, will populate from URL", "name", cr.Spec.ForProvider.Name, "url", cr.Spec.ForProvider.Source.URL, "allocation", info.Allocation)
			needsPopulation = true
		}
	}

	cr.Status.SetConditions(xpv1.Available())

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: !needsPopulation,
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

	// Refresh pool to ensure volume list is up-to-date
	if refreshErr := pool.Refresh(0); refreshErr != nil {
		c.log.Debug("Create: failed to refresh pool", "pool", cr.Spec.ForProvider.Pool, "error", refreshErr.Error())
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

	vol, err := c.client.StorageVolLookupByName(pool, cr.Spec.ForProvider.Name)
	if err == nil {
		fmt.Printf("DEBUG Volume Create: volume %s already exists in pool %s, adopting\n", cr.Spec.ForProvider.Name, cr.Spec.ForProvider.Pool)
		path, pathErr := vol.GetPath()
		if pathErr != nil {
			return managed.ExternalCreation{}, errors.Wrap(pathErr, "cannot get volume path during adoption")
		}
		info, infoErr := c.client.StorageVolGetInfo(vol)
		if infoErr != nil {
			return managed.ExternalCreation{}, errors.Wrap(infoErr, "cannot get volume info during adoption")
		}
		cr.Status.AtProvider.Path = path
		cr.Status.AtProvider.Capacity = int64(info.Capacity)
		cr.Status.AtProvider.Allocation = int64(info.Allocation)
		return managed.ExternalCreation{}, nil
	}

	if !clients.IsNotFound(err) {
		fmt.Printf("DEBUG Volume Create: unexpected lookup error for %s in pool %s: %s\n", cr.Spec.ForProvider.Name, cr.Spec.ForProvider.Pool, err.Error())
		return managed.ExternalCreation{}, errors.Wrap(err, "unexpected error looking up volume")
	}

	fmt.Printf("DEBUG Volume Create: volume %s not found in pool %s, creating\n", cr.Spec.ForProvider.Name, cr.Spec.ForProvider.Pool)

	xml := c.generateVolumeXML(cr, capacity, format)

	vol, err = c.client.StorageVolCreateXML(pool, xml, 0)
	if err != nil {
		fmt.Printf("DEBUG Volume Create error: pool=%s, vol=%s, err=%s\n", cr.Spec.ForProvider.Pool, cr.Spec.ForProvider.Name, err.Error())
		if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "exit status 5") {
			// Refresh pool before retrying lookup
			if refreshErr := pool.Refresh(0); refreshErr != nil {
				c.log.Debug("Create: failed to refresh pool in fallback", "pool", cr.Spec.ForProvider.Pool, "error", refreshErr.Error())
			}
			vol, lookupErr := c.client.StorageVolLookupByName(pool, cr.Spec.ForProvider.Name)
			if lookupErr != nil {
				fmt.Printf("DEBUG Volume lookup failed: pool=%s, vol=%s, lookupErr=%s\n", cr.Spec.ForProvider.Pool, cr.Spec.ForProvider.Name, lookupErr.Error())
				return managed.ExternalCreation{}, errors.Wrap(err, "cannot create volume (and failed to look up existing): "+lookupErr.Error())
			}
			path, pathErr := vol.GetPath()
			if pathErr != nil {
				return managed.ExternalCreation{}, errors.Wrap(err, "cannot create volume (and failed to get path)")
			}
			info, infoErr := c.client.StorageVolGetInfo(vol)
			if infoErr != nil {
				return managed.ExternalCreation{}, errors.Wrap(err, "cannot create volume (and failed to get info)")
			}
			cr.Status.AtProvider.Path = path
			cr.Status.AtProvider.Capacity = int64(info.Capacity)
			cr.Status.AtProvider.Allocation = int64(info.Allocation)
			return managed.ExternalCreation{}, nil
		}
		return managed.ExternalCreation{}, errors.Wrap(err, "cannot create volume")
	}

	path, err := vol.GetPath()
	if err != nil {
		return managed.ExternalCreation{}, err
	}

	cr.Status.AtProvider.Path = path
	cr.Status.AtProvider.Capacity = capacity

	if cr.Spec.ForProvider.Source != nil && cr.Spec.ForProvider.Source.URL != "" {
		if err := c.populateFromURL(ctx, vol, cr.Spec.ForProvider.Source.URL); err != nil {
			return managed.ExternalCreation{}, errors.Wrap(err, "cannot populate volume from URL")
		}
	}

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

	if cr.Spec.ForProvider.Source != nil && cr.Spec.ForProvider.Source.URL != "" && int64(info.Allocation) < 1048576 {
		if err := c.populateFromURL(ctx, vol, cr.Spec.ForProvider.Source.URL); err != nil {
			return managed.ExternalUpdate{}, errors.Wrap(err, "cannot populate volume from URL")
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
	return c.client.Close()
}

// generateVolumeXML creates libvirt volume XML definition
func (c *external) generateVolumeXML(cr *v1beta1.Volume, capacity int64, format string) string {
	target := fmt.Sprintf(`    <format type='%s'/>`, format)

	if cr.Spec.ForProvider.Target != nil && cr.Spec.ForProvider.Target.Permissions != nil {
		perms := cr.Spec.ForProvider.Target.Permissions
		target += `
    <permissions>`
		if perms.Mode != "" {
			target += fmt.Sprintf(`
      <mode>%s</mode>`, perms.Mode)
		}
		if perms.Owner != "" {
			target += fmt.Sprintf(`
      <owner>%s</owner>`, perms.Owner)
		}
		if perms.Group != "" {
			target += fmt.Sprintf(`
      <group>%s</group>`, perms.Group)
		}
		if perms.Label != "" {
			target += fmt.Sprintf(`
      <label>%s</label>`, perms.Label)
		}
		target += `
    </permissions>`
	}

	xml := fmt.Sprintf(`<volume type='file'>
  <name>%s</name>
  <capacity unit='bytes'>%d</capacity>
  <target>
%s
  </target>
</volume>`,
		cr.Spec.ForProvider.Name,
		capacity,
		target)

	return xml
}

// populateFromURL downloads a file from URL and uploads it to the libvirt volume
func (c *external) populateFromURL(ctx context.Context, vol *libvirt.StorageVol, url string) error {
	c.log.Info("Downloading volume content from URL", "url", url)

	tlsConfig := &tls.Config{}
	if sslCertFile := os.Getenv("SSL_CERT_FILE"); sslCertFile != "" {
		pool, err := loadCertPoolFromFile(sslCertFile)
		if err != nil {
			c.log.Info("Warning: cannot load SSL_CERT_FILE, using system CA pool", "file", sslCertFile, "error", err.Error())
		} else {
			tlsConfig.RootCAs = pool
			c.log.Info("Loaded custom CA cert pool for HTTPS downloads", "file", sslCertFile)
		}
	}

	httpClient := &http.Client{
		Timeout: 5 * time.Minute,
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return errors.Wrap(err, "cannot create HTTP request")
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "cannot download from URL")
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_ = resp.Body.Close()
		return fmt.Errorf("HTTP download returned status %d for URL %s", resp.StatusCode, url)
	}

	// Create libvirt stream and upload
	stream, err := c.client.NewStream(0)
	if err != nil {
		return errors.Wrap(err, "cannot create libvirt stream")
	}
	defer func() { _ = stream.Free() }()

	if err := vol.Upload(stream, 0, 0, 0); err != nil {
		return errors.Wrap(err, "cannot start libvirt upload")
	}

	if err := stream.SendAll(func(s *libvirt.Stream, _ int) ([]byte, error) {
		buf := make([]byte, 64*1024)
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			return buf[:n], nil
		}
		if readErr == io.EOF {
			return nil, nil
		}
		return nil, readErr
	}); err != nil {
		return errors.Wrap(err, "cannot send data via libvirt stream")
	}

	if err := stream.Finish(); err != nil {
		return errors.Wrap(err, "cannot finish libvirt stream")
	}

	c.log.Info("Volume populated from URL successfully", "url", url)
	return nil
}

func loadCertPoolFromFile(path string) (*x509.CertPool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(data) {
		return nil, fmt.Errorf("cannot parse CA certificates from %s", path)
	}
	return pool, nil
}
