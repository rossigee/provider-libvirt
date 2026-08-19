package mocks

import (
	"errors"
	"sync"

	"libvirt.org/go/libvirt"
)

// MockLibvirtClient mocks the LibvirtClient for testing
type MockLibvirtClient struct {
	mu sync.RWMutex

	// Domain operations
	DomainLookupByNameFn func(name string) (*MockDomain, error)
	DomainDefineXMLFn    func(xml string) (*MockDomain, error)
	DomainCreateFn       func(domain *MockDomain) error
	DomainDestroyFn      func(domain *MockDomain) error
	DomainShutdownFn     func(domain *MockDomain) error
	DomainUndefineFn     func(domain *MockDomain) error
	DomainSetAutostartFn func(domain *MockDomain, autostart int) error

	// Network operations
	NetworkLookupByNameFn func(name string) (*MockNetwork, error)
	NetworkDefineXMLFn    func(xml string) (*MockNetwork, error)
	NetworkCreateFn       func(network *MockNetwork) error
	NetworkDestroyFn      func(network *MockNetwork) error
	NetworkUndefineFn     func(network *MockNetwork) error
	NetworkSetAutostartFn func(network *MockNetwork, autostart bool) error
	NetworkIsActiveFn     func(network *MockNetwork) (bool, error)
	NetworkIsPersistentFn func(network *MockNetwork) (bool, error)
	NetworkGetAutostartFn func(network *MockNetwork) (bool, error)
	NetworkGetXMLDescFn   func(network *MockNetwork, flags uint32) (string, error)

	// Storage pool operations
	StoragePoolLookupByNameFn   func(name string) (*MockStoragePool, error)
	StoragePoolDefineXMLFn      func(xml string, flags uint32) (*MockStoragePool, error)
	StoragePoolBuildFn          func(pool *MockStoragePool, flags uint32) error
	StoragePoolCreateFn         func(pool *MockStoragePool, flags uint32) error
	StoragePoolDestroyFn        func(pool *MockStoragePool) error
	StoragePoolUndefineFn       func(pool *MockStoragePool) error
	StoragePoolSetAutostartFn   func(pool *MockStoragePool, autostart bool) error
	StoragePoolIsActiveFn       func(pool *MockStoragePool) (bool, error)
	StoragePoolIsPersistentFn   func(pool *MockStoragePool) (bool, error)
	StoragePoolGetAutostartFn   func(pool *MockStoragePool) (bool, error)
	StoragePoolGetInfoFn        func(pool *MockStoragePool) (*libvirt.StoragePoolInfo, error)
	StoragePoolListAllVolumesFn func(pool *MockStoragePool, flags uint32) ([]libvirt.StorageVol, error)

	// Volume operations
	StorageVolLookupByNameFn func(pool *MockStoragePool, name string) (*MockStorageVol, error)
	StorageVolCreateXMLFn    func(pool *MockStoragePool, xml string, flags uint32) (*MockStorageVol, error)
	StorageVolDeleteFn       func(vol *MockStorageVol, flags uint32) error
	StorageVolGetInfoFn      func(vol *MockStorageVol) (*libvirt.StorageVolInfo, error)
	StorageVolGetXMLDescFn   func(vol *MockStorageVol, flags uint32) (string, error)
	StorageVolResizeFn       func(vol *MockStorageVol, capacity uint64, flags libvirt.StorageVolResizeFlags) error

	// Internal state
	domains      map[string]*MockDomain
	networks     map[string]*MockNetwork
	storagePools map[string]*MockStoragePool
	volumes      map[string]*MockStorageVol
}

// NewMockLibvirtClient creates a new mock client
func NewMockLibvirtClient() *MockLibvirtClient {
	return &MockLibvirtClient{
		domains:      make(map[string]*MockDomain),
		networks:     make(map[string]*MockNetwork),
		storagePools: make(map[string]*MockStoragePool),
		volumes:      make(map[string]*MockStorageVol),
	}
}

// MockDomain mocks libvirt.Domain
type MockDomain struct {
	Name      string
	UUID      string
	ID        int
	State     libvirt.DomainState
	XML       string
	Autostart int
	Memory    uint64
	MaxMemory uint64
	VCPUCount uint
	CPUTime   uint64
}

