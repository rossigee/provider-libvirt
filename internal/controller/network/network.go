/*
Copyright 2025 Ross Golder
*/

package network

import (
	"context"
	"fmt"
	"strings"

	"github.com/digitalocean/go-libvirt"
	"github.com/pkg/errors"
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
	errNotNetwork      = "managed resource is not a Network custom resource"
	errTrackPCUsage    = "cannot track ProviderConfig usage"
	errGetPC           = "cannot get ProviderConfig"
	errGetCreds        = "cannot get credentials"
	errNewClient       = "cannot create new libvirt client"
	errCreateNetwork   = "cannot create network"
	errDeleteNetwork   = "cannot delete network"
	errDescribeNetwork = "cannot describe network"
	errUpdateNetwork   = "cannot update network"
	errStartNetwork    = "cannot start network"
	errStopNetwork     = "cannot stop network"
)

// Setup adds a controller that reconciles Network managed resources.
func Setup(mgr ctrl.Manager, l logging.Logger) error {
	name := managed.ControllerName(v1alpha1.NetworkGroupKind.String())

	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.NetworkGroupVersionKind),
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
		For(&v1alpha1.Network{}).
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
	_, ok := mg.(*v1alpha1.Network)
	if !ok {
		return nil, errors.New(errNotNetwork)
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
	cr, ok := mg.(*v1alpha1.Network)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotNetwork)
	}

	// Get external name (network name in libvirt)
	networkName := meta.GetExternalName(cr)
	if networkName == "" {
		networkName = cr.Spec.ForProvider.Name
		meta.SetExternalName(cr, networkName)
	}

	// Look up network by name
	network, err := c.service.NetworkLookupByName(networkName)
	if err != nil {
		// Check if error is "network not found"
		if isNetworkNotFound(err) {
			return managed.ExternalObservation{
				ResourceExists: false,
			}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, errDescribeNetwork)
	}

	// Get network state
	active, err := c.service.NetworkIsActive(network)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errDescribeNetwork)
	}

	persistent, err := c.service.NetworkIsPersistent(network)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errDescribeNetwork)
	}

	autoStart, err := c.service.NetworkGetAutostart(network)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errDescribeNetwork)
	}

	// Get network XML for detailed information
	xml, err := c.service.NetworkGetXMLDesc(network, 0)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errDescribeNetwork)
	}

	// Parse XML to extract network details
	bridgeName := parseNetworkBridgeName(xml)

	// Update status
	cr.Status.AtProvider.UUID = fmt.Sprintf("%x", network.UUID)
	cr.Status.AtProvider.Active = active == 1
	cr.Status.AtProvider.Persistent = persistent == 1
	cr.Status.AtProvider.AutoStart = autoStart == 1
	cr.Status.AtProvider.BridgeName = bridgeName

	// TODO: Get DHCP leases if network is active and has DHCP
	// Note: DHCP lease retrieval is temporarily disabled due to OptString type complexity
	// if active == 1 && cr.Spec.ForProvider.DHCP != nil && 
	//	(cr.Spec.ForProvider.DHCP.Enabled == nil || *cr.Spec.ForProvider.DHCP.Enabled) {
	//	leases, _, err := c.service.NetworkGetDhcpLeases(network, libvirt.OptString{}, 0, 0)
	//	if err == nil {
	//		cr.Status.AtProvider.Leases = convertDHCPLeases(leases)
	//	}
	//}

	// Set Ready condition
	cr.SetConditions(xpv1.Available())

	return managed.ExternalObservation{
		ResourceExists:          true,
		ResourceUpToDate:        isNetworkUpToDate(cr, int(active), int(autoStart)),
		ConnectionDetails:       getNetworkConnectionDetails(cr, network),
		ResourceLateInitialized: false,
	}, nil
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.Network)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotNetwork)
	}

	cr.SetConditions(xpv1.Creating())

	// Generate network XML from spec
	xml, err := generateNetworkXML(cr)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateNetwork)
	}

	// Define network (create persistent configuration)
	network, err := c.service.NetworkDefineXML(xml)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateNetwork)
	}

	// Set external name
	meta.SetExternalName(cr, cr.Spec.ForProvider.Name)

	// Set autostart if requested
	autoStart := cr.Spec.ForProvider.AutoStart
	if autoStart == nil || *autoStart {
		err = c.service.NetworkSetAutostart(network, 1)
		if err != nil {
			return managed.ExternalCreation{}, errors.Wrap(err, errCreateNetwork)
		}
	}

	// Start network if autostart is enabled
	if autoStart == nil || *autoStart {
		err = c.service.NetworkCreate(network)
		if err != nil {
			return managed.ExternalCreation{}, errors.Wrap(err, errStartNetwork)
		}
	}

	return managed.ExternalCreation{}, nil
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.Network)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotNetwork)
	}

	// Network updates are limited in libvirt
	// Most changes require recreation of the network
	// We can handle autostart changes and start/stop operations

	networkName := meta.GetExternalName(cr)
	network, err := c.service.NetworkLookupByName(networkName)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateNetwork)
	}

	// Check if autostart setting needs to be updated
	currentAutoStart, err := c.service.NetworkGetAutostart(network)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateNetwork)
	}

	desiredAutoStart := cr.Spec.ForProvider.AutoStart
	if desiredAutoStart != nil {
		if (*desiredAutoStart && currentAutoStart == 0) || (!*desiredAutoStart && currentAutoStart == 1) {
			newAutoStart := 0
			if *desiredAutoStart {
				newAutoStart = 1
			}
			err = c.service.NetworkSetAutostart(network, int32(newAutoStart))
			if err != nil {
				return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateNetwork)
			}
		}
	}

	// Check if network should be started/stopped based on autostart
	active, err := c.service.NetworkIsActive(network)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateNetwork)
	}

	if desiredAutoStart != nil && *desiredAutoStart && active == 0 {
		// Should be running but isn't
		err = c.service.NetworkCreate(network)
		if err != nil {
			return managed.ExternalUpdate{}, errors.Wrap(err, errStartNetwork)
		}
	} else if desiredAutoStart != nil && !*desiredAutoStart && active == 1 {
		// Should be stopped but is running
		err = c.service.NetworkDestroy(network)
		if err != nil {
			return managed.ExternalUpdate{}, errors.Wrap(err, errStopNetwork)
		}
	}

	return managed.ExternalUpdate{}, nil
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*v1alpha1.Network)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotNetwork)
	}

	cr.SetConditions(xpv1.Deleting())

	networkName := meta.GetExternalName(cr)
	network, err := c.service.NetworkLookupByName(networkName)
	if err != nil {
		if isNetworkNotFound(err) {
			return managed.ExternalDelete{}, nil // Already deleted
		}
		return managed.ExternalDelete{}, errors.Wrap(err, errDeleteNetwork)
	}

	// Stop network if it's running
	active, err := c.service.NetworkIsActive(network)
	if err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, errDeleteNetwork)
	}

	if active == 1 {
		err = c.service.NetworkDestroy(network)
		if err != nil {
			return managed.ExternalDelete{}, errors.Wrap(err, errStopNetwork)
		}
	}

	// Undefine network (delete persistent configuration)
	err = c.service.NetworkUndefine(network)
	if err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, errDeleteNetwork)
	}

	return managed.ExternalDelete{}, nil
}

