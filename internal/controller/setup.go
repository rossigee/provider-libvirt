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

// Package controller wires up all resource controllers for the libvirt provider.
package controller

import (
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"

	"github.com/rossigee/provider-libvirt/internal/controller/cloudinit"
	"github.com/rossigee/provider-libvirt/internal/controller/domain"
	"github.com/rossigee/provider-libvirt/internal/controller/network"
	"github.com/rossigee/provider-libvirt/internal/controller/pool"
	"github.com/rossigee/provider-libvirt/internal/controller/providerconfig"
	"github.com/rossigee/provider-libvirt/internal/controller/volume"
)

// Setup registers all provider-libvirt controllers with the manager.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	for _, setup := range []func(ctrl.Manager, controller.Options) error{
		providerconfig.Setup,
		pool.Setup,
		volume.Setup,
		network.Setup,
		domain.Setup,
		cloudinit.Setup,
	} {
		if err := setup(mgr, o); err != nil {
			return err
		}
	}
	return nil
}
