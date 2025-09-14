/*
Copyright 2025 Ross Golder
*/

package domain

import (
	"context"
	"fmt"
	"strings"

	"github.com/digitalocean/go-libvirt"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
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
	errNotDomain        = "managed resource is not a Domain custom resource"
	errTrackPCUsage     = "cannot track ProviderConfig usage"
	errGetPC            = "cannot get ProviderConfig"
	errGetCreds         = "cannot get credentials"
	errNewClient        = "cannot create new libvirt client"
	errCreateDomain     = "cannot create domain"
	errDeleteDomain     = "cannot delete domain"
	errDescribeDomain   = "cannot describe domain"
	errUpdateDomain     = "cannot update domain"
)

// Setup adds a controller that reconciles Domain managed resources.
func Setup(mgr ctrl.Manager, l logging.Logger) error {
	name := managed.ControllerName(v1alpha1.DomainGroupKind.String())

	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.DomainGroupVersionKind),
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
		For(&v1alpha1.Domain{}).
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
	_, ok := mg.(*v1alpha1.Domain)
	if !ok {
		return nil, errors.New(errNotDomain)
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
	cr, ok := mg.(*v1alpha1.Domain)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotDomain)
	}

	// Get external name (domain name in libvirt)
	domainName := meta.GetExternalName(cr)
	if domainName == "" {
		domainName = cr.Spec.ForProvider.Name
		meta.SetExternalName(cr, domainName)
	}

	// Look up domain in libvirt
	domain, err := c.service.DomainLookupByName(domainName)
	if err != nil {
		// Check if error is "domain not found"
		if isNotFound(err) {
			return managed.ExternalObservation{
				ResourceExists: false,
			}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, errDescribeDomain)
	}

	// Get domain state
	state, _, err := c.service.DomainGetState(domain, 0)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errDescribeDomain)
	}

	// Get domain info
	_, maxmem, memory, vcpu, cputime, err := c.service.DomainGetInfo(domain)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errDescribeDomain)
	}
	_ = maxmem
	_ = cputime

	// Update status
	cr.Status.AtProvider.ID = fmt.Sprintf("%d", domain.ID)
	cr.Status.AtProvider.UUID = fmt.Sprintf("%x", domain.UUID)
	cr.Status.AtProvider.State = stateToString(libvirt.DomainState(state))

	// Set Ready condition based on domain state
	var ready bool
	switch libvirt.DomainState(state) {
	case libvirt.DomainRunning:
		ready = cr.Spec.ForProvider.Running
	case libvirt.DomainShutoff:
		ready = !cr.Spec.ForProvider.Running
	default:
		ready = false
	}

	if ready {
		cr.SetConditions(xpv1.Available())
	} else {
		cr.SetConditions(xpv1.Unavailable())
	}

	return managed.ExternalObservation{
		ResourceExists:          true,
		ResourceUpToDate:        isUpToDate(cr, memory, uint32(vcpu), state),
		ConnectionDetails:       getConnectionDetails(cr, domain),
		ResourceLateInitialized: false,
	}, nil
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.Domain)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotDomain)
	}

	cr.SetConditions(xpv1.Creating())

	// Generate libvirt XML from spec
	xml, err := generateDomainXMLWithClient(cr, c.kube)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateDomain)
	}

	// Define domain in libvirt
	domain, err := c.service.DomainDefineXML(xml)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateDomain)
	}

	// Set external name
	meta.SetExternalName(cr, cr.Spec.ForProvider.Name)

	// Start domain if running is requested
	if cr.Spec.ForProvider.Running {
		err = c.service.DomainCreate(domain)
		if err != nil {
			return managed.ExternalCreation{}, errors.Wrap(err, errCreateDomain)
		}
	}

	// Enable autostart if requested
	if cr.Spec.ForProvider.Autostart {
		err = c.service.DomainSetAutostart(domain, 1)
		if err != nil {
			return managed.ExternalCreation{}, errors.Wrap(err, errCreateDomain)
		}
	}

	return managed.ExternalCreation{}, nil
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.Domain)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotDomain)
	}

	domainName := meta.GetExternalName(cr)
	domain, err := c.service.DomainLookupByName(domainName)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateDomain)
	}

	// Handle running state changes
	state, _, err := c.service.DomainGetState(domain, 0)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateDomain)
	}

	if cr.Spec.ForProvider.Running && libvirt.DomainState(state) != libvirt.DomainRunning {
		err = c.service.DomainCreate(domain)
		if err != nil {
			return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateDomain)
		}
	} else if !cr.Spec.ForProvider.Running && libvirt.DomainState(state) == libvirt.DomainRunning {
		err = c.service.DomainShutdown(domain)
		if err != nil {
			return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateDomain)
		}
	}

	return managed.ExternalUpdate{}, nil
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*v1alpha1.Domain)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotDomain)
	}

	cr.SetConditions(xpv1.Deleting())

	domainName := meta.GetExternalName(cr)
	domain, err := c.service.DomainLookupByName(domainName)
	if err != nil {
		if isNotFound(err) {
			return managed.ExternalDelete{}, nil // Already deleted
		}
		return managed.ExternalDelete{}, errors.Wrap(err, errDeleteDomain)
	}

	// Stop domain if running
	state, _, err := c.service.DomainGetState(domain, 0)
	if err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, errDeleteDomain)
	}

	if libvirt.DomainState(state) == libvirt.DomainRunning {
		err = c.service.DomainDestroy(domain)
		if err != nil {
			return managed.ExternalDelete{}, errors.Wrap(err, errDeleteDomain)
		}
	}

	// Undefine (delete) domain
	err = c.service.DomainUndefine(domain)
	if err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, errDeleteDomain)
	}

	return managed.ExternalDelete{}, nil
}

