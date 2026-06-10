package network

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"libvirt.org/go/libvirt"

	"github.com/rossigee/provider-libvirt/apis/v1beta1"
)

// mockNetworkClient for testing
type mockNetworkClient struct {
	lookupByNameFn    func(name string) (*libvirt.Network, error)
	defineXMLFn       func(xml string) (*libvirt.Network, error)
	createFn          func(n *libvirt.Network) error
	destroyFn         func(n *libvirt.Network) error
	undefineFn        func(n *libvirt.Network) error
	isActiveFn        func(n *libvirt.Network) (bool, error)
	isPersistentFn    func(n *libvirt.Network) (bool, error)
	getAutostartFn    func(n *libvirt.Network) (bool, error)
	setAutostartFn    func(n *libvirt.Network, autostart bool) error
}

func (m *mockNetworkClient) NetworkLookupByName(name string) (*libvirt.Network, error) {
	if m.lookupByNameFn != nil {
		return m.lookupByNameFn(name)
	}
	return &libvirt.Network{}, nil
}

func (m *mockNetworkClient) NetworkDefineXML(xml string) (*libvirt.Network, error) {
	if m.defineXMLFn != nil {
		return m.defineXMLFn(xml)
	}
	return &libvirt.Network{}, nil
}

func (m *mockNetworkClient) NetworkCreate(n *libvirt.Network) error {
	if m.createFn != nil {
		return m.createFn(n)
	}
	return nil
}

func (m *mockNetworkClient) NetworkDestroy(n *libvirt.Network) error {
	if m.destroyFn != nil {
		return m.destroyFn(n)
	}
	return nil
}

func (m *mockNetworkClient) NetworkUndefine(n *libvirt.Network) error {
	if m.undefineFn != nil {
		return m.undefineFn(n)
	}
	return nil
}

func (m *mockNetworkClient) NetworkIsActive(n *libvirt.Network) (bool, error) {
	if m.isActiveFn != nil {
		return m.isActiveFn(n)
	}
	return false, nil
}

func (m *mockNetworkClient) NetworkIsPersistent(n *libvirt.Network) (bool, error) {
	if m.isPersistentFn != nil {
		return m.isPersistentFn(n)
	}
	return false, nil
}

func (m *mockNetworkClient) NetworkGetAutostart(n *libvirt.Network) (bool, error) {
	if m.getAutostartFn != nil {
		return m.getAutostartFn(n)
	}
	return false, nil
}

func (m *mockNetworkClient) NetworkSetAutostart(n *libvirt.Network, autostart bool) error {
	if m.setAutostartFn != nil {
		return m.setAutostartFn(n, autostart)
	}
	return nil
}

// Error path tests

func TestObserveWrongResourceType(t *testing.T) {
	ext := &external{client: &mockNetworkClient{}}
	obs, err := ext.Observe(context.Background(), &v1beta1.Domain{})
	if err == nil {
		t.Error("Observe should fail for wrong resource type")
	}
	if obs.ResourceExists {
		t.Error("ResourceExists should be false on error")
	}
}

func TestObserveNotFound(t *testing.T) {
	mock := &mockNetworkClient{
		lookupByNameFn: func(name string) (*libvirt.Network, error) {
			return nil, errors.New("Network not found")
		},
	}
	ext := &external{client: mock}
	obs, err := ext.Observe(context.Background(), testNetwork())
	if err != nil {
		t.Errorf("Observe should succeed for not-found network: %v", err)
	}
	if obs.ResourceExists {
		t.Error("ResourceExists should be false for not-found network")
	}
}

func TestCreateWrongResourceType(t *testing.T) {
	ext := &external{client: &mockNetworkClient{}}
	_, err := ext.Create(context.Background(), &v1beta1.Domain{})
	if err == nil {
		t.Error("Create should fail for wrong resource type")
	}
}

func TestCreateDefineError(t *testing.T) {
	mock := &mockNetworkClient{
		defineXMLFn: func(xml string) (*libvirt.Network, error) {
			return nil, errors.New("invalid XML")
		},
	}
	ext := &external{client: mock}
	_, err := ext.Create(context.Background(), testNetwork())
	if err == nil {
		t.Error("Create should fail on define error")
	}
}

func TestCreateStartError(t *testing.T) {
	mock := &mockNetworkClient{
		defineXMLFn: func(xml string) (*libvirt.Network, error) {
			return &libvirt.Network{}, nil
		},
		createFn: func(n *libvirt.Network) error {
			return errors.New("network start failed")
		},
	}
	ext := &external{client: mock}
	_, err := ext.Create(context.Background(), testNetwork())
	if err == nil {
		t.Error("Create should fail when start fails")
	}
}

func TestUpdateWrongResourceType(t *testing.T) {
	ext := &external{client: &mockNetworkClient{}}
	_, err := ext.Update(context.Background(), &v1beta1.Domain{})
	if err == nil {
		t.Error("Update should fail for wrong resource type")
	}
}

func TestDeleteWrongResourceType(t *testing.T) {
	ext := &external{client: &mockNetworkClient{}}
	_, err := ext.Delete(context.Background(), &v1beta1.Domain{})
	if err == nil {
		t.Error("Delete should fail for wrong resource type")
	}
}

func TestDeleteNotFound(t *testing.T) {
	mock := &mockNetworkClient{
		lookupByNameFn: func(name string) (*libvirt.Network, error) {
			return nil, errors.New("Network not found")
		},
	}
	ext := &external{client: mock}
	_, err := ext.Delete(context.Background(), testNetwork())
	if err != nil {
		t.Errorf("Delete should succeed (idempotent) for not-found network: %v", err)
	}
}
