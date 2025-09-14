/*
Copyright 2025 Ross Golder
*/

package webhook

import (
	"context"
	"strings"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/rossigee/provider-libvirt/apis/v1alpha1"
)

// NodeDeviceValidator validates NodeDevice resources
type NodeDeviceValidator struct{}

// SetupNodeDeviceWebhookWithManager sets up the webhook with the manager
func SetupNodeDeviceWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&v1alpha1.NodeDevice{}).
		WithValidator(&NodeDeviceValidator{}).
		Complete()
}

var _ webhook.CustomValidator = &NodeDeviceValidator{}

// ValidateCreate implements webhook.CustomValidator
func (v *NodeDeviceValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	nodeDevice, ok := obj.(*v1alpha1.NodeDevice)
	if !ok {
		return nil, errors.New("expected a NodeDevice")
	}

	var allErrs field.ErrorList
	allErrs = append(allErrs, v.validateNodeDeviceSpec(&nodeDevice.Spec.ForProvider, field.NewPath("spec", "forProvider"))...)

	if len(allErrs) == 0 {
		return nil, nil
	}

	return nil, allErrs.ToAggregate()
}

// ValidateUpdate implements webhook.CustomValidator
func (v *NodeDeviceValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	newNodeDevice, ok := newObj.(*v1alpha1.NodeDevice)
	if !ok {
		return nil, errors.New("expected a NodeDevice")
	}

	oldNodeDevice, ok := oldObj.(*v1alpha1.NodeDevice)
	if !ok {
		return nil, errors.New("expected a NodeDevice")
	}

	var allErrs field.ErrorList
	var warnings admission.Warnings

	// Validate the new spec
	allErrs = append(allErrs, v.validateNodeDeviceSpec(&newNodeDevice.Spec.ForProvider, field.NewPath("spec", "forProvider"))...)

	// Check for immutable fields
	allErrs = append(allErrs, v.validateImmutableFields(&oldNodeDevice.Spec.ForProvider, &newNodeDevice.Spec.ForProvider, field.NewPath("spec", "forProvider"))...)

	// Add warnings for potentially disruptive changes
	warnings = append(warnings, v.generateUpdateWarnings(&oldNodeDevice.Spec.ForProvider, &newNodeDevice.Spec.ForProvider)...)

	if len(allErrs) == 0 {
		return warnings, nil
	}

	return warnings, allErrs.ToAggregate()
}

// ValidateDelete implements webhook.CustomValidator
func (v *NodeDeviceValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	nodeDevice, ok := obj.(*v1alpha1.NodeDevice)
	if !ok {
		return nil, errors.New("expected a NodeDevice")
	}

	var warnings admission.Warnings

	// Warn about detached devices
	if nodeDevice.Spec.ForProvider.Detached {
		warnings = append(warnings, "Device is currently detached from host driver - deletion will reattach it")
	}

	// Warn about mediated devices
	if nodeDevice.Spec.ForProvider.Type == "mdev" {
		warnings = append(warnings, "Mediated device will be destroyed - any VMs using it may be affected")
	}

	return warnings, nil
}

// validateNodeDeviceSpec validates the NodeDevice specification
func (v *NodeDeviceValidator) validateNodeDeviceSpec(spec *v1alpha1.NodeDeviceParameters, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	// Validate device type specific configurations
	switch spec.Type {
	case "pci":
		allErrs = append(allErrs, v.validatePCIDevice(spec, fldPath)...)
	case "usb":
		allErrs = append(allErrs, v.validateUSBDevice(spec, fldPath)...)
	case "scsi":
		allErrs = append(allErrs, v.validateSCSIDevice(spec, fldPath)...)
	case "mdev":
		allErrs = append(allErrs, v.validateMediatedDevice(spec, fldPath)...)
	case "net", "storage":
		// These types are discovered, no specific validation needed
	default:
		allErrs = append(allErrs, field.Invalid(fldPath.Child("type"), spec.Type, "unsupported device type"))
	}

	// Note: If detached=true but no driver specified, we'll use default driver (vfio-pci)

	// Validate combinations
	if spec.Type != "mdev" && spec.MediatedDevice != nil {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("mediatedDevice"), spec.MediatedDevice, "mediatedDevice can only be specified for mdev type"))
	}

	return allErrs
}

// validatePCIDevice validates PCI device configuration
func (v *NodeDeviceValidator) validatePCIDevice(spec *v1alpha1.NodeDeviceParameters, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if spec.PCIAddress == nil && spec.Name == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("pciAddress"), "PCI address or device name must be specified for PCI devices"))
		return allErrs
	}

	if spec.PCIAddress != nil {
		allErrs = append(allErrs, v.validatePCIAddress(spec.PCIAddress, fldPath.Child("pciAddress"))...)
	}

	return allErrs
}

// validateUSBDevice validates USB device configuration
func (v *NodeDeviceValidator) validateUSBDevice(spec *v1alpha1.NodeDeviceParameters, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if spec.USBDevice == nil && spec.Name == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("usbDevice"), "USB device configuration or device name must be specified for USB devices"))
		return allErrs
	}

	if spec.USBDevice != nil {
		if spec.USBDevice.VendorID == "" && spec.USBDevice.Bus == 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("usbDevice"), spec.USBDevice, "either vendorID/productID or bus/device must be specified"))
		}

		if spec.USBDevice.VendorID != "" && spec.USBDevice.ProductID == "" {
			allErrs = append(allErrs, field.Required(fldPath.Child("usbDevice", "productId"), "productID must be specified when vendorID is provided"))
		}
	}

	return allErrs
}