func (c *external) Disconnect(ctx context.Context) error {
	// Nothing to disconnect for libvirt client
	return nil
}

// Helper functions

func isNotFound(err error) bool {
	return strings.Contains(err.Error(), "not found") ||
		strings.Contains(err.Error(), "no domain")
}

func stateToString(state libvirt.DomainState) string {
	switch state {
	case libvirt.DomainNostate:
		return "nostate"
	case libvirt.DomainRunning:
		return "running"
	case libvirt.DomainBlocked:
		return "blocked"
	case libvirt.DomainPaused:
		return "paused"
	case libvirt.DomainShutdown:
		return "shutdown"
	case libvirt.DomainShutoff:
		return "shutoff"
	case libvirt.DomainCrashed:
		return "crashed"
	case libvirt.DomainPmsuspended:
		return "pmsuspended"
	default:
		return "unknown"
	}
}

func isUpToDate(cr *v1alpha1.Domain, memory uint64, vcpu uint32, state int32) bool {
	// Check if memory matches (libvirt returns memory in KB)
	if memory != uint64(cr.Spec.ForProvider.Memory/1024) {
		return false
	}

	// Check if vcpu matches
	if vcpu != uint32(cr.Spec.ForProvider.Vcpu) {
		return false
	}

	// Check if running state matches
	isRunning := libvirt.DomainState(state) == libvirt.DomainRunning
	return isRunning == cr.Spec.ForProvider.Running
}

func getConnectionDetails(cr *v1alpha1.Domain, domain libvirt.Domain) managed.ConnectionDetails {
	cd := managed.ConnectionDetails{}
	cd["domain-id"] = []byte(fmt.Sprintf("%d", domain.ID))
	cd["domain-uuid"] = domain.UUID[:]
	cd["domain-name"] = []byte(domain.Name)
	return cd
}

func generateDomainXML(cr *v1alpha1.Domain) (string, error) {
	return generateDomainXMLWithClient(cr, nil)
}

