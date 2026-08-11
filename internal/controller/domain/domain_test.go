package domain

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"

	"github.com/rossigee/provider-libvirt/apis/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"libvirt.org/go/libvirt"
)

type domainModifier func(*v1beta1.Domain)

// mockDomainClient implements DomainClient for testing
type mockDomainClient struct {
	lookupByNameFn func(name string) error
	defineXMLFn    func(xml string) error
	createFn       func(d interface{}) error
	setAutostartFn func(d interface{}, as int) error
	shutdownFn     func(d interface{}) error
	destroyFn      func(d interface{}) error
	undefineFn     func(d interface{}) error
	closeFn        func() error
	closeCalled    bool
}

func (m *mockDomainClient) DomainLookupByName(name string) (*libvirt.Domain, error) {
	if m.lookupByNameFn != nil {
		err := m.lookupByNameFn(name)
		if err != nil {
			return nil, err
		}
	}
	return &libvirt.Domain{}, nil
}

func (m *mockDomainClient) DomainDefineXML(xml string) (*libvirt.Domain, error) {
	if m.defineXMLFn != nil {
		err := m.defineXMLFn(xml)
		if err != nil {
			return nil, errors.Wrap(err, "cannot define domain")
		}
	}
	return &libvirt.Domain{}, nil
}

func (m *mockDomainClient) DomainCreate(d *libvirt.Domain) error {
	if m.createFn != nil {
		return errors.Wrap(m.createFn(d), "cannot start domain")
	}
	return nil
}

func (m *mockDomainClient) DomainSetAutostart(d *libvirt.Domain, as int) error {
	if m.setAutostartFn != nil {
		return errors.Wrap(m.setAutostartFn(d, as), "cannot set autostart")
	}
	return nil
}

func (m *mockDomainClient) DomainShutdown(d *libvirt.Domain) error {
	if m.shutdownFn != nil {
		return errors.Wrap(m.shutdownFn(d), "cannot shutdown domain")
	}
	return nil
}

func (m *mockDomainClient) DomainDestroy(d *libvirt.Domain) error {
	if m.destroyFn != nil {
		return errors.Wrap(m.destroyFn(d), "cannot destroy domain")
	}
	return nil
}

func (m *mockDomainClient) DomainUndefine(d *libvirt.Domain) error {
	if m.undefineFn != nil {
		return errors.Wrap(m.undefineFn(d), "cannot undefine domain")
	}
	return nil
}

func (m *mockDomainClient) Close() error {
	m.closeCalled = true
	if m.closeFn != nil {
		return m.closeFn()
	}
	return nil
}

func testDomain(m ...domainModifier) *v1beta1.Domain {
	d := &v1beta1.Domain{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Domain",
			APIVersion: "libvirt.m.crossplane.io/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-domain",
			Namespace: "default",
		},
		Spec: v1beta1.DomainSpec{
			ForProvider: v1beta1.DomainParameters{
				Name:    "test-vm",
				Memory:  1073741824, // 1GB
				Vcpu:    2,
				Type:    "kvm",
				Arch:    "x86_64",
				Running: boolPtr(true),
			},
		},
	}
	for _, f := range m {
		f(d)
	}
	return d
}

func boolPtr(b bool) *bool {
	return &b
}

func TestDisconnect(t *testing.T) {
	mock := &mockDomainClient{}
	e := &external{client: mock}

	if err := e.Disconnect(context.Background()); err != nil {
		t.Fatalf("Disconnect() returned unexpected error: %v", err)
	}
	if !mock.closeCalled {
		t.Error("Disconnect() did not close the underlying libvirt connection")
	}
}

func TestDisconnectPropagatesCloseError(t *testing.T) {
	wantErr := errors.New("boom")
	mock := &mockDomainClient{closeFn: func() error { return wantErr }}
	e := &external{client: mock}

	err := e.Disconnect(context.Background())
	if err == nil || err.Error() != wantErr.Error() {
		t.Errorf("Disconnect() = %v, want error %v", err, wantErr)
	}
}

