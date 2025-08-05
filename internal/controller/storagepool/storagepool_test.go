/*
Copyright 2025 Ross Golder
*/

package storagepool

import (
	"context"
	"fmt"
	"testing"

	"github.com/digitalocean/go-libvirt"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/rossigee/provider-libvirt/apis/v1alpha1"
	"github.com/rossigee/provider-libvirt/internal/clients"
)

// mockStoragePoolService implements storage pool operations for testing
type mockStoragePoolService struct {
	pools          map[string]*mockStoragePoolInfo
	volumes        map[string][]libvirt.StorageVol
	createError    error
	deleteError    error
	lookupError    error
	startError     error
	stopError      error
	autostartError error
	buildError     error
}

type mockStoragePoolInfo struct {
	libvirt.StoragePool
	name       string
	uuid       [16]byte
	active     int32
	persistent int32
	autostart  int32
	state      uint8
	capacity   uint64
	allocation uint64
	available  uint64
	xml        string
}

func newMockStoragePoolService() *mockStoragePoolService {
	return &mockStoragePoolService{
		pools:   make(map[string]*mockStoragePoolInfo),
		volumes: make(map[string][]libvirt.StorageVol),
	}
}

func (m *mockStoragePoolService) StoragePoolLookupByName(name string) (libvirt.StoragePool, error) {
	if m.lookupError != nil {
		return libvirt.StoragePool{}, m.lookupError
	}

	pool, exists := m.pools[name]
	if !exists {
		return libvirt.StoragePool{}, errors.New("Storage pool not found: no storage pool with matching name")
	}

	return pool.StoragePool, nil
}

func (m *mockStoragePoolService) StoragePoolIsActive(pool libvirt.StoragePool) (int32, error) {
	for _, p := range m.pools {
		if p.StoragePool.Name == pool.Name {
			return p.active, nil
		}
	}
	return 0, errors.New("storage pool not found")
}

func (m *mockStoragePoolService) StoragePoolIsPersistent(pool libvirt.StoragePool) (int32, error) {
	for _, p := range m.pools {
		if p.StoragePool.Name == pool.Name {
			return p.persistent, nil
		}
	}
	return 0, errors.New("storage pool not found")
}

func (m *mockStoragePoolService) StoragePoolGetAutostart(pool libvirt.StoragePool) (int32, error) {
	if m.autostartError != nil {
		return 0, m.autostartError
	}

	for _, p := range m.pools {
		if p.StoragePool.Name == pool.Name {
			return p.autostart, nil
		}
	}
	return 0, errors.New("storage pool not found")
}

func (m *mockStoragePoolService) StoragePoolSetAutostart(pool libvirt.StoragePool, autostart int32) error {
	if m.autostartError != nil {
		return m.autostartError
	}

	for _, p := range m.pools {
		if p.StoragePool.Name == pool.Name {
			p.autostart = autostart
			return nil
		}
	}
	return errors.New("storage pool not found")
}

func (m *mockStoragePoolService) StoragePoolGetInfo(pool libvirt.StoragePool) (uint8, uint64, uint64, uint64, error) {
	for _, p := range m.pools {
		if p.StoragePool.Name == pool.Name {
			return p.state, p.capacity, p.allocation, p.available, nil
		}
	}
	return 0, 0, 0, 0, errors.New("storage pool not found")
}

func (m *mockStoragePoolService) StoragePoolGetXMLDesc(pool libvirt.StoragePool, flags uint32) (string, error) {
	for _, p := range m.pools {
		if p.StoragePool.Name == pool.Name {
			return p.xml, nil
		}
	}
	return "", errors.New("storage pool not found")
}

func (m *mockStoragePoolService) StoragePoolListAllVolumes(pool libvirt.StoragePool, flags int32, maxVols uint32) ([]libvirt.StorageVol, uint32, error) {
	vols, exists := m.volumes[pool.Name]
	if !exists {
		return []libvirt.StorageVol{}, 0, nil
	}
	return vols, uint32(len(vols)), nil
}

