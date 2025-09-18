/*
Copyright 2025 Ross Golder
*/

package domain

import (
	"context"
	"testing"

	"github.com/digitalocean/go-libvirt"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"

	"github.com/rossigee/provider-libvirt/apis/v1alpha1"
)

// mockManagedResource implements resource.Managed for testing wrong types
type mockManagedResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
}

func (m *mockManagedResource) DeepCopyObject() runtime.Object {
	return &mockManagedResource{
		TypeMeta:   m.TypeMeta,
		ObjectMeta: m.ObjectMeta,
	}
}
func (m *mockManagedResource) GetDeletionPolicy() xpv1.DeletionPolicy                         { return xpv1.DeletionOrphan }
func (m *mockManagedResource) SetDeletionPolicy(xpv1.DeletionPolicy)                        {}
func (m *mockManagedResource) GetManagementPolicies() xpv1.ManagementPolicies               { return nil }
func (m *mockManagedResource) SetManagementPolicies(xpv1.ManagementPolicies)                {}
func (m *mockManagedResource) GetProviderConfigReference() *xpv1.Reference                  { return nil }
func (m *mockManagedResource) SetProviderConfigReference(*xpv1.Reference)                   {}
func (m *mockManagedResource) GetWriteConnectionSecretToReference() *xpv1.SecretReference   { return nil }
func (m *mockManagedResource) SetWriteConnectionSecretToReference(*xpv1.SecretReference)    {}
func (m *mockManagedResource) GetCondition(xpv1.ConditionType) xpv1.Condition             { return xpv1.Condition{} }
func (m *mockManagedResource) SetConditions(...xpv1.Condition)                              {}

// LibvirtService defines the interface we need for testing
type LibvirtService interface {
	DomainLookupByName(name string) (libvirt.Domain, error)
	DomainGetState(domain libvirt.Domain, flags uint32) (int32, int32, error)
	DomainGetInfo(domain libvirt.Domain) (int8, uint64, uint64, uint32, uint64, error)
	DomainDefineXML(xml string) (libvirt.Domain, error)
	DomainCreate(domain libvirt.Domain) error
	DomainDestroy(domain libvirt.Domain) error
	DomainShutdown(domain libvirt.Domain) error
	DomainUndefine(domain libvirt.Domain) error
	DomainSetAutostart(domain libvirt.Domain, autostart int32) error
	Close() error
}

// mockLibvirtService implements LibvirtService for testing
type mockLibvirtService struct {
	domains     map[string]*mockDomain
	definedXML  string
	createError error
	destroyError error
	undefineError error
	lookupError  error
	getStateError error
	setAutostartError error
}

type mockDomain struct {
	libvirt.Domain
	id      int32
	uuid    [16]byte
	name    string
	state   libvirt.DomainState
	memory  uint64
	vcpu    uint32
	running bool
}

func newMockLibvirtService() *mockLibvirtService {
	return &mockLibvirtService{
		domains: make(map[string]*mockDomain),
	}
}

func (m *mockLibvirtService) DomainLookupByName(name string) (libvirt.Domain, error) {
	if m.lookupError != nil {
		return libvirt.Domain{}, m.lookupError
	}
	
	domain, exists := m.domains[name]
	if !exists {
		return libvirt.Domain{}, errors.New("Domain not found: no domain with matching name")
	}
	
	return domain.Domain, nil
}

func (m *mockLibvirtService) DomainGetState(domain libvirt.Domain, flags uint32) (int32, int32, error) {
	if m.getStateError != nil {
		return 0, 0, m.getStateError
	}
	
	mockDomain := m.findDomainByID(domain.ID)
	if mockDomain == nil {
		return 0, 0, errors.New("domain not found")
	}
	
	return int32(mockDomain.state), 0, nil
}

func (m *mockLibvirtService) DomainGetInfo(domain libvirt.Domain) (int8, uint64, uint64, uint32, uint64, error) {
	mockDomain := m.findDomainByID(domain.ID)
	if mockDomain == nil {
		return 0, 0, 0, 0, 0, errors.New("domain not found")
	}
	
	return int8(mockDomain.state), mockDomain.memory, mockDomain.memory, mockDomain.vcpu, 0, nil
}