func TestGenerateDomainXML(t *testing.T) {
	cases := map[string]struct {
		domain *v1beta1.Domain
		want   string
	}{
		"BasicDomain": {
			domain: testDomain(),
			want:   "test-vm",
		},
		"DomainWithDisk": {
			domain: testDomain(func(d *v1beta1.Domain) {
				d.Spec.ForProvider.Disk = []v1beta1.DomainDisk{
					{
						File:   "/var/lib/libvirt/images/disk.qcow2",
						Device: "vda",
						Type:   "virtio",
						Bus:    "virtio",
					},
				}
			}),
			want: "test-vm",
		},
		"DomainWithNetwork": {
			domain: testDomain(func(d *v1beta1.Domain) {
				d.Spec.ForProvider.NetworkInterface = []v1beta1.DomainNetworkInterface{
					{
						NetworkName: "default",
						Model:       "virtio",
					},
				}
			}),
			want: "test-vm",
		},
		"DomainWithNetworkAndVlan": {
			domain: testDomain(func(d *v1beta1.Domain) {
				d.Spec.ForProvider.NetworkInterface = []v1beta1.DomainNetworkInterface{
					{
						NetworkName: "default",
						Model:       "virtio",
						Vlan: &v1beta1.DomainInterfaceVlan{
							ID: 100,
						},
					},
				}
			}),
			want: "<tag id='100'/>",
		},
		"DomainWithNetworkAndVlanNativeMode": {
			domain: testDomain(func(d *v1beta1.Domain) {
				d.Spec.ForProvider.NetworkInterface = []v1beta1.DomainNetworkInterface{
					{
						NetworkName: "default",
						Model:       "virtio",
						Vlan: &v1beta1.DomainInterfaceVlan{
							ID:         200,
							NativeMode: "untagged",
						},
					},
				}
			}),
			want: "<tag id='200' nativeMode='untagged'/>",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ext := &external{client: nil}
			got, err := ext.generateDomainXML(context.Background(), tc.domain)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if !contains(got, tc.want) {
				t.Errorf("generateDomainXML(...): expected to contain %q, got:\n%s", tc.want, got)
			}
		})
	}
}

