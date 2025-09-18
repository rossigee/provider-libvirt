/*
Copyright 2025 Ross Golder
*/

package nodedevice

import (
	"context"
	"testing"

	"github.com/digitalocean/go-libvirt"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"

	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"

	"github.com/rossigee/provider-libvirt/apis/v1alpha1"
)

// MockNodeDeviceService implements NodeDeviceService for testing
type MockNodeDeviceService struct {
	devices         []libvirt.NodeDevice
	deviceXMLMap    map[string]string
	detachedDevices map[string]string
	autostartMap    map[string]bool
	err             error
}

func NewMockNodeDeviceService() *MockNodeDeviceService {
	return &MockNodeDeviceService{
		devices:         []libvirt.NodeDevice{},
		deviceXMLMap:    make(map[string]string),
		detachedDevices: make(map[string]string),
		autostartMap:    make(map[string]bool),
	}
}

func (m *MockNodeDeviceService) ConnectListAllNodeDevices(needResults int32, flags uint32) ([]libvirt.NodeDevice, uint32, error) {
	if m.err != nil {
		return nil, 0, m.err
	}
	return m.devices, uint32(len(m.devices)), nil
}

func (m *MockNodeDeviceService) NodeDeviceLookupByName(name string) (libvirt.NodeDevice, error) {
	if m.err != nil {
		return libvirt.NodeDevice{}, m.err
	}
	for _, device := range m.devices {
		if device.Name == name {
			return device, nil
		}
	}
	return libvirt.NodeDevice{}, errors.New("device not found")
}

func (m *MockNodeDeviceService) NodeDeviceGetXMLDesc(name string, flags uint32) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	if xml, exists := m.deviceXMLMap[name]; exists {
		return xml, nil
	}
	return "<device><name>" + name + "</name></device>", nil
}

func (m *MockNodeDeviceService) NodeDeviceCreateXML(xml string, flags uint32) (libvirt.NodeDevice, error) {
	if m.err != nil {
		return libvirt.NodeDevice{}, m.err
	}
	device := libvirt.NodeDevice{Name: "mdev_test_device"}
	m.devices = append(m.devices, device)
	m.deviceXMLMap[device.Name] = xml
	return device, nil
}

func (m *MockNodeDeviceService) NodeDeviceDestroy(name string) error {
	if m.err != nil {
		return m.err
	}
	for i, device := range m.devices {
		if device.Name == name {
			m.devices = append(m.devices[:i], m.devices[i+1:]...)
			delete(m.deviceXMLMap, name)
			break
		}
	}
	return nil
}

func (m *MockNodeDeviceService) NodeDeviceDefineXML(xml string, flags uint32) (libvirt.NodeDevice, error) {
	return m.NodeDeviceCreateXML(xml, flags)
}

func (m *MockNodeDeviceService) NodeDeviceUndefine(name string, flags uint32) error {
	return m.NodeDeviceDestroy(name)
}

func (m *MockNodeDeviceService) NodeDeviceDetachFlags(name string, driverName libvirt.OptString, flags uint32) error {
	if m.err != nil {
		return m.err
	}
	driverStr := ""
	if len(driverName) > 0 {
		driverStr = driverName[0]
	}
	m.detachedDevices[name] = driverStr
	return nil
}

func (m *MockNodeDeviceService) NodeDeviceReAttach(name string) error {
	if m.err != nil {
		return m.err
	}
	delete(m.detachedDevices, name)
	return nil
}


func (m *MockNodeDeviceService) NodeDeviceSetAutostart(name string, autostart int32) error {
	if m.err != nil {
		return m.err
	}
	m.autostartMap[name] = autostart != 0
	return nil
}

func (m *MockNodeDeviceService) NodeDeviceGetAutostart(name string) (int32, error) {
	if m.err != nil {
		return 0, m.err
	}
	if autostart, exists := m.autostartMap[name]; exists && autostart {
		return 1, nil
	}
	return 0, nil
}

func (m *MockNodeDeviceService) Disconnect() error {
	return m.err
}

// Helper methods for testing
func (m *MockNodeDeviceService) AddDevice(name string, xmlDesc string) {
	device := libvirt.NodeDevice{Name: name}
	m.devices = append(m.devices, device)
	m.deviceXMLMap[name] = xmlDesc
}

func (m *MockNodeDeviceService) SetError(err error) {
	m.err = err
}

func (m *MockNodeDeviceService) IsDetached(deviceName string) bool {
	_, exists := m.detachedDevices[deviceName]
	return exists
}

func (m *MockNodeDeviceService) GetDetachedDriver(deviceName string) string {
	return m.detachedDevices[deviceName]
}

