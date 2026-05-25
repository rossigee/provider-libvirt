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

package v1alpha1

import xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"

// GetCondition of this Domain.
func (mg *Domain) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	return mg.Status.GetCondition(ct)
}

// SetConditions of this Domain.
func (mg *Domain) SetConditions(c ...xpv1.Condition) {
	mg.Status.SetConditions(c...)
}

// GetManagementPolicies of this Domain.
func (mg *Domain) GetManagementPolicies() xpv1.ManagementPolicies {
	return mg.Spec.ManagementPolicies
}

// SetManagementPolicies of this Domain.
func (mg *Domain) SetManagementPolicies(p xpv1.ManagementPolicies) {
	mg.Spec.ManagementPolicies = p
}

// GetProviderConfigReference of this Domain.
func (mg *Domain) GetProviderConfigReference() *xpv1.ProviderConfigReference {
	return mg.Spec.ProviderConfigReference
}

// SetProviderConfigReference of this Domain.
func (mg *Domain) SetProviderConfigReference(r *xpv1.ProviderConfigReference) {
	mg.Spec.ProviderConfigReference = r
}

// GetWriteConnectionSecretToReference of this Domain.
func (mg *Domain) GetWriteConnectionSecretToReference() *xpv1.LocalSecretReference {
	return mg.Spec.WriteConnectionSecretToReference
}

// SetWriteConnectionSecretToReference of this Domain.
func (mg *Domain) SetWriteConnectionSecretToReference(r *xpv1.LocalSecretReference) {
	mg.Spec.WriteConnectionSecretToReference = r
}
