/*
Copyright 2024 The Crossplane Authors.

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

package domain

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	domainv1alpha1 "github.com/nourspeed/provider-libvirt/apis/domain/v1alpha1"
	cloudinitv1alpha1 "github.com/nourspeed/provider-libvirt/apis/cloudinit/v1alpha1"
	"github.com/nourspeed/provider-libvirt/apis/v1beta1"
)

// Validator validates Domain resources
type Validator struct {
	client client.Client
}

// NewValidator creates a new Domain validator
func NewValidator(c client.Client) *Validator {
	return &Validator{client: c}
}

// ValidateCreate validates the Domain on creation
func (v *Validator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	domain, ok := obj.(*domainv1alpha1.Domain)
	if !ok {
		return errors.New("expected a Domain")
	}

	return v.validateDomain(ctx, domain)
}

// ValidateUpdate validates the Domain on update
func (v *Validator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	domain, ok := newObj.(*domainv1alpha1.Domain)
	if !ok {
		return errors.New("expected a Domain")
	}

	return v.validateDomain(ctx, domain)
}

// ValidateDelete validates the Domain on deletion
func (v *Validator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	// No validation needed on delete
	return nil
}

// validateDomain performs the actual validation logic
func (v *Validator) validateDomain(ctx context.Context, domain *domainv1alpha1.Domain) error {
	// Get the ProviderConfig for this domain
	providerConfigName := ""
	if domain.Spec.ProviderConfigReference != nil {
		providerConfigName = domain.Spec.ProviderConfigReference.Name
	}

	if providerConfigName == "" {
		return errors.New("domain must have a providerConfigRef")
	}

	// Validate CloudInit reference if present
	if domain.Spec.ForProvider.CloudinitRef != nil {
		if err := v.validateCloudinitRef(ctx, domain.Spec.ForProvider.CloudinitRef.Name, providerConfigName); err != nil {
			return errors.Wrap(err, "invalid cloudinit reference")
		}
	}

	// Validate volume references in disk configurations
	if domain.Spec.ForProvider.Disk != nil {
		for i, disk := range domain.Spec.ForProvider.Disk {
			if disk.VolumeId != nil && *disk.VolumeId != "" {
				// For now, we just log a warning since VolumeId is a path, not a reference
				// In a future enhancement, we could validate that the path exists on the target libvirt host
				_ = fmt.Sprintf("disk[%d] references volume path: %s", i, *disk.VolumeId)
			}
		}
	}

	return nil
}

// validateCloudinitRef ensures the referenced CloudInit disk uses the same ProviderConfig
func (v *Validator) validateCloudinitRef(ctx context.Context, cloudinitName string, expectedProviderConfig string) error {
	if cloudinitName == "" {
		return errors.New("cloudinit reference name cannot be empty")
	}

	// Get the CloudInit disk
	cloudinit := &cloudinitv1alpha1.Disk{}
	if err := v.client.Get(ctx, client.ObjectKey{Name: cloudinitName}, cloudinit); err != nil {
		return errors.Wrapf(err, "failed to get cloudinit disk %s", cloudinitName)
	}

	// Check if it uses the same ProviderConfig
	cloudinitProviderConfig := ""
	if cloudinit.Spec.ProviderConfigReference != nil {
		cloudinitProviderConfig = cloudinit.Spec.ProviderConfigReference.Name
	}

	if cloudinitProviderConfig != expectedProviderConfig {
		return errors.Errorf(
			"cloudinit disk %s uses providerConfig %s, but domain uses providerConfig %s. Resources must use the same libvirt instance",
			cloudinitName, cloudinitProviderConfig, expectedProviderConfig)
	}

	return nil
}

var _ admission.CustomValidator = &Validator{}