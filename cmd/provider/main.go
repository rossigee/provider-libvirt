/*
Copyright 2025 Ross Golder
*/

package main

import (
	"context"
	"github.com/alecthomas/kingpin/v2"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	"github.com/rossigee/provider-libvirt/internal/controller/domain"
	"github.com/rossigee/provider-libvirt/internal/controller/network"
	"github.com/rossigee/provider-libvirt/internal/controller/nodedevice"
	"github.com/rossigee/provider-libvirt/internal/controller/providerconfig"
	"github.com/rossigee/provider-libvirt/internal/controller/secret"
	"github.com/rossigee/provider-libvirt/internal/controller/storagepool"
	"github.com/rossigee/provider-libvirt/internal/controller/volume"
	"github.com/rossigee/provider-libvirt/internal/tracing"
	"github.com/rossigee/provider-libvirt/internal/webhook"
	"os"
	"path/filepath"

	"github.com/rossigee/provider-libvirt/apis/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func main() {
	var (
		app            = kingpin.New(filepath.Base(os.Args[0]), "A Crossplane provider for libvirt").DefaultEnvars()
		debug          = app.Flag("debug", "Run with debug logging.").Short('d').Bool()
		enableWebhooks = app.Flag("enable-webhooks", "Enable validation webhooks.").Default("false").Bool()
	)
	kingpin.MustParse(app.Parse(os.Args[1:]))

	// Set up TLS environment variables for libvirt connections
	// TLS certificates are mounted at /tls/client/ in the container
	if _, err := os.Stat("/tls/client/ca.crt"); err == nil {
		os.Setenv("LIBVIRT_CACERT", "/tls/client/ca.crt")
		os.Setenv("LIBVIRT_TLS_CERT", "/tls/client/tls.crt")
		os.Setenv("LIBVIRT_TLS_KEY", "/tls/client/tls.key")
	}

	zl := zap.New(zap.UseDevMode(*debug))
	ctrl.SetLogger(zl)
	log := logging.NewLogrLogger(zl.WithName("provider-libvirt"))

	shutdownTracing := tracing.Init("provider-libvirt")
	defer shutdownTracing(context.Background())

	s := runtime.NewScheme()
	kingpin.FatalIfError(scheme.AddToScheme(s), "Cannot add k8s types to scheme")
	kingpin.FatalIfError(v1beta1.SchemeBuilder.AddToScheme(s), "Cannot add v1beta1 APIs to scheme")

	cfg, err := ctrl.GetConfig()
	kingpin.FatalIfError(err, "Cannot get API server rest config")

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:                 s,
		LeaderElection:         false,
		LeaderElectionID:       "crossplane-leader-election-provider-libvirt",
		Cache:                  cache.Options{},
		HealthProbeBindAddress: ":9440",
	})
	kingpin.FatalIfError(err, "Cannot create controller manager")

	kingpin.FatalIfError(domain.Setup(mgr, log), "Cannot setup Domain controller")
	kingpin.FatalIfError(network.Setup(mgr, log), "Cannot setup Network controller")
	kingpin.FatalIfError(nodedevice.Setup(mgr, log), "Cannot setup NodeDevice controller")
	kingpin.FatalIfError(providerconfig.Setup(mgr), "Cannot setup ProviderConfig controller")
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
