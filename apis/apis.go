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

// Package apis contains Kubernetes API groups for the Libvirt provider.
package apis

import (
	"k8s.io/apimachinery/pkg/runtime"

	cloudinitv1alpha1 "github.com/rossigee/provider-libvirt/apis/cloudinit/v1alpha1"
	domainv1alpha1 "github.com/rossigee/provider-libvirt/apis/domain/v1alpha1"
	networkv1alpha1 "github.com/rossigee/provider-libvirt/apis/network/v1alpha1"
	poolv1alpha1 "github.com/rossigee/provider-libvirt/apis/pool/v1alpha1"
	v1beta1 "github.com/rossigee/provider-libvirt/apis/v1beta1"
	volumev1alpha1 "github.com/rossigee/provider-libvirt/apis/volume/v1alpha1"
)

// AddToScheme adds all registered types to the given scheme.
func AddToScheme(s *runtime.Scheme) error {
	if err := cloudinitv1alpha1.SchemeBuilder.AddToScheme(s); err != nil {
		return err
	}
	if err := domainv1alpha1.SchemeBuilder.AddToScheme(s); err != nil {
		return err
	}
	if err := networkv1alpha1.SchemeBuilder.AddToScheme(s); err != nil {
		return err
	}
	if err := poolv1alpha1.SchemeBuilder.AddToScheme(s); err != nil {
		return err
	}
	if err := volumev1alpha1.SchemeBuilder.AddToScheme(s); err != nil {
		return err
	}
	return v1beta1.SchemeBuilder.AddToScheme(s)
}
