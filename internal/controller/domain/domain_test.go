package domain

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"libvirt.org/go/libvirt"

	"github.com/rossigee/provider-libvirt/apis/v1beta1"
)

type domainModifier func(*v1beta1.Domain)

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

type mockLibvirtDomain struct {
	mockLookupByName       func(name string) (*libvirt.Domain, error)
	mockCreate             func(domain *libvirt.Domain) error
	mockDestroy            func(domain *libvirt.Domain) error
	mockShutdown           func(domain *libvirt.Domain) error
	mockUndefine           func(domain *libvirt.Domain) error
	mockDefineXML          func(xml string) (*libvirt.Domain, error)
	mockDomainSetAutostart func(domain *libvirt.Domain, autostart int) error
}

func (m *mockLibvirtDomain) DomainLookupByName(name string) (*libvirt.Domain, error) {
	if m.mockLookupByName != nil {
		return m.mockLookupByName(name)
	}
	return nil, errors.New("Domain not found")
}

func (m *mockLibvirtDomain) DomainCreate(domain *libvirt.Domain) error {
	if m.mockCreate != nil {
		return m.mockCreate(domain)
	}
	return nil
}

func (m *mockLibvirtDomain) DomainDestroy(domain *libvirt.Domain) error {
	if m.mockDestroy != nil {
		return m.mockDestroy(domain)
	}
	return nil
}

func (m *mockLibvirtDomain) DomainShutdown(domain *libvirt.Domain) error {
	if m.mockShutdown != nil {
		return m.mockShutdown(domain)
	}
	return nil
}

func (m *mockLibvirtDomain) DomainUndefine(domain *libvirt.Domain) error {
	if m.mockUndefine != nil {
		return m.mockUndefine(domain)
	}
	return nil
}

func (m *mockLibvirtDomain) DomainDefineXML(xml string) (*libvirt.Domain, error) {
	if m.mockDefineXML != nil {
		return m.mockDefineXML(xml)
	}
	return nil, nil
}

func (m *mockLibvirtDomain) DomainSetAutostart(domain *libvirt.Domain, autostart int) error {
	if m.mockDomainSetAutostart != nil {
		return m.mockDomainSetAutostart(domain, autostart)
	}
	return nil
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
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ext := &external{client: nil}
			got := ext.generateDomainXML(tc.domain)
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
