/*
Copyright 2025 Ross Golder
*/

package domain

import (
	"context"
	"encoding/xml"
	"fmt"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/rossigee/provider-libvirt/apis/v1beta1"
	"github.com/rossigee/provider-libvirt/internal/clients"
	"libvirt.org/go/libvirt"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Setup adds a controller that reconciles Domain managed resources.
func Setup(mgr ctrl.Manager, l logging.Logger) error {
	name := managed.ControllerName(v1beta1.DomainGroupKind.String())

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1beta1.DomainGroupVersionKind),
		managed.WithExternalConnector(&connector{
			kube:         mgr.GetClient(),
			usage:        resource.TrackerFn(func(ctx context.Context, mg resource.Managed) error { return nil }),
			newServiceFn: clients.GetLibvirtClient,
			logger:       l.WithValues("controller", name),
		}),
		managed.WithLogger(l.WithValues("controller", name)),
		managed.WithPollInterval(clients.DefaultPollInterval),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorder(name))))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1beta1.Domain{}).
		Complete(r)
}

type connector struct {
	kube         client.Client
	usage        resource.Tracker
	newServiceFn func(ctx context.Context, kube client.Client, mg resource.Managed) (*clients.LibvirtClient, error)
	logger       logging.Logger
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

	return &external{client: libvirtClient, kube: c.kube, logger: c.logger}, nil
}

// DomainClient defines libvirt operations needed for domains
type DomainClient interface {
	DomainLookupByName(name string) (*libvirt.Domain, error)
	DomainDefineXML(xml string) (*libvirt.Domain, error)
	DomainCreate(d *libvirt.Domain) error
	DomainSetAutostart(d *libvirt.Domain, autostart int) error
	DomainShutdown(d *libvirt.Domain) error
	DomainDestroy(d *libvirt.Domain) error
	DomainUndefine(d *libvirt.Domain) error
}

type external struct {
	client DomainClient
	kube   client.Client // Kubernetes client for resolving references
	logger logging.Logger
}

func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1beta1.Domain)
	if !ok {
		return managed.ExternalObservation{}, errors.New("managed resource is not a Domain custom resource")
	}

	// Try to lookup domain by name
	domain, err := c.client.DomainLookupByName(cr.Spec.ForProvider.Name)
	if err != nil {
		if clients.IsNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, err
	}

	// Get domain state
	state, _, err := domain.GetState()
	if err != nil {
		return managed.ExternalObservation{}, err
	}

	domainState := libvirt.DomainState(state)

	// Update status
	cr.Status.AtProvider.State = formatDomainState(domainState)
	uuid, err := domain.GetUUIDString()
	if err != nil {
		return managed.ExternalObservation{}, err
	}
	cr.Status.AtProvider.UUID = uuid

	// Get current XML and parse for disk information
	currentXML, err := domain.GetXMLDesc(0)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, "cannot get current domain XML")
	}

	// Parse disks from domain XML
	disks, err := c.parseDisksFromXML(currentXML)
	if err != nil {
		c.logger.Info("Failed to parse disks from XML, ignoring", "error", err)
		disks = nil
	}
	cr.Status.AtProvider.Disks = disks

	// Check for zombie condition: domain running but has no disks
	if domainState == libvirt.DOMAIN_RUNNING && len(disks) == 0 {
		c.logger.Info("WARNING: Domain is running with no disks attached - potential zombie", "domain", cr.Name)
		cr.Status.SetConditions(
			xpv1.Available(),
			xpv1.Condition{
				Type:               "ZombieRisk",
				Status:             "True",
				Reason:             "NoDisksAttached",
				Message:            "Domain is running but has no boot disk attached",
				LastTransitionTime: metav1.Now(),
			},
		)
	} else {
		cr.Status.SetConditions(xpv1.Available())
	}

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: true,
	}, nil
}

// domainXML is a minimal struct to parse disk elements from libvirt domain XML
type domainXML struct {
	Devices struct {
		Disks []diskXML `xml:"disk"`
	} `xml:"devices"`
}

type diskXML struct {
	Device string `xml:"device,attr"`
	Type   string `xml:"type,attr"`
	Source struct {
		File string `xml:"file,attr"`
		Dev  string `xml:"dev,attr"`
	} `xml:"source"`
	Target struct {
		Dev string `xml:"dev,attr"`
	} `xml:"target"`
	Boot *struct {
		Order int `xml:"order,attr"`
	} `xml:"boot"`
}

