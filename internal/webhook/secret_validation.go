/*
Copyright 2025 Ross Golder
*/

package webhook

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/rossigee/provider-libvirt/apis/v1alpha1"
)

// validateSecret validates Secret resource specifications
func (v *ValidationWebhook) validateSecret(ctx context.Context, secret *v1alpha1.Secret, oldSecret *v1alpha1.Secret) (field.ErrorList, admission.Warnings) {
	var allErrs field.ErrorList
	var warnings admission.Warnings

	spec := secret.Spec.ForProvider
	specPath := field.NewPath("spec", "forProvider")

	// Validate secret type
	validTypes := []string{"volume", "ceph", "iscsi", "tls"}
	if spec.Type == "" {
		allErrs = append(allErrs, field.Required(specPath.Child("type"), "secret type is required"))
	} else if !contains(validTypes, spec.Type) {
		allErrs = append(allErrs, field.NotSupported(specPath.Child("type"), spec.Type, validTypes))
	}

	// Validate usage type
	validUsages := []string{"encryption", "authentication"}
	if spec.Usage == "" {
		allErrs = append(allErrs, field.Required(specPath.Child("usage"), "secret usage is required"))
	} else if !contains(validUsages, spec.Usage) {
		allErrs = append(allErrs, field.NotSupported(specPath.Child("usage"), spec.Usage, validUsages))
	}

	// Validate type and usage combinations
	if spec.Type != "" && spec.Usage != "" {
		validCombinations := map[string][]string{
			"volume": {"encryption"},
			"ceph":   {"authentication"},
			"iscsi":  {"authentication"},
			"tls":    {"authentication"},
		}
		
		if allowedUsages, exists := validCombinations[spec.Type]; exists {
			if !contains(allowedUsages, spec.Usage) {
				allErrs = append(allErrs, field.Invalid(
					specPath.Child("usage"),
					spec.Usage,
					fmt.Sprintf("usage '%s' is not valid for secret type '%s', allowed: %v", spec.Usage, spec.Type, allowedUsages)))
			}
		}
	}

	// Validate data based on type
	dataPath := specPath.Child("data")
	switch spec.Type {
	case "volume":
		volErrs, volWarns := v.validateVolumeSecretData(ctx, spec.Data.Volume, dataPath.Child("volume"))
		allErrs = append(allErrs, volErrs...)
		warnings = append(warnings, volWarns...)
	case "ceph":
		cephErrs, cephWarns := v.validateCephSecretData(ctx, spec.Data.Ceph, dataPath.Child("ceph"))
		allErrs = append(allErrs, cephErrs...)
		warnings = append(warnings, cephWarns...)
	case "iscsi":
		iscsiErrs, iscsiWarns := v.validateISCSISecretData(ctx, spec.Data.ISCSI, dataPath.Child("iscsi"))
		allErrs = append(allErrs, iscsiErrs...)
		warnings = append(warnings, iscsiWarns...)
	case "tls":
		tlsErrs, tlsWarns := v.validateTLSSecretData(ctx, spec.Data.TLS, dataPath.Child("tls"))
		allErrs = append(allErrs, tlsErrs...)
		warnings = append(warnings, tlsWarns...)
	}

	// Validate description length
	if len(spec.Description) > 255 {
		allErrs = append(allErrs, field.TooLong(specPath.Child("description"), spec.Description, 255))
	}

	// Validate update constraints
	if oldSecret != nil {
		updateErrs, updateWarns := v.validateSecretUpdate(secret, oldSecret, specPath)
		allErrs = append(allErrs, updateErrs...)
		warnings = append(warnings, updateWarns...)
	}

	return allErrs, warnings
}

// validateVolumeSecretData validates volume secret data
func (v *ValidationWebhook) validateVolumeSecretData(ctx context.Context, data *v1alpha1.VolumeSecretData, fldPath *field.Path) (field.ErrorList, admission.Warnings) {
	var allErrs field.ErrorList
	var warnings admission.Warnings

	if data == nil {
		allErrs = append(allErrs, field.Required(fldPath, "volume secret data is required"))
		return allErrs, warnings
	}

	// Ensure exactly one secret type is specified
	secretCount := 0
	if data.Passphrase != nil {
		secretCount++
	}
	if data.AESKey != nil {
		secretCount++
	}

	if secretCount == 0 {
		allErrs = append(allErrs, field.Required(fldPath, "must specify either passphrase or aesKey"))
	} else if secretCount > 1 {
		allErrs = append(allErrs, field.Invalid(fldPath, data, "specify only one of passphrase or aesKey"))
	}

	// Validate passphrase reference
	if data.Passphrase != nil {
		refErrs, refWarns := v.validateSecretReference(ctx, data.Passphrase, fldPath.Child("passphrase"))
		allErrs = append(allErrs, refErrs...)
		warnings = append(warnings, refWarns...)
	}

	// Validate AES key reference
	if data.AESKey != nil {
		refErrs, refWarns := v.validateSecretReference(ctx, data.AESKey, fldPath.Child("aesKey"))
		allErrs = append(allErrs, refErrs...)
		warnings = append(warnings, refWarns...)
	}

	// Validate format
	validFormats := []string{"luks", "qcow"}
	if data.Format != "" && !contains(validFormats, data.Format) {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("format"), data.Format, validFormats))
	}

	return allErrs, warnings
}