func (m *mockLibvirtService) DomainDefineXML(xml string) (libvirt.Domain, error) {
	m.definedXML = xml
	
	// Create a new mock domain
	domain := &mockDomain{
		Domain: libvirt.Domain{
			ID:   int32(len(m.domains) + 1),
			Name: "test-domain", // Parse from XML in real implementation
		},
		id:     int32(len(m.domains) + 1),
		name:   "test-domain",
		state:  libvirt.DomainShutoff,
		memory: 1024 * 1024, // 1GB in KB
		vcpu:   2,
		running: false,
	}
	
	// Generate UUID
	for i := 0; i < 16; i++ {
		domain.uuid[i] = byte(i)
		domain.UUID[i] = byte(i)
	}
	
	m.domains[domain.name] = domain
	return domain.Domain, nil
}

func (m *mockLibvirtService) DomainCreate(domain libvirt.Domain) error {
	if m.createError != nil {
		return m.createError
	}
	
	mockDomain := m.findDomainByID(domain.ID)
	if mockDomain == nil {
		return errors.New("domain not found")
	}
	
	mockDomain.state = libvirt.DomainRunning
	mockDomain.running = true
	return nil
}

func (m *mockLibvirtService) DomainDestroy(domain libvirt.Domain) error {
	if m.destroyError != nil {
		return m.destroyError
	}
	
	mockDomain := m.findDomainByID(domain.ID)
	if mockDomain == nil {
		return errors.New("domain not found")
	}
	
	mockDomain.state = libvirt.DomainShutoff
	mockDomain.running = false
	return nil
}

func (m *mockLibvirtService) DomainShutdown(domain libvirt.Domain) error {
	mockDomain := m.findDomainByID(domain.ID)
	if mockDomain == nil {
		return errors.New("domain not found")
	}
	
	mockDomain.state = libvirt.DomainShutoff
	mockDomain.running = false
	return nil
}

func (m *mockLibvirtService) DomainUndefine(domain libvirt.Domain) error {
	if m.undefineError != nil {
		return m.undefineError
	}
	
	mockDomain := m.findDomainByID(domain.ID)
	if mockDomain == nil {
		return errors.New("domain not found")
	}
	
	delete(m.domains, mockDomain.name)
	return nil
}

func (m *mockLibvirtService) DomainSetAutostart(domain libvirt.Domain, autostart int32) error {
	if m.setAutostartError != nil {
		return m.setAutostartError
	}
	return nil
}

func (m *mockLibvirtService) Close() error {
	return nil
}

func (m *mockLibvirtService) findDomainByID(id int32) *mockDomain {
	for _, domain := range m.domains {
		if domain.id == id {
			return domain
		}
	}
	return nil
}

// mockExternal wraps external with a mock service for testing
type mockExternal struct {
	mockService LibvirtService
	kube        client.Client
}

func (e *mockExternal) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	// This is a simplified version that uses our mock service
	// In practice, you'd inject the interface properly into the real external struct
	cr, ok := mg.(*v1alpha1.Domain)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotDomain)
	}

	domainName := cr.Spec.ForProvider.Name
	domain, err := e.mockService.DomainLookupByName(domainName)
	if err != nil {
		if isNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, errDescribeDomain)
	}

	state, _, err := e.mockService.DomainGetState(domain, 0)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errDescribeDomain)
	}

	_, _, memory, vcpu, _, err := e.mockService.DomainGetInfo(domain)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errDescribeDomain)
	}

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: isUpToDate(cr, memory, uint32(vcpu), state),
	}, nil
}

func (e *mockExternal) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.Domain)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotDomain)
	}

	xml, err := generateDomainXML(cr)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateDomain)
	}

	domain, err := e.mockService.DomainDefineXML(xml)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateDomain)
	}

	if cr.Spec.ForProvider.Running {
		err = e.mockService.DomainCreate(domain)
		if err != nil {
			return managed.ExternalCreation{}, errors.Wrap(err, errCreateDomain)
		}
	}

	if cr.Spec.ForProvider.Autostart {
		err = e.mockService.DomainSetAutostart(domain, 1)
		if err != nil {
			return managed.ExternalCreation{}, errors.Wrap(err, errCreateDomain)
		}
	}

	return managed.ExternalCreation{}, nil
}

