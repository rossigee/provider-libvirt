/*
Copyright 2025 Ross Golder
*/

package providerconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/pkg/errors"

	"github.com/rossigee/provider-libvirt/apis/v1beta1"
	"github.com/rossigee/provider-libvirt/internal/clients"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"time"
)

const (
	reconcileTimeout = 60 * time.Second
	pollInterval     = 60 * time.Second
)

// Setup adds a controller that reconciles ProviderConfig resources
// and tests libvirt connectivity.
func Setup(mgr ctrl.Manager) error {
	name := "providerconfig.libvirt.m.crossplane.io"

	r := &reconciler{
		kube: mgr.GetClient(),
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1beta1.ProviderConfig{}).
		Complete(r)
}

type reconciler struct {
	kube client.Client
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	pc := &v1beta1.ProviderConfig{}
	if err := r.kube.Get(ctx, req.NamespacedName, pc); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Extract credentials
	data, err := resource.CommonCredentialExtractor(ctx, pc.Spec.Credentials.Source, r.kube, pc.Spec.Credentials.CommonCredentialSelectors)
	if err != nil {
		log.Error(err, "cannot extract credentials")
		if err := r.updateReadyCondition(ctx, req.NamespacedName, corev1.ConditionFalse, "ConnectionError", fmt.Sprintf("cannot extract credentials: %v", err)); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: pollInterval}, nil
	}

	// Unmarshal credentials to get URI
	creds := map[string]string{}
	if err := json.Unmarshal(data, &creds); err != nil {
		log.Error(err, "cannot unmarshal credentials")
		if err := r.updateReadyCondition(ctx, req.NamespacedName, corev1.ConditionFalse, "ConnectionError", fmt.Sprintf("cannot unmarshal credentials: %v", err)); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: pollInterval}, nil
	}

	uri, ok := creds["uri"]
	if !ok {
		errMsg := "libvirt URI not found in credentials (expected \"uri\" key)"
		log.Error(errors.New(errMsg), "invalid credentials")
		if err := r.updateReadyCondition(ctx, req.NamespacedName, corev1.ConditionFalse, "ConnectionError", errMsg); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: pollInterval}, nil
	}

	// Acquire a shared, pooled connection rather than opening a fresh TLS
	// connection on every poll - this reconciler runs every pollInterval
	// (60s) for every configured ProviderConfig, and repeatedly
	// opening/closing full libvirt connections at that frequency was the
	// primary driver of a slow memory leak that OOMed the provider every
	// few hours.
	if _, err := clients.AcquireConnection(uri); err != nil {
		log.Error(err, "cannot connect to libvirt", "uri", uri)
		if err := r.updateReadyCondition(ctx, req.NamespacedName, corev1.ConditionFalse, "ConnectionError", fmt.Sprintf("cannot connect to %s: %v", uri, err)); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: pollInterval}, nil
	}
	clients.ReleaseConnection(uri)

	log.Info("libvirt connection successful", "uri", uri)
	if err := r.updateReadyCondition(ctx, req.NamespacedName, corev1.ConditionTrue, "Connected", fmt.Sprintf("Successfully connected to %s", uri)); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: pollInterval}, nil
}

// updateReadyCondition sets the Ready condition and writes it, retrying on
// resourceVersion conflicts by re-fetching and reapplying in-process rather
// than returning an error. A returned error causes controller-runtime to
// requeue via a brand new Reconcile() call - which would open another
// libvirt connection just to retry a status write - so conflicts are
// resolved here instead, without needing another connection attempt. If the
// condition already matches the latest object, no write is made at all.
func (r *reconciler) updateReadyCondition(ctx context.Context, key client.ObjectKey, status corev1.ConditionStatus, reason xpv1.ConditionReason, message string) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latest := &v1beta1.ProviderConfig{}
		if err := r.kube.Get(ctx, key, latest); err != nil {
			return err
		}
		if !r.setCondition(latest, status, reason, message) {
			return nil
		}
		return r.kube.Status().Update(ctx, latest)
	})
}

// setCondition sets a condition on the ProviderConfig status. It returns
// false, without modifying pc, if the condition already matches (ignoring
// LastTransitionTime) - this lets callers skip a no-op Status().Update(),
// which would otherwise trigger the controller's own watch and cause an
// immediate re-reconcile far more often than pollInterval intends.
func (r *reconciler) setCondition(pc *v1beta1.ProviderConfig, status corev1.ConditionStatus, reason xpv1.ConditionReason, message string) bool {
	condType := xpv1.ConditionType("Ready")

	for i, c := range pc.Status.Conditions {
		if string(c.Type) == string(condType) {
			if c.Status == status && c.Reason == reason && c.Message == message {
				return false
			}
			pc.Status.Conditions[i] = xpv1.Condition{
				Type:               condType,
				Status:             status,
				LastTransitionTime: metav1.Now(),
				Reason:             reason,
				Message:            message,
			}
			return true
		}
	}

	pc.Status.Conditions = append(pc.Status.Conditions, xpv1.Condition{
		Type:               condType,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	})
	return true
}