// validateCephSecretData validates Ceph secret data
func (v *ValidationWebhook) validateCephSecretData(ctx context.Context, data *v1alpha1.CephSecretData, fldPath *field.Path) (field.ErrorList, admission.Warnings) {
	var allErrs field.ErrorList
	var warnings admission.Warnings

	if data == nil {
		allErrs = append(allErrs, field.Required(fldPath, "ceph secret data is required"))
		return allErrs, warnings
	}

	// Username is required
	if data.Username == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("username"), "ceph username is required"))
	}

	// Key reference is required
	if data.Key == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("key"), "ceph key reference is required"))
	} else {
		refErrs, refWarns := v.validateSecretReference(ctx, data.Key, fldPath.Child("key"))
		allErrs = append(allErrs, refErrs...)
		warnings = append(warnings, refWarns...)
	}

	// Validate monitor addresses
	for i, monitor := range data.Monitors {
		monitorPath := fldPath.Child("monitors").Index(i)
		
		// Monitor should be in host:port format
		hostPort := splitHostPort(monitor)
		if len(hostPort) == 0 {
			allErrs = append(allErrs, field.Invalid(monitorPath, monitor, "invalid monitor format, expected host:port"))
			continue
		}
		
		// Validate host
		allErrs = append(allErrs, validateIPAddress(hostPort[0], monitorPath)...)
		
		// Validate port if specified
		if len(hostPort) > 1 {
			allErrs = append(allErrs, validatePortRange(hostPort[1], monitorPath)...)
		}
	}

	return allErrs, warnings
}

// validateISCSISecretData validates iSCSI secret data
func (v *ValidationWebhook) validateISCSISecretData(ctx context.Context, data *v1alpha1.ISCSISecretData, fldPath *field.Path) (field.ErrorList, admission.Warnings) {
	var allErrs field.ErrorList
	var warnings admission.Warnings

	if data == nil {
		allErrs = append(allErrs, field.Required(fldPath, "iscsi secret data is required"))
		return allErrs, warnings
	}

	// Username is required
	if data.Username == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("username"), "iscsi username is required"))
	}

	// Password reference is required
	if data.Password == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("password"), "iscsi password reference is required"))
	} else {
		refErrs, refWarns := v.validateSecretReference(ctx, data.Password, fldPath.Child("password"))
		allErrs = append(allErrs, refErrs...)
		warnings = append(warnings, refWarns...)
	}

	// Validate target IQN format if specified
	if data.Target != "" {
		iqnPattern := `^iqn\.\d{4}-\d{2}\.[a-z0-9.-]+:[a-zA-Z0-9.-_]+$`
		if !matchesPattern(data.Target, iqnPattern) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("target"), data.Target, "invalid iSCSI IQN format"))
		}
	}

	return allErrs, warnings
}

// validateTLSSecretData validates TLS secret data
func (v *ValidationWebhook) validateTLSSecretData(ctx context.Context, data *v1alpha1.TLSSecretData, fldPath *field.Path) (field.ErrorList, admission.Warnings) {
	var allErrs field.ErrorList
	var warnings admission.Warnings

	if data == nil {
		allErrs = append(allErrs, field.Required(fldPath, "tls secret data is required"))
		return allErrs, warnings
	}

	// Certificate reference is required
	if data.Certificate == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("certificate"), "tls certificate reference is required"))
	} else {
		refErrs, refWarns := v.validateSecretReference(ctx, data.Certificate, fldPath.Child("certificate"))
		allErrs = append(allErrs, refErrs...)
		warnings = append(warnings, refWarns...)
	}

	// Validate private key reference if specified
	if data.PrivateKey != nil {
		refErrs, refWarns := v.validateSecretReference(ctx, data.PrivateKey, fldPath.Child("privateKey"))
		allErrs = append(allErrs, refErrs...)
		warnings = append(warnings, refWarns...)
	}

	// Validate CA certificate reference if specified
	if data.CACertificate != nil {
		refErrs, refWarns := v.validateSecretReference(ctx, data.CACertificate, fldPath.Child("caCertificate"))
		allErrs = append(allErrs, refErrs...)
		warnings = append(warnings, refWarns...)
	}

	// Validate certificate chain reference if specified
	if data.CertificateChain != nil {
		refErrs, refWarns := v.validateSecretReference(ctx, data.CertificateChain, fldPath.Child("certificateChain"))
		allErrs = append(allErrs, refErrs...)
		warnings = append(warnings, refWarns...)
	}

	return allErrs, warnings
}

