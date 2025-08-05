/*
Copyright 2025 Ross Golder
*/

package network

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

// mockNetworkService implements network operations for testing
type mockNetworkService struct {
	networks       map[string]*mockNetwork
	createError    error
	deleteError    error
	lookupError    error
	startError     error
	stopError      error
	autostartError error
}

type mockNetwork struct {
	libvirt.Network
	name       string
	uuid       [16]byte
	active     int32
	persistent int32
	autostart  int32
	xml        string
}

func newMockNetworkService() *mockNetworkService {
	return &mockNetworkService{
		networks: make(map[string]*mockNetwork),
	}
}

func (m *mockNetworkService) NetworkLookupByName(name string) (libvirt.Network, error) {
	if m.lookupError != nil {
		return libvirt.Network{}, m.lookupError
	}

	network, exists := m.networks[name]
	if !exists {
		return libvirt.Network{}, errors.New("Network not found: no network with matching name")
	}

	return network.Network, nil
}

func (m *mockNetworkService) NetworkIsActive(net libvirt.Network) (int32, error) {
	for _, network := range m.networks {
		if network.Network.Name == net.Name {
			return network.active, nil
		}
	}
	return 0, errors.New("network not found")
}

func (m *mockNetworkService) NetworkIsPersistent(net libvirt.Network) (int32, error) {
	for _, network := range m.networks {
		if network.Network.Name == net.Name {
			return network.persistent, nil
		}
	}
	return 0, errors.New("network not found")
}

func (m *mockNetworkService) NetworkGetAutostart(net libvirt.Network) (int32, error) {
	if m.autostartError != nil {
		return 0, m.autostartError
	}

	for _, network := range m.networks {
		if network.Network.Name == net.Name {
			return network.autostart, nil
		}
	}
	return 0, errors.New("network not found")
}

func (m *mockNetworkService) NetworkSetAutostart(net libvirt.Network, autostart int32) error {
	if m.autostartError != nil {
		return m.autostartError
	}

	for _, network := range m.networks {
		if network.Network.Name == net.Name {
			network.autostart = autostart
			return nil
		}
	}
	return errors.New("network not found")
}

func (m *mockNetworkService) NetworkGetXMLDesc(net libvirt.Network, flags uint32) (string, error) {
	for _, network := range m.networks {
		if network.Network.Name == net.Name {
			return network.xml, nil
		}
	}
	return "", errors.New("network not found")
}

func (m *mockNetworkService) NetworkDefineXML(xml string) (libvirt.Network, error) {
	if m.createError != nil {
		return libvirt.Network{}, m.createError
	}

	name := extractNetworkNameFromXML(xml)
	uuid := generateNetworkUUID(name)

	network := &mockNetwork{
		Network: libvirt.Network{
			Name: name,
			UUID: uuid,
		},
		name:       name,
		uuid:       uuid,
		active:     0, // Not active by default
		persistent: 1, // Defined networks are persistent
		autostart:  0, // Not autostart by default
		xml:        xml,
	}

	m.networks[name] = network
	return network.Network, nil
}

func (m *mockNetworkService) NetworkCreate(net libvirt.Network) error {
	if m.startError != nil {
		return m.startError
	}

	for _, network := range m.networks {
		if network.Network.Name == net.Name {
			network.active = 1
			return nil
		}
	}
	return errors.New("network not found")
}

func (m *mockNetworkService) NetworkDestroy(net libvirt.Network) error {
	if m.stopError != nil {
		return m.stopError
	}

	for _, network := range m.networks {
		if network.Network.Name == net.Name {
			network.active = 0
			return nil
		}
	}
	return errors.New("network not found")
}

func (m *mockNetworkService) NetworkUndefine(net libvirt.Network) error {
	if m.deleteError != nil {
		return m.deleteError
	}

	for name, network := range m.networks {
		if network.Network.Name == net.Name {
			delete(m.networks, name)
			return nil
		}
	}
	return errors.New("network not found")
}

// Helper functions

func extractNetworkNameFromXML(xml string) string {
	// Simplified XML parsing for test
	if start := findStringBetween(xml, "<name>", "</name>"); start != "" {
		return start
	}
	return "test-network"
}

func generateNetworkUUID(name string) [16]byte {
	// Generate a simple UUID for testing
	var uuid [16]byte
	copy(uuid[:], []byte(name+"0000000000000000"))
	return uuid
}