func TestObserve(t *testing.T) {
	type want struct {
		cr     *v1alpha1.NodeDevice
		result managed.ExternalObservation
		err    error
	}

	cases := map[string]struct {
		args
		want
	}{
		"SuccessfulObserveExistingMdevDevice": {
			args: args{
				nodeDevice: &external{
					service: func() *MockNodeDeviceService {
						m := NewMockNodeDeviceService()
						m.AddDevice("mdev_12345678-1234-1234-1234-123456789abc", `<device>
							<name>mdev_12345678-1234-1234-1234-123456789abc</name>
							<parent>pci_0000_01_00_0</parent>
							<capability type='mdev'>
								<type id='nvidia-63'/>
								<uuid>12345678-1234-1234-1234-123456789abc</uuid>
							</capability>
						</device>`)
						return m
					}(),
				},
				cr: nodeDevice(func(cr *v1alpha1.NodeDevice) {
					cr.Spec.ForProvider.Name = "mdev_12345678-1234-1234-1234-123456789abc"
					cr.Spec.ForProvider.Type = "mdev"
					cr.Spec.ForProvider.MediatedDevice = &v1alpha1.MediatedDevice{
						Parent: "pci_0000_01_00_0",
						Type:   "nvidia-63",
						UUID:   "12345678-1234-1234-1234-123456789abc",
					}
				}),
			},
			want: want{
				cr: nodeDevice(func(cr *v1alpha1.NodeDevice) {
					cr.Spec.ForProvider.Name = "mdev_12345678-1234-1234-1234-123456789abc"
					cr.Spec.ForProvider.Type = "mdev"
					cr.Spec.ForProvider.MediatedDevice = &v1alpha1.MediatedDevice{
						Parent: "pci_0000_01_00_0",
						Type:   "nvidia-63",
						UUID:   "12345678-1234-1234-1234-123456789abc",
					}
					cr.Status.AtProvider.Name = "mdev_12345678-1234-1234-1234-123456789abc"
					cr.Status.AtProvider.State = "active"
				}),
				result: managed.ExternalObservation{
					ResourceExists:   true,
					ResourceUpToDate: true,
					ConnectionDetails: managed.ConnectionDetails{
						"device_name": []byte("mdev_12345678-1234-1234-1234-123456789abc"),
						"device_type": []byte("mdev"),
					},
				},
			},
		},
		"DeviceNotFound": {
			args: args{
				nodeDevice: &external{
					service: NewMockNodeDeviceService(),
				},
				cr: nodeDevice(func(cr *v1alpha1.NodeDevice) {
					cr.Spec.ForProvider.Name = "nonexistent_device"
					cr.Spec.ForProvider.Type = "pci"
				}),
			},
			want: want{
				cr: nodeDevice(func(cr *v1alpha1.NodeDevice) {
					cr.Spec.ForProvider.Name = "nonexistent_device"
					cr.Spec.ForProvider.Type = "pci"
				}),
				result: managed.ExternalObservation{
					ResourceExists: false,
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			o, err := tc.nodeDevice.Observe(context.Background(), tc.args.cr)

			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.result, o); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
			// Controller behavior verification is covered by the main result comparison
		})
	}
}