func (d *MockDomain) GetState() (libvirt.DomainState, int, error) {
	return d.State, 0, nil
}

func (d *MockDomain) GetInfo() (*libvirt.DomainInfo, error) {
	return &libvirt.DomainInfo{
		State:     d.State,
		MaxMem:    d.MaxMemory,
		Memory:    d.Memory,
		NrVirtCpu: d.VCPUCount,
		CpuTime:   d.CPUTime,
	}, nil
}

func (d *MockDomain) GetName() (string, error) {
	return d.Name, nil
}

func (d *MockDomain) GetUUIDString() (string, error) {
	return d.UUID, nil
}

func (d *MockDomain) Create() error {
	d.State = libvirt.DOMAIN_RUNNING
	return nil
}

func (d *MockDomain) Shutdown() error {
	d.State = libvirt.DOMAIN_SHUTOFF
	return nil
}

func (d *MockDomain) Destroy() error {
	d.State = libvirt.DOMAIN_SHUTOFF
	return nil
}

func (d *MockDomain) Undefine() error {
	return nil
}

func (d *MockDomain) SetAutostart(autostart bool) error {
	if autostart {
		d.Autostart = 1
	} else {
		d.Autostart = 0
	}
	return nil
}

// MockNetwork mocks libvirt.Network
type MockNetwork struct {
	Name       string
	UUID       string
	XML        string
	Active     bool
	Persistent bool
	Autostart  bool
}

func (n *MockNetwork) IsActive() (bool, error) {
	return n.Active, nil
}

func (n *MockNetwork) IsPersistent() (bool, error) {
	return n.Persistent, nil
}

func (n *MockNetwork) GetAutostart() (bool, error) {
	return n.Autostart, nil
}

func (n *MockNetwork) GetXMLDesc(flags libvirt.NetworkXMLFlags) (string, error) {
	return n.XML, nil
}

func (n *MockNetwork) Create() error {
	n.Active = true
	return nil
}

func (n *MockNetwork) Destroy() error {
	n.Active = false
	return nil
}

func (n *MockNetwork) Undefine() error {
	return nil
}

func (n *MockNetwork) SetAutostart(autostart bool) error {
	n.Autostart = autostart
	return nil
}

// MockStoragePool mocks libvirt.StoragePool
type MockStoragePool struct {
	Name       string
	UUID       string
	Type       string
	Path       string
	XML        string
	Active     bool
	Persistent bool
	Autostart  bool
	Capacity   uint64
	Allocation uint64
	Available  uint64
	Volumes    map[string]*MockStorageVol
	mu         sync.RWMutex
}

func (p *MockStoragePool) IsActive() (bool, error) {
	return p.Active, nil
}

func (p *MockStoragePool) IsPersistent() (bool, error) {
	return p.Persistent, nil
}

func (p *MockStoragePool) GetAutostart() (bool, error) {
	return p.Autostart, nil
}

func (p *MockStoragePool) GetInfo() (*libvirt.StoragePoolInfo, error) {
	return &libvirt.StoragePoolInfo{
		Capacity:   p.Capacity,
		Allocation: p.Allocation,
		Available:  p.Available,
	}, nil
}

func (p *MockStoragePool) Create(flags libvirt.StoragePoolCreateFlags) error {
	p.Active = true
	return nil
}

func (p *MockStoragePool) Build(flags libvirt.StoragePoolBuildFlags) error {
	return nil
}

func (p *MockStoragePool) Destroy() error {
	p.Active = false
	return nil
}

func (p *MockStoragePool) Undefine() error {
	return nil
}

func (p *MockStoragePool) SetAutostart(autostart bool) error {
	p.Autostart = autostart
	return nil
}

func (p *MockStoragePool) LookupStorageVolByName(name string) (*libvirt.StorageVol, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if _, ok := p.Volumes[name]; ok {
		return (*libvirt.StorageVol)(nil), nil
	}
	return nil, errors.New("volume not found")
}

