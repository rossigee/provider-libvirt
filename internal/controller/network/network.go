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

package network

import (
	"context"
	"fmt"
	"strings"

	"github.com/digitalocean/go-libvirt"
	"github.com/pkg/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"

	networkv1alpha1 "github.com/rossigee/provider-libvirt/apis/network/v1alpha1"
	v1beta1 "github.com/rossigee/provider-libvirt/apis/v1beta1"
	"github.com/rossigee/provider-libvirt/internal/clients"
)

const (
	errNotNetwork = "managed resource is not a Network"
)

// Setup adds a controller that reconciles Network managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(networkv1alpha1.NetworkGroupKind)

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(networkv1alpha1.NetworkGroupVersionKind),
		managed.WithExternalConnector(&connector{kube: mgr.GetClient()}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithPollInterval(o.PollInterval),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorder(name))),
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&networkv1alpha1.Network{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

type connector struct{ kube client.Client }

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*networkv1alpha1.Network)
	if !ok {
		return nil, errors.New(errNotNetwork)
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
	cr, ok := mg.(*networkv1alpha1.Network)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotNetwork)
	}

	lv := e.client.Libvirt()
	net, err := lv.NetworkLookupByName(cr.Spec.ForProvider.Name)
	if err != nil {
		if isNetworkNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, "cannot observe network")
	}

	active, err := lv.NetworkIsActive(net)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, "cannot check network active state")
	}

	bridge, err := lv.NetworkGetBridgeName(net)
	if err != nil {
		bridge = ""
	}

	cr.Status.AtProvider.UUID = formatUUID(net.UUID)
	cr.Status.AtProvider.BridgeName = bridge
	cr.Status.AtProvider.Active = active == 1
	cr.SetConditions(xpv1.Available())
	meta.SetExternalName(cr, cr.Spec.ForProvider.Name)

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: true,
	}, nil
}

func (e *external) Create(_ context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*networkv1alpha1.Network)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotNetwork)
	}

	cr.SetConditions(xpv1.Creating())
	lv := e.client.Libvirt()

	xml := buildNetworkXML(cr.Spec.ForProvider)
	net, err := lv.NetworkDefineXML(xml)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, "cannot define network")
	}

	if err := lv.NetworkCreate(net); err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, "cannot start network")
	}

	autostart := true
	if cr.Spec.ForProvider.Autostart != nil {
		autostart = *cr.Spec.ForProvider.Autostart
	}
	autostartInt := int32(0)
	if autostart {
		autostartInt = 1
	}
	if err := lv.NetworkSetAutostart(net, autostartInt); err != nil {
		ctrl.LoggerFrom(context.Background()).V(4).Info("NetworkSetAutostart warning", "error", err)
	}

	meta.SetExternalName(cr, cr.Spec.ForProvider.Name)
	return managed.ExternalCreation{}, nil
}

func (e *external) Update(_ context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	_, ok := mg.(*networkv1alpha1.Network)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotNetwork)
	}
	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(_ context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*networkv1alpha1.Network)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotNetwork)
	}

	cr.SetConditions(xpv1.Deleting())
	lv := e.client.Libvirt()

	net, err := lv.NetworkLookupByName(cr.Spec.ForProvider.Name)
	if err != nil {
		if isNetworkNotFound(err) {
			return managed.ExternalDelete{}, nil
		}
		return managed.ExternalDelete{}, errors.Wrap(err, "cannot find network for deletion")
	}

	active, _ := lv.NetworkIsActive(net)
	if active == 1 {
		if err := lv.NetworkDestroy(net); err != nil {
			return managed.ExternalDelete{}, errors.Wrap(err, "cannot destroy network")
		}
	}

	if err := lv.NetworkUndefine(net); err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, "cannot undefine network")
	}

	return managed.ExternalDelete{}, nil
}

func (e *external) Disconnect(_ context.Context) error {
	e.client.Close()
	return nil
}

func buildNetworkXML(p networkv1alpha1.NetworkParameters) string {
	mode := "nat"
	if p.Mode != nil {
		mode = *p.Mode
	}

	forward := fmt.Sprintf(`<forward mode="%s"/>`, mode)
	if mode == "none" {
		forward = ""
	}

	bridge := ""
	if p.Bridge != nil {
		bridge = fmt.Sprintf(`<bridge name="%s"/>`, *p.Bridge)
	}

	domain := ""
	if p.Domain != nil {
		domain = fmt.Sprintf(`<domain name="%s"/>`, *p.Domain)
	}

	ips := buildIPsXML(p)

	return fmt.Sprintf(`<network>
  <name>%s</name>
  %s
  %s
  %s
  %s
</network>`, p.Name, forward, bridge, domain, ips)
}

func buildIPsXML(p networkv1alpha1.NetworkParameters) string {
	if len(p.Addresses) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, cidr := range p.Addresses {
		ip, prefix, ok := parseCIDR(cidr)
		if !ok {
			continue
		}
		dhcp := buildDHCPXML(p.DHCPRanges)
		sb.WriteString(fmt.Sprintf(`<ip address="%s" prefix="%s">%s</ip>`, ip, prefix, dhcp))
	}
	return sb.String()
}

func buildDHCPXML(ranges []networkv1alpha1.NetworkDHCPRange) string {
	if len(ranges) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("<dhcp>")
	for _, r := range ranges {
		sb.WriteString(fmt.Sprintf(`<range start="%s" end="%s"/>`, r.Start, r.End))
	}
	sb.WriteString("</dhcp>")
	return sb.String()
}

func parseCIDR(cidr string) (ip, prefix string, ok bool) {
	parts := strings.SplitN(cidr, "/", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func formatUUID(uuid [16]byte) string {
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16])
}

func isNetworkNotFound(err error) bool {
	if err == nil {
		return false
	}
	lverr, ok := err.(libvirt.Error)
	if !ok {
		return false
	}
	return lverr.Code == uint32(libvirt.ErrNoNetwork)
}