func (m *mockStoragePoolService) StoragePoolDefineXML(xml string, flags uint32) (libvirt.StoragePool, error) {
	if m.createError != nil {
		return libvirt.StoragePool{}, m.createError
	}

	name := extractStoragePoolNameFromXML(xml)
	uuid := generateStoragePoolUUID(name)

	pool := &mockStoragePoolInfo{
		StoragePool: libvirt.StoragePool{
			Name: name,
			UUID: uuid,
		},
		name:       name,
		uuid:       uuid,
		active:     0, // Not active by default
		persistent: 1, // Defined pools are persistent
		autostart:  0, // Not autostart by default
		state:      uint8(libvirt.StoragePoolInactive),
		capacity:   1073741824000, // 1TB default
		allocation: 0,
		available:  1073741824000,
		xml:        xml,
	}

	m.pools[name] = pool
	m.volumes[name] = []libvirt.StorageVol{} // Initialize empty volume list
	return pool.StoragePool, nil
}

func (m *mockStoragePoolService) StoragePoolBuild(pool libvirt.StoragePool, flags uint32) error {
	if m.buildError != nil {
		return m.buildError
	}

	// Mark pool as built (no-op for mock)
	return nil
}

func (m *mockStoragePoolService) StoragePoolCreate(pool libvirt.StoragePool, flags uint32) error {
	if m.startError != nil {
		return m.startError
	}

	for _, p := range m.pools {
		if p.StoragePool.Name == pool.Name {
			p.active = 1
			p.state = uint8(libvirt.StoragePoolRunning)
			return nil
		}
	}
	return errors.New("storage pool not found")
}

func (m *mockStoragePoolService) StoragePoolDestroy(pool libvirt.StoragePool) error {
	if m.stopError != nil {
		return m.stopError
	}

	for _, p := range m.pools {
		if p.StoragePool.Name == pool.Name {
			p.active = 0
			p.state = uint8(libvirt.StoragePoolInactive)
			return nil
		}
	}
	return errors.New("storage pool not found")
}

func (m *mockStoragePoolService) StoragePoolUndefine(pool libvirt.StoragePool) error {
	if m.deleteError != nil {
		return m.deleteError
	}

	for name, p := range m.pools {
		if p.StoragePool.Name == pool.Name {
			delete(m.pools, name)
			delete(m.volumes, name)
			return nil
		}
	}
	return errors.New("storage pool not found")
}

func (m *mockStoragePoolService) StorageVolGetInfo(vol libvirt.StorageVol) (int8, uint64, uint64, error) {
	// Mock implementation for volume info
	return int8(libvirt.StorageVolFile), 10737418240, 1073741824, nil
}

func (m *mockStoragePoolService) StorageVolGetPath(vol libvirt.StorageVol) (string, error) {
	// Mock implementation for volume path
	return "/var/lib/libvirt/images/" + vol.Name + ".qcow2", nil
}

// Helper functions

func extractStoragePoolNameFromXML(xml string) string {
	// Simplified XML parsing for test
	if start := findStringBetween(xml, "<name>", "</name>"); start != "" {
		return start
	}
	return "test-pool"
}

func generateStoragePoolUUID(name string) [16]byte {
	// Generate a simple UUID for testing
	var uuid [16]byte
	copy(uuid[:], []byte(name+"0000000000000000"))
	return uuid
}

func generateMockStoragePoolXML(pool *mockStoragePoolInfo) string {
	return `<pool type='dir'>
  <name>` + pool.name + `</name>
  <uuid>` + string(pool.uuid[:]) + `</uuid>
  <capacity unit='bytes'>` + fmt.Sprintf("%d", pool.capacity) + `</capacity>
  <allocation unit='bytes'>` + fmt.Sprintf("%d", pool.allocation) + `</allocation>
  <available unit='bytes'>` + fmt.Sprintf("%d", pool.available) + `</available>
  <target>
    <path>/var/lib/libvirt/images</path>
  </target>
</pool>`
}

