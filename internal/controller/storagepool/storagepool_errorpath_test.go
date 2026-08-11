package storagepool

import (
	"context"
	"github.com/pkg/errors"

	"github.com/rossigee/provider-libvirt/apis/v1beta1"
	"libvirt.org/go/libvirt"
	"testing"
)

type mockStoragePoolClient struct {
	lookupByNameFn func(name string) (*libvirt.StoragePool, error)
	defineXMLFn    func(xml string, flags uint32) (*libvirt.StoragePool, error)
	createFn       func(sp *libvirt.StoragePool, flags uint32) error
	destroyFn      func(sp *libvirt.StoragePool) error
	undefineFn     func(sp *libvirt.StoragePool) error
	isActiveFn     func(sp *libvirt.StoragePool) (bool, error)
	isPersistentFn func(sp *libvirt.StoragePool) (bool, error)
	getAutostartFn func(sp *libvirt.StoragePool) (bool, error)
	setAutostartFn func(sp *libvirt.StoragePool, autostart bool) error
	getInfoFn      func(sp *libvirt.StoragePool) (*libvirt.StoragePoolInfo, error)
	buildFn        func(sp *libvirt.StoragePool, flags uint32) error
	closeFn        func() error
	closeCalled    bool
}

func (m *mockStoragePoolClient) StoragePoolLookupByName(name string) (*libvirt.StoragePool, error) {
	if m.lookupByNameFn != nil {
		return m.lookupByNameFn(name)
	}
	return &libvirt.StoragePool{}, nil
}

func (m *mockStoragePoolClient) StoragePoolDefineXML(xml string, flags uint32) (*libvirt.StoragePool, error) {
	if m.defineXMLFn != nil {
		return m.defineXMLFn(xml, flags)
	}
	return &libvirt.StoragePool{}, nil
}

func (m *mockStoragePoolClient) StoragePoolCreate(sp *libvirt.StoragePool, flags uint32) error {
	if m.createFn != nil {
		return m.createFn(sp, flags)
	}
	return nil
}

func (m *mockStoragePoolClient) StoragePoolDestroy(sp *libvirt.StoragePool) error {
	if m.destroyFn != nil {
		return m.destroyFn(sp)
	}
	return nil
}

func (m *mockStoragePoolClient) StoragePoolUndefine(sp *libvirt.StoragePool) error {
	if m.undefineFn != nil {
		return m.undefineFn(sp)
	}
	return nil
}

func (m *mockStoragePoolClient) StoragePoolIsActive(sp *libvirt.StoragePool) (bool, error) {
	if m.isActiveFn != nil {
		return m.isActiveFn(sp)
	}
	return false, nil
}

func (m *mockStoragePoolClient) StoragePoolIsPersistent(sp *libvirt.StoragePool) (bool, error) {
	if m.isPersistentFn != nil {
		return m.isPersistentFn(sp)
	}
	return false, nil
}

func (m *mockStoragePoolClient) StoragePoolGetAutostart(sp *libvirt.StoragePool) (bool, error) {
	if m.getAutostartFn != nil {
		return m.getAutostartFn(sp)
	}
	return false, nil
}

func (m *mockStoragePoolClient) StoragePoolSetAutostart(sp *libvirt.StoragePool, autostart bool) error {
	if m.setAutostartFn != nil {
		return m.setAutostartFn(sp, autostart)
	}
	return nil
}

func (m *mockStoragePoolClient) StoragePoolGetInfo(sp *libvirt.StoragePool) (*libvirt.StoragePoolInfo, error) {
	if m.getInfoFn != nil {
		return m.getInfoFn(sp)
	}
	return &libvirt.StoragePoolInfo{}, nil
}

func (m *mockStoragePoolClient) StoragePoolBuild(sp *libvirt.StoragePool, flags uint32) error {
	if m.buildFn != nil {
		return m.buildFn(sp, flags)
	}
	return nil
}

func (m *mockStoragePoolClient) Close() error {
	m.closeCalled = true
	if m.closeFn != nil {
		return m.closeFn()
	}
	return nil
}

func TestStoragePoolDisconnect(t *testing.T) {
	mock := &mockStoragePoolClient{}
	e := &external{client: mock}

	if err := e.Disconnect(context.Background()); err != nil {
		t.Fatalf("Disconnect() returned unexpected error: %v", err)
	}
	if !mock.closeCalled {
		t.Error("Disconnect() did not close the underlying libvirt connection")
	}
}

func TestStoragePoolDisconnectPropagatesCloseError(t *testing.T) {
	wantErr := errors.New("boom")
	mock := &mockStoragePoolClient{closeFn: func() error { return wantErr }}
	e := &external{client: mock}

	err := e.Disconnect(context.Background())
	if err == nil || err.Error() != wantErr.Error() {
		t.Errorf("Disconnect() = %v, want error %v", err, wantErr)
	}
}

// Error path tests

