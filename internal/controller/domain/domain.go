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

package domain

import (
	"context"
	"fmt"
	"strings"

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

	domainv1alpha1 "github.com/rossigee/provider-libvirt/apis/domain/v1alpha1"
	v1beta1 "github.com/rossigee/provider-libvirt/apis/v1beta1"
	"github.com/rossigee/provider-libvirt/internal/clients"
)

const (
	errNotDomain = "managed resource is not a Domain"
)

// Setup adds a controller that reconciles Domain managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(domainv1alpha1.DomainGroupKind)

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(domainv1alpha1.DomainGroupVersionKind),
		managed.WithExternalConnector(&connector{kube: mgr.GetClient()}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithPollInterval(o.PollInterval),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorder(name))),
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&domainv1alpha1.Domain{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

type connector struct{ kube client.Client }

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*domainv1alpha1.Domain)
	if !ok {
		return nil, errors.New(errNotDomain)
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
	cr, ok := mg.(*domainv1alpha1.Domain)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotDomain)
	}

	lv := e.client.Libvirt()
	dom, err := lv.DomainLookupByName(cr.Spec.ForProvider.Name)
	if err != nil {
		if isDomainNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, "cannot observe domain")
	}

	state, _, err := lv.DomainGetState(dom, 0)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, "cannot get domain state")
	}

	cr.Status.AtProvider.UUID = formatUUID(dom.UUID)
	cr.Status.AtProvider.ID = int(dom.ID)
	cr.Status.AtProvider.State = domainStateName(libvirt.DomainState(state))
	cr.SetConditions(xpv1.Available())
	meta.SetExternalName(cr, cr.Spec.ForProvider.Name)

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: true,
	}, nil
}

func (e *external) Create(_ context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*domainv1alpha1.Domain)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotDomain)
	}

	cr.SetConditions(xpv1.Creating())
	lv := e.client.Libvirt()

	xml := buildDomainXML(cr.Spec.ForProvider)
	dom, err := lv.DomainDefineXML(xml)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, "cannot define domain")
	}

	running := true
	if cr.Spec.ForProvider.Running != nil {
		running = *cr.Spec.ForProvider.Running
	}
	if running {
		if err := lv.DomainCreate(dom); err != nil {
			return managed.ExternalCreation{}, errors.Wrap(err, "cannot start domain")
		}
	}

	autostart := false
	if cr.Spec.ForProvider.Autostart != nil {
		autostart = *cr.Spec.ForProvider.Autostart
	}
	autostartInt := int32(0)
	if autostart {
		autostartInt = 1
	}
	if err := lv.DomainSetAutostart(dom, autostartInt); err != nil {
		ctrl.LoggerFrom(context.Background()).V(4).Info("DomainSetAutostart warning", "error", err)
	}

	meta.SetExternalName(cr, cr.Spec.ForProvider.Name)
	return managed.ExternalCreation{}, nil
}

func (e *external) Update(_ context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	_, ok := mg.(*domainv1alpha1.Domain)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotDomain)
	}
	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(_ context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*domainv1alpha1.Domain)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotDomain)
	}

	cr.SetConditions(xpv1.Deleting())
	lv := e.client.Libvirt()

	dom, err := lv.DomainLookupByName(cr.Spec.ForProvider.Name)
	if err != nil {
		if isDomainNotFound(err) {
			return managed.ExternalDelete{}, nil
		}
		return managed.ExternalDelete{}, errors.Wrap(err, "cannot find domain for deletion")
	}

	// Destroy (force-off) if running.
	state, _, err := lv.DomainGetState(dom, 0)
	if err == nil && libvirt.DomainState(state) == libvirt.DomainRunning {
		if err := lv.DomainDestroy(dom); err != nil {
			return managed.ExternalDelete{}, errors.Wrap(err, "cannot destroy domain")
		}
	}

	if err := lv.DomainUndefine(dom); err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, "cannot undefine domain")
	}

	return managed.ExternalDelete{}, nil
}

func (e *external) Disconnect(_ context.Context) error {
	e.client.Close()
	return nil
}

func buildDomainXML(p domainv1alpha1.DomainParameters) string {
	memory := 512
	if p.Memory != nil {
		memory = *p.Memory
	}
	vcpu := 1
	if p.VCPU != nil {
		vcpu = *p.VCPU
	}
	arch := "x86_64"
	if p.Arch != nil {
		arch = *p.Arch
	}
	machine := "pc"
	if p.Machine != nil {
		machine = *p.Machine
	}

	os := fmt.Sprintf(`  <os>
    <type arch="%s" machine="%s">hvm</type>`, arch, machine)
	if p.Firmware != nil {
		os += fmt.Sprintf("\n    <loader readonly='yes' type='pflash'>%s</loader>", *p.Firmware)
	}
	os += "\n  </os>"

	features := `  <features>
    <acpi/>
    <apic/>
  </features>`

	devices := buildDevicesXML(p)

	return fmt.Sprintf(`<domain type="kvm">
  <name>%s</name>
  <memory unit="MiB">%d</memory>
  <vcpu>%d</vcpu>
%s
%s
  <devices>
%s
  </devices>
</domain>`, p.Name, memory, vcpu, os, features, devices)
}