func TestFormatDomainState(t *testing.T) {
	cases := map[string]struct {
		state libvirt.DomainState
		want  string
	}{
		"Running": {
			state: libvirt.DOMAIN_RUNNING,
			want:  "running",
		},
		"Shutoff": {
			state: libvirt.DOMAIN_SHUTOFF,
			want:  "shutoff",
		},
		"Paused": {
			state: libvirt.DOMAIN_PAUSED,
			want:  "paused",
		},
		"Unknown": {
			state: libvirt.DomainState(999),
			want:  "unknown",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := formatDomainState(tc.state)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("formatDomainState(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestDomainParameters(t *testing.T) {
	cases := map[string]struct {
		domain *v1beta1.Domain
		want   interface{}
	}{
		"NameParameter": {
			domain: testDomain(),
			want:   "test-vm",
		},
		"MemoryParameter": {
			domain: testDomain(),
			want:   int64(1073741824),
		},
		"VcpuParameter": {
			domain: testDomain(),
			want:   int32(2),
		},
		"TypeParameter": {
			domain: testDomain(),
			want:   "kvm",
		},
		"ArchParameter": {
			domain: testDomain(),
			want:   "x86_64",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			params := tc.domain.Spec.ForProvider
			if name == "NameParameter" && params.Name != tc.want {
				t.Errorf("Expected name %v, got %v", tc.want, params.Name)
			}
			if name == "MemoryParameter" && params.Memory != tc.want {
				t.Errorf("Expected memory %v, got %v", tc.want, params.Memory)
			}
			if name == "VcpuParameter" && params.Vcpu != tc.want {
				t.Errorf("Expected vcpu %v, got %v", tc.want, params.Vcpu)
			}
			if name == "TypeParameter" && params.Type != tc.want {
				t.Errorf("Expected type %v, got %v", tc.want, params.Type)
			}
			if name == "ArchParameter" && params.Arch != tc.want {
				t.Errorf("Expected arch %v, got %v", tc.want, params.Arch)
			}
		})
	}
}

func TestBoolToInt(t *testing.T) {
	cases := map[string]struct {
		input bool
		want  int
	}{
		"True": {
			input: true,
			want:  1,
		},
		"False": {
			input: false,
			want:  0,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := boolToInt(tc.input)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("boolToInt(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func contains(s, substr string) bool {
	for i := 0; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestGenerateDomainXMLDefaultType(t *testing.T) {
	ext := &external{client: nil}
	domain := testDomain(func(d *v1beta1.Domain) {
		d.Spec.ForProvider.Type = ""
	})

	xml, err := ext.generateDomainXML(context.Background(), domain)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}
	if !contains(xml, "type='kvm'") {
		t.Errorf("Expected default type 'kvm' in XML, got:\n%s", xml)
	}
}

func TestGenerateDomainXMLCustomType(t *testing.T) {
	ext := &external{client: nil}
	domain := testDomain(func(d *v1beta1.Domain) {
		d.Spec.ForProvider.Type = "qemu"
	})

	xml, err := ext.generateDomainXML(context.Background(), domain)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}
	if !contains(xml, "type='qemu'") {
		t.Errorf("Expected type 'qemu' in XML, got:\n%s", xml)
	}
}

func TestGenerateDomainXMLDefaultArch(t *testing.T) {
	ext := &external{client: nil}
	domain := testDomain(func(d *v1beta1.Domain) {
		d.Spec.ForProvider.Arch = ""
	})

	xml, err := ext.generateDomainXML(context.Background(), domain)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}
	if !contains(xml, "arch='x86_64'") {
		t.Errorf("Expected default arch 'x86_64' in XML, got:\n%s", xml)
	}
}

func TestGenerateDomainXMLCustomArch(t *testing.T) {
	ext := &external{client: nil}
	domain := testDomain(func(d *v1beta1.Domain) {
		d.Spec.ForProvider.Arch = "aarch64"
	})

	xml, err := ext.generateDomainXML(context.Background(), domain)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}
	if !contains(xml, "arch='aarch64'") {
		t.Errorf("Expected arch 'aarch64' in XML, got:\n%s", xml)
	}
}

func TestGenerateDomainXMLMemory(t *testing.T) {
	ext := &external{client: nil}
	cases := map[string]int64{
		"1GB":   1073741824,
		"2GB":   2147483648,
		"512MB": 536870912,
	}

	for name, mem := range cases {
		t.Run(name, func(t *testing.T) {
			domain := testDomain(func(d *v1beta1.Domain) {
				d.Spec.ForProvider.Memory = mem
			})
			xml, err := ext.generateDomainXML(context.Background(), domain)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if !contains(xml, memStr(mem)) {
				t.Errorf("Expected memory %d in XML", mem)
			}
		})
	}
}

func memStr(m int64) string {
	return fmt.Sprintf("%d", m)
}

func TestGenerateDomainXMLVCPU(t *testing.T) {
	ext := &external{client: nil}
	cases := map[string]int32{
		"1CPU": 1,
		"2CPU": 2,
		"4CPU": 4,
		"8CPU": 8,
	}

	for name, cpu := range cases {
		t.Run(name, func(t *testing.T) {
			domain := testDomain(func(d *v1beta1.Domain) {
				d.Spec.ForProvider.Vcpu = cpu
			})
			xml, err := ext.generateDomainXML(context.Background(), domain)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if !contains(xml, fmt.Sprintf("%d", cpu)) {
				t.Errorf("Expected vcpu %d in XML", cpu)
			}
		})
	}
}

func TestGenerateDomainXMLConsole(t *testing.T) {
	ext := &external{client: nil}
	domain := testDomain(func(d *v1beta1.Domain) {
		d.Spec.ForProvider.Console = []v1beta1.DomainConsole{
			{Type: "pty"},
		}
	})

	xml, err := ext.generateDomainXML(context.Background(), domain)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}
	if !contains(xml, "console") {
		t.Errorf("Expected console in XML, got:\n%s", xml)
	}
}

func TestGenerateDomainXMLGraphics(t *testing.T) {
	ext := &external{client: nil}
	port := int32(5900)
	domain := testDomain(func(d *v1beta1.Domain) {
		d.Spec.ForProvider.Graphics = []v1beta1.DomainGraphics{
			{Type: "vnc", Port: &port},
		}
	})

	xml, err := ext.generateDomainXML(context.Background(), domain)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}
	if !contains(xml, "graphics") {
		t.Errorf("Expected graphics in XML, got:\n%s", xml)
	}
}

func TestGenerateDomainXMLMultipleDiskks(t *testing.T) {
	ext := &external{client: nil}
	domain := testDomain(func(d *v1beta1.Domain) {
		d.Spec.ForProvider.Disk = []v1beta1.DomainDisk{
			{File: "/var/lib/libvirt/images/disk1.qcow2", Device: "vda"},
			{File: "/var/lib/libvirt/images/disk2.qcow2", Device: "vdb"},
		}
	})

	xml, err := ext.generateDomainXML(context.Background(), domain)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}
	if !contains(xml, "disk1.qcow2") || !contains(xml, "disk2.qcow2") {
		t.Errorf("Expected both disks in XML, got:\n%s", xml)
	}
}

