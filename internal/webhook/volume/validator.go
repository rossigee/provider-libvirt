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

package volume

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	volumev1alpha1 "github.com/nourspeed/provider-libvirt/apis/volume/v1alpha1"
	poolv1alpha1 "github.com/nourspeed/provider-libvirt/apis/pool/v1alpha1"
)

// Validator validates Volume resources
type Validator struct {
	client client.Client
}

// NewValidator creates a new Volume validator
func NewValidator(c client.Client) *Validator {
	return &Validator{client: c}
}

// ValidateCreate validates the Volume on creation
func (v *Validator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	volume, ok := obj.(*volumev1alpha1.Volume)
	if !ok {
		return nil, errors.New("expected a Volume")
	}

	return nil, v.validateVolume(ctx, volume)
}

// ValidateUpdate validates the Volume on update
func (v *Validator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	volume, ok := newObj.(*volumev1alpha1.Volume)
	if !ok {
		return nil, errors.New("expected a Volume")
	}

	return nil, v.validateVolume(ctx, volume)
}

// ValidateDelete validates the Volume on deletion
func (v *Validator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	// No validation needed on delete
	return nil, nil
}

// validateVolume performs the actual validation logic
func (v *Validator) validateVolume(ctx context.Context, volume *volumev1alpha1.Volume) error {
	// Get the ProviderConfig for this volume
	providerConfigName := ""
	if volume.Spec.ProviderConfigReference != nil {
		providerConfigName = volume.Spec.ProviderConfigReference.Name
	}

	if providerConfigName == "" {
		return errors.New("volume must have a providerConfigRef")
	}

	// Validate pool reference if present
	if volume.Spec.ForProvider.Pool != nil && *volume.Spec.ForProvider.Pool != "" {
		if err := v.validatePoolRef(ctx, *volume.Spec.ForProvider.Pool, providerConfigName); err != nil {
			return errors.Wrap(err, "invalid pool reference")
		}
	}

	// Validate base volume reference if present
	if volume.Spec.ForProvider.BaseVolumeID != nil && *volume.Spec.ForProvider.BaseVolumeID != "" {
		// BaseVolumeID is typically a path or ID, not a k8s reference
		// Future enhancement: could validate it exists on the same libvirt instance
	}

	return nil
}

// validatePoolRef ensures the referenced Pool uses the same ProviderConfig
func (v *Validator) validatePoolRef(ctx context.Context, poolName string, expectedProviderConfig string) error {
	if poolName == "" {
		return errors.New("pool reference name cannot be empty")
	}

	// List all pools and find one with matching name
	poolList := &poolv1alpha1.PoolList{}
	if err := v.client.List(ctx, poolList); err != nil {
		return errors.Wrap(err, "failed to list pools")
	}

	for _, pool := range poolList.Items {
		if pool.Spec.ForProvider.Name != nil && *pool.Spec.ForProvider.Name == poolName {
			// Check if it uses the same ProviderConfig
			poolProviderConfig := ""
			if pool.Spec.ProviderConfigReference != nil {
				poolProviderConfig = pool.Spec.ProviderConfigReference.Name
			}

			if poolProviderConfig != expectedProviderConfig {
				return errors.Errorf(
					"pool %s uses providerConfig %s, but volume uses providerConfig %s. Resources must use the same libvirt instance",
					poolName, poolProviderConfig, expectedProviderConfig)
			}
			return nil
		}
	}

	// Pool not found in k8s - it might exist only on the libvirt side
	// This is not necessarily an error
	return nil
}

var _ admission.CustomValidator = &Validator{}