func TestObserveWrongResourceType(t *testing.T) {
	ext := &external{client: &mockStoragePoolClient{}}
	obs, err := ext.Observe(context.Background(), &v1beta1.Domain{})
	if err == nil {
		t.Error("Observe should fail for wrong resource type")
	}
	if obs.ResourceExists {
		t.Error("ResourceExists should be false on error")
	}
}

func TestObserveNotFound(t *testing.T) {
	mock := &mockStoragePoolClient{
		lookupByNameFn: func(name string) (*libvirt.StoragePool, error) {
			return nil, errors.New("StoragePool not found")
		},
	}
	ext := &external{client: mock}
	obs, err := ext.Observe(context.Background(), testStoragePool())
	if err != nil {
		t.Errorf("Observe should succeed for not-found pool: %v", err)
	}
	if obs.ResourceExists {
		t.Error("ResourceExists should be false for not-found pool")
	}
}

func TestCreateWrongResourceType(t *testing.T) {
	ext := &external{client: &mockStoragePoolClient{}}
	_, err := ext.Create(context.Background(), &v1beta1.Domain{})
	if err == nil {
		t.Error("Create should fail for wrong resource type")
	}
}

func TestCreateDefineError(t *testing.T) {
	mock := &mockStoragePoolClient{
		defineXMLFn: func(xml string, flags uint32) (*libvirt.StoragePool, error) {
			return nil, errors.New("invalid XML")
		},
	}
	ext := &external{client: mock}
	_, err := ext.Create(context.Background(), testStoragePool())
	if err == nil {
		t.Error("Create should fail on define error")
	}
}

func TestCreateBuildError(t *testing.T) {
	mock := &mockStoragePoolClient{
		defineXMLFn: func(xml string, flags uint32) (*libvirt.StoragePool, error) {
			return &libvirt.StoragePool{}, nil
		},
		buildFn: func(sp *libvirt.StoragePool, flags uint32) error {
			return errors.New("build failed")
		},
	}
	ext := &external{client: mock}
	_, err := ext.Create(context.Background(), testStoragePool())
	if err == nil {
		t.Error("Create should fail on build error")
	}
}

func TestDeleteWrongResourceType(t *testing.T) {
	ext := &external{client: &mockStoragePoolClient{}}
	_, err := ext.Delete(context.Background(), &v1beta1.Domain{})
	if err == nil {
		t.Error("Delete should fail for wrong resource type")
	}
}

func TestDeleteNotFound(t *testing.T) {
	mock := &mockStoragePoolClient{
		lookupByNameFn: func(name string) (*libvirt.StoragePool, error) {
			return nil, errors.New("StoragePool not found")
		},
	}
	ext := &external{client: mock}
	_, err := ext.Delete(context.Background(), testStoragePool())
	if err != nil {
		t.Errorf("Delete should succeed (idempotent) for not-found pool: %v", err)
	}
}

func TestUpdateWrongResourceType(t *testing.T) {
	ext := &external{client: &mockStoragePoolClient{}}
	_, err := ext.Update(context.Background(), &v1beta1.Domain{})
	if err == nil {
		t.Error("Update should fail for wrong resource type")
	}
}

func TestUpdatePoolNotFound(t *testing.T) {
	mock := &mockStoragePoolClient{
		lookupByNameFn: func(name string) (*libvirt.StoragePool, error) {
			return nil, errors.New("StoragePool not found")
		},
	}
	ext := &external{client: mock}
	_, err := ext.Update(context.Background(), testStoragePool())
	if err == nil {
		t.Error("Update should fail when pool not found")
	}
}

func TestObserveActive(t *testing.T) {
	mock := &mockStoragePoolClient{
		lookupByNameFn: func(name string) (*libvirt.StoragePool, error) {
			return &libvirt.StoragePool{}, nil
		},
		isActiveFn: func(sp *libvirt.StoragePool) (bool, error) {
			return true, nil
		},
		isPersistentFn: func(sp *libvirt.StoragePool) (bool, error) {
			return true, nil
		},
		getAutostartFn: func(sp *libvirt.StoragePool) (bool, error) {
			return true, nil
		},
		getInfoFn: func(sp *libvirt.StoragePool) (*libvirt.StoragePoolInfo, error) {
			return &libvirt.StoragePoolInfo{
				Capacity:   1073741824,
				Allocation: 536870912,
				Available:  536870912,
			}, nil
		},
	}
	ext := &external{client: mock}
	obs, err := ext.Observe(context.Background(), testStoragePool())
	if err != nil {
		t.Errorf("Observe failed: %v", err)
	}
	if !obs.ResourceExists {
		t.Error("ResourceExists should be true for found pool")
	}
	if !obs.ResourceUpToDate {
		t.Error("ResourceUpToDate should be true")
	}
}

