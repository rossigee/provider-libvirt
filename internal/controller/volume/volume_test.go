/*
Copyright 2025 Ross Golder
*/

package volume

import (
	"context"
	"testing"

	"github.com/digitalocean/go-libvirt"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/rossigee/provider-libvirt/apis/v1alpha1"
)

// mockVolumeService implements storage operations for testing
type mockVolumeService struct {
	pools       map[string]*mockStoragePool
	volumes     map[string]*mockStorageVolume
	createError error
	deleteError error
	lookupError error
	getInfoError error
}

type mockStoragePool struct {
	libvirt.StoragePool
	name    string
	volumes map[string]*mockStorageVolume
}

type mockStorageVolume struct {
	libvirt.StorageVol
	key        string
	name       string
	path       string
	volType    int8
	capacity   uint64
	allocation uint64
	format     string
	pool       string
}

func newMockVolumeService() *mockVolumeService {
	return &mockVolumeService{
		pools:   make(map[string]*mockStoragePool),
		volumes: make(map[string]*mockStorageVolume),
	}
}

func (m *mockVolumeService) StoragePool(name string) (libvirt.StoragePool, error) {
	if m.lookupError != nil {
		return libvirt.StoragePool{}, m.lookupError
	}
	
	pool, exists := m.pools[name]
	if !exists {
		return libvirt.StoragePool{}, errors.New("Storage pool not found: no storage pool with matching name")
	}
	
	return pool.StoragePool, nil
}

func (m *mockVolumeService) StorageVolLookupByName(pool libvirt.StoragePool, name string) (libvirt.StorageVol, error) {
	if m.lookupError != nil {
		return libvirt.StorageVol{}, m.lookupError
	}
	
	// Find the pool
	var poolName string
	for pName, p := range m.pools {
		if p.Name == pool.Name {
			poolName = pName
			break
		}
	}
	
	if poolName == "" {
		return libvirt.StorageVol{}, errors.New("pool not found")
	}
	
	volume, exists := m.pools[poolName].volumes[name]
	if !exists {
		return libvirt.StorageVol{}, errors.New("Storage volume not found: no storage vol with matching name")
	}
	
	return volume.StorageVol, nil
}

func (m *mockVolumeService) StorageVolGetInfo(vol libvirt.StorageVol) (int8, uint64, uint64, error) {
	if m.getInfoError != nil {
		return 0, 0, 0, m.getInfoError
	}
	
	// Find volume by key
	for _, volume := range m.volumes {
		if volume.key == vol.Key {
			return volume.volType, volume.capacity, volume.allocation, nil
		}
	}
	
	return 0, 0, 0, errors.New("volume not found")
}

func (m *mockVolumeService) StorageVolGetXMLDesc(vol libvirt.StorageVol, flags uint32) (string, error) {
	// Find volume by key
	for _, volume := range m.volumes {
		if volume.key == vol.Key {
			return generateMockVolumeXML(volume), nil
		}
	}
	
	return "", errors.New("volume not found")
}

func (m *mockVolumeService) StorageVolCreateXML(pool libvirt.StoragePool, xml string, flags libvirt.StorageVolCreateFlags) (libvirt.StorageVol, error) {
	if m.createError != nil {
		return libvirt.StorageVol{}, m.createError
	}
	
	// Parse volume name from XML (simplified)
	name := extractNameFromXML(xml)
	key := generateVolumeKey(name)
	path := "/var/lib/libvirt/images/" + name + ".qcow2"
	
	volume := &mockStorageVolume{
		StorageVol: libvirt.StorageVol{
			Key:  key,
			Name: name,
		},
		key:        key,
		name:       name,
		path:       path,
		volType:    int8(libvirt.StorageVolFile),
		capacity:   10737418240, // 10GB default
		allocation: 1073741824,  // 1GB default
		format:     "qcow2",
		pool:       pool.Name,
	}
	
	// Find pool and add volume
	for _, p := range m.pools {
		if p.Name == pool.Name {
			p.volumes[name] = volume
			m.volumes[key] = volume
			break
		}
	}
	
	return volume.StorageVol, nil
}

func (m *mockVolumeService) StorageVolCreateXMLFrom(pool libvirt.StoragePool, xml string, clonevol libvirt.StorageVol, flags libvirt.StorageVolCreateFlags) (libvirt.StorageVol, error) {
	// Similar to StorageVolCreateXML but cloning from another volume
	return m.StorageVolCreateXML(pool, xml, flags)
}