func generateMockNetworkXML(network *mockNetwork) string {
	return `<network>
  <name>` + network.name + `</name>
  <uuid>` + string(network.uuid[:]) + `</uuid>
  <bridge name='virbr0' stp='on' delay='0'/>
  <forward mode='nat'/>
  <ip address='192.168.122.1' netmask='255.255.255.0'>
    <dhcp>
      <range start='192.168.122.2' end='192.168.122.254'/>
    </dhcp>
  </ip>
</network>`
}

// mockNetworkExternal wraps external with a mock service for testing
type mockNetworkExternal struct {
	mockService *mockNetworkService
	kube        client.Client
}

func (e *mockNetworkExternal) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.Network)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotNetwork)
	}

	networkName := cr.Spec.ForProvider.Name

	network, err := e.mockService.NetworkLookupByName(networkName)
	if err != nil {
		if isNetworkNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, errDescribeNetwork)
	}

	active, err := e.mockService.NetworkIsActive(network)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errDescribeNetwork)
	}

	persistent, err := e.mockService.NetworkIsPersistent(network)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errDescribeNetwork)
	}

	autoStart, err := e.mockService.NetworkGetAutostart(network)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errDescribeNetwork)
	}

	xml, err := e.mockService.NetworkGetXMLDesc(network, 0)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errDescribeNetwork)
	}

	bridgeName := parseNetworkBridgeName(xml)

	// Update status
	cr.Status.AtProvider.UUID = string(network.UUID[:])
	cr.Status.AtProvider.Active = active == 1
	cr.Status.AtProvider.Persistent = persistent == 1
	cr.Status.AtProvider.AutoStart = autoStart == 1
	cr.Status.AtProvider.BridgeName = bridgeName

	cr.SetConditions(xpv1.Available())

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: isNetworkUpToDate(cr, int(active), int(autoStart)),
	}, nil
}

func (e *mockNetworkExternal) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.Network)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotNetwork)
	}

	xml, err := generateNetworkXML(cr)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateNetwork)
	}

	network, err := e.mockService.NetworkDefineXML(xml)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateNetwork)
	}

	// Set autostart if requested
	autoStart := cr.Spec.ForProvider.AutoStart
	if autoStart == nil || *autoStart {
		err = e.mockService.NetworkSetAutostart(network, 1)
		if err != nil {
			return managed.ExternalCreation{}, errors.Wrap(err, errCreateNetwork)
		}
	}

	// Start network if autostart is enabled
	if autoStart == nil || *autoStart {
		err = e.mockService.NetworkCreate(network)
		if err != nil {
			return managed.ExternalCreation{}, errors.Wrap(err, errStartNetwork)
		}
	}

	return managed.ExternalCreation{}, nil
}

func (e *mockNetworkExternal) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.Network)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotNetwork)
	}

	networkName := cr.Spec.ForProvider.Name
	network, err := e.mockService.NetworkLookupByName(networkName)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateNetwork)
	}

	// Check if autostart setting needs to be updated
	currentAutoStart, err := e.mockService.NetworkGetAutostart(network)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateNetwork)
	}

	desiredAutoStart := cr.Spec.ForProvider.AutoStart
	if desiredAutoStart != nil {
		if (*desiredAutoStart && currentAutoStart == 0) || (!*desiredAutoStart && currentAutoStart == 1) {
			newAutoStart := int32(0)
			if *desiredAutoStart {
				newAutoStart = 1
			}
			err = e.mockService.NetworkSetAutostart(network, newAutoStart)
			if err != nil {
				return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateNetwork)
			}
		}
	}

	return managed.ExternalUpdate{}, nil
}

func (e *mockNetworkExternal) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*v1alpha1.Network)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotNetwork)
	}

	networkName := cr.Spec.ForProvider.Name
	network, err := e.mockService.NetworkLookupByName(networkName)
	if err != nil {
		if isNetworkNotFound(err) {
			return managed.ExternalDelete{}, nil // Already deleted
		}
		return managed.ExternalDelete{}, errors.Wrap(err, errDeleteNetwork)
	}

	// Stop network if it's running
	active, err := e.mockService.NetworkIsActive(network)
	if err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, errDeleteNetwork)
	}

	if active == 1 {
		err = e.mockService.NetworkDestroy(network)
		if err != nil {
			return managed.ExternalDelete{}, errors.Wrap(err, errStopNetwork)
		}
	}

	// Undefine network
	err = e.mockService.NetworkUndefine(network)
	if err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, errDeleteNetwork)
	}

	return managed.ExternalDelete{}, nil
}

