/*
Copyright 2025 Ross Golder
*/

package webhook

import (
	"context"
	"net"

	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/rossigee/provider-libvirt/apis/v1alpha1"
)

// validateNetwork validates Network resource specifications
func (v *ValidationWebhook) validateNetwork(ctx context.Context, network *v1alpha1.Network, oldNetwork *v1alpha1.Network) (field.ErrorList, admission.Warnings) {
	var allErrs field.ErrorList
	var warnings admission.Warnings

	spec := network.Spec.ForProvider
	specPath := field.NewPath("spec", "forProvider")

	// Validate basic fields
	allErrs = append(allErrs, validateResourceName(spec.Name, specPath.Child("name"))...)

	// Validate network mode
	validModes := []string{"nat", "isolated", "bridge", "routed", "open"}
	if spec.Mode != "" && !contains(validModes, spec.Mode) {
		allErrs = append(allErrs, field.NotSupported(specPath.Child("mode"), spec.Mode, validModes))
	}

	// Validate bridge configuration
	if spec.Bridge != nil {
		bridgeErrs := v.validateNetworkBridge(spec.Bridge, specPath.Child("bridge"))
		allErrs = append(allErrs, bridgeErrs...)
	}

	// Validate IP configuration
	if spec.IP != nil {
		ipErrs, ipWarns := v.validateNetworkIP(spec.IP, specPath.Child("ip"))
		allErrs = append(allErrs, ipErrs...)
		warnings = append(warnings, ipWarns...)
	}

	// Validate domain configuration
	if spec.Domain != nil {
		domainErrs := v.validateNetworkDomain(spec.Domain, specPath.Child("domain"))
		allErrs = append(allErrs, domainErrs...)
	}

	// Validate DHCP configuration
	if spec.DHCP != nil {
		dhcpErrs := v.validateNetworkDHCP(spec.DHCP, specPath.Child("dhcp"))
		allErrs = append(allErrs, dhcpErrs...)
	}

	// Validate DNS configuration
	if spec.DNS != nil {
		dnsErrs := v.validateNetworkDNS(spec.DNS, specPath.Child("dns"))
		allErrs = append(allErrs, dnsErrs...)
	}

	// Validate update constraints
	if oldNetwork != nil {
		updateErrs, updateWarns := v.validateNetworkUpdate(network, oldNetwork, specPath)
		allErrs = append(allErrs, updateErrs...)
		warnings = append(warnings, updateWarns...)
	}

	return allErrs, warnings
}

// validateNetworkBridge validates Network bridge configuration
func (v *ValidationWebhook) validateNetworkBridge(bridge *v1alpha1.NetworkBridge, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	// Validate bridge name
	if bridge.Name != "" {
		// Bridge names should be valid interface names
		if len(bridge.Name) > 15 {
			allErrs = append(allErrs, field.TooLong(fldPath.Child("name"), bridge.Name, 15))
		}
		
		// Bridge names should follow Linux interface naming conventions
		bridgeNamePattern := `^[a-zA-Z0-9][a-zA-Z0-9_-]*$`
		if !matchesPattern(bridge.Name, bridgeNamePattern) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("name"), bridge.Name, "invalid bridge name pattern"))
		}
	}

	// Validate bridge delay (using the actual field name)
	if bridge.Delay != nil {
		if *bridge.Delay < 0 || *bridge.Delay > 30 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("delay"), *bridge.Delay, "bridge delay must be between 0 and 30 seconds"))
		}
	}

	return allErrs
}

