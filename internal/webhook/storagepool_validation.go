/*
Copyright 2025 Ross Golder
*/

package webhook

import (
	"context"

	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/rossigee/provider-libvirt/apis/v1alpha1"
)

// validateStoragePool validates StoragePool resource specifications
func (v *ValidationWebhook) validateStoragePool(ctx context.Context, pool *v1alpha1.StoragePool, oldPool *v1alpha1.StoragePool) (field.ErrorList, admission.Warnings) {
	var allErrs field.ErrorList
	var warnings admission.Warnings

	spec := pool.Spec.ForProvider
	specPath := field.NewPath("spec", "forProvider")

	// Validate basic fields
	allErrs = append(allErrs, validateResourceName(spec.Name, specPath.Child("name"))...)

	// Validate storage pool type
	validTypes := []string{"dir", "fs", "netfs", "logical", "disk", "iscsi", "scsi", "mpath", "rbd", "sheepdog", "gluster", "zfs"}
	if spec.Type == "" {
		allErrs = append(allErrs, field.Required(specPath.Child("type"), "storage pool type is required"))
	} else if !contains(validTypes, spec.Type) {
		allErrs = append(allErrs, field.NotSupported(specPath.Child("type"), spec.Type, validTypes))
	}

	// Validate target configuration
	if spec.Target == nil {
		allErrs = append(allErrs, field.Required(specPath.Child("target"), "target configuration is required"))
	} else {
		// Validate target path
		if spec.Target.Path == "" {
			allErrs = append(allErrs, field.Required(specPath.Child("target", "path"), "target path is required"))
		} else {
			allErrs = append(allErrs, validateFilePath(spec.Target.Path, specPath.Child("target", "path"))...)
		}
	}

	// Validate capacity if specified
	if spec.Capacity != nil {
		if *spec.Capacity <= 0 {
			allErrs = append(allErrs, field.Invalid(specPath.Child("capacity"), *spec.Capacity, "capacity must be greater than 0"))
		}
	}

	// Note: Allocation field validation removed as it doesn't exist in current API

	// Validate update constraints
	if oldPool != nil {
		updateErrs, updateWarns := v.validateStoragePoolUpdate(pool, oldPool, specPath)
		allErrs = append(allErrs, updateErrs...)
		warnings = append(warnings, updateWarns...)
	}

	return allErrs, warnings
}

// Note: Type-specific pool validation functions removed for simplicity.
// Basic validation is sufficient for the initial webhook implementation.

// validateStoragePoolUpdate validates constraints for StoragePool updates
func (v *ValidationWebhook) validateStoragePoolUpdate(newPool, oldPool *v1alpha1.StoragePool, fldPath *field.Path) (field.ErrorList, admission.Warnings) {
	var allErrs field.ErrorList
	var warnings admission.Warnings

	newSpec := newPool.Spec.ForProvider
	oldSpec := oldPool.Spec.ForProvider

	// Pool name cannot be changed
	if newSpec.Name != oldSpec.Name {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("name"), newSpec.Name, "storage pool name cannot be changed"))
	}

	// Pool type cannot be changed
	if newSpec.Type != oldSpec.Type {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("type"), newSpec.Type, "storage pool type cannot be changed"))
	}

	// Target path cannot be changed for most pool types
	if !equalTargetPath(newSpec.Target, oldSpec.Target) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("target", "path"), newSpec.Target, "storage pool target path cannot be changed"))
	}

	// Source configuration cannot be changed
	if !equalSource(newSpec.Source, oldSpec.Source) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("source"), newSpec.Source, "storage pool source cannot be changed"))
	}

	// Note: Capacity update validation removed as Capacity field may not exist in current API

	return allErrs, warnings
}

// Helper functions

// Note: splitHostPort function moved to utils.go to avoid duplication

// equalSource compares two source configurations for equality
func equalSource(src1, src2 *v1alpha1.StoragePoolSource) bool {
	if src1 == nil && src2 == nil {
		return true
	}
	if src1 == nil || src2 == nil {
		return false
	}
	
	// This is a simplified comparison - compare key fields
	return equalDevice(src1.Device, src2.Device) &&
		equalHost(src1.Host, src2.Host) &&
		src1.Dir == src2.Dir &&
		src1.Name == src2.Name &&
		equalFormat(src1.Format, src2.Format)
}

// equalTargetPath compares two target configurations for path equality
func equalTargetPath(target1, target2 *v1alpha1.StoragePoolTarget) bool {
	if target1 == nil && target2 == nil {
		return true
	}
	if target1 == nil || target2 == nil {
		return false
	}
	return target1.Path == target2.Path
}

// equalDevice compares two device configurations
func equalDevice(dev1, dev2 *v1alpha1.StoragePoolDevice) bool {
	if dev1 == nil && dev2 == nil {
		return true
	}
	if dev1 == nil || dev2 == nil {
		return false
	}
	return dev1.Path == dev2.Path
}

// equalHost compares two host configurations
func equalHost(host1, host2 *v1alpha1.StoragePoolHost) bool {
	if host1 == nil && host2 == nil {
		return true
	}
	if host1 == nil || host2 == nil {
		return false
	}
	return host1.Name == host2.Name
}

// equalFormat compares two format configurations
func equalFormat(fmt1, fmt2 *v1alpha1.StoragePoolFormat) bool {
	if fmt1 == nil && fmt2 == nil {
		return true
	}
	if fmt1 == nil || fmt2 == nil {
		return false
	}
	return fmt1.Type == fmt2.Type
}