func TestObserveInactive(t *testing.T) {
	mock := &mockStoragePoolClient{
		lookupByNameFn: func(name string) (*libvirt.StoragePool, error) {
			return &libvirt.StoragePool{}, nil
		},
		isActiveFn: func(sp *libvirt.StoragePool) (bool, error) {
			return false, nil
		},
		isPersistentFn: func(sp *libvirt.StoragePool) (bool, error) {
			return true, nil
		},
		getAutostartFn: func(sp *libvirt.StoragePool) (bool, error) {
			return false, nil
		},
		getInfoFn: func(sp *libvirt.StoragePool) (*libvirt.StoragePoolInfo, error) {
			return &libvirt.StoragePoolInfo{}, nil
		},
	}
	ext := &external{client: mock}
	obs, err := ext.Observe(context.Background(), testStoragePool())
	if err != nil {
		t.Errorf("Observe failed: %v", err)
	}
	if !obs.ResourceExists {
		t.Error("ResourceExists should be true")
	}
}

func TestObserveInfoError(t *testing.T) {
	mock := &mockStoragePoolClient{
		lookupByNameFn: func(name string) (*libvirt.StoragePool, error) {
			return &libvirt.StoragePool{}, nil
		},
		isActiveFn: func(sp *libvirt.StoragePool) (bool, error) {
			return true, nil
		},
		isPersistentFn: func(sp *libvirt.StoragePool) (bool, error) {
			return true, nil
		},
		getAutostartFn: func(sp *libvirt.StoragePool) (bool, error) {
			return true, nil
		},
		getInfoFn: func(sp *libvirt.StoragePool) (*libvirt.StoragePoolInfo, error) {
			return nil, errors.New("getinfo failed")
		},
	}
	ext := &external{client: mock}
	_, err := ext.Observe(context.Background(), testStoragePool())
	if err == nil {
		t.Error("Observe should fail when getinfo fails")
	}
}

func TestCreateStartError(t *testing.T) {
	mock := &mockStoragePoolClient{
		defineXMLFn: func(xml string, flags uint32) (*libvirt.StoragePool, error) {
			return &libvirt.StoragePool{}, nil
		},
		buildFn: func(sp *libvirt.StoragePool, flags uint32) error {
			return nil
		},
		createFn: func(sp *libvirt.StoragePool, flags uint32) error {
			return errors.New("start failed")
		},
	}
	ext := &external{client: mock}
	_, err := ext.Create(context.Background(), testStoragePool())
	if err == nil {
		t.Error("Create should fail when start fails")
	}
}

func TestCreateSetAutostartError(t *testing.T) {
	autostart := true
	cr := testStoragePool(func(p *v1beta1.StoragePool) {
		p.Spec.ForProvider.Autostart = &autostart
	})

	mock := &mockStoragePoolClient{
		defineXMLFn: func(xml string, flags uint32) (*libvirt.StoragePool, error) {
			return &libvirt.StoragePool{}, nil
		},
		buildFn: func(sp *libvirt.StoragePool, flags uint32) error {
			return nil
		},
		createFn: func(sp *libvirt.StoragePool, flags uint32) error {
			return nil
		},
		setAutostartFn: func(sp *libvirt.StoragePool, autostart bool) error {
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
	mock := &mockStoragePoolClient{
		lookupByNameFn: func(name string) (*libvirt.StoragePool, error) {
			return &libvirt.StoragePool{}, nil
		},
		setAutostartFn: func(sp *libvirt.StoragePool, autostart bool) error {
			return errors.New("setautostart failed")
		},
	}
	ext := &external{client: mock}
	_, err := ext.Update(context.Background(), testStoragePool())
	if err == nil {
		t.Error("Update should fail when setautostart fails")
	}
}

func TestDeleteDestroyError(t *testing.T) {
	mock := &mockStoragePoolClient{
		lookupByNameFn: func(name string) (*libvirt.StoragePool, error) {
			return &libvirt.StoragePool{}, nil
		},
		isActiveFn: func(sp *libvirt.StoragePool) (bool, error) {
			return true, nil
		},
		destroyFn: func(sp *libvirt.StoragePool) error {
			return errors.New("destroy failed")
		},
	}
	ext := &external{client: mock}
	_, err := ext.Delete(context.Background(), testStoragePool())
	if err == nil {
		t.Error("Delete should fail when destroy fails")
	}
}

func TestDeleteUndefineError(t *testing.T) {
	mock := &mockStoragePoolClient{
		lookupByNameFn: func(name string) (*libvirt.StoragePool, error) {
			return &libvirt.StoragePool{}, nil
		},
		destroyFn: func(sp *libvirt.StoragePool) error {
			return nil
		},
		undefineFn: func(sp *libvirt.StoragePool) error {
			return errors.New("undefine failed")
		},
	}
	ext := &external{client: mock}
	_, err := ext.Delete(context.Background(), testStoragePool())
	if err == nil {
		t.Error("Delete should fail when undefine fails")
	}
}
