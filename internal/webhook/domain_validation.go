/*
Copyright 2025 Ross Golder
*/

package webhook

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/rossigee/provider-libvirt/apis/v1alpha1"
)

// validateDomain validates Domain resource specifications
func (v *ValidationWebhook) validateDomain(ctx context.Context, domain *v1alpha1.Domain, oldDomain *v1alpha1.Domain) (field.ErrorList, admission.Warnings) {
	var allErrs field.ErrorList
	var warnings admission.Warnings

	spec := domain.Spec.ForProvider
	specPath := field.NewPath("spec", "forProvider")

	// Validate basic fields
	allErrs = append(allErrs, validateResourceName(spec.Name, specPath.Child("name"))...)
	allErrs = append(allErrs, validateMemorySize(spec.Memory, specPath.Child("memory"))...)
	allErrs = append(allErrs, validateVcpuCount(int(spec.Vcpu), specPath.Child("vcpu"))...)

	// Validate domain type
	validTypes := []string{"kvm", "qemu", "xen", "lxc"}
	if spec.Type != "" && !contains(validTypes, spec.Type) {
		allErrs = append(allErrs, field.NotSupported(specPath.Child("type"), spec.Type, validTypes))
	}

	// Validate architecture
	validArchs := []string{"x86_64", "i686", "aarch64", "armv7l", "ppc64", "ppc64le", "s390x"}
	if spec.Arch != "" && !contains(validArchs, spec.Arch) {
		allErrs = append(allErrs, field.NotSupported(specPath.Child("arch"), spec.Arch, validArchs))
	}

	// Validate boot devices
	validBootDevs := []string{"hd", "cdrom", "network", "fd"}
	for i, boot := range spec.Boot {
		if !contains(validBootDevs, boot) {
			allErrs = append(allErrs, field.NotSupported(specPath.Child("boot").Index(i), boot, validBootDevs))
		}
	}

	// Validate disks
	for i, disk := range spec.Disk {
		diskPath := specPath.Child("disk").Index(i)
		diskErrs, diskWarns := v.validateDomainDisk(ctx, &disk, diskPath)
		allErrs = append(allErrs, diskErrs...)
		warnings = append(warnings, diskWarns...)
	}

	// Validate network interfaces
	for i, netif := range spec.NetworkInterface {
		netifPath := specPath.Child("networkInterface").Index(i)
		netifErrs, netifWarns := v.validateDomainNetworkInterface(ctx, &netif, netifPath)
		allErrs = append(allErrs, netifErrs...)
		warnings = append(warnings, netifWarns...)
	}

	// Validate console configuration
	if spec.Console != nil {
		consoleErrs := v.validateDomainConsole(spec.Console, specPath.Child("console"))
		allErrs = append(allErrs, consoleErrs...)
	}

	// Validate graphics configuration
	if spec.Graphics != nil {
		graphicsErrs := v.validateDomainGraphics(spec.Graphics, specPath.Child("graphics"))
		allErrs = append(allErrs, graphicsErrs...)
	}

	// Validate update constraints
	if oldDomain != nil {
		updateErrs, updateWarns := v.validateDomainUpdate(domain, oldDomain, specPath)
		allErrs = append(allErrs, updateErrs...)
		warnings = append(warnings, updateWarns...)
	}

	return allErrs, warnings
}

// validateDomainDisk validates Domain disk configurations
func (v *ValidationWebhook) validateDomainDisk(ctx context.Context, disk *v1alpha1.DomainDisk, fldPath *field.Path) (field.ErrorList, admission.Warnings) {
	var allErrs field.ErrorList
	var warnings admission.Warnings

	// Ensure at least one disk source is specified
	sourceCount := 0
	if disk.VolumeRef != nil {
		sourceCount++
	}
	if disk.VolumeID != "" {
		sourceCount++
		warnings = append(warnings, "volumeId is deprecated, use volumeRef instead")
	}
	if disk.File != "" {
		sourceCount++
	}

	if sourceCount == 0 {
		allErrs = append(allErrs, field.Required(fldPath, "must specify one of volumeRef, volumeId, or file"))
	} else if sourceCount > 1 {
		allErrs = append(allErrs, field.Invalid(fldPath, disk, "specify only one of volumeRef, volumeId, or file"))
	}

	// Validate file path if specified
	if disk.File != "" {
		allErrs = append(allErrs, validateFilePath(disk.File, fldPath.Child("file"))...)
	}

	// Validate Volume reference if specified
	if disk.VolumeRef != nil {
		if disk.VolumeRef.Name == "" {
			allErrs = append(allErrs, field.Required(fldPath.Child("volumeRef", "name"), "volume name is required"))
		} else {
			// Check if Volume exists and is ready
			volume := &v1alpha1.Volume{}
			if err := v.Get(ctx, types.NamespacedName{Name: disk.VolumeRef.Name}, volume); err != nil {
				warnings = append(warnings, fmt.Sprintf("Referenced Volume '%s' not found: %v", disk.VolumeRef.Name, err))
			}
		}
	}

	// Validate disk type
	validTypes := []string{"virtio", "ide", "scsi", "sata", "usb"}
	if disk.Type != "" && !contains(validTypes, disk.Type) {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("type"), disk.Type, validTypes))
	}

	// Validate device name pattern
	if disk.Device != "" {
		// Device names should follow libvirt patterns (vda, hda, sda, etc.)
		validDevicePattern := `^(vd|hd|sd|xvd)[a-z]$`
		if !matchesPattern(disk.Device, validDevicePattern) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("device"), disk.Device, "invalid device name pattern"))
		}
	}

	// Validate boot order
	if disk.BootOrder != nil {
		if *disk.BootOrder < 1 || *disk.BootOrder > 255 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("bootOrder"), *disk.BootOrder, "boot order must be between 1 and 255"))
		}
	}

	// Validate WWN format
	if disk.WWN != "" {
		wwnPattern := `^[0-9a-fA-F]{16}$`
		if !matchesPattern(disk.WWN, wwnPattern) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("wwn"), disk.WWN, "WWN must be 16 hexadecimal characters"))
		}
	}

	return allErrs, warnings
}