// validateSCSIDevice validates SCSI device configuration
func (v *NodeDeviceValidator) validateSCSIDevice(spec *v1alpha1.NodeDeviceParameters, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if spec.SCSIDevice == nil && spec.Name == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("scsiDevice"), "SCSI device configuration or device name must be specified for SCSI devices"))
		return allErrs
	}

	if spec.SCSIDevice != nil {
		if spec.SCSIDevice.Adapter == "" {
			allErrs = append(allErrs, field.Required(fldPath.Child("scsiDevice", "adapter"), "SCSI adapter must be specified"))
		}
	}

	return allErrs
}

// validateMediatedDevice validates mediated device configuration
func (v *NodeDeviceValidator) validateMediatedDevice(spec *v1alpha1.NodeDeviceParameters, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if spec.MediatedDevice == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("mediatedDevice"), "mediated device configuration is required for mdev type"))
		return allErrs
	}

	mdev := spec.MediatedDevice

	if mdev.Parent == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("mediatedDevice", "parent"), "parent device must be specified"))
	}

	if mdev.Type == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("mediatedDevice", "type"), "mediated device type must be specified"))
	}

	// Validate UUID format if provided
	if mdev.UUID != "" && !isValidUUID(mdev.UUID) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("mediatedDevice", "uuid"), mdev.UUID, "invalid UUID format"))
	}

	return allErrs
}

// validatePCIAddress validates PCI address format
func (v *NodeDeviceValidator) validatePCIAddress(addr *v1alpha1.PCIAddress, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	// Validate hex format for PCI address components
	if !isValidHexFormat(addr.Domain, 4) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("domain"), addr.Domain, "domain must be in format 0xXXXX"))
	}

	if !isValidHexFormat(addr.Bus, 2) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("bus"), addr.Bus, "bus must be in format 0xXX"))
	}

	if !isValidHexFormat(addr.Slot, 2) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("slot"), addr.Slot, "slot must be in format 0xXX"))
	}

	if !isValidHexFormat(addr.Function, 1) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("function"), addr.Function, "function must be in format 0xX"))
	}

	return allErrs
}

// validateImmutableFields checks for changes to immutable fields
func (v *NodeDeviceValidator) validateImmutableFields(oldSpec, newSpec *v1alpha1.NodeDeviceParameters, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	// Device type is immutable
	if oldSpec.Type != newSpec.Type {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("type"), newSpec.Type, "device type cannot be changed"))
	}

	// PCI address is immutable
	if oldSpec.PCIAddress != nil && newSpec.PCIAddress != nil {
		if !v.equalPCIAddress(oldSpec.PCIAddress, newSpec.PCIAddress) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("pciAddress"), newSpec.PCIAddress, "PCI address cannot be changed"))
		}
	} else if (oldSpec.PCIAddress == nil) != (newSpec.PCIAddress == nil) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("pciAddress"), newSpec.PCIAddress, "PCI address cannot be added or removed"))
	}

	// Mediated device parent and type are immutable
	if oldSpec.Type == "mdev" && newSpec.Type == "mdev" {
		if oldSpec.MediatedDevice != nil && newSpec.MediatedDevice != nil {
			if oldSpec.MediatedDevice.Parent != newSpec.MediatedDevice.Parent {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("mediatedDevice", "parent"), newSpec.MediatedDevice.Parent, "mediated device parent cannot be changed"))
			}
			if oldSpec.MediatedDevice.Type != newSpec.MediatedDevice.Type {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("mediatedDevice", "type"), newSpec.MediatedDevice.Type, "mediated device type cannot be changed"))
			}
		}
	}

	return allErrs
}

// generateUpdateWarnings generates warnings for potentially disruptive changes
func (v *NodeDeviceValidator) generateUpdateWarnings(oldSpec, newSpec *v1alpha1.NodeDeviceParameters) admission.Warnings {
	var warnings admission.Warnings

	// Warn about detachment state changes
	if oldSpec.Detached != newSpec.Detached {
		if newSpec.Detached {
			warnings = append(warnings, "Device will be detached from host driver - may affect running VMs")
		} else {
			warnings = append(warnings, "Device will be reattached to host driver")
		}
	}

	// Warn about driver changes
	if oldSpec.Driver != newSpec.Driver && oldSpec.Detached && newSpec.Detached {
		warnings = append(warnings, "Driver change may require device reattachment")
	}

	return warnings
}

// Helper functions

func isValidUUID(uuid string) bool {
	// Basic UUID format validation
	parts := strings.Split(uuid, "-")
	if len(parts) != 5 {
		return false
	}
	lengths := []int{8, 4, 4, 4, 12}
	for i, part := range parts {
		if len(part) != lengths[i] {
			return false
		}
		for _, c := range part {
			if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
				return false
			}
		}
	}
	return true
}

func isValidHexFormat(value string, expectedLength int) bool {
	if !strings.HasPrefix(value, "0x") {
		return false
	}
	hex := value[2:]
	if len(hex) != expectedLength {
		return false
	}
	for _, c := range hex {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}

func (v *NodeDeviceValidator) equalPCIAddress(a1, a2 *v1alpha1.PCIAddress) bool {
	return a1.Domain == a2.Domain &&
		a1.Bus == a2.Bus &&
		a1.Slot == a2.Slot &&
		a1.Function == a2.Function
}