// mockStoragePoolExternal wraps external with a mock service for testing
type mockStoragePoolExternal struct {
	mockService *mockStoragePoolService
	kube        client.Client
}

func (e *mockStoragePoolExternal) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.StoragePool)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotStoragePool)
	}

	poolName := cr.Spec.ForProvider.Name

	pool, err := e.mockService.StoragePoolLookupByName(poolName)
	if err != nil {
		if isStoragePoolNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, errDescribeStoragePool)
	}

	active, err := e.mockService.StoragePoolIsActive(pool)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errDescribeStoragePool)
	}

	persistent, err := e.mockService.StoragePoolIsPersistent(pool)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errDescribeStoragePool)
	}

	autoStart, err := e.mockService.StoragePoolGetAutostart(pool)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errDescribeStoragePool)
	}

	state, capacity, allocation, available, err := e.mockService.StoragePoolGetInfo(pool)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errDescribeStoragePool)
	}

	var volumes []v1alpha1.StoragePoolVolume
	if active == 1 {
		libvirtVolumes, _, err := e.mockService.StoragePoolListAllVolumes(pool, 0, 0)
		if err == nil {
			volumes = convertStoragePoolVolumes(libvirtVolumes, &clients.LibvirtClient{})
		}
	}

	// Update status
	cr.Status.AtProvider.UUID = fmt.Sprintf("%x", pool.UUID)
	cr.Status.AtProvider.Active = active == 1
	cr.Status.AtProvider.Persistent = persistent == 1
	cr.Status.AtProvider.AutoStart = autoStart == 1
	cr.Status.AtProvider.State = storagePoolStateToString(libvirt.StoragePoolState(state))
	cr.Status.AtProvider.Capacity = int64(capacity)
	cr.Status.AtProvider.Allocation = int64(allocation)
	cr.Status.AtProvider.Available = int64(available)
	cr.Status.AtProvider.VolumeCount = int32(len(volumes))
	cr.Status.AtProvider.Volumes = volumes

	cr.SetConditions(xpv1.Available())

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: isStoragePoolUpToDate(cr, int(active), int(autoStart)),
	}, nil
}

func (e *mockStoragePoolExternal) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.StoragePool)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotStoragePool)
	}

	xml, err := generateStoragePoolXML(cr)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateStoragePool)
	}

	pool, err := e.mockService.StoragePoolDefineXML(xml, 0)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateStoragePool)
	}

	// Set autostart if requested
	autoStart := cr.Spec.ForProvider.AutoStart
	if autoStart == nil || *autoStart {
		err = e.mockService.StoragePoolSetAutostart(pool, 1)
		if err != nil {
			return managed.ExternalCreation{}, errors.Wrap(err, errCreateStoragePool)
		}
	}

	// Build pool if needed
	if needsBuilding(cr.Spec.ForProvider.Type) {
		err = e.mockService.StoragePoolBuild(pool, 0)
		if err != nil {
			return managed.ExternalCreation{}, errors.Wrap(err, errCreateStoragePool)
		}
	}

	// Start pool if autostart is enabled
	if autoStart == nil || *autoStart {
		err = e.mockService.StoragePoolCreate(pool, 0)
		if err != nil {
			return managed.ExternalCreation{}, errors.Wrap(err, errStartStoragePool)
		}
	}

	return managed.ExternalCreation{}, nil
}

func (e *mockStoragePoolExternal) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.StoragePool)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotStoragePool)
	}

	poolName := cr.Spec.ForProvider.Name
	pool, err := e.mockService.StoragePoolLookupByName(poolName)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateStoragePool)
	}

	// Check if autostart setting needs to be updated
	currentAutoStart, err := e.mockService.StoragePoolGetAutostart(pool)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateStoragePool)
	}

	desiredAutoStart := cr.Spec.ForProvider.AutoStart
	if desiredAutoStart != nil {
		if (*desiredAutoStart && currentAutoStart == 0) || (!*desiredAutoStart && currentAutoStart == 1) {
			newAutoStart := int32(0)
			if *desiredAutoStart {
				newAutoStart = 1
			}
			err = e.mockService.StoragePoolSetAutostart(pool, newAutoStart)
			if err != nil {
				return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateStoragePool)
			}
		}
	}

	return managed.ExternalUpdate{}, nil
}

