/*
Copyright 2025 Ross Golder
*/

package main

import (
	"os"
	"path/filepath"

	"github.com/alecthomas/kingpin/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	"github.com/crossplane/crossplane-runtime/pkg/logging"

	"github.com/rossigee/provider-libvirt/apis/v1alpha1"
	"github.com/rossigee/provider-libvirt/internal/controller/domain"
	"github.com/rossigee/provider-libvirt/internal/controller/network"
	"github.com/rossigee/provider-libvirt/internal/controller/secret"
	"github.com/rossigee/provider-libvirt/internal/controller/storagepool"
	"github.com/rossigee/provider-libvirt/internal/controller/volume"
	"github.com/rossigee/provider-libvirt/internal/webhook"
)

func main() {
	var (
		app            = kingpin.New(filepath.Base(os.Args[0]), "A Crossplane provider for libvirt").DefaultEnvars()
		debug          = app.Flag("debug", "Run with debug logging.").Short('d').Bool()
		enableWebhooks = app.Flag("enable-webhooks", "Enable validation webhooks.").Default("false").Bool()
	)
	kingpin.MustParse(app.Parse(os.Args[1:]))

	zl := zap.New(zap.UseDevMode(*debug))
	log := logging.NewLogrLogger(zl.WithName("provider-libvirt"))
	if *debug {
		ctrl.SetLogger(zl)
	}

	cfg, err := ctrl.GetConfig()
	kingpin.FatalIfError(err, "Cannot get API server rest config")

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		LeaderElection:         false,
		LeaderElectionID:       "crossplane-leader-election-provider-libvirt",
		Cache:                  cache.Options{},
		HealthProbeBindAddress: ":9440",
	})
	kingpin.FatalIfError(err, "Cannot create controller manager")

	kingpin.FatalIfError(v1alpha1.SchemeBuilder.AddToScheme(mgr.GetScheme()), "Cannot add v1alpha1 APIs to scheme")

	kingpin.FatalIfError(domain.Setup(mgr, log), "Cannot setup Domain controller")
	kingpin.FatalIfError(network.Setup(mgr, log), "Cannot setup Network controller")
	kingpin.FatalIfError(secret.Setup(mgr, log), "Cannot setup Secret controller")
	kingpin.FatalIfError(storagepool.Setup(mgr, log), "Cannot setup StoragePool controller")
	kingpin.FatalIfError(volume.Setup(mgr, log), "Cannot setup Volume controller")

	// Setup webhooks if enabled
	if *enableWebhooks {
		log.Info("Setting up validation webhooks")
		kingpin.FatalIfError(webhook.SetupWebhookWithManager(mgr), "Cannot setup validation webhooks")
	}

	kingpin.FatalIfError(mgr.AddHealthzCheck("healthz", healthz.Ping), "Cannot add health check")
	kingpin.FatalIfError(mgr.AddReadyzCheck("readyz", healthz.Ping), "Cannot add ready check")

	log.Info("Starting controller manager")
	kingpin.FatalIfError(mgr.Start(ctrl.SetupSignalHandler()), "Cannot start controller manager")
}