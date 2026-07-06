package secret

import (
	"context"
	"testing"

	"github.com/rossigee/provider-libvirt/apis/v1beta1"
)

// Error path tests for secret controller

func TestObserveWrongResourceType(t *testing.T) {
	ext := &external{service: nil, kube: nil}
	obs, err := ext.Observe(context.Background(), &v1beta1.Domain{})
	if err == nil {
		t.Error("Observe should fail for wrong resource type")
	}
	if obs.ResourceExists {
		t.Error("ResourceExists should be false on error")
	}
}

func TestObserveNoUUID(t *testing.T) {
	ext := &external{service: nil, kube: nil}
	cr := &v1beta1.Secret{}
	cr.Status.AtProvider.UUID = ""

	obs, err := ext.Observe(context.Background(), cr)
	if err != nil {
		t.Errorf("Observe should succeed for secret with no UUID: %v", err)
	}
	if obs.ResourceExists {
		t.Error("ResourceExists should be false for secret without UUID")
	}
}

func TestCreateWrongResourceType(t *testing.T) {
	ext := &external{service: nil, kube: nil}
	_, err := ext.Create(context.Background(), &v1beta1.Domain{})
	if err == nil {
		t.Error("Create should fail for wrong resource type")
	}
}

func TestUpdateWrongResourceType(t *testing.T) {
	ext := &external{service: nil, kube: nil}
	_, err := ext.Update(context.Background(), &v1beta1.Domain{})
	if err == nil {
		t.Error("Update should fail for wrong resource type")
	}
}

func TestDeleteWrongResourceType(t *testing.T) {
	ext := &external{service: nil, kube: nil}
	_, err := ext.Delete(context.Background(), &v1beta1.Domain{})
	if err == nil {
		t.Error("Delete should fail for wrong resource type")
	}
}

func TestDeleteNoUUID(t *testing.T) {
	ext := &external{service: nil, kube: nil}
	cr := &v1beta1.Secret{}
	cr.Status.AtProvider.UUID = ""

	_, err := ext.Delete(context.Background(), cr)
	if err != nil {
		t.Errorf("Delete should succeed for secret with no UUID: %v", err)
	}
}
