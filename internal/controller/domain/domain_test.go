package domain

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
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

func TestGenerateDomainXMLDefaultType(t *testing.T) {
	ext := &external{client: nil}
	domain := testDomain(func(d *v1beta1.Domain) {
		d.Spec.ForProvider.Type = ""
	})

	xml := ext.generateDomainXML(domain)
	if !contains(xml, "type='kvm'") {
		t.Errorf("Expected default type 'kvm' in XML, got:\n%s", xml)
	}
}

func TestGenerateDomainXMLCustomType(t *testing.T) {
	ext := &external{client: nil}
	domain := testDomain(func(d *v1beta1.Domain) {
		d.Spec.ForProvider.Type = "qemu"
	})

	xml := ext.generateDomainXML(domain)
	if !contains(xml, "type='qemu'") {
		t.Errorf("Expected type 'qemu' in XML, got:\n%s", xml)
	}
}

func TestGenerateDomainXMLDefaultArch(t *testing.T) {
	ext := &external{client: nil}
	domain := testDomain(func(d *v1beta1.Domain) {
		d.Spec.ForProvider.Arch = ""
	})

	xml := ext.generateDomainXML(domain)
	if !contains(xml, "arch='x86_64'") {
		t.Errorf("Expected default arch 'x86_64' in XML, got:\n%s", xml)
	}
}

func TestGenerateDomainXMLCustomArch(t *testing.T) {
	ext := &external{client: nil}
	domain := testDomain(func(d *v1beta1.Domain) {
		d.Spec.ForProvider.Arch = "aarch64"
	})

	xml := ext.generateDomainXML(domain)
	if !contains(xml, "arch='aarch64'") {
		t.Errorf("Expected arch 'aarch64' in XML, got:\n%s", xml)
	}
}

func TestGenerateDomainXMLMemory(t *testing.T) {
	ext := &external{client: nil}
	cases := map[string]int64{
		"1GB":  1073741824,
		"2GB":  2147483648,
		"512MB": 536870912,
	}

	for name, mem := range cases {
		t.Run(name, func(t *testing.T) {
			domain := testDomain(func(d *v1beta1.Domain) {
				d.Spec.ForProvider.Memory = mem
			})
			xml := ext.generateDomainXML(domain)
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
		"1CPU":  1,
		"2CPU":  2,
		"4CPU":  4,
		"8CPU":  8,
	}

	for name, cpu := range cases {
		t.Run(name, func(t *testing.T) {
			domain := testDomain(func(d *v1beta1.Domain) {
				d.Spec.ForProvider.Vcpu = cpu
			})
			xml := ext.generateDomainXML(domain)
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

	xml := ext.generateDomainXML(domain)
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

	xml := ext.generateDomainXML(domain)
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

	xml := ext.generateDomainXML(domain)
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

	xml := ext.generateDomainXML(domain)
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