func generateDomainXMLWithClient(cr *v1alpha1.Domain, kube client.Client) (string, error) {
	// This is a simplified XML generator
	// In production, you'd want a more robust XML templating system
	
	spec := cr.Spec.ForProvider
	
	// Set defaults
	domainType := spec.Type
	if domainType == "" {
		domainType = "kvm"
	}
	
	arch := spec.Arch
	if arch == "" {
		arch = "x86_64"
	}

	xml := fmt.Sprintf(`<domain type='%s'>
  <name>%s</name>
  <memory unit='bytes'>%d</memory>
  <currentMemory unit='bytes'>%d</currentMemory>
  <vcpu placement='static'>%d</vcpu>
  <os>
    <type arch='%s'>hvm</type>`, 
		domainType, spec.Name, spec.Memory, spec.Memory, spec.Vcpu, arch)

	// Add boot devices
	if len(spec.Boot) > 0 {
		for _, boot := range spec.Boot {
			xml += fmt.Sprintf("\n    <boot dev='%s'/>", boot)
		}
	} else {
		xml += "\n    <boot dev='hd'/>"
	}

	xml += "\n  </os>"

	// Add basic features
	xml += `
  <features>
    <acpi/>
    <apic/>
    <vmport state='off'/>
  </features>
  <cpu mode='host-model' check='partial'/>
  <clock offset='utc'>
    <timer name='rtc' tickpolicy='catchup'/>
    <timer name='pit' tickpolicy='delay'/>
    <timer name='hpet' present='no'/>
  </clock>
  <on_poweroff>destroy</on_poweroff>
  <on_reboot>restart</on_reboot>
  <on_crash>destroy</on_crash>
  <pm>
    <suspend-to-mem enabled='no'/>
    <suspend-to-disk enabled='no'/>
  </pm>
  <devices>
    <emulator>/usr/bin/qemu-system-x86_64</emulator>`

	// Add disks
	for i, disk := range spec.Disk {
		diskType := disk.Type
		if diskType == "" {
			diskType = "virtio"
		}
		
		// Determine target device name
		target := disk.Device
		if target == "" {
			target = fmt.Sprintf("vd%c", 'a'+i)
		}
		
		// Resolve disk source (Volume reference or direct file path)
		diskSource, err := resolveDiskSource(disk, kube)
		if err != nil {
			return "", errors.Wrap(err, "failed to resolve disk source")
		}
		
		xml += fmt.Sprintf(`
    <disk type='file' device='disk'>
      <driver name='qemu' type='qcow2'/>
      <source file='%s'/>
      <target dev='%s' bus='%s'/>`, diskSource, target, diskType)
		
		// Add WWN if specified
		if disk.WWN != "" {
			xml += fmt.Sprintf(`
      <wwn>%s</wwn>`, disk.WWN)
		}
		
		// Add boot order if specified
		if disk.BootOrder != nil {
			xml += fmt.Sprintf(`
      <boot order='%d'/>`, *disk.BootOrder)
		}
		
		xml += `
    </disk>`
	}

	// Add network interfaces
	for _, netif := range spec.NetworkInterface {
		model := netif.Model
		if model == "" {
			model = "virtio"
		}
		
		// Resolve network source (Network reference or direct network name)
		networkSource, interfaceType, err := resolveNetworkSource(netif, kube)
		if err != nil {
			return "", errors.Wrap(err, "failed to resolve network source")
		}
		
		xml += fmt.Sprintf(`
    <interface type='%s'>`, interfaceType)
		
		// Add source based on interface type
		switch interfaceType {
		case "network":
			xml += fmt.Sprintf(`
      <source network='%s'/>`, networkSource)
		case "bridge":
			xml += fmt.Sprintf(`
      <source bridge='%s'/>`, networkSource)
		}
		
		xml += fmt.Sprintf(`
      <model type='%s'/>`, model)
		
		if netif.MAC != "" {
			xml += fmt.Sprintf(`
      <mac address='%s'/>`, netif.MAC)
		}
		
		xml += `
    </interface>`
	}

	// Add console
	if spec.Console != nil {
		consoleType := spec.Console.Type
		if consoleType == "" {
			consoleType = "pty"
		}
		
		xml += fmt.Sprintf(`
    <console type='%s'>
      <target type='serial' port='0'/>
    </console>`, consoleType)
	}

	// Add graphics
	if spec.Graphics != nil {
		graphicsType := spec.Graphics.Type
		if graphicsType == "" {
			graphicsType = "spice"
		}
		
		listenAddr := spec.Graphics.ListenAddress
		if listenAddr == "" {
			listenAddr = "127.0.0.1"
		}
		
		autoport := "yes"
		if !spec.Graphics.Autoport {
			autoport = "no"
		}
		
		xml += fmt.Sprintf(`
    <graphics type='%s' autoport='%s'>
      <listen type='address' address='%s'/>
    </graphics>`, graphicsType, autoport, listenAddr)
	}

	xml += `
  </devices>
</domain>`

	return xml, nil
}

