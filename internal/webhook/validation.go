/*
Copyright 2025 Ross Golder
*/

package webhook

import (
	"context"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/rossigee/provider-libvirt/apis/v1alpha1"
)

// ValidationWebhook provides validation for libvirt resources
type ValidationWebhook struct {
	client.Client
}

// SetupWebhookWithManager sets up the validation webhook with the manager
func SetupWebhookWithManager(mgr ctrl.Manager) error {
	webhook := &ValidationWebhook{
		Client: mgr.GetClient(),
	}

	return ctrl.NewWebhookManagedBy(mgr).
		For(&v1alpha1.Domain{}).
		WithValidator(webhook).
		For(&v1alpha1.Volume{}).
		WithValidator(webhook).
		For(&v1alpha1.Network{}).
		WithValidator(webhook).
		For(&v1alpha1.StoragePool{}).
		WithValidator(webhook).
		For(&v1alpha1.Secret{}).
		WithValidator(webhook).
		Complete()
}

// ValidateCreate implements webhook.Validator
func (v *ValidationWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return v.validateObject(ctx, obj, nil)
}

// ValidateUpdate implements webhook.Validator
func (v *ValidationWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	return v.validateObject(ctx, newObj, oldObj)
}

// ValidateDelete implements webhook.Validator
func (v *ValidationWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	// No validation needed for delete operations
	return nil, nil
}

// validateObject routes validation to specific resource validators
func (v *ValidationWebhook) validateObject(ctx context.Context, obj runtime.Object, oldObj runtime.Object) (admission.Warnings, error) {
	var allErrs field.ErrorList
	var warnings admission.Warnings

	switch o := obj.(type) {
	case *v1alpha1.Domain:
		var oldDomain *v1alpha1.Domain
		if oldObj != nil {
			oldDomain = oldObj.(*v1alpha1.Domain)
		}
		errs, warns := v.validateDomain(ctx, o, oldDomain)
		allErrs = append(allErrs, errs...)
		warnings = append(warnings, warns...)

	case *v1alpha1.Volume:
		var oldVolume *v1alpha1.Volume
		if oldObj != nil {
			oldVolume = oldObj.(*v1alpha1.Volume)
		}
		errs, warns := v.validateVolume(ctx, o, oldVolume)
		allErrs = append(allErrs, errs...)
		warnings = append(warnings, warns...)

	case *v1alpha1.Network:
		var oldNetwork *v1alpha1.Network
		if oldObj != nil {
			oldNetwork = oldObj.(*v1alpha1.Network)
		}
		errs, warns := v.validateNetwork(ctx, o, oldNetwork)
		allErrs = append(allErrs, errs...)
		warnings = append(warnings, warns...)

	case *v1alpha1.StoragePool:
		var oldPool *v1alpha1.StoragePool
		if oldObj != nil {
			oldPool = oldObj.(*v1alpha1.StoragePool)
		}
		errs, warns := v.validateStoragePool(ctx, o, oldPool)
		allErrs = append(allErrs, errs...)
		warnings = append(warnings, warns...)

	case *v1alpha1.Secret:
		var oldSecret *v1alpha1.Secret
		if oldObj != nil {
			oldSecret = oldObj.(*v1alpha1.Secret)
		}
		errs, warns := v.validateSecret(ctx, o, oldSecret)
		allErrs = append(allErrs, errs...)
		warnings = append(warnings, warns...)
	}

	if len(allErrs) > 0 {
		return warnings, allErrs.ToAggregate()
	}

	return warnings, nil
}

// Common validation utilities

// validateResourceName validates Kubernetes resource names
func validateResourceName(name string, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if name == "" {
		allErrs = append(allErrs, field.Required(fldPath, "name is required"))
		return allErrs
	}

	// Kubernetes DNS-1123 subdomain validation
	if len(name) > 253 {
		allErrs = append(allErrs, field.TooLong(fldPath, name, 253))
	}

	if !regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`).MatchString(name) {
		allErrs = append(allErrs, field.Invalid(fldPath, name, "must be a valid DNS-1123 subdomain"))
	}

	return allErrs
}

// validateMemorySize validates memory specifications
func validateMemorySize(memory int64, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if memory <= 0 {
		allErrs = append(allErrs, field.Invalid(fldPath, memory, "memory must be greater than 0"))
	}

	// Minimum 128MB
	if memory < 128*1024*1024 {
		allErrs = append(allErrs, field.Invalid(fldPath, memory, "memory must be at least 128MB"))
	}

	// Maximum 1TB (reasonable limit)
	if memory > 1024*1024*1024*1024 {
		allErrs = append(allErrs, field.Invalid(fldPath, memory, "memory cannot exceed 1TB"))
	}

	return allErrs
}

// validateVcpuCount validates CPU specifications
func validateVcpuCount(vcpu int, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if vcpu <= 0 {
		allErrs = append(allErrs, field.Invalid(fldPath, vcpu, "vcpu must be greater than 0"))
	}

	if vcpu > 256 {
		allErrs = append(allErrs, field.Invalid(fldPath, vcpu, "vcpu cannot exceed 256"))
	}

	return allErrs
}


// validateIPAddress validates IP address format
func validateIPAddress(ip string, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if ip == "" {
		return allErrs // IP is optional in some contexts
	}

	if net.ParseIP(ip) == nil {
		allErrs = append(allErrs, field.Invalid(fldPath, ip, "invalid IP address"))
	}

	return allErrs
}

// validatePortRange validates port ranges
func validatePortRange(portRange string, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if portRange == "" {
		return allErrs
	}

	parts := strings.Split(portRange, "-")
	if len(parts) > 2 {
		allErrs = append(allErrs, field.Invalid(fldPath, portRange, "invalid port range format"))
		return allErrs
	}

	for i, part := range parts {
		port, err := strconv.Atoi(part)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Index(i), part, "invalid port number"))
			continue
		}

		if port < 1 || port > 65535 {
			allErrs = append(allErrs, field.Invalid(fldPath.Index(i), port, "port must be between 1 and 65535"))
		}
	}

	if len(parts) == 2 {
		start, _ := strconv.Atoi(parts[0])
		end, _ := strconv.Atoi(parts[1])
		if start > end {
			allErrs = append(allErrs, field.Invalid(fldPath, portRange, "start port must be less than or equal to end port"))
		}
	}

	return allErrs
}

// validateMACAddress validates MAC address format
func validateMACAddress(mac string, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if mac == "" {
		return allErrs // MAC is optional
	}

	// Standard MAC address format: XX:XX:XX:XX:XX:XX
	macRegex := regexp.MustCompile(`^([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})$`)
	if !macRegex.MatchString(mac) {
		allErrs = append(allErrs, field.Invalid(fldPath, mac, "invalid MAC address format"))
	}

	return allErrs
}

// validateFilePath validates file system paths
func validateFilePath(path string, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if path == "" {
		allErrs = append(allErrs, field.Required(fldPath, "path is required"))
		return allErrs
	}

	// Must be absolute path
	if !strings.HasPrefix(path, "/") {
		allErrs = append(allErrs, field.Invalid(fldPath, path, "path must be absolute"))
	}

	// Check for dangerous path patterns
	dangerousPatterns := []string{"../", "./", "//"}
	for _, pattern := range dangerousPatterns {
		if strings.Contains(path, pattern) {
			allErrs = append(allErrs, field.Invalid(fldPath, path, fmt.Sprintf("path contains dangerous pattern: %s", pattern)))
		}
	}

	return allErrs
}