// validateDomainNetworkInterface validates Domain network interface configurations
func (v *ValidationWebhook) validateDomainNetworkInterface(ctx context.Context, netif *v1alpha1.DomainNetworkInterface, fldPath *field.Path) (field.ErrorList, admission.Warnings) {
	var allErrs field.ErrorList
	var warnings admission.Warnings

	// Ensure at least one network source is specified
	sourceCount := 0
	if netif.NetworkRef != nil {
		sourceCount++
	}
	if netif.NetworkName != "" {
		sourceCount++
	}
	if netif.Bridge != "" {
		sourceCount++
	}

	if sourceCount == 0 {
		allErrs = append(allErrs, field.Required(fldPath, "must specify one of networkRef, networkName, or bridge"))
	} else if sourceCount > 1 {
		allErrs = append(allErrs, field.Invalid(fldPath, netif, "specify only one of networkRef, networkName, or bridge"))
	}

	// Validate Network reference if specified
	if netif.NetworkRef != nil {
		if netif.NetworkRef.Name == "" {
			allErrs = append(allErrs, field.Required(fldPath.Child("networkRef", "name"), "network name is required"))
		} else {
			// Check if Network exists and is ready
			network := &v1alpha1.Network{}
			if err := v.Get(ctx, types.NamespacedName{Name: netif.NetworkRef.Name}, network); err != nil {
				warnings = append(warnings, fmt.Sprintf("Referenced Network '%s' not found: %v", netif.NetworkRef.Name, err))
			}
		}
	}

	// Validate model type
	validModels := []string{"virtio", "e1000", "rtl8139", "ne2k_pci", "vmxnet3"}
	if netif.Model != "" && !contains(validModels, netif.Model) {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("model"), netif.Model, validModels))
	}

	// Validate MAC address
	if netif.MAC != "" {
		allErrs = append(allErrs, validateMACAddress(netif.MAC, fldPath.Child("mac"))...)
	}

	return allErrs, warnings
}

// validateDomainConsole validates Domain console configuration
func (v *ValidationWebhook) validateDomainConsole(console *v1alpha1.DomainConsole, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	// Validate console type
	validTypes := []string{"pty", "serial", "tcp", "udp", "unix", "file"}
	if console.Type != "" && !contains(validTypes, console.Type) {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("type"), console.Type, validTypes))
	}

	return allErrs
}

// validateDomainGraphics validates Domain graphics configuration
func (v *ValidationWebhook) validateDomainGraphics(graphics *v1alpha1.DomainGraphics, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	// Validate graphics type
	validTypes := []string{"vnc", "spice", "rdp", "desktop"}
	if graphics.Type != "" && !contains(validTypes, graphics.Type) {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("type"), graphics.Type, validTypes))
	}

	// Validate listen address
	if graphics.ListenAddress != "" {
		allErrs = append(allErrs, validateIPAddress(graphics.ListenAddress, fldPath.Child("listenAddress"))...)
	}

	// Note: Port validation removed as DomainGraphics doesn't have a Port field in current API

	return allErrs
}

// validateDomainUpdate validates constraints for Domain updates
func (v *ValidationWebhook) validateDomainUpdate(newDomain, oldDomain *v1alpha1.Domain, fldPath *field.Path) (field.ErrorList, admission.Warnings) {
	var allErrs field.ErrorList
	var warnings admission.Warnings

	newSpec := newDomain.Spec.ForProvider
	oldSpec := oldDomain.Spec.ForProvider

	// Domain name cannot be changed
	if newSpec.Name != oldSpec.Name {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("name"), newSpec.Name, "domain name cannot be changed"))
	}

	// Warn about changes that require VM restart
	if newSpec.Memory != oldSpec.Memory {
		warnings = append(warnings, "Memory changes require VM restart")
	}

	if newSpec.Vcpu != oldSpec.Vcpu {
		warnings = append(warnings, "CPU changes require VM restart")
	}

	if newSpec.Type != oldSpec.Type {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("type"), newSpec.Type, "domain type cannot be changed"))
	}

	if newSpec.Arch != oldSpec.Arch {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("arch"), newSpec.Arch, "architecture cannot be changed"))
	}

	return allErrs, warnings
}