func (p *MockStoragePool) ListAllStorageVolumes(flags uint32) ([]libvirt.StorageVol, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return []libvirt.StorageVol{}, nil
}

func (p *MockStoragePool) StorageVolCreateXML(xml string, flags libvirt.StorageVolCreateFlags) (*libvirt.StorageVol, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Create volume from XML
	mockVol := &MockStorageVol{
		XML:      xml,
		Path:     p.Path + "/volume.img",
		Capacity: 1073741824, // 1GB default
	}

	// Store volume
	if mockVol.Path != "" {
		p.Volumes[mockVol.Path] = mockVol
	}

	return (*libvirt.StorageVol)(nil), nil
}

// MockStorageVol mocks libvirt.StorageVol
type MockStorageVol struct {
	Name       string
	Path       string
	Format     string
	Capacity   uint64
	Allocation uint64
	XML        string
}

func (v *MockStorageVol) GetPath() (string, error) {
	return v.Path, nil
}

func (v *MockStorageVol) GetInfo() (*libvirt.StorageVolInfo, error) {
	return &libvirt.StorageVolInfo{
		Capacity:   v.Capacity,
		Allocation: v.Allocation,
	}, nil
}

func (v *MockStorageVol) GetXMLDesc(flags uint32) (string, error) {
	return v.XML, nil
}

func (v *MockStorageVol) Delete(flags libvirt.StorageVolDeleteFlags) error {
	return nil
}

func (v *MockStorageVol) Resize(capacity uint64, flags libvirt.StorageVolResizeFlags) error {
	v.Capacity = capacity
	return nil
}

// Implementation methods for MockLibvirtClient

func (c *MockLibvirtClient) DomainLookupByName(name string) (*libvirt.Domain, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.DomainLookupByNameFn != nil {
		_, err := c.DomainLookupByNameFn(name)
		if err != nil {
			return nil, err
		}
		return (*libvirt.Domain)(nil), nil
	}

	if _, ok := c.domains[name]; ok {
		return (*libvirt.Domain)(nil), nil
	}
	return nil, errors.New("domain not found")
}

func (c *MockLibvirtClient) DomainDefineXML(xml string) (*libvirt.Domain, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.DomainDefineXMLFn != nil {
		_, err := c.DomainDefineXMLFn(xml)
		if err != nil {
			return (*libvirt.Domain)(nil), err
		}
	}

	mockDomain := &MockDomain{XML: xml, State: libvirt.DOMAIN_SHUTOFF}
	c.domains[mockDomain.Name] = mockDomain
	return (*libvirt.Domain)(nil), nil
}

func (c *MockLibvirtClient) DomainCreate(domain *libvirt.Domain) error {
	if c.DomainCreateFn != nil {
		return c.DomainCreateFn((*MockDomain)(nil))
	}
	return nil
}

func (c *MockLibvirtClient) DomainDestroy(domain *libvirt.Domain) error {
	if c.DomainDestroyFn != nil {
		return c.DomainDestroyFn((*MockDomain)(nil))
	}
	return nil
}

func (c *MockLibvirtClient) DomainShutdown(domain *libvirt.Domain) error {
	if c.DomainShutdownFn != nil {
		return c.DomainShutdownFn((*MockDomain)(nil))
	}
	return nil
}

func (c *MockLibvirtClient) DomainUndefine(domain *libvirt.Domain) error {
	if c.DomainUndefineFn != nil {
		return c.DomainUndefineFn((*MockDomain)(nil))
	}
	return nil
}

func (c *MockLibvirtClient) DomainSetAutostart(domain *libvirt.Domain, autostart int) error {
	if c.DomainSetAutostartFn != nil {
		return c.DomainSetAutostartFn((*MockDomain)(nil), autostart)
	}
	return nil
}

func (c *MockLibvirtClient) NetworkLookupByName(name string) (*libvirt.Network, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.NetworkLookupByNameFn != nil {
		_, err := c.NetworkLookupByNameFn(name)
		return (*libvirt.Network)(nil), err
	}

	if _, ok := c.networks[name]; ok {
		return (*libvirt.Network)(nil), nil
	}
	return nil, errors.New("network not found")
}