func (e *mockNetworkExternal) Disconnect(ctx context.Context) error {
	return nil
}

func createMockNetworkExternal(mockService *mockNetworkService) *mockNetworkExternal {
	return &mockNetworkExternal{
		mockService: mockService,
		kube:        fake.NewClientBuilder().Build(),
	}
}

// Test cases

func TestNetworkExternal_Observe(t *testing.T) {
	tests := []struct {
		name      string
		mockSetup func(*mockNetworkService)
		network   *v1alpha1.Network
		want      managed.ExternalObservation
		wantErr   bool
		errMsg    string
	}{
		{
			name: "NetworkNotFound",
			mockSetup: func(m *mockNetworkService) {
				// Don't add any networks to the mock
			},
			network: &v1alpha1.Network{
				Spec: v1alpha1.NetworkSpec{
					ForProvider: v1alpha1.NetworkParameters{
						Name: "nonexistent-network",
						Mode: "nat",
					},
				},
			},
			want: managed.ExternalObservation{
				ResourceExists: false,
			},
			wantErr: false,
		},
		{
			name: "NetworkExists",
			mockSetup: func(m *mockNetworkService) {
				network := &mockNetwork{
					Network: libvirt.Network{
						Name: "test-network",
						UUID: generateNetworkUUID("test-network"),
					},
					name:       "test-network",
					uuid:       generateNetworkUUID("test-network"),
					active:     1,
					persistent: 1,
					autostart:  1,
					xml:        generateMockNetworkXML(&mockNetwork{name: "test-network"}),
				}
				m.networks["test-network"] = network
			},
			network: &v1alpha1.Network{
				Spec: v1alpha1.NetworkSpec{
					ForProvider: v1alpha1.NetworkParameters{
						Name:      "test-network",
						Mode:      "nat",
						AutoStart: boolPtr(true),
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
			name: "NetworkExistsButNotUpToDate",
			mockSetup: func(m *mockNetworkService) {
				network := &mockNetwork{
					Network: libvirt.Network{
						Name: "test-network",
						UUID: generateNetworkUUID("test-network"),
					},
					name:       "test-network",
					uuid:       generateNetworkUUID("test-network"),
					active:     0, // Not active
					persistent: 1,
					autostart:  0, // Not autostart
					xml:        generateMockNetworkXML(&mockNetwork{name: "test-network"}),
				}
				m.networks["test-network"] = network
			},
			network: &v1alpha1.Network{
				Spec: v1alpha1.NetworkSpec{
					ForProvider: v1alpha1.NetworkParameters{
						Name:      "test-network",
						Mode:      "nat",
						AutoStart: boolPtr(true), // Wants autostart but network doesn't have it
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
			mockService := newMockNetworkService()
			if tt.mockSetup != nil {
				tt.mockSetup(mockService)
			}

			e := createMockNetworkExternal(mockService)

			got, err := e.Observe(context.Background(), tt.network)

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

func TestNetworkExternal_Create(t *testing.T) {
	tests := []struct {
		name      string
		mockSetup func(*mockNetworkService)
		network   *v1alpha1.Network
		wantErr   bool
		errMsg    string
		validate  func(*testing.T, *mockNetworkService, *v1alpha1.Network)
	}{
		{
			name: "CreateNetworkSuccess",
			mockSetup: func(m *mockNetworkService) {
				// No setup needed - mock will create the network
			},
			network: &v1alpha1.Network{
				Spec: v1alpha1.NetworkSpec{
					ForProvider: v1alpha1.NetworkParameters{
						Name:      "test-network",
						Mode:      "nat",
						AutoStart: boolPtr(true),
						IP: &v1alpha1.NetworkIP{
							Address: "192.168.100.1/24",
							Family:  "ipv4",
						},
					},
				},
			},
			wantErr: false,
			validate: func(t *testing.T, m *mockNetworkService, network *v1alpha1.Network) {
				networkName := network.Spec.ForProvider.Name

				mockNet, exists := m.networks[networkName]
				if !exists {
					t.Error("Expected network to be created")
					return
				}

				if mockNet.persistent != 1 {
					t.Error("Expected network to be persistent")
				}

				if mockNet.autostart != 1 {
					t.Error("Expected network to have autostart enabled")
				}

				if mockNet.active != 1 {
					t.Error("Expected network to be active")
				}
			},
		},
		{
			name: "CreateNetworkWithoutAutoStart",
			mockSetup: func(m *mockNetworkService) {
				// No setup needed
			},
			network: &v1alpha1.Network{
				Spec: v1alpha1.NetworkSpec{
					ForProvider: v1alpha1.NetworkParameters{
						Name:      "test-network",
						Mode:      "isolated",
						AutoStart: boolPtr(false),
					},
				},
			},
			wantErr: false,
			validate: func(t *testing.T, m *mockNetworkService, network *v1alpha1.Network) {
				networkName := network.Spec.ForProvider.Name

				mockNet, exists := m.networks[networkName]
				if !exists {
					t.Error("Expected network to be created")
					return
				}

				if mockNet.autostart != 0 {
					t.Error("Expected network to have autostart disabled")
				}

				if mockNet.active != 0 {
					t.Error("Expected network to be inactive")
				}
			},
		},
		{
			name: "CreateError",
			mockSetup: func(m *mockNetworkService) {
				m.createError = errors.New("failed to create network")
			},
			network: &v1alpha1.Network{
				Spec: v1alpha1.NetworkSpec{
					ForProvider: v1alpha1.NetworkParameters{
						Name: "test-network",
						Mode: "nat",
					},
				},
			},
			wantErr: true,
			errMsg:  errCreateNetwork,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := newMockNetworkService()
			if tt.mockSetup != nil {
				tt.mockSetup(mockService)
			}

			e := createMockNetworkExternal(mockService)

			_, err := e.Create(context.Background(), tt.network)

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
				tt.validate(t, mockService, tt.network)
			}
		})
	}
}

func TestNetworkExternal_Delete(t *testing.T) {
	tests := []struct {
		name      string
		mockSetup func(*mockNetworkService)
		network   *v1alpha1.Network
		wantErr   bool
		errMsg    string
		validate  func(*testing.T, *mockNetworkService, *v1alpha1.Network)
	}{
		{
			name: "DeleteNetworkSuccess",
			mockSetup: func(m *mockNetworkService) {
				network := &mockNetwork{
					Network: libvirt.Network{
						Name: "test-network",
						UUID: generateNetworkUUID("test-network"),
					},
					name:       "test-network",
					uuid:       generateNetworkUUID("test-network"),
					active:     1, // Network is active
					persistent: 1,
					autostart:  1,
					xml:        generateMockNetworkXML(&mockNetwork{name: "test-network"}),
				}
				m.networks["test-network"] = network
			},
			network: &v1alpha1.Network{
				Spec: v1alpha1.NetworkSpec{
					ForProvider: v1alpha1.NetworkParameters{
						Name: "test-network",
						Mode: "nat",
					},
				},
			},
			wantErr: false,
			validate: func(t *testing.T, m *mockNetworkService, network *v1alpha1.Network) {
				networkName := network.Spec.ForProvider.Name

				_, exists := m.networks[networkName]
				if exists {
					t.Error("Expected network to be deleted")
				}
			},
		},
		{
			name: "DeleteNonexistentNetwork",
			mockSetup: func(m *mockNetworkService) {
				// Don't add any networks
			},
			network: &v1alpha1.Network{
				Spec: v1alpha1.NetworkSpec{
					ForProvider: v1alpha1.NetworkParameters{
						Name: "nonexistent-network",
						Mode: "nat",
					},
				},
			},
			wantErr: false, // Should succeed (already deleted)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := newMockNetworkService()
			if tt.mockSetup != nil {
				tt.mockSetup(mockService)
			}

			e := createMockNetworkExternal(mockService)

			_, err := e.Delete(context.Background(), tt.network)

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
				tt.validate(t, mockService, tt.network)
			}
		})
	}
}

func TestGenerateNetworkXML(t *testing.T) {
	tests := []struct {
		name     string
		network  *v1alpha1.Network
		wantErr  bool
		validate func(*testing.T, string)
	}{
		{
			name: "NATNetwork",
			network: &v1alpha1.Network{
				Spec: v1alpha1.NetworkSpec{
					ForProvider: v1alpha1.NetworkParameters{
						Name: "test-nat",
						Mode: "nat",
						Bridge: &v1alpha1.NetworkBridge{
							Name: "virbr0",
							STP: &v1alpha1.NetworkSTP{
								Enabled: boolPtr(true),
							},
						},
						IP: &v1alpha1.NetworkIP{
							Address: "192.168.122.1/24",
							Family:  "ipv4",
						},
						DHCP: &v1alpha1.NetworkDHCP{
							Enabled: boolPtr(true),
							Range: &v1alpha1.NetworkDHCPRange{
								Start: "192.168.122.2",
								End:   "192.168.122.254",
							},
						},
						Forward: &v1alpha1.NetworkForward{
							Mode: "nat",
						},
					},
				},
			},
			wantErr: false,
			validate: func(t *testing.T, xml string) {
				if !containsSubstring(xml, "<name>test-nat</name>") {
					t.Error("XML should contain network name")
				}
				if !containsSubstring(xml, "<bridge name='virbr0'") {
					t.Error("XML should contain bridge configuration")
				}
				if !containsSubstring(xml, "<forward mode='nat'>") {
					t.Error("XML should contain forward mode")
				}
				if !containsSubstring(xml, "<ip address='192.168.122.1/24'>") {
					t.Error("XML should contain IP configuration")
				}
				if !containsSubstring(xml, "<range start='192.168.122.2' end='192.168.122.254'/>") {
					t.Error("XML should contain DHCP range")
				}
			},
		},
		{
			name: "IsolatedNetwork",
			network: &v1alpha1.Network{
				Spec: v1alpha1.NetworkSpec{
					ForProvider: v1alpha1.NetworkParameters{
						Name: "isolated-net",
						Mode: "isolated",
						IP: &v1alpha1.NetworkIP{
							Address: "10.0.0.1/24",
							Family:  "ipv4",
						},
						DHCP: &v1alpha1.NetworkDHCP{
							Enabled: boolPtr(true),
							Hosts: []v1alpha1.NetworkDHCPHost{
								{
									MAC:  "52:54:00:12:34:56",
									Name: "test-vm",
									IP:   "10.0.0.50",
								},
							},
						},
						DNS: &v1alpha1.NetworkDNS{
							Enable: boolPtr(true),
							Hosts: []v1alpha1.NetworkDNSHost{
								{
									IP:       "10.0.0.50",
									Hostname: "test-vm.local",
								},
							},
						},
					},
				},
			},
			wantErr: false,
			validate: func(t *testing.T, xml string) {
				if !containsSubstring(xml, "<name>isolated-net</name>") {
					t.Error("XML should contain network name")
				}
				if !containsSubstring(xml, "<ip address='10.0.0.1/24'>") {
					t.Error("XML should contain IP configuration")
				}
				if !containsSubstring(xml, "<host mac='52:54:00:12:34:56' ip='10.0.0.50' name='test-vm'/>") {
					t.Error("XML should contain DHCP host configuration")
				}
				if !containsSubstring(xml, "<host ip='10.0.0.50'>") {
					t.Error("XML should contain DNS host configuration")
				}
				if !containsSubstring(xml, "<hostname>test-vm.local</hostname>") {
					t.Error("XML should contain hostname")
				}
			},
		},
		{
			name: "BridgeNetwork",
			network: &v1alpha1.Network{
				Spec: v1alpha1.NetworkSpec{
					ForProvider: v1alpha1.NetworkParameters{
						Name: "br0",
						Mode: "bridge",
						Bridge: &v1alpha1.NetworkBridge{
							Name: "br0",
						},
						Forward: &v1alpha1.NetworkForward{
							Mode: "bridge",
							Interfaces: []v1alpha1.NetworkInterface{
								{
									Dev: "eth0",
								},
							},
						},
					},
				},
			},
			wantErr: false,
			validate: func(t *testing.T, xml string) {
				if !containsSubstring(xml, "<name>br0</name>") {
					t.Error("XML should contain network name")
				}
				if !containsSubstring(xml, "<bridge name='br0'/>") {
					t.Error("XML should contain bridge configuration")
				}
				if !containsSubstring(xml, "<forward mode='bridge'>") {
					t.Error("XML should contain forward mode")
				}
				if !containsSubstring(xml, "<interface dev='eth0'/>") {
					t.Error("XML should contain interface configuration")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			xml, err := generateNetworkXML(tt.network)

			if tt.wantErr {
				if err == nil {
					t.Errorf("generateNetworkXML() error = nil, wantErr %v", tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("generateNetworkXML() error = %v, wantErr %v", err, tt.wantErr)
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