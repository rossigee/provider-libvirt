package volume

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"libvirt.org/go/libvirt"

	"github.com/rossigee/provider-libvirt/apis/v1beta1"
)

type mockVolumeClient struct {
	lookupByNameFn    func(pool *libvirt.StoragePool, name string) (*libvirt.StorageVol, error)
	createXMLFn       func(pool *libvirt.StoragePool, xml string, flags uint32) (*libvirt.StorageVol, error)
	deleteFn          func(vol *libvirt.StorageVol, flags uint32) error
	getInfoFn         func(vol *libvirt.StorageVol) (*libvirt.StorageVolInfo, error)
	resizeFn          func(volume *libvirt.StorageVol, capacity uint64, flags libvirt.StorageVolResizeFlags) error
	poolLookupByNameFn func(name string) (*libvirt.StoragePool, error)
}

func (m *mockVolumeClient) StorageVolLookupByName(pool *libvirt.StoragePool, name string) (*libvirt.StorageVol, error) {
	if m.lookupByNameFn != nil {
		return m.lookupByNameFn(pool, name)
	}
	return &libvirt.StorageVol{}, nil
}

func (m *mockVolumeClient) StorageVolCreateXML(pool *libvirt.StoragePool, xml string, flags uint32) (*libvirt.StorageVol, error) {
	if m.createXMLFn != nil {
		return m.createXMLFn(pool, xml, flags)
	}
	return &libvirt.StorageVol{}, nil
}

func (m *mockVolumeClient) StorageVolDelete(vol *libvirt.StorageVol, flags uint32) error {
	if m.deleteFn != nil {
		return m.deleteFn(vol, flags)
	}
	return nil
}

func (m *mockVolumeClient) StorageVolGetInfo(vol *libvirt.StorageVol) (*libvirt.StorageVolInfo, error) {
	if m.getInfoFn != nil {
		return m.getInfoFn(vol)
	}
	return &libvirt.StorageVolInfo{}, nil
}

func (m *mockVolumeClient) StorageVolResize(volume *libvirt.StorageVol, capacity uint64, flags libvirt.StorageVolResizeFlags) error {
	if m.resizeFn != nil {
		return m.resizeFn(volume, capacity, flags)
	}
	return nil
}

func (m *mockVolumeClient) StoragePoolLookupByName(name string) (*libvirt.StoragePool, error) {
	if m.poolLookupByNameFn != nil {
		return m.poolLookupByNameFn(name)
	}
	return &libvirt.StoragePool{}, nil
}

// Error path tests

func TestObserveWrongResourceType(t *testing.T) {
	ext := &external{client: &mockVolumeClient{}}
	obs, err := ext.Observe(context.Background(), &v1beta1.Domain{})
	if err == nil {
		t.Error("Observe should fail for wrong resource type")
	}
	if obs.ResourceExists {
		t.Error("ResourceExists should be false on error")
	}
}

func TestObservePoolNotFound(t *testing.T) {
	mock := &mockVolumeClient{
		poolLookupByNameFn: func(name string) (*libvirt.StoragePool, error) {
			return nil, errors.New("StoragePool not found")
		},
	}
	ext := &external{client: mock}
	obs, err := ext.Observe(context.Background(), testVolume())
	if err != nil {
		t.Errorf("Observe should succeed for not-found pool: %v", err)
	}
	if obs.ResourceExists {
		t.Error("ResourceExists should be false for not-found pool")
	}
}

func TestCreateWrongResourceType(t *testing.T) {
	ext := &external{client: &mockVolumeClient{}}
	_, err := ext.Create(context.Background(), &v1beta1.Domain{})
	if err == nil {
		t.Error("Create should fail for wrong resource type")
	}
}

func TestCreatePoolNotFound(t *testing.T) {
	mock := &mockVolumeClient{
		poolLookupByNameFn: func(name string) (*libvirt.StoragePool, error) {
			return nil, errors.New("StoragePool not found")
		},
	}
	ext := &external{client: mock}
	_, err := ext.Create(context.Background(), testVolume())
	if err == nil {
		t.Error("Create should fail when pool not found")
	}
}

func TestCreateXMLError(t *testing.T) {
	mock := &mockVolumeClient{
		poolLookupByNameFn: func(name string) (*libvirt.StoragePool, error) {
			return &libvirt.StoragePool{}, nil
		},
		createXMLFn: func(pool *libvirt.StoragePool, xml string, flags uint32) (*libvirt.StorageVol, error) {
			return nil, errors.New("invalid volume XML")
		},
	}
	ext := &external{client: mock}
	_, err := ext.Create(context.Background(), testVolume())
	if err == nil {
		t.Error("Create should fail on XML error")
	}
}

func TestDeleteWrongResourceType(t *testing.T) {
	ext := &external{client: &mockVolumeClient{}}
	_, err := ext.Delete(context.Background(), &v1beta1.Domain{})
	if err == nil {
		t.Error("Delete should fail for wrong resource type")
	}
}

func TestDeletePoolNotFound(t *testing.T) {
	mock := &mockVolumeClient{
		poolLookupByNameFn: func(name string) (*libvirt.StoragePool, error) {
			return nil, errors.New("StoragePool not found")
		},
	}
	ext := &external{client: mock}
	_, err := ext.Delete(context.Background(), testVolume())
	if err != nil {
		t.Errorf("Delete should succeed (idempotent) for not-found pool: %v", err)
	}
}

func TestUpdateWrongResourceType(t *testing.T) {
	ext := &external{client: &mockVolumeClient{}}
	_, err := ext.Update(context.Background(), &v1beta1.Domain{})
	if err == nil {
		t.Error("Update should fail for wrong resource type")
	}
}