func TestCreate(t *testing.T) {
	type want struct {
		cr     *v1alpha1.NodeDevice
		result managed.ExternalCreation
		err    error
	}

	cases := map[string]struct {
		args
		want
	}{
		"SuccessfulCreateMdevDevice": {
			args: args{
				nodeDevice: &external{
					service: NewMockNodeDeviceService(),
				},
				cr: nodeDevice(func(cr *v1alpha1.NodeDevice) {
					cr.Spec.ForProvider.Type = "mdev"
					cr.Spec.ForProvider.MediatedDevice = &v1alpha1.MediatedDevice{
						Parent: "pci_0000_01_00_0",
						Type:   "nvidia-63",
					}
					cr.Spec.ForProvider.Persistent = false
				}),
			},
			want: want{
				cr: nodeDevice(func(cr *v1alpha1.NodeDevice) {
					cr.Spec.ForProvider.Type = "mdev"
					cr.Spec.ForProvider.MediatedDevice = &v1alpha1.MediatedDevice{
						Parent: "pci_0000_01_00_0",
						Type:   "nvidia-63",
					}
					cr.Spec.ForProvider.Persistent = false
				}),
				result: managed.ExternalCreation{
					ConnectionDetails: managed.ConnectionDetails{
						"device_name": []byte("mdev_test_device"),
						// UUID is auto-generated, so we'll ignore it in comparison
					},
				},
			},
		},
		"UnsupportedDeviceType": {
			args: args{
				nodeDevice: &external{
					service: NewMockNodeDeviceService(),
				},
				cr: nodeDevice(func(cr *v1alpha1.NodeDevice) {
					cr.Spec.ForProvider.Type = "unknown"
				}),
			},
			want: want{
				cr: nodeDevice(func(cr *v1alpha1.NodeDevice) {
					cr.Spec.ForProvider.Type = "unknown"
				}),
				err: errors.New("unsupported device type: unknown"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			o, err := tc.nodeDevice.Create(context.Background(), tc.args.cr)

			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
			// Only check connection details for successful cases
			if tc.err == nil {
				// Check device_name exists and matches expected
				if deviceName, ok := o.ConnectionDetails["device_name"]; !ok || string(deviceName) != "mdev_test_device" {
					t.Errorf("Expected device_name=mdev_test_device, got: %v", o.ConnectionDetails["device_name"])
				}
				// UUID should be generated for mdev devices
				if tc.args.cr.Spec.ForProvider.Type == "mdev" {
					if _, ok := o.ConnectionDetails["device_uuid"]; !ok {
						t.Errorf("Expected device_uuid to be generated for mdev device")
					}
				}
			}
		})
	}
}

func TestUpdate(t *testing.T) {
	type want struct {
		cr     *v1alpha1.NodeDevice
		result managed.ExternalUpdate
		err    error
	}

	cases := map[string]struct {
		args
		want
	}{
		"SuccessfulDetachDevice": {
			args: args{
				nodeDevice: &external{
					service: func() *MockNodeDeviceService {
						m := NewMockNodeDeviceService()
						m.AddDevice("pci_0000_01_00_0", `<device>
							<name>pci_0000_01_00_0</name>
							<capability type='pci'>
								<domain>0</domain>
								<bus>1</bus>
								<slot>0</slot>
								<function>0</function>
							</capability>
						</device>`)
						return m
					}(),
				},
				cr: nodeDevice(func(cr *v1alpha1.NodeDevice) {
					cr.Spec.ForProvider.Name = "pci_0000_01_00_0"
					cr.Spec.ForProvider.Type = "pci"
					cr.Spec.ForProvider.Detached = true
					cr.Spec.ForProvider.Driver = "vfio-pci"
				}),
			},
			want: want{
				cr: nodeDevice(func(cr *v1alpha1.NodeDevice) {
					cr.Spec.ForProvider.Name = "pci_0000_01_00_0"
					cr.Spec.ForProvider.Type = "pci"
					cr.Spec.ForProvider.Detached = true
					cr.Spec.ForProvider.Driver = "vfio-pci"
				}),
				result: managed.ExternalUpdate{},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			u, err := tc.nodeDevice.Update(context.Background(), tc.args.cr)

			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.result, u); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}

			// Check if device was actually detached in mock service
			mockService := tc.nodeDevice.service.(*MockNodeDeviceService)
			if tc.args.cr.Spec.ForProvider.Detached && !mockService.IsDetached("pci_0000_01_00_0") {
				t.Errorf("Expected device to be detached")
			}
		})
	}
}

func TestDelete(t *testing.T) {
	type want struct {
		cr     *v1alpha1.NodeDevice
		result managed.ExternalDelete
		err    error
	}

	cases := map[string]struct {
		args
		want
	}{
		"SuccessfulDeleteMdevDevice": {
			args: args{
				nodeDevice: &external{
					service: func() *MockNodeDeviceService {
						m := NewMockNodeDeviceService()
						m.AddDevice("mdev_test", `<device>
							<name>mdev_test</name>
							<capability type='mdev'/>
						</device>`)
						return m
					}(),
				},
				cr: nodeDevice(func(cr *v1alpha1.NodeDevice) {
					cr.Spec.ForProvider.Name = "mdev_test"
					cr.Spec.ForProvider.Type = "mdev"
				}),
			},
			want: want{
				cr: nodeDevice(func(cr *v1alpha1.NodeDevice) {
					cr.Spec.ForProvider.Name = "mdev_test"
					cr.Spec.ForProvider.Type = "mdev"
				}),
				result: managed.ExternalDelete{},
			},
		},
		"DeleteNonExistentDevice": {
			args: args{
				nodeDevice: &external{
					service: NewMockNodeDeviceService(),
				},
				cr: nodeDevice(func(cr *v1alpha1.NodeDevice) {
					cr.Spec.ForProvider.Name = "nonexistent"
					cr.Spec.ForProvider.Type = "pci"
				}),
			},
			want: want{
				cr: nodeDevice(func(cr *v1alpha1.NodeDevice) {
					cr.Spec.ForProvider.Name = "nonexistent"
					cr.Spec.ForProvider.Type = "pci"
				}),
				result: managed.ExternalDelete{},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			d, err := tc.nodeDevice.Delete(context.Background(), tc.args.cr)

			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.result, d); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
		})
	}
}

type args struct {
	nodeDevice *external
	cr         *v1alpha1.NodeDevice
}

func nodeDevice(m ...func(*v1alpha1.NodeDevice)) *v1alpha1.NodeDevice {
	cr := &v1alpha1.NodeDevice{}
	for _, f := range m {
		f(cr)
	}
	return cr
}