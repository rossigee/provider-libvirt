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
	"github.com/rossigee/provider-libvirt/internal/utils"
)

// validateVolume validates Volume resource specifications
func (v *ValidationWebhook) validateVolume(ctx context.Context, volume *v1alpha1.Volume, oldVolume *v1alpha1.Volume) (field.ErrorList, admission.Warnings) {
	var allErrs field.ErrorList
	var warnings admission.Warnings

	spec := volume.Spec.ForProvider
	specPath := field.NewPath("spec", "forProvider")

	// Validate basic fields
	allErrs = append(allErrs, validateResourceName(spec.Name, specPath.Child("name"))...)

	// Validate capacity - either Size or Capacity must be specified
	capacityErrs, capacityWarns := v.validateVolumeCapacity(spec, specPath)
	allErrs = append(allErrs, capacityErrs...)
	warnings = append(warnings, capacityWarns...)

	// Validate format
	validFormats := []string{"qcow2", "raw", "vmdk", "vdi", "vhd", "qed"}
	if spec.Format != "" && !contains(validFormats, spec.Format) {
		allErrs = append(allErrs, field.NotSupported(specPath.Child("format"), spec.Format, validFormats))
	}

	// Validate storage pool - use Pool field instead of StoragePoolRef
	if spec.Pool != "" {
		// Check if StoragePool exists by name
		storagePool := &v1alpha1.StoragePool{}
		if err := v.Get(ctx, types.NamespacedName{Name: spec.Pool}, storagePool); err != nil {
			warnings = append(warnings, fmt.Sprintf("Referenced StoragePool '%s' not found: %v", spec.Pool, err))
		}
	}

	// Validate backing store configuration
	if spec.BackingStore != nil {
		if spec.BackingStore.Path == "" {
			allErrs = append(allErrs, field.Required(specPath.Child("backingStore", "path"), "backing store path is required"))
		} else {
			allErrs = append(allErrs, validateFilePath(spec.BackingStore.Path, specPath.Child("backingStore", "path"))...)
		}
		
		// Validate backing store format
		validFormats := []string{"qcow2", "raw", "vmdk", "vdi", "qed", "bochs", "cloop", "dmg", "vpc", "vhdx"}
		if spec.BackingStore.Format != "" && !contains(validFormats, spec.BackingStore.Format) {
			allErrs = append(allErrs, field.NotSupported(specPath.Child("backingStore", "format"), spec.BackingStore.Format, validFormats))
		}
	}

	// Validate encryption configuration
	if spec.Encryption != nil {
		encErrs, encWarns := v.validateVolumeEncryption(ctx, spec.Encryption, specPath.Child("encryption"))
		allErrs = append(allErrs, encErrs...)
		warnings = append(warnings, encWarns...)
	}

	// Validate update constraints
	if oldVolume != nil {
		updateErrs, updateWarns := v.validateVolumeUpdate(volume, oldVolume, specPath)
		allErrs = append(allErrs, updateErrs...)
		warnings = append(warnings, updateWarns...)
	}

	return allErrs, warnings
}

// validateVolumeCapacity validates Volume capacity configuration
func (v *ValidationWebhook) validateVolumeCapacity(spec v1alpha1.VolumeParameters, fldPath *field.Path) (field.ErrorList, admission.Warnings) {
	var allErrs field.ErrorList
	var warnings admission.Warnings

	// Check that at least one of Size or Capacity is specified
	hasSize := spec.Size != ""
	hasCapacity := spec.Capacity != nil

	if !hasSize && !hasCapacity {
		allErrs = append(allErrs, field.Required(fldPath, "either size or capacity must be specified"))
		return allErrs, warnings
	}

	// Warn if both are specified
	if hasSize && hasCapacity {
		warnings = append(warnings, "Both size and capacity specified; size takes precedence over capacity")
	}

	// If capacity field is used (legacy), validate it
	if hasCapacity {
		capacity := *spec.Capacity
		if capacity <= 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("capacity"), capacity, "capacity must be greater than 0"))
		}

		// Minimum 1MB capacity
		if capacity < 1024*1024 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("capacity"), capacity, "capacity must be at least 1MB"))
		}

		// Maximum 10TB capacity (reasonable limit)
		if capacity > 10*1024*1024*1024*1024 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("capacity"), capacity, "capacity cannot exceed 10TB"))
		}

		if hasCapacity && !hasSize {
			warnings = append(warnings, "capacity field is deprecated; use size field instead (e.g., '100G')")
		}
	}

	// If size field is used, validate it
	if hasSize {
		capacityBytes, err := utils.ParseSize(spec.Size)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("size"), spec.Size, fmt.Sprintf("invalid size format: %v", err)))
		} else {
			// Validate parsed capacity
			if capacityBytes <= 0 {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("size"), spec.Size, "size must be greater than 0"))
			}

			// Maximum 10TB capacity (reasonable limit)
			if capacityBytes > 10*1024*1024*1024*1024 {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("size"), spec.Size, "size cannot exceed 10TB"))
			}
		}
	}

	return allErrs, warnings
}

