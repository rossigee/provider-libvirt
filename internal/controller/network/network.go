/*
Copyright 2025 Ross Golder
*/

package network

import (
	"context"
	"fmt"
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

// NetworkClient defines libvirt operations needed for networks
type NetworkClient interface {
	NetworkLookupByName(name string) (*libvirt.Network, error)
	NetworkDefineXML(xml string) (*libvirt.Network, error)
	NetworkCreate(n *libvirt.Network) error
	NetworkDestroy(n *libvirt.Network) error
	NetworkUndefine(n *libvirt.Network) error
	NetworkIsActive(n *libvirt.Network) (bool, error)
	NetworkIsPersistent(n *libvirt.Network) (bool, error)
	NetworkGetAutostart(n *libvirt.Network) (bool, error)
	NetworkSetAutostart(n *libvirt.Network, autostart bool) error
}

// Setup adds a controller that reconciles Network managed resources.
func Setup(mgr ctrl.Manager, l logging.Logger) error {
	name := managed.ControllerName(v1beta1.NetworkGroupKind.String())

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1beta1.NetworkGroupVersionKind),
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
		For(&v1beta1.Network{}).
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
	client NetworkClient
}

func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1beta1.Network)
	if !ok {
		return managed.ExternalObservation{}, errors.New("managed resource is not a Network custom resource")
	}

	// Lookup network by name
	network, err := c.client.NetworkLookupByName(cr.Spec.ForProvider.Name)
	if err != nil {
		if clients.IsNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, err
	}

	// Get network state
	active, err := c.client.NetworkIsActive(network)
	if err != nil {
		return managed.ExternalObservation{}, err
	}

	cr.Status.AtProvider.Active = active

	// Get persistence status
	persistent, err := c.client.NetworkIsPersistent(network)
	if err != nil {
		return managed.ExternalObservation{}, err
	}
	cr.Status.AtProvider.Persistent = persistent

	// Get autostart status
	autostart, err := c.client.NetworkGetAutostart(network)
	if err != nil {
		return managed.ExternalObservation{}, err
	}
	cr.Status.AtProvider.Autostart = autostart

	cr.Status.SetConditions(xpv1.Available())

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: true,
	}, nil
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1beta1.Network)
	if !ok {
		return managed.ExternalCreation{}, errors.New("managed resource is not a Network custom resource")
	}

	cr.Status.SetConditions(xpv1.Creating())

	xml := c.generateNetworkXML(cr)

	network, err := c.client.NetworkDefineXML(xml)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, "cannot define network")
	}

	// Start network (default)
	if err := c.client.NetworkCreate(network); err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, "cannot start network")
	}

	// Set autostart if specified
	if cr.Spec.ForProvider.Autostart != nil && *cr.Spec.ForProvider.Autostart {
		if err := c.client.NetworkSetAutostart(network, true); err != nil {
			return managed.ExternalCreation{}, errors.Wrap(err, "cannot set autostart")
		}
	}

	cr.Status.AtProvider.Active = true

	return managed.ExternalCreation{}, nil
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1beta1.Network)
	if !ok {
		return managed.ExternalUpdate{}, errors.New("managed resource is not a Network custom resource")
	}

	// Lookup existing network
	network, err := c.client.NetworkLookupByName(cr.Spec.ForProvider.Name)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, "cannot find network")
	}

	// Update autostart
	autostart := cr.Spec.ForProvider.Autostart != nil && *cr.Spec.ForProvider.Autostart
	if err := c.client.NetworkSetAutostart(network, autostart); err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, "cannot set autostart")
	}

	return managed.ExternalUpdate{}, nil
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*v1beta1.Network)
	if !ok {
		return managed.ExternalDelete{}, errors.New("managed resource is not a Network custom resource")
	}

	cr.Status.SetConditions(xpv1.Deleting())

	network, err := c.client.NetworkLookupByName(cr.Spec.ForProvider.Name)
	if err != nil {
		if clients.IsNotFound(err) {
			return managed.ExternalDelete{}, nil
		}
		return managed.ExternalDelete{}, errors.Wrap(err, "cannot find network")
	}

	// Stop network if active
	active, err := c.client.NetworkIsActive(network)
	if err != nil {
		return managed.ExternalDelete{}, err
	}

	if active {
		if err := c.client.NetworkDestroy(network); err != nil {
			return managed.ExternalDelete{}, errors.Wrap(err, "cannot destroy network")
		}
	}

	// Undefine network
	if err := c.client.NetworkUndefine(network); err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, "cannot undefine network")
	}

	return managed.ExternalDelete{}, nil
}

func (c *external) Disconnect(_ context.Context) error {
	return nil
}

// generateNetworkXML creates libvirt network XML definition
func (c *external) generateNetworkXML(cr *v1beta1.Network) string {
	params := cr.Spec.ForProvider

	mode := "nat"
	if params.Mode != "" {
		mode = params.Mode
	}

	xml := fmt.Sprintf(`<network>
  <name>%s</name>
  <forward mode='%s'/>
`, params.Name, mode)

	// Add bridge if mode is bridge
	if mode == "bridge" && params.Bridge != nil {
		stpDelay := "0"
		if params.Bridge.STPDelay != nil {
			stpDelay = fmt.Sprintf("%d", *params.Bridge.STPDelay)
		}
		xml += fmt.Sprintf(`  <bridge name='%s' stp='on' delay='%s'/>
`, params.Bridge.Name, stpDelay)
	} else {
		xml += fmt.Sprintf(`  <bridge name='virbr-%s' stp='on' delay='0'/>
`, params.Name)
	}

	// Add IP configuration
	if len(params.IP) > 0 {
		for _, ipConfig := range params.IP {
			xml += fmt.Sprintf(`  <ip address='%s' netmask='%s'>
`, ipConfig.Address, ipConfig.Netmask)

			// Add DHCP configuration
			if params.DHCP != nil && (params.DHCP.Enabled == nil || *params.DHCP.Enabled) {
				xml += `    <dhcp>
`

				// Add DHCP ranges (multiple ranges if configured)
				if len(params.DHCP.Ranges) > 0 {
					for _, dhcpRange := range params.DHCP.Ranges {
						xml += fmt.Sprintf(`      <range start='%s' end='%s'/>
`, dhcpRange.Start, dhcpRange.End)

						// Add static host assignments within range
						if len(dhcpRange.Hosts) > 0 {
							for _, host := range dhcpRange.Hosts {
								xml += fmt.Sprintf(`      <host mac='%s' name='%s' ip='%s'/>
`, host.MAC, host.Name, host.IP)
							}
						}
					}
				} else if params.DHCP.Start != "" && params.DHCP.End != "" {
					// Fallback to simple range if no ranges specified
					xml += fmt.Sprintf(`      <range start='%s' end='%s'/>
`, params.DHCP.Start, params.DHCP.End)
				}

				xml += `    </dhcp>
`
			}

			xml += `  </ip>
`
		}
	}

	// Add domain configuration if provided
	if params.Domain != "" {
		xml += fmt.Sprintf(`  <domain name='%s'/>
`, params.Domain)
	}

	// Add DNS configuration if provided
	if params.DNS != nil {
		xml += `  <dns`
		if params.DNS.Enabled != nil && !*params.DNS.Enabled {
			xml += ` enable='no'`
		}
		xml += `>`

		if len(params.DNS.Forwarders) > 0 {
			for _, forwarder := range params.DNS.Forwarders {
				xml += fmt.Sprintf(`
    <forwarder addr='%s'/>`, forwarder)
			}
		}

		xml += `
  </dns>
`
	}

	xml += `</network>`

	return xml
}
