package domain

import (
	"context"

	"github.com/pkg/errors"

	"testing"

	"github.com/rossigee/provider-libvirt/apis/v1beta1"
)

// Error path tests for domain controller

func TestObserveWrongResourceType(t *testing.T) {
	mock := &mockDomainClient{}

	ext := &external{client: mock}

	// Pass a non-Domain resource
	obs, err := ext.Observe(context.Background(), &v1beta1.Network{})
	if err == nil {
		t.Error("Observe should fail for wrong resource type")
	}
	if obs.ResourceExists {
		t.Error("ResourceExists should be false on error")
	}
	if err.Error() != "managed resource is not a Domain custom resource" {
		t.Errorf("Expected 'not a Domain' error, got: %v", err)
	}
}

func TestCreateWrongResourceType(t *testing.T) {
	mock := &mockDomainClient{}
	ext := &external{client: mock}

	_, err := ext.Create(context.Background(), &v1beta1.Network{})
	if err == nil {
		t.Error("Create should fail for wrong resource type")
	}
	if err.Error() != "managed resource is not a Domain custom resource" {
		t.Errorf("Expected 'not a Domain' error, got: %v", err)
	}
}

func TestUpdateWrongResourceType(t *testing.T) {
	mock := &mockDomainClient{}
	ext := &external{client: mock}

	_, err := ext.Update(context.Background(), &v1beta1.Network{})
	if err == nil {
		t.Error("Update should fail for wrong resource type")
	}
	if err.Error() != "managed resource is not a Domain custom resource" {
		t.Errorf("Expected 'not a Domain' error, got: %v", err)
	}
}

func TestDeleteWrongResourceType(t *testing.T) {
	mock := &mockDomainClient{}
	ext := &external{client: mock}

	_, err := ext.Delete(context.Background(), &v1beta1.Network{})
	if err == nil {
		t.Error("Delete should fail for wrong resource type")
	}
	if err.Error() != "managed resource is not a Domain custom resource" {
		t.Errorf("Expected 'not a Domain' error, got: %v", err)
	}
}

func TestObserveLookupError(t *testing.T) {
	mock := &mockDomainClient{
		lookupByNameFn: func(name string) error {
			return errors.New("connection refused")
		},
	}

	ext := &external{client: mock}
	cr := testDomain()

	obs, err := ext.Observe(context.Background(), cr)
	if err == nil {
		t.Error("Observe should fail on lookup error")
	}
	if obs.ResourceExists {
		t.Error("ResourceExists should be false on error")
	}
	if err.Error() != "connection refused" {
		t.Errorf("Expected 'connection refused', got: %v", err)
	}
}

func TestCreateLookupError(t *testing.T) {
	mock := &mockDomainClient{
		defineXMLFn: func(xml string) error {
			return errors.New("invalid XML")
		},
	}

	ext := &external{client: mock}
	cr := testDomain()

	_, err := ext.Create(context.Background(), cr)
	if err == nil {
		t.Error("Create should fail on define error")
	}
	if !contains(err.Error(), "invalid XML") {
		t.Errorf("Expected 'invalid XML' in error, got: %v", err)
	}
}

func TestUpdateLookupError(t *testing.T) {
	mock := &mockDomainClient{
		lookupByNameFn: func(name string) error {
			return errors.New("domain not found")
		},
	}

	ext := &external{client: mock}
	cr := testDomain()

	_, err := ext.Update(context.Background(), cr)
	if err == nil {
		t.Error("Update should fail when domain not found")
	}
	if !contains(err.Error(), "domain not found") {
		t.Errorf("Expected 'domain not found' in error, got: %v", err)
	}
}

func TestDeleteLookupError(t *testing.T) {
	mock := &mockDomainClient{
		lookupByNameFn: func(name string) error {
			return errors.New("connection error")
		},
	}

	ext := &external{client: mock}
	cr := testDomain()

	_, err := ext.Delete(context.Background(), cr)
	if err == nil {
		t.Error("Delete should fail on connection error")
	}
	if !contains(err.Error(), "connection error") {
		t.Errorf("Expected 'connection error' in error, got: %v", err)
	}
}

func TestCreateStartError(t *testing.T) {
	mock := &mockDomainClient{
		defineXMLFn: func(xml string) error {
			return nil
		},
		createFn: func(d interface{}) error {
			return errors.New("insufficient resources")
		},
	}

	ext := &external{client: mock}
	cr := testDomain()

	_, err := ext.Create(context.Background(), cr)
	if err == nil {
		t.Error("Create should fail when start fails")
	}
	if !contains(err.Error(), "insufficient resources") {
		t.Errorf("Expected 'insufficient resources' in error, got: %v", err)
	}
}

func TestCreateDefineError(t *testing.T) {
	mock := &mockDomainClient{
		defineXMLFn: func(xml string) error {
			return errors.New("define failed")
		},
	}
	ext := &external{client: mock}
	_, err := ext.Create(context.Background(), testDomain())
	if err == nil {
		t.Error("Create should fail on define error")
	}
}

func TestCreateAutostartError(t *testing.T) {
	mock := &mockDomainClient{
		defineXMLFn: func(xml string) error {
			return nil
		},
		createFn: func(d interface{}) error {
			return nil
		},
		setAutostartFn: func(d interface{}, as int) error {
			return errors.New("permission denied")
		},
	}

	ext := &external{client: mock}
	cr := testDomain(func(d *v1beta1.Domain) {
		t := true
		d.Spec.ForProvider.Autostart = &t
	})

	_, err := ext.Create(context.Background(), cr)
	if err == nil {
		t.Error("Create should fail when autostart fails")
	}
	if !contains(err.Error(), "permission denied") {
		t.Errorf("Expected 'permission denied' in error, got: %v", err)
	}
}

// Note: Tests for Update/Delete operations with state changes (GetState calls)
// are not included here due to CGO limitations - libvirt.Domain requires
// a real libvirt connection to call GetState(). These would be better tested
// with integration tests using mock-libvirtd HTTP API.
