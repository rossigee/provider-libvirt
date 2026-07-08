/*
Copyright 2025 Ross Golder
*/

package webhook

import (
	"sigs.k8s.io/controller-runtime"
)

// SetupWebhookWithManager sets up the webhooks with the manager.
func SetupWebhookWithManager(mgr ctrl.Manager) error {
	// Minimal webhook setup - validation can be added later
	return nil
}