func (m *mockVolumeService) StorageVolDelete(vol libvirt.StorageVol, flags libvirt.StorageVolDeleteFlags) error {
	if m.deleteError != nil {
		return m.deleteError
	}
	
	// Remove volume from all collections
	delete(m.volumes, vol.Key)
	for _, pool := range m.pools {
		for name, volume := range pool.volumes {
			if volume.key == vol.Key {
				delete(pool.volumes, name)
				break
			}
		}
	}
	
	return nil
}

func (m *mockVolumeService) StorageVolResize(vol libvirt.StorageVol, capacity uint64, flags libvirt.StorageVolResizeFlags) error {
	// Find volume and update capacity
	for _, volume := range m.volumes {
		if volume.key == vol.Key {
			volume.capacity = capacity
			return nil
		}
	}
	return errors.New("volume not found")
}

// Helper functions

func generateMockVolumeXML(volume *mockStorageVolume) string {
	return `<volume type='file'>
  <name>` + volume.name + `</name>
  <target>
    <path>` + volume.path + `</path>
    <format type='` + volume.format + `'/>
  </target>
</volume>`
}

func extractNameFromXML(xml string) string {
	// Simplified XML parsing for test
	if start := findStringBetween(xml, "<name>", "</name>"); start != "" {
		return start
	}
	return "test-volume"
}

func generateVolumeKey(name string) string {
	return "/var/lib/libvirt/images/" + name + ".qcow2"
}

func findStringBetween(str, start, end string) string {
	// Simple helper to extract text between two strings
	s := findIndex(str, start)
	if s == -1 {
		return ""
	}
	s += len(start)
	e := findIndex(str[s:], end)
	if e == -1 {
		return ""
	}
	return str[s : s+e]
}

func findIndex(str, substr string) int {
	for i := 0; i <= len(str)-len(substr); i++ {
		if str[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// mockVolumeExternal wraps external with a mock service for testing
type mockVolumeExternal struct {
	mockService *mockVolumeService
	kube        client.Client
}

func (e *mockVolumeExternal) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.Volume)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotVolume)
	}

	volumeName := cr.Spec.ForProvider.Name
	poolName := cr.Spec.ForProvider.Pool

	pool, err := e.mockService.StoragePool(poolName)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errDescribeVolume)
	}

	volume, err := e.mockService.StorageVolLookupByName(pool, volumeName)
	if err != nil {
		if isVolumeNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, errDescribeVolume)
	}

	volType, capacity, allocation, err := e.mockService.StorageVolGetInfo(volume)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errDescribeVolume)
	}

	xml, err := e.mockService.StorageVolGetXMLDesc(volume, 0)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errDescribeVolume)
	}

	format, path := parseVolumeXML(xml)

	// Update status
	cr.Status.AtProvider.Key = volume.Key
	cr.Status.AtProvider.Path = path
	cr.Status.AtProvider.Type = storageVolTypeToString(libvirt.StorageVolType(volType))
	cr.Status.AtProvider.Capacity = int64(capacity)
	cr.Status.AtProvider.Allocation = int64(allocation)
	cr.Status.AtProvider.Format = format
	cr.Status.AtProvider.Pool = poolName

	cr.SetConditions(xpv1.Available())

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: isVolumeUpToDate(cr, capacity),
	}, nil
}

func (e *mockVolumeExternal) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.Volume)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotVolume)
	}

	poolName := cr.Spec.ForProvider.Pool
	pool, err := e.mockService.StoragePool(poolName)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateVolume)
	}

	xml, err := generateVolumeXML(cr)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateVolume)
	}

	_, err = e.mockService.StorageVolCreateXML(pool, xml, libvirt.StorageVolCreatePreallocMetadata)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateVolume)
	}

	return managed.ExternalCreation{}, nil
}

func (e *mockVolumeExternal) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	return managed.ExternalUpdate{}, nil
}

func (e *mockVolumeExternal) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*v1alpha1.Volume)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotVolume)
	}

	volumeName := cr.Spec.ForProvider.Name
	poolName := cr.Spec.ForProvider.Pool

	pool, err := e.mockService.StoragePool(poolName)
	if err != nil {
		if isPoolNotFound(err) {
			return managed.ExternalDelete{}, nil
		}
		return managed.ExternalDelete{}, errors.Wrap(err, errDeleteVolume)
	}

	volume, err := e.mockService.StorageVolLookupByName(pool, volumeName)
	if err != nil {
		if isVolumeNotFound(err) {
			return managed.ExternalDelete{}, nil
		}
		return managed.ExternalDelete{}, errors.Wrap(err, errDeleteVolume)
	}

	err = e.mockService.StorageVolDelete(volume, libvirt.StorageVolDeleteNormal)
	if err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, errDeleteVolume)
	}

	return managed.ExternalDelete{}, nil
}