func (e *mockStoragePoolExternal) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*v1alpha1.StoragePool)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotStoragePool)
	}

	poolName := cr.Spec.ForProvider.Name
	pool, err := e.mockService.StoragePoolLookupByName(poolName)
	if err != nil {
		if isStoragePoolNotFound(err) {
			return managed.ExternalDelete{}, nil // Already deleted
		}
		return managed.ExternalDelete{}, errors.Wrap(err, errDeleteStoragePool)
	}

	// Stop pool if it's running
	active, err := e.mockService.StoragePoolIsActive(pool)
	if err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, errDeleteStoragePool)
	}

	if active == 1 {
		err = e.mockService.StoragePoolDestroy(pool)
		if err != nil {
			return managed.ExternalDelete{}, errors.Wrap(err, errStopStoragePool)
		}
	}

	// Undefine pool
	err = e.mockService.StoragePoolUndefine(pool)
	if err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, errDeleteStoragePool)
	}

	return managed.ExternalDelete{}, nil
}

func (e *mockStoragePoolExternal) Disconnect(ctx context.Context) error {
	return nil
}

func createMockStoragePoolExternal(mockService *mockStoragePoolService) *mockStoragePoolExternal {
	return &mockStoragePoolExternal{
		mockService: mockService,
		kube:        fake.NewClientBuilder().Build(),
	}
}

// Test cases

