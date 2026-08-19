package domain

import "testing"

// TestDisableRootPortHotplug is a regression test for a QEMU/libvirt package
// regression (installed 2026-08-06 on the itx hypervisors) where the ACPI
// D3cold->D0 power transition for a hotplug-managed pcie-root-port randomly
// fails during guest boot, leaving the virtio-pci device behind it
// (sometimes the NIC) inaccessible to the guest kernel. Disabling hotplug on
// libvirt's auto-generated root ports removes the race; verified via 4
// reproducible before/after tests against live runner VMs on 2026-08-19.
func TestDisableRootPortHotplug(t *testing.T) {
	input := `<domain type='kvm'>
  <devices>
    <controller type='pci' index='1' model='pcie-root-port'>
      <model name='pcie-root-port'/>
      <target chassis='1' port='0x10'/>
      <alias name='pci.1'/>
    </controller>
    <controller type='pci' index='2' model='pcie-root-port'>
      <model name='pcie-root-port'/>
      <target chassis='2' port='0x11'/>
      <alias name='pci.2'/>
    </controller>
  </devices>
</domain>`

	want := `<domain type='kvm'>
  <devices>
    <controller type='pci' index='1' model='pcie-root-port'>
      <model name='pcie-root-port'/>
      <target chassis='1' port='0x10' hotplug='off'/>
      <alias name='pci.1'/>
    </controller>
    <controller type='pci' index='2' model='pcie-root-port'>
      <model name='pcie-root-port'/>
      <target chassis='2' port='0x11' hotplug='off'/>
      <alias name='pci.2'/>
    </controller>
  </devices>
</domain>`

	got := disableRootPortHotplug(input)
	if got != want {
		t.Errorf("disableRootPortHotplug mismatch:\ngot:  %s\nwant: %s", got, want)
	}
}

func TestDisableRootPortHotplugNoop(t *testing.T) {
	// No pcie-root-port targets present (e.g. non-q35 machine type) -> unchanged.
	input := `<domain type='kvm'><devices><disk type='disk'/></devices></domain>`
	if got := disableRootPortHotplug(input); got != input {
		t.Errorf("expected no-op for XML without pcie-root-port targets, got: %s", got)
	}
}

func TestDisableRootPortHotplugIdempotent(t *testing.T) {
	// Re-running against already-patched XML must not duplicate the attribute.
	once := disableRootPortHotplug(`<target chassis='1' port='0x10'/>`)
	twice := disableRootPortHotplug(once)
	if once != `<target chassis='1' port='0x10' hotplug='off'/>` {
		t.Fatalf("unexpected first pass result: %s", once)
	}
	if twice != once {
		t.Errorf("expected idempotent result, first=%q second=%q", once, twice)
	}
}