func (e *mockVolumeExternal) Disconnect(ctx context.Context) error {
	return nil
}

func createMockVolumeExternal(mockService *mockVolumeService) *mockVolumeExternal {
	return &mockVolumeExternal{
		mockService: mockService,
		kube:        fake.NewClientBuilder().Build(),
	}
}

// Test cases

func TestVolumeExternal_Observe(t *testing.T) {
	tests := []struct {
		name      string
		mockSetup func(*mockVolumeService)
		volume    *v1alpha1.Volume
		want      managed.ExternalObservation
		wantErr   bool
		errMsg    string
	}{
		{
			name: "VolumeNotFound",
			mockSetup: func(m *mockVolumeService) {
				// Create pool but no volume
				pool := &mockStoragePool{
					StoragePool: libvirt.StoragePool{Name: "default"},
					name:        "default",
					volumes:     make(map[string]*mockStorageVolume),
				}
				m.pools["default"] = pool
				// Don't set lookup error here - the mock will return "not found" for missing volumes
			},
			volume: &v1alpha1.Volume{
				Spec: v1alpha1.VolumeSpec{
					ForProvider: v1alpha1.VolumeParameters{
						Name: "nonexistent-volume",
						Pool: "default",
					},
				},
			},
			want: managed.ExternalObservation{
				ResourceExists: false,
			},
			wantErr: false,
		},
		{
			name: "VolumeExists",
			mockSetup: func(m *mockVolumeService) {
				// Create pool and volume
				pool := &mockStoragePool{
					StoragePool: libvirt.StoragePool{Name: "default"},
					name:        "default",
					volumes:     make(map[string]*mockStorageVolume),
				}
				volume := &mockStorageVolume{
					StorageVol: libvirt.StorageVol{
						Key:  "/var/lib/libvirt/images/test-volume.qcow2",
						Name: "test-volume",
					},
					key:        "/var/lib/libvirt/images/test-volume.qcow2",
					name:       "test-volume",
					path:       "/var/lib/libvirt/images/test-volume.qcow2",
					volType:    int8(libvirt.StorageVolFile),
					capacity:   10737418240,
					allocation: 1073741824,
					format:     "qcow2",
					pool:       "default",
				}
				pool.volumes["test-volume"] = volume
				m.pools["default"] = pool
				m.volumes[volume.key] = volume
			},
			volume: &v1alpha1.Volume{
				Spec: v1alpha1.VolumeSpec{
					ForProvider: v1alpha1.VolumeParameters{
						Name:     "test-volume",
						Pool:     "default",
						Capacity: 10737418240,
						Format:   "qcow2",
					},
				},
			},
			want: managed.ExternalObservation{
				ResourceExists:   true,
				ResourceUpToDate: true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := newMockVolumeService()
			if tt.mockSetup != nil {
				tt.mockSetup(mockService)
			}
			
			e := createMockVolumeExternal(mockService)
			
			got, err := e.Observe(context.Background(), tt.volume)
			
			if tt.wantErr {
				if err == nil {
					t.Errorf("Observe() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if tt.errMsg != "" && !containsSubstring(err.Error(), tt.errMsg) {
					t.Errorf("Observe() error = %v, want error containing %v", err, tt.errMsg)
				}
				return
			}
			
			if err != nil {
				t.Errorf("Observe() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if got.ResourceExists != tt.want.ResourceExists {
				t.Errorf("Observe() ResourceExists = %v, want %v", got.ResourceExists, tt.want.ResourceExists)
			}
			
			if got.ResourceUpToDate != tt.want.ResourceUpToDate {
				t.Errorf("Observe() ResourceUpToDate = %v, want %v", got.ResourceUpToDate, tt.want.ResourceUpToDate)
			}
		})
	}
}

func TestVolumeExternal_Create(t *testing.T) {
	tests := []struct {
		name      string
		mockSetup func(*mockVolumeService)
		volume    *v1alpha1.Volume
		wantErr   bool
		errMsg    string
		validate  func(*testing.T, *mockVolumeService, *v1alpha1.Volume)
	}{
		{
			name: "CreateVolumeSuccess",
			mockSetup: func(m *mockVolumeService) {
				// Create pool
				pool := &mockStoragePool{
					StoragePool: libvirt.StoragePool{Name: "default"},
					name:        "default",
					volumes:     make(map[string]*mockStorageVolume),
				}
				m.pools["default"] = pool
			},
			volume: &v1alpha1.Volume{
				Spec: v1alpha1.VolumeSpec{
					ForProvider: v1alpha1.VolumeParameters{
						Name:     "test-volume",
						Pool:     "default",
						Capacity: 5368709120, // 5GB
						Format:   "qcow2",
					},
				},
			},
			wantErr: false,
			validate: func(t *testing.T, m *mockVolumeService, volume *v1alpha1.Volume) {
				volumeName := volume.Spec.ForProvider.Name
				poolName := volume.Spec.ForProvider.Pool
				
				pool, exists := m.pools[poolName]
				if !exists {
					t.Error("Expected pool to exist")
					return
				}
				
				_, exists = pool.volumes[volumeName]
				if !exists {
					t.Error("Expected volume to be created")
				}
			},
		},
		{
			name: "CreateError",
			mockSetup: func(m *mockVolumeService) {
				pool := &mockStoragePool{
					StoragePool: libvirt.StoragePool{Name: "default"},
					name:        "default",
					volumes:     make(map[string]*mockStorageVolume),
				}
				m.pools["default"] = pool
				m.createError = errors.New("failed to create volume")
			},
			volume: &v1alpha1.Volume{
				Spec: v1alpha1.VolumeSpec{
					ForProvider: v1alpha1.VolumeParameters{
						Name:     "test-volume",
						Pool:     "default",
						Capacity: 5368709120,
						Format:   "qcow2",
					},
				},
			},
			wantErr: true,
			errMsg:  errCreateVolume,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := newMockVolumeService()
			if tt.mockSetup != nil {
				tt.mockSetup(mockService)
			}
			
			e := createMockVolumeExternal(mockService)
			
			_, err := e.Create(context.Background(), tt.volume)
			
			if tt.wantErr {
				if err == nil {
					t.Errorf("Create() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if tt.errMsg != "" && !containsSubstring(err.Error(), tt.errMsg) {
					t.Errorf("Create() error = %v, want error containing %v", err, tt.errMsg)
				}
				return
			}
			
			if err != nil {
				t.Errorf("Create() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if tt.validate != nil {
				tt.validate(t, mockService, tt.volume)
			}
		})
	}
}

func TestGenerateVolumeXML(t *testing.T) {
	tests := []struct {
		name    string
		volume  *v1alpha1.Volume
		wantErr bool
		validate func(*testing.T, string)
	}{
		{
			name: "BasicVolume",
			volume: &v1alpha1.Volume{
				Spec: v1alpha1.VolumeSpec{
					ForProvider: v1alpha1.VolumeParameters{
						Name:     "test-volume",
						Pool:     "default",
						Capacity: 5368709120, // 5GB
						Format:   "qcow2",
					},
				},
			},
			wantErr: false,
			validate: func(t *testing.T, xml string) {
				if !containsSubstring(xml, "<name>test-volume</name>") {
					t.Error("XML should contain volume name")
				}
				if !containsSubstring(xml, "<capacity unit='bytes'>5368709120</capacity>") {
					t.Error("XML should contain capacity specification")
				}
				if !containsSubstring(xml, "<format type='qcow2'/>") {
					t.Error("XML should contain format specification")
				}
			},
		},
		{
			name: "VolumeWithBackingStore",
			volume: &v1alpha1.Volume{
				Spec: v1alpha1.VolumeSpec{
					ForProvider: v1alpha1.VolumeParameters{
						Name:     "test-volume",
						Pool:     "default",
						Capacity: 5368709120,
						Format:   "qcow2",
						BackingStore: &v1alpha1.VolumeBackingStore{
							Path:   "/var/lib/libvirt/images/base.qcow2",
							Format: "qcow2",
						},
					},
				},
			},
			wantErr: false,
			validate: func(t *testing.T, xml string) {
				if !containsSubstring(xml, "<backingStore>") {
					t.Error("XML should contain backing store specification")
				}
				if !containsSubstring(xml, "/var/lib/libvirt/images/base.qcow2") {
					t.Error("XML should contain backing store path")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			xml, err := generateVolumeXML(tt.volume)
			
			if tt.wantErr {
				if err == nil {
					t.Errorf("generateVolumeXML() error = nil, wantErr %v", tt.wantErr)
				}
				return
			}
			
			if err != nil {
				t.Errorf("generateVolumeXML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if tt.validate != nil {
				tt.validate(t, xml)
			}
		})
	}
}

// Helper function (reused from domain tests)
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}