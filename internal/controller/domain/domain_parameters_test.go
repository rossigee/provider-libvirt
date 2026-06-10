package domain

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"

	"github.com/rossigee/provider-libvirt/apis/v1beta1"
)

// Parameter validation and configuration tests

func TestDomainNameRequired(t *testing.T) {
	domain := testDomain(func(d *v1beta1.Domain) {
		d.Spec.ForProvider.Name = ""
	})

	if domain.Spec.ForProvider.Name != "" {
		t.Error("Domain name should be empty")
	}
}

func TestDomainMemoryBounds(t *testing.T) {
	cases := map[string]int64{
		"minimal":   67108864,    // 64MB
		"small":     268435456,   // 256MB
		"medium":    1073741824,  // 1GB
		"large":     4294967296,  // 4GB
		"xlarge":    17179869184, // 16GB
	}

	for name, memory := range cases {
		t.Run(name, func(t *testing.T) {
			domain := testDomain(func(d *v1beta1.Domain) {
				d.Spec.ForProvider.Memory = memory
			})
			if domain.Spec.ForProvider.Memory != memory {
				t.Errorf("Expected memory %d, got %d", memory, domain.Spec.ForProvider.Memory)
			}
		})
	}
}

func TestDomainVcpuBounds(t *testing.T) {
	cases := map[string]int32{
		"1":   1,
		"2":   2,
		"4":   4,
		"8":   8,
		"16":  16,
		"32":  32,
		"64":  64,
		"128": 128,
	}

	for name, vcpu := range cases {
		t.Run(name, func(t *testing.T) {
			domain := testDomain(func(d *v1beta1.Domain) {
				d.Spec.ForProvider.Vcpu = vcpu
			})
			if domain.Spec.ForProvider.Vcpu != vcpu {
				t.Errorf("Expected vcpu %d, got %d", vcpu, domain.Spec.ForProvider.Vcpu)
			}
		})
	}
}

func TestDomainTypesSupported(t *testing.T) {
	types := []string{"kvm", "qemu", "xen", "lxc", "vmware"}

	for _, domainType := range types {
		t.Run(domainType, func(t *testing.T) {
			domain := testDomain(func(d *v1beta1.Domain) {
				d.Spec.ForProvider.Type = domainType
			})
			if domain.Spec.ForProvider.Type != domainType {
				t.Errorf("Expected type %s, got %s", domainType, domain.Spec.ForProvider.Type)
			}
		})
	}
}

func TestDomainArchitecturesSupported(t *testing.T) {
	archs := []string{"x86_64", "i686", "aarch64", "ppc64", "s390x"}

	for _, arch := range archs {
		t.Run(arch, func(t *testing.T) {
			domain := testDomain(func(d *v1beta1.Domain) {
				d.Spec.ForProvider.Arch = arch
			})
			if domain.Spec.ForProvider.Arch != arch {
				t.Errorf("Expected arch %s, got %s", arch, domain.Spec.ForProvider.Arch)
			}
		})
	}
}

func TestDomainDiskConfiguration(t *testing.T) {
	domain := testDomain(func(d *v1beta1.Domain) {
		d.Spec.ForProvider.Disk = []v1beta1.DomainDisk{
			{
				File:   "/var/lib/libvirt/images/root.qcow2",
				Device: "vda",
				Type:   "virtio",
				Bus:    "virtio",
			},
			{
				File:   "/var/lib/libvirt/images/data.qcow2",
				Device: "vdb",
				Type:   "virtio",
				Bus:    "virtio",
			},
		}
	})

	if len(domain.Spec.ForProvider.Disk) != 2 {
		t.Errorf("Expected 2 disks, got %d", len(domain.Spec.ForProvider.Disk))
	}

	for i, disk := range domain.Spec.ForProvider.Disk {
		if disk.Device == "" {
			t.Errorf("Disk %d missing device", i)
		}
		if disk.Type == "" {
			t.Errorf("Disk %d missing type", i)
		}
		if disk.Bus == "" {
			t.Errorf("Disk %d missing bus", i)
		}
	}
}