func TestGenerateDomainXMLDiskAttributes(t *testing.T) {
	ext := &external{client: nil}
	domain := testDomain(func(d *v1beta1.Domain) {
		d.Spec.ForProvider.Disk = []v1beta1.DomainDisk{
			{
				File:   "/var/lib/libvirt/images/disk.qcow2",
				Device: "vda",
				Type:   "virtio",
				Bus:    "virtio",
			},
		}
	})

	xml, err := ext.generateDomainXML(context.Background(), domain)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}
	if !contains(xml, "virtio") {
		t.Errorf("Expected disk type virtio in XML, got:\n%s", xml)
	}
}

func TestFormatDomainStateAllValues(t *testing.T) {
	cases := map[string]struct {
		state libvirt.DomainState
		want  string
	}{
		"NoState": {
			state: libvirt.DOMAIN_NOSTATE,
			want:  "nostate",
		},
		"Running": {
			state: libvirt.DOMAIN_RUNNING,
			want:  "running",
		},
		"Blocked": {
			state: libvirt.DOMAIN_BLOCKED,
			want:  "blocked",
		},
		"Paused": {
			state: libvirt.DOMAIN_PAUSED,
			want:  "paused",
		},
		"Shutdown": {
			state: libvirt.DOMAIN_SHUTDOWN,
			want:  "shutdown",
		},
		"Shutoff": {
			state: libvirt.DOMAIN_SHUTOFF,
			want:  "shutoff",
		},
		"Crashed": {
			state: libvirt.DOMAIN_CRASHED,
			want:  "crashed",
		},
		"Pmsuspended": {
			state: libvirt.DOMAIN_PMSUSPENDED,
			want:  "pmsuspended",
		},
		"Unknown": {
			state: libvirt.DomainState(999),
			want:  "unknown",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := formatDomainState(tc.state)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("formatDomainState(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestBoolToIntBothValues(t *testing.T) {
	cases := map[string]struct {
		input bool
		want  int
	}{
		"TrueToOne": {
			input: true,
			want:  1,
		},
		"FalseToZero": {
			input: false,
			want:  0,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := boolToInt(tc.input)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("boolToInt(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestGenerateDomainXMLMinimalConfig(t *testing.T) {
	ext := &external{client: nil}
	domain := testDomain(func(d *v1beta1.Domain) {
		d.Spec.ForProvider.Disk = nil
		d.Spec.ForProvider.NetworkInterface = nil
		d.Spec.ForProvider.Console = nil
		d.Spec.ForProvider.Graphics = nil
	})

	xml, err := ext.generateDomainXML(context.Background(), domain)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}
	if !contains(xml, "test-vm") || !contains(xml, "kvm") {
		t.Errorf("Expected minimal domain XML, got:\n%s", xml)
	}
}

func TestGenerateDomainXMLComplexConfig(t *testing.T) {
	ext := &external{client: nil}
	port := int32(5900)
	domain := testDomain(func(d *v1beta1.Domain) {
		d.Spec.ForProvider.Disk = []v1beta1.DomainDisk{
			{File: "/var/lib/libvirt/images/root.qcow2", Device: "vda", Type: "virtio", Bus: "virtio"},
			{File: "/var/lib/libvirt/images/data.qcow2", Device: "vdb", Type: "virtio", Bus: "virtio"},
		}
		d.Spec.ForProvider.NetworkInterface = []v1beta1.DomainNetworkInterface{
			{NetworkName: "default", Model: "virtio"},
			{NetworkName: "bridge0", Model: "e1000"},
		}
		d.Spec.ForProvider.Console = []v1beta1.DomainConsole{{Type: "pty"}}
		d.Spec.ForProvider.Graphics = []v1beta1.DomainGraphics{{Type: "vnc", Port: &port}}
	})

	xml, err := ext.generateDomainXML(context.Background(), domain)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}
	if !contains(xml, "root.qcow2") || !contains(xml, "data.qcow2") {
		t.Errorf("Expected all disks in XML")
	}
	if !contains(xml, "default") || !contains(xml, "bridge0") {
		t.Errorf("Expected all networks in XML")
	}
}