func TestStoragePoolExternal_Observe(t *testing.T) {
	tests := []struct {
		name      string
		mockSetup func(*mockStoragePoolService)
		pool      *v1alpha1.StoragePool
		want      managed.ExternalObservation
		wantErr   bool
		errMsg    string
	}{
		{
			name: "StoragePoolNotFound",
			mockSetup: func(m *mockStoragePoolService) {
				// Don't add any pools to the mock
			},
			pool: &v1alpha1.StoragePool{
				Spec: v1alpha1.StoragePoolSpec{
					ForProvider: v1alpha1.StoragePoolParameters{
						Name: "nonexistent-pool",
						Type: "dir",
						Target: &v1alpha1.StoragePoolTarget{
							Path: "/var/lib/libvirt/images",
						},
					},
				},
			},
			want: managed.ExternalObservation{
				ResourceExists: false,
			},
			wantErr: false,
		},
		{
			name: "StoragePoolExists",
			mockSetup: func(m *mockStoragePoolService) {
				pool := &mockStoragePoolInfo{
					StoragePool: libvirt.StoragePool{
						Name: "test-pool",
						UUID: generateStoragePoolUUID("test-pool"),
					},
					name:       "test-pool",
					uuid:       generateStoragePoolUUID("test-pool"),
					active:     1,
					persistent: 1,
					autostart:  1,
					state:      uint8(libvirt.StoragePoolRunning),
					capacity:   1073741824000, // 1TB
					allocation: 107374182400,  // 100GB
					available:  966367641600,  // 900GB
					xml:        generateMockStoragePoolXML(&mockStoragePoolInfo{name: "test-pool"}),
				}
				m.pools["test-pool"] = pool
				m.volumes["test-pool"] = []libvirt.StorageVol{}
			},
			pool: &v1alpha1.StoragePool{
				Spec: v1alpha1.StoragePoolSpec{
					ForProvider: v1alpha1.StoragePoolParameters{
						Name:      "test-pool",
						Type:      "dir",
						AutoStart: boolPtr(true),
						Target: &v1alpha1.StoragePoolTarget{
							Path: "/var/lib/libvirt/images",
						},
					},
				},
			},
			want: managed.ExternalObservation{
				ResourceExists:   true,
				ResourceUpToDate: true,
			},
			wantErr: false,
		},
		{
			name: "StoragePoolExistsButNotUpToDate",
			mockSetup: func(m *mockStoragePoolService) {
				pool := &mockStoragePoolInfo{
					StoragePool: libvirt.StoragePool{
						Name: "test-pool",
						UUID: generateStoragePoolUUID("test-pool"),
					},
					name:       "test-pool",
					uuid:       generateStoragePoolUUID("test-pool"),
					active:     0, // Not active
					persistent: 1,
					autostart:  0, // Not autostart
					state:      uint8(libvirt.StoragePoolInactive),
					capacity:   1073741824000,
					allocation: 0,
					available:  1073741824000,
					xml:        generateMockStoragePoolXML(&mockStoragePoolInfo{name: "test-pool"}),
				}
				m.pools["test-pool"] = pool
				m.volumes["test-pool"] = []libvirt.StorageVol{}
			},
			pool: &v1alpha1.StoragePool{
				Spec: v1alpha1.StoragePoolSpec{
					ForProvider: v1alpha1.StoragePoolParameters{
						Name:      "test-pool",
						Type:      "dir",
						AutoStart: boolPtr(true), // Wants autostart but pool doesn't have it
						Target: &v1alpha1.StoragePoolTarget{
							Path: "/var/lib/libvirt/images",
						},
					},
				},
			},
			want: managed.ExternalObservation{
				ResourceExists:   true,
				ResourceUpToDate: false,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := newMockStoragePoolService()
			if tt.mockSetup != nil {
				tt.mockSetup(mockService)
			}

			e := createMockStoragePoolExternal(mockService)

			got, err := e.Observe(context.Background(), tt.pool)

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

func TestStoragePoolExternal_Create(t *testing.T) {
	tests := []struct {
		name      string
		mockSetup func(*mockStoragePoolService)
		pool      *v1alpha1.StoragePool
		wantErr   bool
		errMsg    string
		validate  func(*testing.T, *mockStoragePoolService, *v1alpha1.StoragePool)
	}{
		{
			name: "CreateStoragePoolSuccess",
			mockSetup: func(m *mockStoragePoolService) {
				// No setup needed - mock will create the pool
			},
			pool: &v1alpha1.StoragePool{
				Spec: v1alpha1.StoragePoolSpec{
					ForProvider: v1alpha1.StoragePoolParameters{
						Name:      "test-pool",
						Type:      "dir",
						AutoStart: boolPtr(true),
						Target: &v1alpha1.StoragePoolTarget{
							Path: "/var/lib/libvirt/images",
							Permissions: &v1alpha1.StoragePoolPermissions{
								Owner: int32Ptr(107),
								Group: int32Ptr(107),
								Mode:  int32Ptr(0755),
							},
						},
					},
				},
			},
			wantErr: false,
			validate: func(t *testing.T, m *mockStoragePoolService, pool *v1alpha1.StoragePool) {
				poolName := pool.Spec.ForProvider.Name

				mockPool, exists := m.pools[poolName]
				if !exists {
					t.Error("Expected storage pool to be created")
					return
				}

				if mockPool.persistent != 1 {
					t.Error("Expected storage pool to be persistent")
				}

				if mockPool.autostart != 1 {
					t.Error("Expected storage pool to have autostart enabled")
				}

				if mockPool.active != 1 {
					t.Error("Expected storage pool to be active")
				}
			},
		},
		{
			name: "CreateStoragePoolWithoutAutoStart",
			mockSetup: func(m *mockStoragePoolService) {
				// No setup needed
			},
			pool: &v1alpha1.StoragePool{
				Spec: v1alpha1.StoragePoolSpec{
					ForProvider: v1alpha1.StoragePoolParameters{
						Name:      "test-pool",
						Type:      "dir",
						AutoStart: boolPtr(false),
						Target: &v1alpha1.StoragePoolTarget{
							Path: "/var/lib/libvirt/images",
						},
					},
				},
			},
			wantErr: false,
			validate: func(t *testing.T, m *mockStoragePoolService, pool *v1alpha1.StoragePool) {
				poolName := pool.Spec.ForProvider.Name

				mockPool, exists := m.pools[poolName]
				if !exists {
					t.Error("Expected storage pool to be created")
					return
				}

				if mockPool.autostart != 0 {
					t.Error("Expected storage pool to have autostart disabled")
				}

				if mockPool.active != 0 {
					t.Error("Expected storage pool to be inactive")
				}
			},
		},
		{
			name: "CreateStoragePoolWithBuilding",
			mockSetup: func(m *mockStoragePoolService) {
				// No setup needed
			},
			pool: &v1alpha1.StoragePool{
				Spec: v1alpha1.StoragePoolSpec{
					ForProvider: v1alpha1.StoragePoolParameters{
						Name:      "test-fs-pool",
						Type:      "fs", // This type needs building
						AutoStart: boolPtr(true),
						Target: &v1alpha1.StoragePoolTarget{
							Path: "/mnt/storage",
						},
						Source: &v1alpha1.StoragePoolSource{
							Device: &v1alpha1.StoragePoolDevice{
								Path: "/dev/sdb1",
							},
							Format: &v1alpha1.StoragePoolFormat{
								Type: "ext4",
							},
						},
					},
				},
			},
			wantErr: false,
			validate: func(t *testing.T, m *mockStoragePoolService, pool *v1alpha1.StoragePool) {
				poolName := pool.Spec.ForProvider.Name

				mockPool, exists := m.pools[poolName]
				if !exists {
					t.Error("Expected storage pool to be created")
					return
				}

				if mockPool.active != 1 {
					t.Error("Expected storage pool to be active after building")
				}
			},
		},
		{
			name: "CreateError",
			mockSetup: func(m *mockStoragePoolService) {
				m.createError = errors.New("failed to create storage pool")
			},
			pool: &v1alpha1.StoragePool{
				Spec: v1alpha1.StoragePoolSpec{
					ForProvider: v1alpha1.StoragePoolParameters{
						Name: "test-pool",
						Type: "dir",
						Target: &v1alpha1.StoragePoolTarget{
							Path: "/var/lib/libvirt/images",
						},
					},
				},
			},
			wantErr: true,
			errMsg:  errCreateStoragePool,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := newMockStoragePoolService()
			if tt.mockSetup != nil {
				tt.mockSetup(mockService)
			}

			e := createMockStoragePoolExternal(mockService)

			_, err := e.Create(context.Background(), tt.pool)

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
				tt.validate(t, mockService, tt.pool)
			}
		})
	}
}

func TestStoragePoolExternal_Delete(t *testing.T) {
	tests := []struct {
		name      string
		mockSetup func(*mockStoragePoolService)
		pool      *v1alpha1.StoragePool
		wantErr   bool
		errMsg    string
		validate  func(*testing.T, *mockStoragePoolService, *v1alpha1.StoragePool)
	}{
		{
			name: "DeleteStoragePoolSuccess",
			mockSetup: func(m *mockStoragePoolService) {
				pool := &mockStoragePoolInfo{
					StoragePool: libvirt.StoragePool{
						Name: "test-pool",
						UUID: generateStoragePoolUUID("test-pool"),
					},
					name:       "test-pool",
					uuid:       generateStoragePoolUUID("test-pool"),
					active:     1, // Pool is active
					persistent: 1,
					autostart:  1,
					state:      uint8(libvirt.StoragePoolRunning),
					capacity:   1073741824000,
					allocation: 0,
					available:  1073741824000,
					xml:        generateMockStoragePoolXML(&mockStoragePoolInfo{name: "test-pool"}),
				}
				m.pools["test-pool"] = pool
				m.volumes["test-pool"] = []libvirt.StorageVol{}
			},
			pool: &v1alpha1.StoragePool{
				Spec: v1alpha1.StoragePoolSpec{
					ForProvider: v1alpha1.StoragePoolParameters{
						Name: "test-pool",
						Type: "dir",
						Target: &v1alpha1.StoragePoolTarget{
							Path: "/var/lib/libvirt/images",
						},
					},
				},
			},
			wantErr: false,
			validate: func(t *testing.T, m *mockStoragePoolService, pool *v1alpha1.StoragePool) {
				poolName := pool.Spec.ForProvider.Name

				_, exists := m.pools[poolName]
				if exists {
					t.Error("Expected storage pool to be deleted")
				}

				_, volExists := m.volumes[poolName]
				if volExists {
					t.Error("Expected volume list to be cleaned up")
				}
			},
		},
		{
			name: "DeleteNonexistentStoragePool",
			mockSetup: func(m *mockStoragePoolService) {
				// Don't add any pools
			},
			pool: &v1alpha1.StoragePool{
				Spec: v1alpha1.StoragePoolSpec{
					ForProvider: v1alpha1.StoragePoolParameters{
						Name: "nonexistent-pool",
						Type: "dir",
						Target: &v1alpha1.StoragePoolTarget{
							Path: "/var/lib/libvirt/images",
						},
					},
				},
			},
			wantErr: false, // Should succeed (already deleted)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := newMockStoragePoolService()
			if tt.mockSetup != nil {
				tt.mockSetup(mockService)
			}

			e := createMockStoragePoolExternal(mockService)

			_, err := e.Delete(context.Background(), tt.pool)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Delete() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if tt.errMsg != "" && !containsSubstring(err.Error(), tt.errMsg) {
					t.Errorf("Delete() error = %v, want error containing %v", err, tt.errMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("Delete() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.validate != nil {
				tt.validate(t, mockService, tt.pool)
			}
		})
	}
}

func TestGenerateStoragePoolXML(t *testing.T) {
	tests := []struct {
		name     string
		pool     *v1alpha1.StoragePool
		wantErr  bool
		validate func(*testing.T, string)
	}{
		{
			name: "DirectoryPool",
			pool: &v1alpha1.StoragePool{
				Spec: v1alpha1.StoragePoolSpec{
					ForProvider: v1alpha1.StoragePoolParameters{
						Name: "test-dir-pool",
						Type: "dir",
						Target: &v1alpha1.StoragePoolTarget{
							Path: "/var/lib/libvirt/images",
							Permissions: &v1alpha1.StoragePoolPermissions{
								Owner: int32Ptr(107),
								Group: int32Ptr(107),
								Mode:  int32Ptr(0755),
							},
						},
					},
				},
			},
			wantErr: false,
			validate: func(t *testing.T, xml string) {
				if !containsSubstring(xml, "<pool type='dir'>") {
					t.Error("XML should contain pool type")
				}
				if !containsSubstring(xml, "<name>test-dir-pool</name>") {
					t.Error("XML should contain pool name")
				}
				if !containsSubstring(xml, "<path>/var/lib/libvirt/images</path>") {
					t.Error("XML should contain target path")
				}
				if !containsSubstring(xml, "<owner>107</owner>") {
					t.Error("XML should contain owner permissions")
				}
				if !containsSubstring(xml, "<mode>0755</mode>") {
					t.Error("XML should contain mode permissions")
				}
			},
		},
		{
			name: "FilesystemPool",
			pool: &v1alpha1.StoragePool{
				Spec: v1alpha1.StoragePoolSpec{
					ForProvider: v1alpha1.StoragePoolParameters{
						Name: "fs-pool",
						Type: "fs",
						Target: &v1alpha1.StoragePoolTarget{
							Path: "/mnt/storage",
						},
						Source: &v1alpha1.StoragePoolSource{
							Device: &v1alpha1.StoragePoolDevice{
								Path: "/dev/sdb1",
							},
							Format: &v1alpha1.StoragePoolFormat{
								Type: "ext4",
							},
						},
					},
				},
			},
			wantErr: false,
			validate: func(t *testing.T, xml string) {
				if !containsSubstring(xml, "<pool type='fs'>") {
					t.Error("XML should contain pool type")
				}
				if !containsSubstring(xml, "<name>fs-pool</name>") {
					t.Error("XML should contain pool name")
				}
				if !containsSubstring(xml, "<path>/mnt/storage</path>") {
					t.Error("XML should contain target path")
				}
				if !containsSubstring(xml, "<device path='/dev/sdb1'/>") {
					t.Error("XML should contain source device")
				}
				if !containsSubstring(xml, "<format type='ext4'/>") {
					t.Error("XML should contain format type")
				}
			},
		},
		{
			name: "NFSPool",
			pool: &v1alpha1.StoragePool{
				Spec: v1alpha1.StoragePoolSpec{
					ForProvider: v1alpha1.StoragePoolParameters{
						Name: "nfs-pool",
						Type: "netfs",
						Target: &v1alpha1.StoragePoolTarget{
							Path: "/mnt/nfs",
						},
						Source: &v1alpha1.StoragePoolSource{
							Host: &v1alpha1.StoragePoolHost{
								Name: "nfs.example.com",
								Port: int32Ptr(2049),
							},
							Dir: "/exports/libvirt",
							Format: &v1alpha1.StoragePoolFormat{
								Type: "nfs",
							},
						},
					},
				},
			},
			wantErr: false,
			validate: func(t *testing.T, xml string) {
				if !containsSubstring(xml, "<pool type='netfs'>") {
					t.Error("XML should contain pool type")
				}
				if !containsSubstring(xml, "<host name='nfs.example.com' port='2049'/>") {
					t.Error("XML should contain host configuration")
				}
				if !containsSubstring(xml, "<dir path='/exports/libvirt'/>") {
					t.Error("XML should contain directory path")
				}
				if !containsSubstring(xml, "<format type='nfs'/>") {
					t.Error("XML should contain format type")
				}
			},
		},
		{
			name: "iSCSIPoolWithAuth",
			pool: &v1alpha1.StoragePool{
				Spec: v1alpha1.StoragePoolSpec{
					ForProvider: v1alpha1.StoragePoolParameters{
						Name: "iscsi-pool",
						Type: "iscsi",
						Target: &v1alpha1.StoragePoolTarget{
							Path: "/dev/disk/by-path",
						},
						Source: &v1alpha1.StoragePoolSource{
							Host: &v1alpha1.StoragePoolHost{
								Name: "iscsi.example.com",
								Port: int32Ptr(3260),
							},
							Device: &v1alpha1.StoragePoolDevice{
								Path: "iqn.2021-01.com.example:target1",
							},
							Auth: &v1alpha1.StoragePoolAuth{
								Type:     "chap",
								Username: "libvirt-user",
								Secret: &v1alpha1.StoragePoolAuthSecret{
									Type:  "iscsi",
									Usage: "libvirt-iscsi-secret",
								},
							},
						},
					},
				},
			},
			wantErr: false,
			validate: func(t *testing.T, xml string) {
				if !containsSubstring(xml, "<pool type='iscsi'>") {
					t.Error("XML should contain pool type")
				}
				if !containsSubstring(xml, "<host name='iscsi.example.com' port='3260'/>") {
					t.Error("XML should contain host configuration")
				}
				if !containsSubstring(xml, "<device path='iqn.2021-01.com.example:target1'/>") {
					t.Error("XML should contain device path")
				}
				if !containsSubstring(xml, "<auth type='chap' username='libvirt-user'>") {
					t.Error("XML should contain authentication")
				}
				if !containsSubstring(xml, "<secret type='iscsi' usage='libvirt-iscsi-secret'/>") {
					t.Error("XML should contain secret reference")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			xml, err := generateStoragePoolXML(tt.pool)

			if tt.wantErr {
				if err == nil {
					t.Errorf("generateStoragePoolXML() error = nil, wantErr %v", tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("generateStoragePoolXML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.validate != nil {
				tt.validate(t, xml)
			}
		})
	}
}

// Helper functions

func boolPtr(b bool) *bool {
	return &b
}

func int32Ptr(i int32) *int32 {
	return &i
}

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

func findStringBetween(str, start, end string) string {
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