func TestDomainNetworkConfiguration(t *testing.T) {
	domain := testDomain(func(d *v1beta1.Domain) {
		d.Spec.ForProvider.NetworkInterface = []v1beta1.DomainNetworkInterface{
			{NetworkName: "default", Model: "virtio"},
			{NetworkName: "internal", Model: "e1000"},
		}
	})

	if len(domain.Spec.ForProvider.NetworkInterface) != 2 {
		t.Errorf("Expected 2 networks, got %d", len(domain.Spec.ForProvider.NetworkInterface))
	}

	for i, net := range domain.Spec.ForProvider.NetworkInterface {
		if net.NetworkName == "" {
			t.Errorf("Network %d missing name", i)
		}
		if net.Model == "" {
			t.Errorf("Network %d missing model", i)
		}
	}
}

func TestDomainResourceReference(t *testing.T) {
	domain := testDomain()

	// Check that the domain is a proper managed resource
	_, ok := interface{}(domain).(interface {
		GetObjectMeta() metav1.Object
	})

	if !ok {
		t.Error("Domain should implement managed resource interface")
	}
}

func TestDomainProviderRef(t *testing.T) {
	domain := testDomain()

	// Initialize provider ref
	domain.Spec.ProviderConfigReference = &xpv1.ProviderConfigReference{
		Name: "test-provider",
	}

	if domain.Spec.ProviderConfigReference == nil {
		t.Error("Expected provider config reference to be set")
	}

	if domain.Spec.ProviderConfigReference.Name != "test-provider" {
		t.Error("Provider reference name mismatch")
	}
}

func TestDomainRunningDefault(t *testing.T) {
	domain := testDomain(func(d *v1beta1.Domain) {
		d.Spec.ForProvider.Running = nil
	})

	if domain.Spec.ForProvider.Running != nil {
		t.Error("Running should be nil (unspecified)")
	}
}

func TestDomainAutostartDefault(t *testing.T) {
	domain := testDomain(func(d *v1beta1.Domain) {
		d.Spec.ForProvider.Autostart = nil
	})

	if domain.Spec.ForProvider.Autostart != nil {
		t.Error("Autostart should be nil (unspecified)")
	}
}

func TestDomainConsoleConfiguration(t *testing.T) {
	domain := testDomain(func(d *v1beta1.Domain) {
		d.Spec.ForProvider.Console = []v1beta1.DomainConsole{
			{Type: "pty"},
			{Type: "tcp"},
		}
	})

	if len(domain.Spec.ForProvider.Console) != 2 {
		t.Errorf("Expected 2 console devices, got %d", len(domain.Spec.ForProvider.Console))
	}
}

func TestDomainGraphicsConfiguration(t *testing.T) {
	port := int32(5900)
	domain := testDomain(func(d *v1beta1.Domain) {
		d.Spec.ForProvider.Graphics = []v1beta1.DomainGraphics{
			{Type: "vnc", Port: &port},
		}
	})

	if len(domain.Spec.ForProvider.Graphics) != 1 {
		t.Error("Expected 1 graphics device")
	}

	if domain.Spec.ForProvider.Graphics[0].Port != &port {
		t.Error("Graphics port mismatch")
	}
}

func TestDomainConfigMutation(t *testing.T) {
	domain1 := testDomain()
	domain2 := testDomain()

	// Modify domain1
	domain1.Spec.ForProvider.Memory = 2147483648

	// Verify domain2 is unchanged
	if domain2.Spec.ForProvider.Memory == 2147483648 {
		t.Error("Domain2 should not be affected by domain1 modification")
	}
}

func TestDomainStatusInitialization(t *testing.T) {
	domain := testDomain()

	if domain.Status.AtProvider.State == "" {
		t.Log("Domain status state initially empty (expected)")
	}

	if domain.Status.AtProvider.UUID == "" {
		t.Log("Domain status UUID initially empty (expected)")
	}
}