func (c *external) parseDisksFromXML(xmlData string) ([]v1beta1.DiskInfo, error) {
	var domain domainXML
	if err := xml.Unmarshal([]byte(xmlData), &domain); err != nil {
		return nil, errors.Wrap(err, "failed to parse domain XML for disks")
	}

	disks := make([]v1beta1.DiskInfo, 0, len(domain.Devices.Disks))
	for _, d := range domain.Devices.Disks {
		diskInfo := v1beta1.DiskInfo{
			Device: d.Target.Dev,
			Type:   d.Device,
		}
		if d.Source.File != "" {
			diskInfo.Source = d.Source.File
		} else if d.Source.Dev != "" {
			diskInfo.Source = d.Source.Dev
		}
		if d.Boot != nil && d.Boot.Order > 0 {
			order := int32(d.Boot.Order)
			diskInfo.BootOrder = &order
		}
		disks = append(disks, diskInfo)
	}

	return disks, nil
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1beta1.Domain)
	if !ok {
		return managed.ExternalCreation{}, errors.New("managed resource is not a Domain custom resource")
	}

	cr.Status.SetConditions(xpv1.Creating())

	xml := c.generateDomainXML(cr)

	domain, err := c.client.DomainDefineXML(xml)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, "cannot define domain")
	}

	// Start domain if running is true (default)
	running := cr.Spec.ForProvider.Running == nil || *cr.Spec.ForProvider.Running
	if running {
		if err := c.client.DomainCreate(domain); err != nil {
			return managed.ExternalCreation{}, errors.Wrap(err, "cannot start domain")
		}
	}

	// Set autostart if specified
	if cr.Spec.ForProvider.Autostart != nil && *cr.Spec.ForProvider.Autostart {
		if err := c.client.DomainSetAutostart(domain, 1); err != nil {
			return managed.ExternalCreation{}, errors.Wrap(err, "cannot set autostart")
		}
	}

	uuid, err := domain.GetUUIDString()
	if err != nil {
		return managed.ExternalCreation{}, err
	}
	cr.Status.AtProvider.UUID = uuid

	return managed.ExternalCreation{}, nil
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1beta1.Domain)
	if !ok {
		return managed.ExternalUpdate{}, errors.New("managed resource is not a Domain custom resource")
	}

	// Lookup existing domain
	domain, err := c.client.DomainLookupByName(cr.Spec.ForProvider.Name)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, "cannot find domain")
	}

	// Get current state
	state, _, err := domain.GetState()
	if err != nil {
		return managed.ExternalUpdate{}, err
	}

	// Handle running state
	running := cr.Spec.ForProvider.Running == nil || *cr.Spec.ForProvider.Running
	isRunning := libvirt.DomainState(state) == libvirt.DOMAIN_RUNNING

	if running && !isRunning {
		if err := c.client.DomainCreate(domain); err != nil {
			return managed.ExternalUpdate{}, errors.Wrap(err, "cannot start domain")
		}
	} else if !running && isRunning {
		if err := c.client.DomainShutdown(domain); err != nil {
			return managed.ExternalUpdate{}, errors.Wrap(err, "cannot shutdown domain")
		}
	}

	// Update autostart
	autostart := cr.Spec.ForProvider.Autostart != nil && *cr.Spec.ForProvider.Autostart
	if err := c.client.DomainSetAutostart(domain, boolToInt(autostart)); err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, "cannot set autostart")
	}

	return managed.ExternalUpdate{}, nil
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*v1beta1.Domain)
	if !ok {
		return managed.ExternalDelete{}, errors.New("managed resource is not a Domain custom resource")
	}

	cr.Status.SetConditions(xpv1.Deleting())

	domain, err := c.client.DomainLookupByName(cr.Spec.ForProvider.Name)
	if err != nil {
		if clients.IsNotFound(err) {
			return managed.ExternalDelete{}, nil
		}
		return managed.ExternalDelete{}, errors.Wrap(err, "cannot find domain")
	}

	// Get domain state
	state, _, err := domain.GetState()
	if err != nil {
		return managed.ExternalDelete{}, err
	}

	// Stop domain if running
	if libvirt.DomainState(state) == libvirt.DOMAIN_RUNNING {
		if err := c.client.DomainDestroy(domain); err != nil {
			return managed.ExternalDelete{}, errors.Wrap(err, "cannot destroy domain")
		}
	}

	// Undefine domain
	if err := c.client.DomainUndefine(domain); err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, "cannot undefine domain")
	}

	return managed.ExternalDelete{}, nil
}

func (c *external) Disconnect(_ context.Context) error {
	return nil
}