func (c *external) Disconnect(ctx context.Context) error {
	// Nothing to disconnect for libvirt client
	return nil
}

// Helper functions

func isNetworkNotFound(err error) bool {
	return strings.Contains(err.Error(), "not found") ||
		strings.Contains(err.Error(), "no network")
}

func isNetworkUpToDate(cr *v1alpha1.Network, active int, autoStart int) bool {
	// Check if autostart matches desired state
	desiredAutoStart := cr.Spec.ForProvider.AutoStart
	if desiredAutoStart != nil {
		if (*desiredAutoStart && autoStart == 0) || (!*desiredAutoStart && autoStart == 1) {
			return false
		}
	}

	// Check if network should be active based on autostart
	if desiredAutoStart != nil && *desiredAutoStart && active == 0 {
		return false
	} else if desiredAutoStart != nil && !*desiredAutoStart && active == 1 {
		return false
	}

	// Additional checks can be added here for other network properties
	return true
}

func getNetworkConnectionDetails(cr *v1alpha1.Network, network libvirt.Network) managed.ConnectionDetails {
	cd := managed.ConnectionDetails{}
	cd["network-uuid"] = []byte(fmt.Sprintf("%x", network.UUID))
	cd["network-name"] = []byte(network.Name)
	if cr.Status.AtProvider.BridgeName != "" {
		cd["bridge-name"] = []byte(cr.Status.AtProvider.BridgeName)
	}
	if cr.Spec.ForProvider.IP != nil {
		cd["network-cidr"] = []byte(cr.Spec.ForProvider.IP.Address)
	}
	return cd
}