// validateNetworkIP validates Network IP configuration
func (v *ValidationWebhook) validateNetworkIP(ip *v1alpha1.NetworkIP, fldPath *field.Path) (field.ErrorList, admission.Warnings) {
	var allErrs field.ErrorList
	var warnings admission.Warnings

	// Validate IP address
	if ip.Address != "" {
		allErrs = append(allErrs, validateIPAddress(ip.Address, fldPath.Child("address"))...)
	}

	// Validate netmask or prefix
	if ip.Netmask != "" && ip.Prefix != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, ip, "specify either netmask or prefix, not both"))
	}

	if ip.Netmask != "" {
		// Validate netmask format
		if net.ParseIP(ip.Netmask) == nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("netmask"), ip.Netmask, "invalid netmask"))
		}
	}

	if ip.Prefix != nil {
		// Validate prefix length
		if *ip.Prefix < 1 || *ip.Prefix > 32 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("prefix"), *ip.Prefix, "prefix must be between 1 and 32"))
		}
	}

	// Validate family
	validFamilies := []string{"ipv4", "ipv6"}
	if ip.Family != "" && !contains(validFamilies, ip.Family) {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("family"), ip.Family, validFamilies))
	}

	// Note: LocalPtr field doesn't exist in current API

	return allErrs, warnings
}

// validateNetworkDomain validates Network domain configuration
func (v *ValidationWebhook) validateNetworkDomain(domain *v1alpha1.NetworkDomain, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	// Validate domain name
	if domain.Name != "" {
		// Domain names should be valid DNS names
		domainPattern := `^[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?)*$`
		if !matchesPattern(domain.Name, domainPattern) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("name"), domain.Name, "invalid domain name format"))
		}
	}

	// Validate local only setting
	if domain.LocalOnly != nil {
		// No specific validation needed for boolean
	}

	return allErrs
}

// validateNetworkDHCP validates Network DHCP configuration
func (v *ValidationWebhook) validateNetworkDHCP(dhcp *v1alpha1.NetworkDHCP, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	// Validate DHCP range
	if dhcp.Range != nil {
		rangeErrs := v.validateDHCPRange(dhcp.Range, fldPath.Child("range"))
		allErrs = append(allErrs, rangeErrs...)
	}

	// Validate static hosts 
	for i, host := range dhcp.Hosts {
		hostPath := fldPath.Child("hosts").Index(i)
		hostErrs := v.validateDHCPHost(&host, hostPath)
		allErrs = append(allErrs, hostErrs...)
	}

	// Validate bootp configuration
	if dhcp.Bootp != nil {
		bootpErrs := v.validateDHCPBootp(dhcp.Bootp, fldPath.Child("bootp"))
		allErrs = append(allErrs, bootpErrs...)
	}

	return allErrs
}

// validateDHCPRange validates DHCP range configuration
func (v *ValidationWebhook) validateDHCPRange(dhcpRange *v1alpha1.NetworkDHCPRange, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	// Validate start IP
	if dhcpRange.Start != "" {
		allErrs = append(allErrs, validateIPAddress(dhcpRange.Start, fldPath.Child("start"))...)
	}

	// Validate end IP
	if dhcpRange.End != "" {
		allErrs = append(allErrs, validateIPAddress(dhcpRange.End, fldPath.Child("end"))...)
	}

	// Validate that start IP is before end IP
	if dhcpRange.Start != "" && dhcpRange.End != "" {
		startIP := net.ParseIP(dhcpRange.Start)
		endIP := net.ParseIP(dhcpRange.End)
		if startIP != nil && endIP != nil {
			// Compare IP addresses as integers
			if compareIPs(startIP, endIP) > 0 {
				allErrs = append(allErrs, field.Invalid(fldPath, dhcpRange, "start IP must be less than or equal to end IP"))
			}
		}
	}

	return allErrs
}

// validateDHCPHost validates DHCP static host configuration
func (v *ValidationWebhook) validateDHCPHost(host *v1alpha1.NetworkDHCPHost, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	// Validate MAC address
	if host.MAC != "" {
		allErrs = append(allErrs, validateMACAddress(host.MAC, fldPath.Child("mac"))...)
	}

	// Validate IP address
	if host.IP != "" {
		allErrs = append(allErrs, validateIPAddress(host.IP, fldPath.Child("ip"))...)
	}

	// Validate hostname
	if host.Name != "" {
		hostnamePattern := `^[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?$`
		if !matchesPattern(host.Name, hostnamePattern) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("name"), host.Name, "invalid hostname format"))
		}
	}

	return allErrs
}