// generateDomainXML creates libvirt domain XML definition
func (c *external) generateDomainXML(cr *v1beta1.Domain) string {
	params := cr.Spec.ForProvider

	domainType := "kvm"
	if params.Type != "" {
		domainType = params.Type
	}

	arch := "x86_64"
	if params.Arch != "" {
		arch = params.Arch
	}

	xml := fmt.Sprintf(`<domain type='%s'>
  <name>%s</name>
  <memory unit='bytes'>%d</memory>
  <currentMemory unit='bytes'>%d</currentMemory>
  <vcpu placement='static'>%d</vcpu>
  <os>
    <type arch='%s'`,
		domainType,
		params.Name,
		params.Memory,
		params.Memory,
		params.Vcpu,
		arch)

	// Add machine type if specified
	if params.Machine != "" {
		xml += fmt.Sprintf(` machine='%s'`, params.Machine)
	}

	xml += `>hvm</type>`

	// Add boot devices if specified
	if len(params.Boot) > 0 {
		for _, bootDev := range params.Boot {
			xml += fmt.Sprintf(`
    <boot dev='%s'/>`, bootDev)
		}
	}

	xml += `
  </os>
  <devices>`

	// Add emulator
	xml += "\n    <emulator>/usr/bin/qemu-system-" + arch + "</emulator>"

	// Add disks
	for i, disk := range params.Disk {
		device := "vda"
		if disk.Device != "" {
			device = disk.Device
		} else if i > 0 {
			device = fmt.Sprintf("vd%c", 'a'+rune(i))
		}

		bus := "virtio"
		if disk.Bus != "" {
			bus = disk.Bus
		}

		source := ""
		if disk.File != "" {
			source = disk.File
		}

		if source != "" {
			bootOrder := ""
			if disk.BootOrder != nil {
				bootOrder = fmt.Sprintf(` boot='%d'`, *disk.BootOrder)
			}

			wwn := ""
			if disk.WWN != "" {
				wwn = fmt.Sprintf(`
      <wwn>%s</wwn>`, disk.WWN)
			}

			deviceType := "disk"
			if disk.Type == "cdrom" {
				deviceType = "cdrom"
			}
			xml += fmt.Sprintf(`
    <disk type='file' device='%s'%s>
      <driver name='qemu' type='raw'/>
      <source file='%s'/>
      <target dev='%s' bus='%s'/>%s
    </disk>`, deviceType, bootOrder, source, device, bus, wwn)
		}
	}

	// Add network interfaces
	for _, ni := range params.NetworkInterface {
		network := ni.NetworkName
		mac := ""
		model := "virtio"

		if ni.Model != "" {
			model = ni.Model
		}

		if ni.Mac != "" {
			mac = fmt.Sprintf(`<mac address='%s'/>`, ni.Mac)
		}

		if network != "" {
			waitLease := ""
			if ni.WaitForLease {
				waitLease = ` waitForLease='yes'`
			}

			vlan := ""
			if ni.Vlan != nil {
				nativeMode := ""
				if ni.Vlan.NativeMode != "" {
					nativeMode = fmt.Sprintf(` nativeMode='%s'`, ni.Vlan.NativeMode)
				}
				vlan = fmt.Sprintf(`
      <vlan>
        <tag id='%d'%s/>
      </vlan>`, ni.Vlan.ID, nativeMode)
			}

			xml += fmt.Sprintf(`
    <interface type='network'>
      %s
      <source network='%s'%s/>
      <model type='%s'/>%s
    </interface>`, mac, network, waitLease, model, vlan)
		}
	}

	// Add console
	if len(params.Console) > 0 {
		for _, c := range params.Console {
			cType := "pty"
			if c.Type != "" {
				cType = c.Type
			}
			target := `<target type='virtio'/>`
			if c.Target != "" {
				target = fmt.Sprintf(`<target type='%s'/>`, c.Target)
			}
			xml += fmt.Sprintf(`
    <console type='%s'>
      %s
    </console>`, cType, target)
		}
	} else {
		xml += `
    <console type='pty'>
      <target type='virtio'/>
    </console>`
	}

	// Add graphics
	if len(params.Graphics) > 0 {
		for _, g := range params.Graphics {
			gType := "vnc"
			if g.Type != "" {
				gType = g.Type
			}

			port := ""
			if g.Port != nil {
				port = fmt.Sprintf(` port='%d'`, *g.Port)
			} else if g.Autoport {
				port = ` port='-1' autoport='yes'`
			}

			listen := ""
			if g.Listen != "" {
				listen = fmt.Sprintf(` listen='%s'`, g.Listen)
			} else if g.ListenAddress != "" {
				listen = fmt.Sprintf(` listen='%s'`, g.ListenAddress)
			}

			xml += fmt.Sprintf(`
    <graphics type='%s'%s%s/>`, gType, port, listen)
		}
	} else {
		xml += `
    <graphics type='vnc' port='-1' autoport='yes'/>`
	}

	xml += `
  </devices>
</domain>`

	return xml
}

// formatDomainState returns human-readable domain state
func formatDomainState(state libvirt.DomainState) string {
	switch state {
	case libvirt.DOMAIN_NOSTATE:
		return "nostate"
	case libvirt.DOMAIN_RUNNING:
		return "running"
	case libvirt.DOMAIN_BLOCKED:
		return "blocked"
	case libvirt.DOMAIN_PAUSED:
		return "paused"
	case libvirt.DOMAIN_SHUTDOWN:
		return "shutdown"
	case libvirt.DOMAIN_SHUTOFF:
		return "shutoff"
	case libvirt.DOMAIN_CRASHED:
		return "crashed"
	case libvirt.DOMAIN_PMSUSPENDED:
		return "pmsuspended"
	default:
		return "unknown"
	}
}

// boolToInt converts boolean to libvirt int (0 or 1)
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