func (e *mockExternal) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.Domain)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotDomain)
	}

	domainName := cr.Spec.ForProvider.Name
	// Check if we have external name annotation
	if externalName := cr.GetAnnotations(); externalName != nil {
		if name, exists := externalName["crossplane.io/external-name"]; exists {
			domainName = name
		}
	}

	domain, err := e.mockService.DomainLookupByName(domainName)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateDomain)
	}

	// Handle running state changes
	state, _, err := e.mockService.DomainGetState(domain, 0)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateDomain)
	}

	if cr.Spec.ForProvider.Running && libvirt.DomainState(state) != libvirt.DomainRunning {
		err = e.mockService.DomainCreate(domain)
		if err != nil {
			return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateDomain)
		}
	} else if !cr.Spec.ForProvider.Running && libvirt.DomainState(state) == libvirt.DomainRunning {
		err = e.mockService.DomainShutdown(domain)
		if err != nil {
			return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateDomain)
		}
	}

	return managed.ExternalUpdate{}, nil
}

func (e *mockExternal) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*v1alpha1.Domain)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotDomain)
	}

	domainName := cr.Spec.ForProvider.Name
	domain, err := e.mockService.DomainLookupByName(domainName)
	if err != nil {
		if isNotFound(err) {
			return managed.ExternalDelete{}, nil // Already deleted
		}
		return managed.ExternalDelete{}, errors.Wrap(err, errDeleteDomain)
	}

	state, _, err := e.mockService.DomainGetState(domain, 0)
	if err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, errDeleteDomain)
	}

	if libvirt.DomainState(state) == libvirt.DomainRunning {
		err = e.mockService.DomainDestroy(domain)
		if err != nil {
			return managed.ExternalDelete{}, errors.Wrap(err, errDeleteDomain)
		}
	}

	err = e.mockService.DomainUndefine(domain)
	if err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, errDeleteDomain)
	}

	return managed.ExternalDelete{}, nil
}

func (e *mockExternal) Disconnect(ctx context.Context) error {
	return nil
}

func createMockExternal(mockService *mockLibvirtService) *mockExternal {
	return &mockExternal{
		mockService: mockService,
		kube:        fake.NewClientBuilder().Build(),
	}
}

