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

package webhook

import (
	ctrl "sigs.k8s.io/controller-runtime"

	domainv1alpha1 "github.com/nourspeed/provider-libvirt/apis/domain/v1alpha1"
	volumev1alpha1 "github.com/nourspeed/provider-libvirt/apis/volume/v1alpha1"
	domainwebhook "github.com/nourspeed/provider-libvirt/internal/webhook/domain"
	volumewebhook "github.com/nourspeed/provider-libvirt/internal/webhook/volume"
)

// Setup configures webhooks for the provider
func Setup(mgr ctrl.Manager) error {
	// Setup Domain validation webhook
	if err := ctrl.NewWebhookManagedBy(mgr).
		For(&domainv1alpha1.Domain{}).
		WithValidator(domainwebhook.NewValidator(mgr.GetClient())).
		Complete(); err != nil {
		return err
	}

	// Setup Volume validation webhook
	if err := ctrl.NewWebhookManagedBy(mgr).
		For(&volumev1alpha1.Volume{}).
		WithValidator(volumewebhook.NewValidator(mgr.GetClient())).
		Complete(); err != nil {
		return err
	}

	return nil
}