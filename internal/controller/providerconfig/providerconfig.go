/*
Copyright 2025 Ross Golder
*/

package providerconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/pkg/errors"
	"github.com/rossigee/provider-libvirt/apis/v1beta1"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"libvirt.org/go/libvirt"
	"sigs.k8s.io/controller-runtime"
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
		r.setCondition(pc, corev1.ConditionFalse, "ConnectionError", fmt.Sprintf("cannot extract credentials: %v", err))
		if err := r.kube.Status().Update(ctx, pc); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: pollInterval}, nil
	}

	// Unmarshal credentials to get URI
	creds := map[string]string{}
	if err := json.Unmarshal(data, &creds); err != nil {
		log.Error(err, "cannot unmarshal credentials")
		r.setCondition(pc, corev1.ConditionFalse, "ConnectionError", fmt.Sprintf("cannot unmarshal credentials: %v", err))
		if err := r.kube.Status().Update(ctx, pc); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: pollInterval}, nil
	}

	uri, ok := creds["uri"]
	if !ok {
		errMsg := "libvirt URI not found in credentials (expected \"uri\" key)"
		log.Error(errors.New(errMsg), "invalid credentials")
		r.setCondition(pc, corev1.ConditionFalse, "ConnectionError", errMsg)
		if err := r.kube.Status().Update(ctx, pc); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: pollInterval}, nil
	}

	// Attempt libvirt connection
	conn, err := libvirt.NewConnect(uri)
	if err != nil {
		log.Error(err, "cannot connect to libvirt", "uri", uri)
		r.setCondition(pc, corev1.ConditionFalse, "ConnectionError", fmt.Sprintf("cannot connect to %s: %v", uri, err))
		if err := r.kube.Status().Update(ctx, pc); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: pollInterval}, nil
	}

	// Connected - close immediately
	if _, err := conn.Close(); err != nil {
		log.Error(err, "error closing test connection")
	}

	log.Info("libvirt connection successful", "uri", uri)
	r.setCondition(pc, corev1.ConditionTrue, "Connected", fmt.Sprintf("Successfully connected to %s", uri))
	if err := r.kube.Status().Update(ctx, pc); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: pollInterval}, nil
}

// setCondition sets a condition on the ProviderConfig status
func (r *reconciler) setCondition(pc *v1beta1.ProviderConfig, status corev1.ConditionStatus, reason xpv1.ConditionReason, message string) {
	condType := xpv1.ConditionType("Ready")
	now := metav1.Now()

	// Update or add the condition
	for i, c := range pc.Status.Conditions {
		if string(c.Type) == string(condType) {
			pc.Status.Conditions[i] = xpv1.Condition{
				Type:               condType,
				Status:             status,
				LastTransitionTime: now,
				Reason:             reason,
				Message:            message,
			}
			return
		}
	}

	pc.Status.Conditions = append(pc.Status.Conditions, xpv1.Condition{
		Type:               condType,
		Status:             status,
		LastTransitionTime: now,
		Reason:             reason,
		Message:            message,
	})
}
