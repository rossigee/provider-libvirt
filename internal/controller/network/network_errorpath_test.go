package network

import (
	"context"

	"github.com/pkg/errors"

	"testing"

	"github.com/rossigee/provider-libvirt/apis/v1beta1"
	"libvirt.org/go/libvirt"
)

// mockNetworkClient for testing
type mockNetworkClient struct {
	lookupByNameFn func(name string) (*libvirt.Network, error)
	defineXMLFn    func(xml string) (*libvirt.Network, error)
	createFn       func(n *libvirt.Network) error
	destroyFn      func(n *libvirt.Network) error
	undefineFn     func(n *libvirt.Network) error
	isActiveFn     func(n *libvirt.Network) (bool, error)
	isPersistentFn func(n *libvirt.Network) (bool, error)
	getAutostartFn func(n *libvirt.Network) (bool, error)
	setAutostartFn func(n *libvirt.Network, autostart bool) error
	closeFn        func() error
	closeCalled    bool
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

func (m *mockNetworkClient) Close() error {
	m.closeCalled = true
	if m.closeFn != nil {
		return m.closeFn()
	}
	return nil
}

func TestNetworkDisconnect(t *testing.T) {
	mock := &mockNetworkClient{}
	e := &external{client: mock}

	if err := e.Disconnect(context.Background()); err != nil {
		t.Fatalf("Disconnect() returned unexpected error: %v", err)
	}
	if !mock.closeCalled {
		t.Error("Disconnect() did not close the underlying libvirt connection")
	}
}

func TestNetworkDisconnectPropagatesCloseError(t *testing.T) {
	wantErr := errors.New("boom")
	mock := &mockNetworkClient{closeFn: func() error { return wantErr }}
	e := &external{client: mock}

	err := e.Disconnect(context.Background())
	if err == nil || err.Error() != wantErr.Error() {
		t.Errorf("Disconnect() = %v, want error %v", err, wantErr)
	}
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

func TestUpdateNetworkNotFound(t *testing.T) {
	mock := &mockNetworkClient{
		lookupByNameFn: func(name string) (*libvirt.Network, error) {
			return nil, errors.New("Network not found")
		},
	}
	ext := &external{client: mock}
	_, err := ext.Update(context.Background(), testNetwork())
	if err == nil {
		t.Error("Update should fail when network not found")
	}
}

func TestObserveActive(t *testing.T) {
	mock := &mockNetworkClient{
		lookupByNameFn: func(name string) (*libvirt.Network, error) {
			return &libvirt.Network{}, nil
		},
		isActiveFn: func(n *libvirt.Network) (bool, error) {
			return true, nil
		},
		isPersistentFn: func(n *libvirt.Network) (bool, error) {
			return true, nil
		},
		getAutostartFn: func(n *libvirt.Network) (bool, error) {
			return true, nil
		},
	}
	ext := &external{client: mock}
	obs, err := ext.Observe(context.Background(), testNetwork())
	if err != nil {
		t.Errorf("Observe failed: %v", err)
	}
	if !obs.ResourceExists {
		t.Error("ResourceExists should be true for found network")
	}
}

func TestObserveInactive(t *testing.T) {
	mock := &mockNetworkClient{
		lookupByNameFn: func(name string) (*libvirt.Network, error) {
			return &libvirt.Network{}, nil
		},
		isActiveFn: func(n *libvirt.Network) (bool, error) {
			return false, nil
		},
		isPersistentFn: func(n *libvirt.Network) (bool, error) {
			return false, nil
		},
		getAutostartFn: func(n *libvirt.Network) (bool, error) {
			return false, nil
		},
	}
	ext := &external{client: mock}
	obs, err := ext.Observe(context.Background(), testNetwork())
	if err != nil {
		t.Errorf("Observe failed: %v", err)
	}
	if !obs.ResourceExists {
		t.Error("ResourceExists should be true")
	}
}

func TestObserveIsActiveError(t *testing.T) {
	mock := &mockNetworkClient{
		lookupByNameFn: func(name string) (*libvirt.Network, error) {
			return &libvirt.Network{}, nil
		},
		isActiveFn: func(n *libvirt.Network) (bool, error) {
			return false, errors.New("isactive failed")
		},
	}
	ext := &external{client: mock}
	_, err := ext.Observe(context.Background(), testNetwork())
	if err == nil {
		t.Error("Observe should fail when isactive fails")
	}
}

func TestObserveIsPersistentError(t *testing.T) {
	mock := &mockNetworkClient{
		lookupByNameFn: func(name string) (*libvirt.Network, error) {
			return &libvirt.Network{}, nil
		},
		isActiveFn: func(n *libvirt.Network) (bool, error) {
			return true, nil
		},
		isPersistentFn: func(n *libvirt.Network) (bool, error) {
			return false, errors.New("ispersistent failed")
		},
	}
	ext := &external{client: mock}
	_, err := ext.Observe(context.Background(), testNetwork())
	if err == nil {
		t.Error("Observe should fail when ispersistent fails")
	}
}

func TestObserveGetAutostartError(t *testing.T) {
	mock := &mockNetworkClient{
		lookupByNameFn: func(name string) (*libvirt.Network, error) {
			return &libvirt.Network{}, nil
		},
		isActiveFn: func(n *libvirt.Network) (bool, error) {
			return true, nil
		},
		isPersistentFn: func(n *libvirt.Network) (bool, error) {
			return true, nil
		},
		getAutostartFn: func(n *libvirt.Network) (bool, error) {
			return false, errors.New("getautostart failed")
		},
	}
	ext := &external{client: mock}
	_, err := ext.Observe(context.Background(), testNetwork())
	if err == nil {
		t.Error("Observe should fail when getautostart fails")
	}
}

func TestCreateSetAutostartError(t *testing.T) {
	autostart := true
	cr := testNetwork(func(n *v1beta1.Network) {
		n.Spec.ForProvider.Autostart = &autostart
	})

	mock := &mockNetworkClient{
		defineXMLFn: func(xml string) (*libvirt.Network, error) {
			return &libvirt.Network{}, nil
		},
		createFn: func(n *libvirt.Network) error {
			return nil
		},
		setAutostartFn: func(n *libvirt.Network, autostart bool) error {
			return errors.New("setautostart failed")
		},
	}
	ext := &external{client: mock}
	_, err := ext.Create(context.Background(), cr)
	if err == nil {
		t.Error("Create should fail when setautostart fails")
	}
}

func TestUpdateSetAutostartError(t *testing.T) {
	mock := &mockNetworkClient{
		lookupByNameFn: func(name string) (*libvirt.Network, error) {
			return &libvirt.Network{}, nil
		},
		setAutostartFn: func(n *libvirt.Network, autostart bool) error {
			return errors.New("setautostart failed")
		},
	}
	ext := &external{client: mock}
	_, err := ext.Update(context.Background(), testNetwork())
	if err == nil {
		t.Error("Update should fail when setautostart fails")
	}
}

func TestDeleteDestroyError(t *testing.T) {
	mock := &mockNetworkClient{
		lookupByNameFn: func(name string) (*libvirt.Network, error) {
			return &libvirt.Network{}, nil
		},
		isActiveFn: func(n *libvirt.Network) (bool, error) {
			return true, nil
		},
		destroyFn: func(n *libvirt.Network) error {
			return errors.New("destroy failed")
		},
	}
	ext := &external{client: mock}
	_, err := ext.Delete(context.Background(), testNetwork())
	if err == nil {
		t.Error("Delete should fail when destroy fails")
	}
}

func TestDeleteUndefineError(t *testing.T) {
	mock := &mockNetworkClient{
		lookupByNameFn: func(name string) (*libvirt.Network, error) {
			return &libvirt.Network{}, nil
		},
		isActiveFn: func(n *libvirt.Network) (bool, error) {
			return false, nil
		},
		undefineFn: func(n *libvirt.Network) error {
			return errors.New("undefine failed")
		},
	}
	ext := &external{client: mock}
	_, err := ext.Delete(context.Background(), testNetwork())
	if err == nil {
		t.Error("Delete should fail when undefine fails")
	}
}