// resolveDiskSource resolves a disk source from Volume reference or falls back to direct file path
func resolveDiskSource(disk v1alpha1.DomainDisk, kube client.Client) (string, error) {
	// Try Volume reference first (preferred)
	if disk.VolumeRef != nil {
		if kube == nil {
			return "", errors.New("cannot resolve Volume reference without kubernetes client")
		}
		
		volume := &v1alpha1.Volume{}
		err := kube.Get(context.Background(), types.NamespacedName{
			Name:      disk.VolumeRef.Name,
			Namespace: "", // Resources are cluster-scoped in this provider
		}, volume)
		if err != nil {
			return "", errors.Wrapf(err, "failed to get Volume %s", disk.VolumeRef.Name)
		}
		
		// Check if volume is ready
		if !volume.Status.GetCondition(xpv1.TypeReady).Equal(xpv1.Available()) {
			return "", errors.Errorf("Volume %s is not ready", disk.VolumeRef.Name)
		}
		
		// Return the volume path from status
		if volume.Status.AtProvider.Path != "" {
			return volume.Status.AtProvider.Path, nil
		}
		
		return "", errors.Errorf("Volume %s does not have a path", disk.VolumeRef.Name)
	}
	
	// Fall back to VolumeID (deprecated)
	if disk.VolumeID != "" {
		volume := &v1alpha1.Volume{}
		err := kube.Get(context.Background(), types.NamespacedName{
			Name: disk.VolumeID,
		}, volume)
		if err != nil {
			return "", errors.Wrapf(err, "failed to get Volume %s", disk.VolumeID)
		}
		
		if volume.Status.AtProvider.Path != "" {
			return volume.Status.AtProvider.Path, nil
		}
		
		return "", errors.Errorf("Volume %s does not have a path", disk.VolumeID)
	}
	
	// Fall back to direct file path
	if disk.File != "" {
		return disk.File, nil
	}
	
	return "", errors.New("no disk source specified (volumeRef, volumeId, or file required)")
}

// resolveNetworkSource resolves a network source from Network reference or falls back to direct network name
func resolveNetworkSource(netif v1alpha1.DomainNetworkInterface, kube client.Client) (string, string, error) {
	// Try Network reference first (preferred)
	if netif.NetworkRef != nil {
		if kube == nil {
			return "", "", errors.New("cannot resolve Network reference without kubernetes client")
		}
		
		network := &v1alpha1.Network{}
		err := kube.Get(context.Background(), types.NamespacedName{
			Name:      netif.NetworkRef.Name,
			Namespace: "", // Resources are cluster-scoped in this provider
		}, network)
		if err != nil {
			return "", "", errors.Wrapf(err, "failed to get Network %s", netif.NetworkRef.Name)
		}
		
		// Check if network is ready
		if !network.Status.GetCondition(xpv1.TypeReady).Equal(xpv1.Available()) {
			return "", "", errors.Errorf("Network %s is not ready", netif.NetworkRef.Name)
		}
		
		// Return the network name and determine interface type based on network mode
		networkName := network.Spec.ForProvider.Name
		if networkName == "" {
			networkName = network.Name
		}
		
		// Determine interface type based on network mode
		interfaceType := "network" // Default for libvirt networks
		if network.Spec.ForProvider.Mode == "bridge" {
			interfaceType = "bridge"
		}
		
		return networkName, interfaceType, nil
	}
	
	// Fall back to direct bridge name
	if netif.Bridge != "" {
		return netif.Bridge, "bridge", nil
	}
	
	// Fall back to direct network name
	if netif.NetworkName != "" {
		return netif.NetworkName, "network", nil
	}
	
	return "", "", errors.New("no network source specified (networkRef, bridge, or networkName required)")
}