func buildDevicesXML(p domainv1alpha1.DomainParameters) string {
	var sb strings.Builder

	// Emulator
	sb.WriteString("    <emulator>/usr/bin/qemu-system-x86_64</emulator>\n")

	// Disks
	diskIdx := 0
	for _, d := range p.Disks {
		bus := "virtio"
		if d.Bus != nil {
			bus = *d.Bus
		}
		dev := diskDevName(bus, diskIdx)

		if d.VolumeID != nil {
			fmt.Fprintf(&sb, `    <disk type="volume" device="disk">
      <driver name="qemu" type="qcow2"/>
      <source pool="%s" volume="%s"/>
      <target dev="%s" bus="%s"/>
    </disk>
`, volumePoolName(*d.VolumeID), volumeName(*d.VolumeID), dev, bus)
		} else if d.URL != nil {
			fmt.Fprintf(&sb, `    <disk type="network" device="disk">
      <driver name="qemu"/>
      <source protocol="%s" name="%s"/>
      <target dev="%s" bus="%s"/>
    </disk>
`, urlProtocol(*d.URL), urlPath(*d.URL), dev, bus)
		}
		diskIdx++
	}

	// Cloud-init ISO (cdrom)
	if p.Cloudinit != nil {
		fmt.Fprintf(&sb, `    <disk type="volume" device="cdrom">
      <driver name="qemu" type="raw"/>
      <source volume="%s"/>
      <target dev="sda" bus="sata"/>
      <readonly/>
    </disk>
`, *p.Cloudinit)
	}

	// Network interfaces
	for _, nic := range p.NetworkInterfaces {
		model := "virtio"
		if nic.Model != nil {
			model = *nic.Model
		}
		macAttr := ""
		if nic.MAC != nil {
			macAttr = fmt.Sprintf(`      <mac address="%s"/>`, *nic.MAC) + "\n"
		}

		if nic.NetworkID != nil {
			fmt.Fprintf(&sb, `    <interface type="network">
      <source network="%s"/>
%s      <model type="%s"/>
    </interface>
`, *nic.NetworkID, macAttr, model)
		} else if nic.Bridge != nil {
			fmt.Fprintf(&sb, `    <interface type="bridge">
      <source bridge="%s"/>
%s      <model type="%s"/>
    </interface>
`, *nic.Bridge, macAttr, model)
		}
	}

	// Serial console
	sb.WriteString(`    <serial type="pty">
      <target type="isa-serial" port="0"/>
    </serial>
    <console type="pty">
      <target type="serial" port="0"/>
    </console>
`)

	return sb.String()
}

// diskDevName returns a disk device name like vda, vdb, sda, sdb based on bus and index.
func diskDevName(bus string, idx int) string {
	prefix := "vd"
	if bus == "scsi" || bus == "sata" || bus == "ide" {
		prefix = "sd"
	}
	return fmt.Sprintf("%s%c", prefix, 'b'+rune(idx))
}

// volumePoolName extracts the pool name from "pool/volume" formatted VolumeID.
func volumePoolName(volumeID string) string {
	parts := strings.SplitN(volumeID, "/", 2)
	if len(parts) == 2 {
		return parts[0]
	}
	return "default"
}

// volumeName extracts the volume name from "pool/volume" formatted VolumeID.
func volumeName(volumeID string) string {
	parts := strings.SplitN(volumeID, "/", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return volumeID
}

// urlProtocol returns the protocol part of a URL (http, https, ftp, etc.).
func urlProtocol(u string) string {
	if idx := strings.Index(u, "://"); idx >= 0 {
		return u[:idx]
	}
	return "http"
}

// urlPath returns the host+path part of a URL without the scheme.
func urlPath(u string) string {
	if idx := strings.Index(u, "://"); idx >= 0 {
		return u[idx+3:]
	}
	return u
}

func formatUUID(uuid [16]byte) string {
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16])
}

func domainStateName(state libvirt.DomainState) string {
	switch state {
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

func isDomainNotFound(err error) bool {
	if err == nil {
		return false
	}
	lverr, ok := err.(libvirt.Error)
	if !ok {
		return false
	}
	return lverr.Code == uint32(libvirt.ErrNoDomain)
}