// validateSecretReference validates a Kubernetes secret reference
func (v *ValidationWebhook) validateSecretReference(ctx context.Context, ref *v1alpha1.SecretReference, fldPath *field.Path) (field.ErrorList, admission.Warnings) {
	var allErrs field.ErrorList
	var warnings admission.Warnings

	if ref == nil {
		allErrs = append(allErrs, field.Required(fldPath, "secret reference is required"))
		return allErrs, warnings
	}

	// Name is required
	if ref.Name == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "secret name is required"))
	}

	// Key is required
	if ref.Key == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("key"), "secret key is required"))
	}

	// Validate namespace (use default if not specified)
	namespace := ref.Namespace
	if namespace == "" {
		namespace = "default"
	}

	// Check if the referenced Kubernetes secret exists
	if ref.Name != "" {
		secret := &corev1.Secret{}
		secretKey := types.NamespacedName{
			Name:      ref.Name,
			Namespace: namespace,
		}
		
		if err := v.Get(ctx, secretKey, secret); err != nil {
			warnings = append(warnings, fmt.Sprintf("Referenced Kubernetes secret '%s/%s' not found: %v", namespace, ref.Name, err))
		} else if ref.Key != "" {
			// Check if the specified key exists in the secret
			if _, exists := secret.Data[ref.Key]; !exists {
				warnings = append(warnings, fmt.Sprintf("Key '%s' not found in secret '%s/%s'", ref.Key, namespace, ref.Name))
			}
		}
	}

	return allErrs, warnings
}

// validateSecretUpdate validates constraints for Secret updates
func (v *ValidationWebhook) validateSecretUpdate(newSecret, oldSecret *v1alpha1.Secret, fldPath *field.Path) (field.ErrorList, admission.Warnings) {
	var allErrs field.ErrorList
	var warnings admission.Warnings

	newSpec := newSecret.Spec.ForProvider
	oldSpec := oldSecret.Spec.ForProvider

	// Secret type cannot be changed
	if newSpec.Type != oldSpec.Type {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("type"), newSpec.Type, "secret type cannot be changed"))
	}

	// Secret usage cannot be changed
	if newSpec.Usage != oldSpec.Usage {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("usage"), newSpec.Usage, "secret usage cannot be changed"))
	}

	// Ephemeral setting cannot be changed
	if !equalBoolPtr(newSpec.Ephemeral, oldSpec.Ephemeral) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("ephemeral"), newSpec.Ephemeral, "ephemeral setting cannot be changed"))
	}

	// Private setting cannot be changed
	if !equalBoolPtr(newSpec.Private, oldSpec.Private) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("private"), newSpec.Private, "private setting cannot be changed"))
	}

	// Data structure cannot be changed (but secret values can be updated)
	if newSpec.Type == oldSpec.Type {
		switch newSpec.Type {
		case "volume":
			if !equalVolumeSecretDataStructure(newSpec.Data.Volume, oldSpec.Data.Volume) {
				warnings = append(warnings, "Volume secret data structure changes may require recreation")
			}
		case "ceph":
			if !equalCephSecretDataStructure(newSpec.Data.Ceph, oldSpec.Data.Ceph) {
				warnings = append(warnings, "Ceph secret data structure changes may require recreation")
			}
		case "iscsi":
			if !equalISCSISecretDataStructure(newSpec.Data.ISCSI, oldSpec.Data.ISCSI) {
				warnings = append(warnings, "iSCSI secret data structure changes may require recreation")
			}
		case "tls":
			if !equalTLSSecretDataStructure(newSpec.Data.TLS, oldSpec.Data.TLS) {
				warnings = append(warnings, "TLS secret data structure changes may require recreation")
			}
		}
	}

	return allErrs, warnings
}

// Helper functions for comparing data structures

func equalBoolPtr(a, b *bool) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func equalVolumeSecretDataStructure(a, b *v1alpha1.VolumeSecretData) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	
	// Check if the type of secret changed (passphrase vs AES key)
	aHasPassphrase := a.Passphrase != nil
	bHasPassphrase := b.Passphrase != nil
	aHasAESKey := a.AESKey != nil
	bHasAESKey := b.AESKey != nil
	
	return aHasPassphrase == bHasPassphrase && aHasAESKey == bHasAESKey && a.Format == b.Format
}

func equalCephSecretDataStructure(a, b *v1alpha1.CephSecretData) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	
	return a.Username == b.Username && len(a.Monitors) == len(b.Monitors)
}

func equalISCSISecretDataStructure(a, b *v1alpha1.ISCSISecretData) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	
	return a.Username == b.Username && a.Target == b.Target
}

func equalTLSSecretDataStructure(a, b *v1alpha1.TLSSecretData) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	
	// Check which certificate types are present
	aHasCert := a.Certificate != nil
	bHasCert := b.Certificate != nil
	aHasKey := a.PrivateKey != nil
	bHasKey := b.PrivateKey != nil
	aHasCA := a.CACertificate != nil
	bHasCA := b.CACertificate != nil
	aHasChain := a.CertificateChain != nil
	bHasChain := b.CertificateChain != nil
	
	return aHasCert == bHasCert && aHasKey == bHasKey && aHasCA == bHasCA && aHasChain == bHasChain
}