// validateVolumeEncryption validates Volume encryption configuration
func (v *ValidationWebhook) validateVolumeEncryption(ctx context.Context, encryption *v1alpha1.VolumeEncryption, fldPath *field.Path) (field.ErrorList, admission.Warnings) {
	var allErrs field.ErrorList
	var warnings admission.Warnings

	// Validate encryption format
	validFormats := []string{"luks", "qcow"}
	if encryption.Format != "" && !contains(validFormats, encryption.Format) {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("format"), encryption.Format, validFormats))
	}

	// Ensure at least one secret source is specified
	sourceCount := 0
	if encryption.SecretRef != nil {
		sourceCount++
	}
	if encryption.Secret != nil {
		sourceCount++
		warnings = append(warnings, "encryption.secret is deprecated, use secretRef instead")
	}

	if sourceCount == 0 {
		allErrs = append(allErrs, field.Required(fldPath, "must specify secretRef or secret"))
	} else if sourceCount > 1 {
		allErrs = append(allErrs, field.Invalid(fldPath, encryption, "specify only one of secretRef or secret"))
	}

	// Validate Secret reference if specified
	if encryption.SecretRef != nil {
		if encryption.SecretRef.Name == "" {
			allErrs = append(allErrs, field.Required(fldPath.Child("secretRef", "name"), "secret name is required"))
		} else {
			// Check if Secret exists and is ready
			secret := &v1alpha1.Secret{}
			if err := v.Get(ctx, types.NamespacedName{Name: encryption.SecretRef.Name}, secret); err != nil {
				warnings = append(warnings, fmt.Sprintf("Referenced Secret '%s' not found: %v", encryption.SecretRef.Name, err))
			} else {
				// Secret must be of type 'volume' with usage 'encryption'
				if secret.Spec.ForProvider.Type != "volume" {
					allErrs = append(allErrs, field.Invalid(
						fldPath.Child("secretRef"),
						encryption.SecretRef.Name,
						"referenced secret must be of type 'volume'"))
				}
				if secret.Spec.ForProvider.Usage != "encryption" {
					allErrs = append(allErrs, field.Invalid(
						fldPath.Child("secretRef"),
						encryption.SecretRef.Name,
						"referenced secret must have usage 'encryption'"))
				}
			}
		}
	}

	// Validate legacy secret configuration
	if encryption.Secret != nil {
		legacyErrs := v.validateLegacyVolumeSecret(encryption.Secret, fldPath.Child("secret"))
		allErrs = append(allErrs, legacyErrs...)
	}

	return allErrs, warnings
}

// validateLegacyVolumeSecret validates legacy Volume secret configuration
func (v *ValidationWebhook) validateLegacyVolumeSecret(secret *v1alpha1.VolumeEncryptionSecret, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	// Validate secret type
	validTypes := []string{"passphrase", "aes"}
	if secret.Type != "" && !contains(validTypes, secret.Type) {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("type"), secret.Type, validTypes))
	}

	// Validate UUID format if specified
	if secret.UUID != "" {
		uuidPattern := `^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`
		if !matchesPattern(secret.UUID, uuidPattern) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("uuid"), secret.UUID, "invalid UUID format"))
		}
	}

	return allErrs
}

// validateVolumeUpdate validates constraints for Volume updates
func (v *ValidationWebhook) validateVolumeUpdate(newVolume, oldVolume *v1alpha1.Volume, fldPath *field.Path) (field.ErrorList, admission.Warnings) {
	var allErrs field.ErrorList
	var warnings admission.Warnings

	newSpec := newVolume.Spec.ForProvider
	oldSpec := oldVolume.Spec.ForProvider

	// Volume name cannot be changed
	if newSpec.Name != oldSpec.Name {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("name"), newSpec.Name, "volume name cannot be changed"))
	}

	// Format cannot be changed
	if newSpec.Format != oldSpec.Format {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("format"), newSpec.Format, "volume format cannot be changed"))
	}

	// Storage pool cannot be changed
	if newSpec.Pool != oldSpec.Pool {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("pool"), newSpec.Pool, "storage pool cannot be changed"))
	}

	// Backing store cannot be changed
	if !equalBackingStore(newSpec.BackingStore, oldSpec.BackingStore) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("backingStore"), newSpec.BackingStore, "backing store cannot be changed"))
	}

	// Capacity can only be increased - need to resolve both old and new capacities
	oldCapacity := int64(0)
	newCapacity := int64(0)

	// Resolve old capacity
	if oldSpec.Size != "" {
		var err error
		oldCapacity, err = utils.ParseSize(oldSpec.Size)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("Cannot parse old size value: %v", err))
		}
	} else if oldSpec.Capacity != nil {
		oldCapacity = *oldSpec.Capacity
	}

	// Resolve new capacity
	if newSpec.Size != "" {
		var err error
		newCapacity, err = utils.ParseSize(newSpec.Size)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("size"), newSpec.Size, fmt.Sprintf("invalid size format: %v", err)))
		}
	} else if newSpec.Capacity != nil {
		newCapacity = *newSpec.Capacity
	}

	// Only validate if we could resolve both capacities
	if oldCapacity > 0 && newCapacity > 0 {
		if newCapacity < oldCapacity {
			allErrs = append(allErrs, field.Invalid(fldPath, newCapacity, "volume capacity cannot be decreased"))
		} else if newCapacity > oldCapacity {
			warnings = append(warnings, "Volume capacity increase requires filesystem expansion")
		}
	}

	// Encryption cannot be added or removed
	hasOldEncryption := oldSpec.Encryption != nil
	hasNewEncryption := newSpec.Encryption != nil
	if hasOldEncryption != hasNewEncryption {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("encryption"), newSpec.Encryption, "encryption cannot be added or removed after creation"))
	}

	return allErrs, warnings
}

// equalBackingStore compares two backing store configurations for equality
func equalBackingStore(bs1, bs2 *v1alpha1.VolumeBackingStore) bool {
	if bs1 == nil && bs2 == nil {
		return true
	}
	if bs1 == nil || bs2 == nil {
		return false
	}
	return bs1.Path == bs2.Path && bs1.Format == bs2.Format
}