func (c *MockLibvirtClient) NetworkDefineXML(xml string) (*libvirt.Network, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.NetworkDefineXMLFn != nil {
		_, err := c.NetworkDefineXMLFn(xml)
		return (*libvirt.Network)(nil), err
	}

	_ = &MockNetwork{XML: xml}
	return (*libvirt.Network)(nil), nil
}

func (c *MockLibvirtClient) NetworkCreate(network *libvirt.Network) error {
	if c.NetworkCreateFn != nil {
		return c.NetworkCreateFn((*MockNetwork)(nil))
	}
	return nil
}

func (c *MockLibvirtClient) NetworkDestroy(network *libvirt.Network) error {
	if c.NetworkDestroyFn != nil {
		return c.NetworkDestroyFn((*MockNetwork)(nil))
	}
	return nil
}

func (c *MockLibvirtClient) NetworkUndefine(network *libvirt.Network) error {
	if c.NetworkUndefineFn != nil {
		return c.NetworkUndefineFn((*MockNetwork)(nil))
	}
	return nil
}

func (c *MockLibvirtClient) NetworkSetAutostart(network *libvirt.Network, autostart bool) error {
	if c.NetworkSetAutostartFn != nil {
		return c.NetworkSetAutostartFn((*MockNetwork)(nil), autostart)
	}
	return nil
}

func (c *MockLibvirtClient) NetworkIsActive(network *libvirt.Network) (bool, error) {
	if c.NetworkIsActiveFn != nil {
		return c.NetworkIsActiveFn((*MockNetwork)(nil))
	}
	return false, nil
}

func (c *MockLibvirtClient) NetworkIsPersistent(network *libvirt.Network) (bool, error) {
	if c.NetworkIsPersistentFn != nil {
		return c.NetworkIsPersistentFn((*MockNetwork)(nil))
	}
	return false, nil
}

func (c *MockLibvirtClient) NetworkGetAutostart(network *libvirt.Network) (bool, error) {
	if c.NetworkGetAutostartFn != nil {
		return c.NetworkGetAutostartFn((*MockNetwork)(nil))
	}
	return false, nil
}

func (c *MockLibvirtClient) NetworkGetXMLDesc(network *libvirt.Network, flags uint32) (string, error) {
	if c.NetworkGetXMLDescFn != nil {
		return c.NetworkGetXMLDescFn((*MockNetwork)(nil), flags)
	}
	return "", nil
}

func (c *MockLibvirtClient) StoragePoolLookupByName(name string) (*libvirt.StoragePool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.StoragePoolLookupByNameFn != nil {
		_, err := c.StoragePoolLookupByNameFn(name)
		return (*libvirt.StoragePool)(nil), err
	}

	if _, ok := c.storagePools[name]; ok {
		return (*libvirt.StoragePool)(nil), nil
	}
	return nil, errors.New("storage pool not found")
}

func (c *MockLibvirtClient) StoragePoolDefineXML(xml string, flags uint32) (*libvirt.StoragePool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.StoragePoolDefineXMLFn != nil {
		_, err := c.StoragePoolDefineXMLFn(xml, flags)
		return (*libvirt.StoragePool)(nil), err
	}

	_ = &MockStoragePool{XML: xml, Volumes: make(map[string]*MockStorageVol)}
	return (*libvirt.StoragePool)(nil), nil
}

func (c *MockLibvirtClient) StoragePoolBuild(pool *libvirt.StoragePool, flags uint32) error {
	if c.StoragePoolBuildFn != nil {
		return c.StoragePoolBuildFn((*MockStoragePool)(nil), flags)
	}
	return nil
}

func (c *MockLibvirtClient) StoragePoolCreate(pool *libvirt.StoragePool, flags uint32) error {
	if c.StoragePoolCreateFn != nil {
		return c.StoragePoolCreateFn((*MockStoragePool)(nil), flags)
	}
	return nil
}

func (c *MockLibvirtClient) StoragePoolDestroy(pool *libvirt.StoragePool) error {
	if c.StoragePoolDestroyFn != nil {
		return c.StoragePoolDestroyFn((*MockStoragePool)(nil))
	}
	return nil
}

