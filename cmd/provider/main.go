/*
Copyright 2025 Ross Golder
*/

package main

import (
	"context"

	"github.com/alecthomas/kingpin/v2"
	xpcontroller "github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/feature"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/statemetrics"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"os"
	"path/filepath"

	"github.com/rossigee/provider-libvirt/internal/clients"
	"github.com/rossigee/provider-libvirt/internal/controller/domain"
	"github.com/rossigee/provider-libvirt/internal/controller/network"
	"github.com/rossigee/provider-libvirt/internal/controller/nodedevice"
	"github.com/rossigee/provider-libvirt/internal/controller/providerconfig"
	"github.com/rossigee/provider-libvirt/internal/controller/secret"
	"github.com/rossigee/provider-libvirt/internal/controller/storagepool"
	"github.com/rossigee/provider-libvirt/internal/controller/volume"
	"github.com/rossigee/provider-libvirt/internal/features"
	"github.com/rossigee/provider-libvirt/internal/tracing"
	"github.com/rossigee/provider-libvirt/internal/webhook"

	"github.com/rossigee/provider-libvirt/apis/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	metricserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

func main() {
	var (
		app                      = kingpin.New(filepath.Base(os.Args[0]), "A Crossplane provider for libvirt").DefaultEnvars()
		debug                    = app.Flag("debug", "Run with debug logging.").Short('d').Bool()
		enableWebhooks           = app.Flag("enable-webhooks", "Enable validation webhooks.").Default("false").Bool()
		pollInterval             = app.Flag("poll", "Poll interval controls how often an individual resource should be checked for drift.").Default("1m").Duration()
		maxReconcileRate         = app.Flag("max-reconcile-rate", "The global maximum rate per second at which resources may be checked for drift.").Default("10").Int()
		pollStateMetricInterval  = app.Flag("poll-state-metric", "State metric recording interval").Default("5s").Duration()
		metricsBindAddress       = app.Flag("metrics-bind-address", "The address the metrics endpoint binds to.").Default(":8080").String()
		enableManagementPolicies = app.Flag("enable-management-policies", "Enable support for management policies.").Default("true").Envar("ENABLE_MANAGEMENT_POLICIES").Bool()
	)
	kingpin.MustParse(app.Parse(os.Args[1:]))

	// Set up TLS environment variables for libvirt connections
	// TLS certificates are mounted at /tls/client/ in the container
	if _, err := os.Stat("/tls/client/ca.crt"); err == nil {
		_ = os.Setenv("LIBVIRT_CACERT", "/tls/client/ca.crt")
		_ = os.Setenv("LIBVIRT_TLS_CERT", "/tls/client/tls.crt")
		_ = os.Setenv("LIBVIRT_TLS_KEY", "/tls/client/tls.key")
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
		Metrics: metricserver.Options{
			BindAddress: *metricsBindAddress,
		},
	})
	kingpin.FatalIfError(err, "Cannot create controller manager")

	if err := setupRBAC(mgr.GetClient(), log); err != nil {
		log.Info("RBAC setup warning (may be transient)", "error", err)
	}

	mrStateMetrics := statemetrics.NewMRStateMetrics()
	metrics.Registry.MustRegister(mrStateMetrics)

	mo := xpcontroller.MetricOptions{
		PollStateMetricInterval: *pollStateMetricInterval,
		MRStateMetrics:          mrStateMetrics,
	}

	featureFlags := &feature.Flags{}
	if *enableManagementPolicies {
		featureFlags.Enable(features.EnableAlphaManagementPolicies)
		log.Info("Alpha feature enabled", "flag", features.EnableAlphaManagementPolicies)
	}

	o := xpcontroller.Options{
		Logger:                  log,
		MaxConcurrentReconciles: *maxReconcileRate,
		PollInterval:            *pollInterval,
		GlobalRateLimiter:       ratelimiter.NewGlobal(*maxReconcileRate),
		Features:                featureFlags,
		MetricOptions:           &mo,
	}

	kingpin.FatalIfError(domain.Setup(mgr, o), "Cannot setup Domain controller")
	kingpin.FatalIfError(network.Setup(mgr, o), "Cannot setup Network controller")
	kingpin.FatalIfError(nodedevice.Setup(mgr, o), "Cannot setup NodeDevice controller")
	kingpin.FatalIfError(providerconfig.Setup(mgr), "Cannot setup ProviderConfig controller")
	kingpin.FatalIfError(secret.Setup(mgr, o), "Cannot setup Secret controller")
	kingpin.FatalIfError(storagepool.Setup(mgr, o), "Cannot setup StoragePool controller")
	kingpin.FatalIfError(volume.Setup(mgr, o), "Cannot setup Volume controller")

	kingpin.FatalIfError(mgr.Add(statemetrics.NewMRStateRecorder(mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, &v1beta1.DomainList{}, o.MetricOptions.PollStateMetricInterval)), "Cannot register state metrics for Domain")
	kingpin.FatalIfError(mgr.Add(statemetrics.NewMRStateRecorder(mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, &v1beta1.NetworkList{}, o.MetricOptions.PollStateMetricInterval)), "Cannot register state metrics for Network")
	kingpin.FatalIfError(mgr.Add(statemetrics.NewMRStateRecorder(mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, &v1beta1.VolumeList{}, o.MetricOptions.PollStateMetricInterval)), "Cannot register state metrics for Volume")
	kingpin.FatalIfError(mgr.Add(statemetrics.NewMRStateRecorder(mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, &v1beta1.StoragePoolList{}, o.MetricOptions.PollStateMetricInterval)), "Cannot register state metrics for StoragePool")
	kingpin.FatalIfError(mgr.Add(statemetrics.NewMRStateRecorder(mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, &v1beta1.NodeDeviceList{}, o.MetricOptions.PollStateMetricInterval)), "Cannot register state metrics for NodeDevice")
	kingpin.FatalIfError(mgr.Add(statemetrics.NewMRStateRecorder(mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, &v1beta1.SecretList{}, o.MetricOptions.PollStateMetricInterval)), "Cannot register state metrics for Secret")

	// Setup webhooks if enabled
	if *enableWebhooks {
		log.Info("Setting up validation webhooks")
		kingpin.FatalIfError(webhook.SetupWebhookWithManager(mgr), "Cannot setup validation webhooks")
	}

	kingpin.FatalIfError(mgr.AddHealthzCheck("healthz", healthz.Ping), "Cannot add health check")
	kingpin.FatalIfError(mgr.AddReadyzCheck("readyz", healthz.Ping), "Cannot add ready check")
	kingpin.FatalIfError(mgr.Add(manager.RunnableFunc(clients.StartConnectionReaper)), "Cannot add libvirt connection reaper")

	log.Info("Starting controller manager")
	kingpin.FatalIfError(mgr.Start(ctrl.SetupSignalHandler()), "Cannot start controller manager")
}