func parseNetworkBridgeName(xml string) string {
	// This is a simplified XML parser
	// In production, you'd want a proper XML parser

	// Look for <bridge name='...' />
	start := strings.Index(xml, "<bridge")
	if start == -1 {
		return ""
	}

	nameStart := strings.Index(xml[start:], "name='")
	if nameStart == -1 {
		return ""
	}
	nameStart += start + 6 // length of "name='"

	nameEnd := strings.Index(xml[nameStart:], "'")
	if nameEnd == -1 {
		return ""
	}

	return xml[nameStart : nameStart+nameEnd]
}

// TODO: Implement DHCP lease conversion when OptString type handling is resolved
// func convertDHCPLeases(leases []libvirt.NetworkDhcpLease) []v1alpha1.NetworkLease {
//	result := make([]v1alpha1.NetworkLease, len(leases))
//	for i, lease := range leases {
//		result[i] = v1alpha1.NetworkLease{
//			MAC:      string(lease.Mac),
//			IP:       string(lease.Ipaddr),
//			Hostname: string(lease.Hostname),
//			ClientID: string(lease.Clientid),
//		}
//	}
//	return result
//}

func generateNetworkXML(cr *v1alpha1.Network) (string, error) {
	spec := cr.Spec.ForProvider

	// Start building the network XML
	xml := fmt.Sprintf("<network>\n  <name>%s</name>\n", spec.Name)

	// Add UUID if we're updating an existing network
	if cr.Status.AtProvider.UUID != "" {
		xml += fmt.Sprintf("  <uuid>%s</uuid>\n", cr.Status.AtProvider.UUID)
	}

	// Add bridge configuration
	if spec.Bridge != nil {
		xml += "  <bridge"
		if spec.Bridge.Name != "" {
			xml += fmt.Sprintf(" name='%s'", spec.Bridge.Name)
		}
		if spec.Bridge.MAC != "" {
			xml += fmt.Sprintf(" macaddr='%s'", spec.Bridge.MAC)
		}
		if spec.Bridge.STP != nil {
			if spec.Bridge.STP.Enabled != nil && *spec.Bridge.STP.Enabled {
				xml += " stp='on'"
			} else {
				xml += " stp='off'"
			}
		}
		if spec.Bridge.Delay != nil {
			xml += fmt.Sprintf(" delay='%d'", *spec.Bridge.Delay)
		}
		xml += "/>\n"
	}

	// Add domain configuration
	if spec.Domain != nil {
		xml += fmt.Sprintf("  <domain name='%s'", spec.Domain.Name)
		if spec.Domain.LocalOnly != nil && *spec.Domain.LocalOnly {
			xml += " localOnly='yes'"
		}
		xml += "/>\n"
	}

	// Add forward configuration
	if spec.Forward != nil {
		xml += fmt.Sprintf("  <forward mode='%s'", spec.Forward.Mode)
		if spec.Forward.Dev != "" {
			xml += fmt.Sprintf(" dev='%s'", spec.Forward.Dev)
		}
		if spec.Forward.Managed != nil && !*spec.Forward.Managed {
			xml += " managed='no'"
		}
		xml += ">\n"

		// Add NAT configuration
		if spec.Forward.NAT != nil {
			xml += "    <nat>\n"
			for _, port := range spec.Forward.NAT.Ports {
				xml += fmt.Sprintf("      <port start='%d' end='%d'/>\n", port.Start, port.End)
			}
			for _, addr := range spec.Forward.NAT.Addresses {
				xml += fmt.Sprintf("      <address start='%s' end='%s'/>\n", addr.Start, addr.End)
			}
			xml += "    </nat>\n"
		}

		// Add interfaces
		for _, iface := range spec.Forward.Interfaces {
			xml += fmt.Sprintf("    <interface dev='%s'", iface.Dev)
			if iface.Connections != nil {
				xml += fmt.Sprintf(" connections='%d'", *iface.Connections)
			}
			xml += "/>\n"
		}

		// Add PF configuration
		if spec.Forward.PF != nil {
			xml += fmt.Sprintf("    <pf dev='%s'/>\n", spec.Forward.PF.Dev)
		}

		xml += "  </forward>\n"
	}

	// Add IP configuration
	if spec.IP != nil {
		xml += fmt.Sprintf("  <ip address='%s'", spec.IP.Address)
		if spec.IP.Netmask != "" {
			xml += fmt.Sprintf(" netmask='%s'", spec.IP.Netmask)
		}
		if spec.IP.Prefix != nil {
			xml += fmt.Sprintf(" prefix='%d'", *spec.IP.Prefix)
		}
		if spec.IP.Family != "" && spec.IP.Family != "ipv4" {
			xml += fmt.Sprintf(" family='%s'", spec.IP.Family)
		}
		if spec.IP.Local != nil && *spec.IP.Local {
			xml += " localPtr='yes'"
		}
		xml += ">\n"

		// Add DHCP configuration
		if spec.DHCP != nil && (spec.DHCP.Enabled == nil || *spec.DHCP.Enabled) {
			xml += "    <dhcp>\n"

			// Add DHCP range
			if spec.DHCP.Range != nil {
				xml += fmt.Sprintf("      <range start='%s' end='%s'/>\n",
					spec.DHCP.Range.Start, spec.DHCP.Range.End)
			}

			// Add static host assignments
			for _, host := range spec.DHCP.Hosts {
				xml += fmt.Sprintf("      <host mac='%s' ip='%s'", host.MAC, host.IP)
				if host.Name != "" {
					xml += fmt.Sprintf(" name='%s'", host.Name)
				}
				if host.ID != "" {
					xml += fmt.Sprintf(" id='%s'", host.ID)
				}
				xml += "/>\n"
			}

			// Add bootp configuration
			if spec.DHCP.Bootp != nil {
				xml += "      <bootp"
				if spec.DHCP.Bootp.File != "" {
					xml += fmt.Sprintf(" file='%s'", spec.DHCP.Bootp.File)
				}
				if spec.DHCP.Bootp.Server != "" {
					xml += fmt.Sprintf(" server='%s'", spec.DHCP.Bootp.Server)
				}
				xml += "/>\n"
			}

			xml += "    </dhcp>\n"
		}

		xml += "  </ip>\n"
	}

	// Add DNS configuration
	if spec.DNS != nil {
		xml += "  <dns"
		if spec.DNS.Enable != nil && !*spec.DNS.Enable {
			xml += " enable='no'"
		}
		xml += ">\n"

		// Add forwarders
		for _, forwarder := range spec.DNS.Forwarders {
			xml += fmt.Sprintf("    <forwarder addr='%s'", forwarder.Address)
			if forwarder.Domain != "" {
				xml += fmt.Sprintf(" domain='%s'", forwarder.Domain)
			}
			if forwarder.Port != nil && *forwarder.Port != 53 {
				xml += fmt.Sprintf(" port='%d'", *forwarder.Port)
			}
			xml += "/>\n"
		}

		// Add host records
		for _, host := range spec.DNS.Hosts {
			xml += fmt.Sprintf("    <host ip='%s'>\n", host.IP)
			xml += fmt.Sprintf("      <hostname>%s</hostname>\n", host.Hostname)
			xml += "    </host>\n"
		}

		// Add SRV records
		for _, srv := range spec.DNS.SRV {
			xml += fmt.Sprintf("    <srv service='%s' protocol='%s' target='%s' port='%d'",
				srv.Service, srv.Protocol, srv.Target, srv.Port)
			if srv.Priority != nil {
				xml += fmt.Sprintf(" priority='%d'", *srv.Priority)
			}
			if srv.Weight != nil {
				xml += fmt.Sprintf(" weight='%d'", *srv.Weight)
			}
			if srv.Domain != "" {
				xml += fmt.Sprintf(" domain='%s'", srv.Domain)
			}
			xml += "/>\n"
		}

		// Add TXT records
		for _, txt := range spec.DNS.TXT {
			xml += fmt.Sprintf("    <txt name='%s' value='%s'/>\n", txt.Name, txt.Value)
		}

		xml += "  </dns>\n"
	}

	xml += "</network>"

	return xml, nil
}