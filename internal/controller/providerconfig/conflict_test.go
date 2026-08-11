/*
Copyright 2025 Ross Golder
*/

package providerconfig

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/rossigee/provider-libvirt/apis/v1beta1"
)

// conflictingStatusWriter fails the first N Update calls with a conflict
// error before delegating to the real SubResourceWriter, simulating a
// concurrent writer racing the reconciler between Get and Status().Update().
type conflictingStatusWriter struct {
	client.SubResourceWriter
	failuresRemaining *int
}

func (w *conflictingStatusWriter) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	if *w.failuresRemaining > 0 {
		*w.failuresRemaining--
		return apierrors.NewConflict(schema.GroupResource{Group: "libvirt.m.crossplane.io", Resource: "providerconfigs"}, obj.GetName(), nil)
	}
	return w.SubResourceWriter.Update(ctx, obj, opts...)
}

type conflictingClient struct {
	client.Client
	failuresRemaining int
}

func (c *conflictingClient) Status() client.SubResourceWriter {
	return &conflictingStatusWriter{SubResourceWriter: c.Client.Status(), failuresRemaining: &c.failuresRemaining}
}

func newFakeClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := v1beta1.SchemeBuilder.AddToScheme(scheme); err != nil {
		t.Fatalf("cannot build scheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("cannot build scheme: %v", err)
	}
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).WithStatusSubresource(&v1beta1.ProviderConfig{}).Build()
}

func TestUpdateReadyConditionRetriesOnConflict(t *testing.T) {
	pc := &v1beta1.ProviderConfig{}
	pc.Name = "itx-001"
	pc.Namespace = "infra-operator-system"

	base := newFakeClient(t, pc)
	cl := &conflictingClient{Client: base, failuresRemaining: 2}
	r := &reconciler{kube: cl}

	err := r.updateReadyCondition(context.Background(), client.ObjectKeyFromObject(pc), corev1.ConditionTrue, "Connected", "Successfully connected to uri")
	if err != nil {
		t.Fatalf("updateReadyCondition() = %v, want nil (conflicts should be retried in-process, not returned)", err)
	}
	if cl.failuresRemaining != 0 {
		t.Errorf("expected all injected conflicts to be exhausted by retry, %d remaining", cl.failuresRemaining)
	}

	got := &v1beta1.ProviderConfig{}
	if err := base.Get(context.Background(), client.ObjectKeyFromObject(pc), got); err != nil {
		t.Fatalf("Get() after update: %v", err)
	}
	if len(got.Status.Conditions) != 1 || got.Status.Conditions[0].Status != corev1.ConditionTrue {
		t.Errorf("condition not persisted after conflict retries: %+v", got.Status.Conditions)
	}
}