func setupRBAC(c client.Client, l logging.Logger) error {
	ctx := context.Background()

	rules := []rbacv1.PolicyRule{
		{APIGroups: []string{"libvirt.m.crossplane.io"}, Resources: []string{"domains", "domains/status", "networks", "networks/status", "nodedevices", "nodedevices/status", "providerconfigs", "providerconfigs/status", "providerconfigusages", "providerconfigusages/status", "secrets", "secrets/status", "storagepools", "storagepools/status", "volumes", "volumes/status"}, Verbs: []string{"get", "list", "watch", "update", "patch", "create"}},
		{
			APIGroups: []string{"libvirt.m.crossplane.io"},
			Resources: []string{"*/finalizers"},
			Verbs:     []string{"update"},
		},
		{APIGroups: []string{"", "coordination.k8s.io"}, Resources: []string{"secrets", "configmaps", "events", "leases"}, Verbs: []string{"*"}},
	}

	system := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "crossplane:provider:provider-libvirt:system",
			Labels: map[string]string{"rbac.crossplane.io/system": "provider-libvirt"},
		},
		Rules: rules,
	}
	if err := c.Create(ctx, system); err != nil && !errors.IsAlreadyExists(err) {
		return err
	}
	if err := c.Update(ctx, system); err != nil {
		l.Info("system role update", "err", err)
	}

	binding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "crossplane:provider:provider-libvirt:system"},
		RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: "crossplane:provider:provider-libvirt:system"},
		Subjects:   []rbacv1.Subject{{Kind: "ServiceAccount", Name: "provider-libvirt", Namespace: "crossplane-system"}},
	}
	if err := c.Create(ctx, binding); err != nil && !errors.IsAlreadyExists(err) {
		return err
	}
	if err := c.Update(ctx, binding); err != nil {
		l.Info("system binding update", "err", err)
	}

	edit := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "crossplane:provider:provider-libvirt:aggregate-to-edit",
			Labels: map[string]string{
				"rbac.crossplane.io/aggregate-to-edit": "true", "rbac.crossplane.io/aggregate-to-admin": "true",
				"rbac.crossplane.io/aggregate-to-crossplane": "true", "rbac.crossplane.io/system": "provider-libvirt",
			},
		},
		Rules: withVerbs(rules, []string{"*"}),
	}
	if err := c.Create(ctx, edit); err != nil && !errors.IsAlreadyExists(err) {
		l.Info("aggregate-to-edit create warning (non-fatal)", "err", err)
	}
	_ = c.Update(ctx, edit)

	view := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "crossplane:provider:provider-libvirt:aggregate-to-view",
			Labels: map[string]string{"rbac.crossplane.io/aggregate-to-view": "true", "rbac.crossplane.io/system": "provider-libvirt"},
		},
		Rules: withVerbs(rules, []string{"get", "list", "watch"}),
	}
	if err := c.Create(ctx, view); err != nil && !errors.IsAlreadyExists(err) {
		l.Info("aggregate-to-view create warning (non-fatal)", "err", err)
	}
	_ = c.Update(ctx, view)

	l.Info("provider self-managed RBAC roles ensured")
	return nil
}

func withVerbs(r []rbacv1.PolicyRule, verbs []string) []rbacv1.PolicyRule {
	out := make([]rbacv1.PolicyRule, len(r))
	for i := range r {
		out[i] = r[i]
		out[i].Verbs = verbs
	}
	return out
}
