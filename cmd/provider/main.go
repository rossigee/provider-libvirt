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

package main

import (
	"os"
	"path/filepath"
	"runtime"
	"time"

	kingpin "github.com/alecthomas/kingpin/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	xpcontroller "github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/feature"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"

	"github.com/rossigee/provider-libvirt/apis"
	"github.com/rossigee/provider-libvirt/internal/controller"
)

func main() {
	var (
		app              = kingpin.New(filepath.Base(os.Args[0]), "Libvirt support for Crossplane.").DefaultEnvars()
		debug            = app.Flag("debug", "Run with debug logging.").Short('d').Bool()
		leaderElection   = app.Flag("leader-election", "Use leader election for the controller manager.").Short('l').Default("false").OverrideDefaultFromEnvar("LEADER_ELECTION").Bool()
		leaderElectionNS = app.Flag("leader-election-namespace", "Namespace to use for leader election.").Default("crossplane-system").OverrideDefaultFromEnvar("LEADER_ELECTION_NAMESPACE").String()
		pollInterval     = app.Flag("poll", "How often individual resources will be checked for drift from the desired state.").Short('p').Default("5m").Duration()
		maxReconcileRate = app.Flag("max-reconcile-rate", "The global maximum rate per second at which resources may be checked for drift from the desired state.").Default("10").Int()
		syncPeriod       = app.Flag("sync", "How often all resources will be double-checked for drift from the desired state.").Short('s').Default("1h").Duration()
	)

	kingpin.MustParse(app.Parse(os.Args[1:]))

	zl := zap.New(zap.UseDevMode(*debug))
	log := logging.NewLogrLogger(zl.WithName("provider-libvirt"))
	ctrl.SetLogger(zl.WithName("controller-runtime"))

	log.Info("Provider starting up",
		"provider", "provider-libvirt",
		"go-version", runtime.Version(),
		"platform", runtime.GOOS+"/"+runtime.GOARCH,
		"sync-period", syncPeriod.String(),
		"poll-interval", pollInterval.String(),
		"max-reconcile-rate", *maxReconcileRate,
		"leader-election", *leaderElection,
		"leader-election-namespace", *leaderElectionNS,
		"debug-mode", *debug,
	)

	cfg, err := ctrl.GetConfig()
	kingpin.FatalIfError(err, "Cannot get API server rest config")

	mgr, err := ctrl.NewManager(ratelimiter.LimitRESTConfig(cfg, *maxReconcileRate), ctrl.Options{
		Cache: cache.Options{
			SyncPeriod: syncPeriod,
		},
		LeaderElection:             *leaderElection,
		LeaderElectionID:           "crossplane-leader-election-provider-libvirt",
		LeaderElectionNamespace:    *leaderElectionNS,
		LeaderElectionResourceLock: "leases",
		LeaseDuration:              func() *time.Duration { d := 60 * time.Second; return &d }(),
		RenewDeadline:              func() *time.Duration { d := 50 * time.Second; return &d }(),
	})
	kingpin.FatalIfError(err, "Cannot create controller manager")

	o := xpcontroller.Options{
		Logger:                  log,
		MaxConcurrentReconciles: *maxReconcileRate,
		PollInterval:            *pollInterval,
		GlobalRateLimiter:       ratelimiter.NewGlobal(*maxReconcileRate),
		Features:                &feature.Flags{},
	}

	log.Info("Adding libvirt APIs to scheme")
	kingpin.FatalIfError(apis.AddToScheme(mgr.GetScheme()), "Cannot add libvirt APIs to scheme")

	log.Info("Setting up libvirt controllers")
	kingpin.FatalIfError(controller.Setup(mgr, o), "Cannot setup libvirt controllers")

	kingpin.FatalIfError(mgr.AddHealthzCheck("healthz", healthz.Ping), "Cannot add health check")
	kingpin.FatalIfError(mgr.AddReadyzCheck("readyz", healthz.Ping), "Cannot add ready check")

	kingpin.FatalIfError(mgr.Start(ctrl.SetupSignalHandler()), "Cannot start controller manager")
}
