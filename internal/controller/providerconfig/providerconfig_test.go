/*
Copyright 2025 Ross Golder
*/

package providerconfig

import (
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/rossigee/provider-libvirt/apis/v1beta1"
)

func TestSetConditionSkipsNoOpUpdate(t *testing.T) {
	r := &reconciler{}
	pc := &v1beta1.ProviderConfig{}

	if changed := r.setCondition(pc, corev1.ConditionTrue, "Connected", "Successfully connected to uri"); !changed {
		t.Fatal("setCondition() = false on first write, want true")
	}
	if len(pc.Status.Conditions) != 1 {
		t.Fatalf("got %d conditions, want 1", len(pc.Status.Conditions))
	}
	firstTransition := pc.Status.Conditions[0].LastTransitionTime

	if changed := r.setCondition(pc, corev1.ConditionTrue, "Connected", "Successfully connected to uri"); changed {
		t.Error("setCondition() = true on repeat write of identical condition, want false (this is what stops the self-retriggering reconcile loop)")
	}
	if pc.Status.Conditions[0].LastTransitionTime != firstTransition {
		t.Error("setCondition() must not touch LastTransitionTime when the condition is unchanged")
	}
}

func TestSetConditionReportsRealChange(t *testing.T) {
	r := &reconciler{}
	pc := &v1beta1.ProviderConfig{}

	r.setCondition(pc, corev1.ConditionTrue, "Connected", "Successfully connected to uri")

	if changed := r.setCondition(pc, corev1.ConditionFalse, "ConnectionError", "cannot connect to uri"); !changed {
		t.Error("setCondition() = false when status/reason/message actually differ, want true")
	}
	if got := pc.Status.Conditions[0].Reason; got != "ConnectionError" {
		t.Errorf("Reason = %q, want ConnectionError", got)
	}
}