// validateDHCPBootp validates DHCP bootp configuration
func (v *ValidationWebhook) validateDHCPBootp(bootp *v1alpha1.NetworkBootp, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	// Validate boot file
	if bootp.File != "" {
		// Boot file should be a valid file path or filename
		if len(bootp.File) > 255 {
			allErrs = append(allErrs, field.TooLong(fldPath.Child("file"), bootp.File, 255))
		}
	}

	// Validate server address
	if bootp.Server != "" {
		allErrs = append(allErrs, validateIPAddress(bootp.Server, fldPath.Child("server"))...)
	}

	return allErrs
}

// validateNetworkDNS validates Network DNS configuration
func (v *ValidationWebhook) validateNetworkDNS(dns *v1alpha1.NetworkDNS, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	// Validate enable setting
	if dns.Enable != nil {
		// No specific validation needed for boolean
	}

	// Note: Complex DNS validation removed as many fields don't exist in current API
	// Basic DNS enable validation is sufficient for now

	return allErrs
}

// validateDNSForwarder validates DNS forwarder configuration
// Note: DNS forwarder and host validation functions removed as these types don't exist in current API

// validateNetworkUpdate validates constraints for Network updates
func (v *ValidationWebhook) validateNetworkUpdate(newNetwork, oldNetwork *v1alpha1.Network, fldPath *field.Path) (field.ErrorList, admission.Warnings) {
	var allErrs field.ErrorList
	var warnings admission.Warnings

	newSpec := newNetwork.Spec.ForProvider
	oldSpec := oldNetwork.Spec.ForProvider

	// Network name cannot be changed
	if newSpec.Name != oldSpec.Name {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("name"), newSpec.Name, "network name cannot be changed"))
	}

	// Mode cannot be changed
	if newSpec.Mode != oldSpec.Mode {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("mode"), newSpec.Mode, "network mode cannot be changed"))
	}

	// Bridge name cannot be changed
	if (newSpec.Bridge == nil) != (oldSpec.Bridge == nil) ||
		(newSpec.Bridge != nil && oldSpec.Bridge != nil && newSpec.Bridge.Name != oldSpec.Bridge.Name) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("bridge"), newSpec.Bridge, "bridge configuration cannot be changed"))
	}

	// IP configuration changes require network restart
	if !equalIPConfig(newSpec.IP, oldSpec.IP) {
		warnings = append(warnings, "IP configuration changes require network restart")
	}

	return allErrs, warnings
}

// compareIPs compares two IP addresses
func compareIPs(ip1, ip2 net.IP) int {
	// Convert to 4-byte representation for IPv4
	ip1 = ip1.To4()
	ip2 = ip2.To4()
	if ip1 == nil || ip2 == nil {
		return 0 // Invalid comparison
	}

	for i := 0; i < 4; i++ {
		if ip1[i] < ip2[i] {
			return -1
		}
		if ip1[i] > ip2[i] {
			return 1
		}
	}
	return 0
}

// equalIPConfig compares two IP configurations for equality
func equalIPConfig(ip1, ip2 *v1alpha1.NetworkIP) bool {
	if ip1 == nil && ip2 == nil {
		return true
	}
	if ip1 == nil || ip2 == nil {
		return false
	}
	
	return ip1.Address == ip2.Address &&
		ip1.Netmask == ip2.Netmask &&
		((ip1.Prefix == nil && ip2.Prefix == nil) || 
		 (ip1.Prefix != nil && ip2.Prefix != nil && *ip1.Prefix == *ip2.Prefix)) &&
		ip1.Family == ip2.Family
}