func TestExternal_Observe(t *testing.T) {
	type args struct {
		ctx context.Context
		mg  resource.Managed
	}

	tests := []struct {
		name       string
		mockSetup  func(*mockLibvirtService)
		args       args
		want       managed.ExternalObservation
		wantErr    bool
		errMsg     string
	}{
		{
			name: "NotDomain",
			args: args{
				ctx: context.Background(),
				mg:  &mockManagedResource{}, // Use a mock instead
			},
			wantErr: true,
			errMsg:  errNotDomain,
		},
		{
			name: "DomainNotFound",
			mockSetup: func(m *mockLibvirtService) {
				m.lookupError = errors.New("Domain not found: no domain with matching name")
			},
			args: args{
				ctx: context.Background(),
				mg: &v1alpha1.Domain{
					Spec: v1alpha1.DomainSpec{
						ForProvider: v1alpha1.DomainParameters{
							Name: "nonexistent-domain",
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
			name: "DomainExists",
			mockSetup: func(m *mockLibvirtService) {
				// Pre-create domain
				domain := &mockDomain{
					Domain: libvirt.Domain{
						ID:   1,
						Name: "test-domain",
					},
					id:     1,
					name:   "test-domain",
					state:  libvirt.DomainRunning,
					memory: 2097152, // 2GB in KB
					vcpu:   2,
					running: true,
				}
				for i := 0; i < 16; i++ {
					domain.uuid[i] = byte(i)
					domain.UUID[i] = byte(i)
				}
				m.domains["test-domain"] = domain
			},
			args: args{
				ctx: context.Background(),
				mg: &v1alpha1.Domain{
					Spec: v1alpha1.DomainSpec{
						ForProvider: v1alpha1.DomainParameters{
							Name:    "test-domain",
							Memory:  2147483648, // 2GB
							Vcpu:    2,
							Running: true,
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := newMockLibvirtService()
			if tt.mockSetup != nil {
				tt.mockSetup(mockClient)
			}
			
			e := createMockExternal(mockClient)
			
			got, err := e.Observe(tt.args.ctx, tt.args.mg)
			
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

func TestExternal_Create(t *testing.T) {
	type args struct {
		ctx context.Context
		mg  resource.Managed
	}

	tests := []struct {
		name      string
		mockSetup func(*mockLibvirtService)
		args      args
		wantErr   bool
		errMsg    string
		validate  func(*testing.T, *mockLibvirtService, resource.Managed)
	}{
		{
			name: "NotDomain",
			args: args{
				ctx: context.Background(),
				mg:  &mockManagedResource{}, // Wrong type
			},
			wantErr: true,
			errMsg:  errNotDomain,
		},
		{
			name: "CreateDomainSuccess",
			args: args{
				ctx: context.Background(),
				mg: &v1alpha1.Domain{
					Spec: v1alpha1.DomainSpec{
						ForProvider: v1alpha1.DomainParameters{
							Name:      "test-domain",
							Memory:    2147483648, // 2GB
							Vcpu:      2,
							Running:   true,
							Autostart: true,
						},
					},
				},
			},
			wantErr: false,
			validate: func(t *testing.T, m *mockLibvirtService, mg resource.Managed) {
				if m.definedXML == "" {
					t.Error("Expected XML to be defined")
				}
				
				domain, exists := m.domains["test-domain"]
				if !exists {
					t.Error("Expected domain to be created")
					return
				}
				
				if domain.state != libvirt.DomainRunning {
					t.Errorf("Expected domain to be running, got state %v", domain.state)
				}
			},
		},
		{
			name: "CreateError",
			mockSetup: func(m *mockLibvirtService) {
				m.createError = errors.New("failed to start domain")
			},
			args: args{
				ctx: context.Background(),
				mg: &v1alpha1.Domain{
					Spec: v1alpha1.DomainSpec{
						ForProvider: v1alpha1.DomainParameters{
							Name:    "test-domain",
							Memory:  2147483648,
							Vcpu:    2,
							Running: true,
						},
					},
				},
			},
			wantErr: true,
			errMsg:  errCreateDomain,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := newMockLibvirtService()
			if tt.mockSetup != nil {
				tt.mockSetup(mockClient)
			}
			
			e := createMockExternal(mockClient)
			
			_, err := e.Create(tt.args.ctx, tt.args.mg)
			
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
				tt.validate(t, mockClient, tt.args.mg)
			}
		})
	}
}

func TestExternal_Delete(t *testing.T) {
	type args struct {
		ctx context.Context
		mg  resource.Managed
	}

	tests := []struct {
		name      string
		mockSetup func(*mockLibvirtService)
		args      args
		wantErr   bool
		errMsg    string
		validate  func(*testing.T, *mockLibvirtService)
	}{
		{
			name: "NotDomain",
			args: args{
				ctx: context.Background(),
				mg:  &mockManagedResource{}, // Wrong type
			},
			wantErr: true,
			errMsg:  errNotDomain,
		},
		{
			name: "DomainNotFound",
			mockSetup: func(m *mockLibvirtService) {
				m.lookupError = errors.New("Domain not found: no domain with matching name")
			},
			args: args{
				ctx: context.Background(),
				mg: &v1alpha1.Domain{
					Spec: v1alpha1.DomainSpec{
						ForProvider: v1alpha1.DomainParameters{
							Name: "nonexistent-domain",
						},
					},
				},
			},
			wantErr: false, // Should not error if domain doesn't exist
		},
		{
			name: "DeleteRunningDomain",
			mockSetup: func(m *mockLibvirtService) {
				// Pre-create running domain
				domain := &mockDomain{
					Domain: libvirt.Domain{
						ID:   1,
						Name: "test-domain",
					},
					id:     1,
					name:   "test-domain",
					state:  libvirt.DomainRunning,
					running: true,
				}
				m.domains["test-domain"] = domain
			},
			args: args{
				ctx: context.Background(),
				mg: &v1alpha1.Domain{
					Spec: v1alpha1.DomainSpec{
						ForProvider: v1alpha1.DomainParameters{
							Name: "test-domain",
						},
					},
				},
			},
			wantErr: false,
			validate: func(t *testing.T, m *mockLibvirtService) {
				_, exists := m.domains["test-domain"]
				if exists {
					t.Error("Expected domain to be deleted")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := newMockLibvirtService()
			if tt.mockSetup != nil {
				tt.mockSetup(mockClient)
			}
			
			e := createMockExternal(mockClient)
			
			_, err := e.Delete(tt.args.ctx, tt.args.mg)
			
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
				tt.validate(t, mockClient)
			}
		})
	}
}

func TestGenerateDomainXML(t *testing.T) {
	tests := []struct {
		name    string
		domain  *v1alpha1.Domain
		wantErr bool
		validate func(*testing.T, string)
	}{
		{
			name: "BasicDomain",
			domain: &v1alpha1.Domain{
				Spec: v1alpha1.DomainSpec{
					ForProvider: v1alpha1.DomainParameters{
						Name:   "test-domain",
						Memory: 2147483648, // 2GB
						Vcpu:   2,
						Type:   "kvm",
						Arch:   "x86_64",
					},
				},
			},
			wantErr: false,
			validate: func(t *testing.T, xml string) {
				if !containsSubstring(xml, "<name>test-domain</name>") {
					t.Error("XML should contain domain name")
				}
				if !containsSubstring(xml, "<memory unit='bytes'>2147483648</memory>") {
					t.Error("XML should contain memory specification")
				}
				if !containsSubstring(xml, "<vcpu placement='static'>2</vcpu>") {
					t.Error("XML should contain vcpu specification")
				}
				if !containsSubstring(xml, "type='kvm'") {
					t.Error("XML should contain domain type")
				}
			},
		},
		{
			name: "DomainWithDefaults",
			domain: &v1alpha1.Domain{
				Spec: v1alpha1.DomainSpec{
					ForProvider: v1alpha1.DomainParameters{
						Name:   "default-domain",
						Memory: 1073741824, // 1GB
						Vcpu:   1,
						// No Type/Arch specified - should use defaults
					},
				},
			},
			wantErr: false,
			validate: func(t *testing.T, xml string) {
				if !containsSubstring(xml, "type='kvm'") {
					t.Error("XML should contain default domain type (kvm)")
				}
				if !containsSubstring(xml, "arch='x86_64'") {
					t.Error("XML should contain default arch (x86_64)")
				}
				if !containsSubstring(xml, "<boot dev='hd'/>") {
					t.Error("XML should contain default boot device")
				}
			},
		},
		{
			name: "DomainWithCustomBoot",
			domain: &v1alpha1.Domain{
				Spec: v1alpha1.DomainSpec{
					ForProvider: v1alpha1.DomainParameters{
						Name:   "boot-domain",
						Memory: 1073741824,
						Vcpu:   1,
						Boot:   []string{"cdrom", "hd"},
					},
				},
			},
			wantErr: false,
			validate: func(t *testing.T, xml string) {
				if !containsSubstring(xml, "<boot dev='cdrom'/>") {
					t.Error("XML should contain cdrom boot device")
				}
				if !containsSubstring(xml, "<boot dev='hd'/>") {
					t.Error("XML should contain hd boot device")
				}
			},
		},
		{
			name: "DomainWithDisks",
			domain: &v1alpha1.Domain{
				Spec: v1alpha1.DomainSpec{
					ForProvider: v1alpha1.DomainParameters{
						Name:   "test-domain",
						Memory: 1073741824, // 1GB
						Vcpu:   1,
						Disk: []v1alpha1.DomainDisk{
							{
								File: "/var/lib/libvirt/images/test.qcow2",
								Type: "virtio",
							},
							{
								File: "/var/lib/libvirt/images/data.qcow2",
								// No type specified - should default
							},
						},
					},
				},
			},
			wantErr: false,
			validate: func(t *testing.T, xml string) {
				if !containsSubstring(xml, "<disk type='file' device='disk'>") {
					t.Error("XML should contain disk specification")
				}
				if !containsSubstring(xml, "/var/lib/libvirt/images/test.qcow2") {
					t.Error("XML should contain first disk file path")
				}
				if !containsSubstring(xml, "/var/lib/libvirt/images/data.qcow2") {
					t.Error("XML should contain second disk file path")
				}
				if !containsSubstring(xml, "bus='virtio'") {
					t.Error("XML should contain disk bus type")
				}
				if !containsSubstring(xml, "dev='vda'") {
					t.Error("XML should contain first disk target")
				}
				if !containsSubstring(xml, "dev='vdb'") {
					t.Error("XML should contain second disk target")
				}
			},
		},
		{
			name: "DomainWithNetworkInterface",
			domain: &v1alpha1.Domain{
				Spec: v1alpha1.DomainSpec{
					ForProvider: v1alpha1.DomainParameters{
						Name:   "net-domain",
						Memory: 1073741824,
						Vcpu:   1,
						NetworkInterface: []v1alpha1.DomainNetworkInterface{
							{
								NetworkName: "default",
								Model:       "virtio",
								MAC:         "52:54:00:12:34:56",
							},
							{
								NetworkName: "isolated",
								// Model and MAC not specified - should use defaults
							},
						},
					},
				},
			},
			wantErr: false,
			validate: func(t *testing.T, xml string) {
				if !containsSubstring(xml, "<interface type='network'>") {
					t.Error("XML should contain network interface")
				}
				if !containsSubstring(xml, "<source network='default'/>") {
					t.Error("XML should contain default network")
				}
				if !containsSubstring(xml, "<source network='isolated'/>") {
					t.Error("XML should contain isolated network")
				}
				if !containsSubstring(xml, "<mac address='52:54:00:12:34:56'/>") {
					t.Error("XML should contain MAC address")
				}
				if !containsSubstring(xml, "<model type='virtio'/>") {
					t.Error("XML should contain network model")
				}
			},
		},
		{
			name: "DomainWithConsole",
			domain: &v1alpha1.Domain{
				Spec: v1alpha1.DomainSpec{
					ForProvider: v1alpha1.DomainParameters{
						Name:   "console-domain",
						Memory: 1073741824,
						Vcpu:   1,
						Console: &v1alpha1.DomainConsole{
							Type: "pty",
						},
					},
				},
			},
			wantErr: false,
			validate: func(t *testing.T, xml string) {
				if !containsSubstring(xml, "<console type='pty'>") {
					t.Error("XML should contain console configuration")
				}
				if !containsSubstring(xml, "<target type='serial' port='0'/>") {
					t.Error("XML should contain console target")
				}
			},
		},
		{
			name: "DomainWithGraphics",
			domain: &v1alpha1.Domain{
				Spec: v1alpha1.DomainSpec{
					ForProvider: v1alpha1.DomainParameters{
						Name:   "graphics-domain",
						Memory: 1073741824,
						Vcpu:   1,
						Graphics: &v1alpha1.DomainGraphics{
							Type:          "vnc",
							ListenAddress: "0.0.0.0",
							Autoport:      false,
						},
					},
				},
			},
			wantErr: false,
			validate: func(t *testing.T, xml string) {
				if !containsSubstring(xml, "<graphics type='vnc' autoport='no'>") {
					t.Error("XML should contain graphics configuration")
				}
				if !containsSubstring(xml, "<listen type='address' address='0.0.0.0'/>") {
					t.Error("XML should contain graphics listen address")
				}
			},
		},
		{
			name: "DomainWithDefaultGraphics",
			domain: &v1alpha1.Domain{
				Spec: v1alpha1.DomainSpec{
					ForProvider: v1alpha1.DomainParameters{
						Name:   "default-graphics-domain",
						Memory: 1073741824,
						Vcpu:   1,
						Graphics: &v1alpha1.DomainGraphics{
							// Use defaults
							Autoport: true,
						},
					},
				},
			},
			wantErr: false,
			validate: func(t *testing.T, xml string) {
				if !containsSubstring(xml, "<graphics type='spice' autoport='yes'>") {
					t.Error("XML should contain default graphics type (spice)")
				}
				if !containsSubstring(xml, "<listen type='address' address='127.0.0.1'/>") {
					t.Error("XML should contain default listen address")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			xml, err := generateDomainXML(tt.domain)
			
			if tt.wantErr {
				if err == nil {
					t.Errorf("generateDomainXML() error = nil, wantErr %v", tt.wantErr)
				}
				return
			}
			
			if err != nil {
				t.Errorf("generateDomainXML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if tt.validate != nil {
				tt.validate(t, xml)
			}
		})
	}
}

func TestExternal_Update(t *testing.T) {
	type args struct {
		ctx context.Context
		mg  resource.Managed
	}

	tests := []struct {
		name      string
		mockSetup func(*mockLibvirtService)
		args      args
		wantErr   bool
		errMsg    string
	}{
		{
			name: "NotDomain",
			args: args{
				ctx: context.Background(),
				mg:  &mockManagedResource{},
			},
			wantErr: true,
			errMsg:  errNotDomain,
		},
		{
			name: "DomainNotFound",
			mockSetup: func(m *mockLibvirtService) {
				m.lookupError = errors.New("Domain not found")
			},
			args: args{
				ctx: context.Background(),
				mg: &v1alpha1.Domain{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							"crossplane.io/external-name": "test-domain",
						},
					},
					Spec: v1alpha1.DomainSpec{
						ForProvider: v1alpha1.DomainParameters{
							Name: "test-domain",
						},
					},
				},
			},
			wantErr: true,
			errMsg:  errUpdateDomain,
		},
		{
			name: "StartStoppedDomain",
			mockSetup: func(m *mockLibvirtService) {
				// Pre-create stopped domain
				domain := &mockDomain{
					Domain: libvirt.Domain{
						ID:   1,
						Name: "test-domain",
					},
					id:     1,
					name:   "test-domain",
					state:  libvirt.DomainShutoff,
					running: false,
				}
				m.domains["test-domain"] = domain
			},
			args: args{
				ctx: context.Background(),
				mg: &v1alpha1.Domain{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							"crossplane.io/external-name": "test-domain",
						},
					},
					Spec: v1alpha1.DomainSpec{
						ForProvider: v1alpha1.DomainParameters{
							Name:    "test-domain",
							Running: true,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "StopRunningDomain",
			mockSetup: func(m *mockLibvirtService) {
				// Pre-create running domain
				domain := &mockDomain{
					Domain: libvirt.Domain{
						ID:   1,
						Name: "test-domain",
					},
					id:     1,
					name:   "test-domain",
					state:  libvirt.DomainRunning,
					running: true,
				}
				m.domains["test-domain"] = domain
			},
			args: args{
				ctx: context.Background(),
				mg: &v1alpha1.Domain{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							"crossplane.io/external-name": "test-domain",
						},
					},
					Spec: v1alpha1.DomainSpec{
						ForProvider: v1alpha1.DomainParameters{
							Name:    "test-domain",
							Running: false,
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := newMockLibvirtService()
			if tt.mockSetup != nil {
				tt.mockSetup(mockClient)
			}
			
			e := createMockExternal(mockClient)
			
			_, err := e.Update(tt.args.ctx, tt.args.mg)
			
			if tt.wantErr {
				if err == nil {
					t.Errorf("Update() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if tt.errMsg != "" && !containsSubstring(err.Error(), tt.errMsg) {
					t.Errorf("Update() error = %v, want error containing %v", err, tt.errMsg)
				}
				return
			}
			
			if err != nil {
				t.Errorf("Update() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestHelperFunctions(t *testing.T) {
	t.Run("isNotFound", func(t *testing.T) {
		tests := []struct {
			name string
			err  error
			want bool
		}{
			{
				name: "NotFoundError",
				err:  errors.New("domain not found"),
				want: true,
			},
			{
				name: "NoDomainError",
				err:  errors.New("no domain with matching name"),
				want: true,
			},
			{
				name: "OtherError",
				err:  errors.New("connection failed"),
				want: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if got := isNotFound(tt.err); got != tt.want {
					t.Errorf("isNotFound() = %v, want %v", got, tt.want)
				}
			})
		}
	})

	t.Run("stateToString", func(t *testing.T) {
		tests := []struct {
			name  string
			state libvirt.DomainState
			want  string
		}{
			{"NoState", libvirt.DomainNostate, "nostate"},
			{"Running", libvirt.DomainRunning, "running"},
			{"Blocked", libvirt.DomainBlocked, "blocked"},
			{"Paused", libvirt.DomainPaused, "paused"},
			{"Shutdown", libvirt.DomainShutdown, "shutdown"},
			{"Shutoff", libvirt.DomainShutoff, "shutoff"},
			{"Crashed", libvirt.DomainCrashed, "crashed"},
			{"PMSuspended", libvirt.DomainPmsuspended, "pmsuspended"},
			{"Unknown", libvirt.DomainState(99), "unknown"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if got := stateToString(tt.state); got != tt.want {
					t.Errorf("stateToString() = %v, want %v", got, tt.want)
				}
			})
		}
	})

	t.Run("isUpToDate", func(t *testing.T) {
		tests := []struct {
			name   string
			cr     *v1alpha1.Domain
			memory uint64
			vcpu   uint32
			state  int32
			want   bool
		}{
			{
				name: "UpToDate",
				cr: &v1alpha1.Domain{
					Spec: v1alpha1.DomainSpec{
						ForProvider: v1alpha1.DomainParameters{
							Memory:  2147483648, // 2GB
							Vcpu:    2,
							Running: true,
						},
					},
				},
				memory: 2097152, // 2GB in KB
				vcpu:   2,
				state:  int32(libvirt.DomainRunning),
				want:   true,
			},
			{
				name: "MemoryMismatch",
				cr: &v1alpha1.Domain{
					Spec: v1alpha1.DomainSpec{
						ForProvider: v1alpha1.DomainParameters{
							Memory:  2147483648, // 2GB
							Vcpu:    2,
							Running: true,
						},
					},
				},
				memory: 1048576, // 1GB in KB
				vcpu:   2,
				state:  int32(libvirt.DomainRunning),
				want:   false,
			},
			{
				name: "VcpuMismatch",
				cr: &v1alpha1.Domain{
					Spec: v1alpha1.DomainSpec{
						ForProvider: v1alpha1.DomainParameters{
							Memory:  2147483648,
							Vcpu:    2,
							Running: true,
						},
					},
				},
				memory: 2097152,
				vcpu:   4, // Different VCPU count
				state:  int32(libvirt.DomainRunning),
				want:   false,
			},
			{
				name: "StateMismatch",
				cr: &v1alpha1.Domain{
					Spec: v1alpha1.DomainSpec{
						ForProvider: v1alpha1.DomainParameters{
							Memory:  2147483648,
							Vcpu:    2,
							Running: true, // Should be running
						},
					},
				},
				memory: 2097152,
				vcpu:   2,
				state:  int32(libvirt.DomainShutoff), // But is shut off
				want:   false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if got := isUpToDate(tt.cr, tt.memory, tt.vcpu, tt.state); got != tt.want {
					t.Errorf("isUpToDate() = %v, want %v", got, tt.want)
				}
			})
		}
	})

	t.Run("getConnectionDetails", func(t *testing.T) {
		cr := &v1alpha1.Domain{
			Spec: v1alpha1.DomainSpec{
				ForProvider: v1alpha1.DomainParameters{
					Name: "test-domain",
				},
			},
		}
		domain := libvirt.Domain{
			ID:   123,
			Name: "test-domain",
			UUID: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		}

		cd := getConnectionDetails(cr, domain)

		if string(cd["domain-id"]) != "123" {
			t.Errorf("Expected domain-id = 123, got %s", string(cd["domain-id"]))
		}
		if string(cd["domain-name"]) != "test-domain" {
			t.Errorf("Expected domain-name = test-domain, got %s", string(cd["domain-name"]))
		}
		expectedUUID := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
		for i, b := range cd["domain-uuid"] {
			if b != expectedUUID[i] {
				t.Errorf("UUID mismatch at position %d: got %d, want %d", i, b, expectedUUID[i])
			}
		}
	})
}

// Helper function
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