func (c *MockLibvirtClient) StoragePoolUndefine(pool *libvirt.StoragePool) error {
	if c.StoragePoolUndefineFn != nil {
		return c.StoragePoolUndefineFn((*MockStoragePool)(nil))
	}
	return nil
}

func (c *MockLibvirtClient) StoragePoolSetAutostart(pool *libvirt.StoragePool, autostart bool) error {
	if c.StoragePoolSetAutostartFn != nil {
		return c.StoragePoolSetAutostartFn((*MockStoragePool)(nil), autostart)
	}
	return nil
}

func (c *MockLibvirtClient) StoragePoolIsActive(pool *libvirt.StoragePool) (bool, error) {
	if c.StoragePoolIsActiveFn != nil {
		return c.StoragePoolIsActiveFn((*MockStoragePool)(nil))
	}
	return false, nil
}

func (c *MockLibvirtClient) StoragePoolIsPersistent(pool *libvirt.StoragePool) (bool, error) {
	if c.StoragePoolIsPersistentFn != nil {
		return c.StoragePoolIsPersistentFn((*MockStoragePool)(nil))
	}
	return false, nil
}

func (c *MockLibvirtClient) StoragePoolGetAutostart(pool *libvirt.StoragePool) (bool, error) {
	if c.StoragePoolGetAutostartFn != nil {
		return c.StoragePoolGetAutostartFn((*MockStoragePool)(nil))
	}
	return false, nil
}

func (c *MockLibvirtClient) StoragePoolGetInfo(pool *libvirt.StoragePool) (*libvirt.StoragePoolInfo, error) {
	if c.StoragePoolGetInfoFn != nil {
		return c.StoragePoolGetInfoFn((*MockStoragePool)(nil))
	}
	return &libvirt.StoragePoolInfo{}, nil
}

func (c *MockLibvirtClient) StorageVolLookupByName(pool *libvirt.StoragePool, name string) (*libvirt.StorageVol, error) {
	if c.StorageVolLookupByNameFn != nil {
		_, err := c.StorageVolLookupByNameFn((*MockStoragePool)(nil), name)
		return (*libvirt.StorageVol)(nil), err
	}
	return nil, errors.New("volume not found")
}

func (c *MockLibvirtClient) StorageVolCreateXML(pool *libvirt.StoragePool, xml string, flags uint32) (*libvirt.StorageVol, error) {
	if c.StorageVolCreateXMLFn != nil {
		_, err := c.StorageVolCreateXMLFn((*MockStoragePool)(nil), xml, flags)
		return (*libvirt.StorageVol)(nil), err
	}
	return (*libvirt.StorageVol)(nil), nil
}

func (c *MockLibvirtClient) StorageVolDelete(vol *libvirt.StorageVol, flags uint32) error {
	if c.StorageVolDeleteFn != nil {
		return c.StorageVolDeleteFn((*MockStorageVol)(nil), flags)
	}
	return nil
}

func (c *MockLibvirtClient) StorageVolGetInfo(vol *libvirt.StorageVol) (*libvirt.StorageVolInfo, error) {
	if c.StorageVolGetInfoFn != nil {
		return c.StorageVolGetInfoFn((*MockStorageVol)(nil))
	}
	return &libvirt.StorageVolInfo{}, nil
}

func (c *MockLibvirtClient) StorageVolGetXMLDesc(vol *libvirt.StorageVol, flags uint32) (string, error) {
	if c.StorageVolGetXMLDescFn != nil {
		return c.StorageVolGetXMLDescFn((*MockStorageVol)(nil), flags)
	}
	return "", nil
}

func (c *MockLibvirtClient) StorageVolResize(vol *libvirt.StorageVol, capacity uint64, flags libvirt.StorageVolResizeFlags) error {
	if c.StorageVolResizeFn != nil {
		return c.StorageVolResizeFn((*MockStorageVol)(nil), capacity, flags)
	}
	return nil
}

func (c *MockLibvirtClient) Close() error {
	return nil
}

func (c *MockLibvirtClient) Disconnect() error {
	return nil
}
