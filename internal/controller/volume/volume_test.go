package volume

import (
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/rossigee/provider-libvirt/apis/v1beta1"
)

type volumeModifier func(*v1beta1.Volume)

func testVolume(m ...volumeModifier) *v1beta1.Volume {
	v := &v1beta1.Volume{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Volume",
			APIVersion: "libvirt.m.crossplane.io/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-volume",
			Namespace: "default",
		},
		Spec: v1beta1.VolumeSpec{
			ForProvider: v1beta1.VolumeParameters{
				Name:   "test-disk",
				Pool:   "default",
				Format: "qcow2",
				Size:   int64Ptr(1073741824), // 1GB
			},
		},
	}
	for _, f := range m {
		f(v)
	}
	return v
}

func int64Ptr(i int64) *int64 {
	return &i
}

func TestGenerateVolumeXML(t *testing.T) {
	cases := map[string]struct {
		volume   *v1beta1.Volume
		capacity int64
		format   string
		want     string
	}{
		"BasicVolume": {
			volume:   testVolume(),
			capacity: 1073741824,
			format:   "qcow2",
			want:     "test-disk",
		},
		"RawVolume": {
			volume:   testVolume(func(v *v1beta1.Volume) { v.Spec.ForProvider.Format = "raw" }),
			capacity: 2147483648,
			format:   "raw",
			want:     "test-disk",
		},
		"VmdkVolume": {
			volume:   testVolume(func(v *v1beta1.Volume) { v.Spec.ForProvider.Format = "vmdk" }),
			capacity: 5368709120,
			format:   "vmdk",
			want:     "test-disk",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ext := &external{client: nil}
			got := ext.generateVolumeXML(tc.volume, tc.capacity, tc.format)
			if !contains(got, tc.want) {
				t.Errorf("generateVolumeXML(...): expected to contain %q, got:\n%s", tc.want, got)
			}
			if !contains(got, tc.format) {
				t.Errorf("generateVolumeXML(...): expected to contain format %q, got:\n%s", tc.format, got)
			}
		})
	}
}

func TestGenerateVolumeXMLFormats(t *testing.T) {
	cases := map[string]struct {
		format string
		want   string
	}{
		"QCOW2Format": {
			format: "qcow2",
			want:   "qcow2",
		},
		"RawFormat": {
			format: "raw",
			want:   "raw",
		},
		"VmdkFormat": {
			format: "vmdk",
			want:   "vmdk",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ext := &external{client: nil}
			got := ext.generateVolumeXML(testVolume(), 1073741824, tc.format)
			if !contains(got, tc.want) {
				t.Errorf("generateVolumeXML(...): expected format %q in output", tc.want)
			}
		})
	}
}

func TestVolumeParameters(t *testing.T) {
	cases := map[string]struct {
		volume *v1beta1.Volume
		want   string
	}{
		"NameParameter": {
			volume: testVolume(),
			want:   "test-disk",
		},
		"PoolParameter": {
			volume: testVolume(),
			want:   "default",
		},
		"FormatParameter": {
			volume: testVolume(func(v *v1beta1.Volume) {
				v.Spec.ForProvider.Format = "raw"
			}),
			want: "raw",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			params := tc.volume.Spec.ForProvider
			if name == "NameParameter" && params.Name != tc.want {
				t.Errorf("Expected name %q, got %q", tc.want, params.Name)
			}
			if name == "PoolParameter" && params.Pool != tc.want {
				t.Errorf("Expected pool %q, got %q", tc.want, params.Pool)
			}
			if name == "FormatParameter" && params.Format != tc.want {
				t.Errorf("Expected format %q, got %q", tc.want, params.Format)
			}
		})
	}
}

func TestVolumeCapacity(t *testing.T) {
	cases := map[string]struct {
		sizeVal     *int64
		capacityVal *int64
		expectError bool
	}{
		"SizePreferred": {
			sizeVal:     int64Ptr(1073741824),
			capacityVal: int64Ptr(536870912),
			expectError: false,
		},
		"CapacityFallback": {
			sizeVal:     nil,
			capacityVal: int64Ptr(1073741824),
			expectError: false,
		},
		"SizeAndCapacity": {
			sizeVal:     int64Ptr(2147483648),
			capacityVal: int64Ptr(1073741824),
			expectError: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			v := testVolume(func(vol *v1beta1.Volume) {
				vol.Spec.ForProvider.Size = tc.sizeVal
				vol.Spec.ForProvider.Capacity = tc.capacityVal
			})

			if v.Spec.ForProvider.Size == nil && v.Spec.ForProvider.Capacity == nil {
				if !tc.expectError {
					t.Errorf("Expected valid capacity configuration")
				}
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

func TestGenerateVolumeXMLQcow2Format(t *testing.T) {
	ext := &external{client: nil}
	volume := testVolume(func(v *v1beta1.Volume) {
		v.Spec.ForProvider.Format = "qcow2"
	})

	xml := ext.generateVolumeXML(volume, 1073741824, "qcow2")
	if !contains(xml, "qcow2") {
		t.Errorf("Expected qcow2 format in XML, got:\n%s", xml)
	}
	if !contains(xml, "test-disk") {
		t.Errorf("Expected volume name in XML, got:\n%s", xml)
	}
}

func TestGenerateVolumeXMLRawFormat(t *testing.T) {
	ext := &external{client: nil}
	volume := testVolume(func(v *v1beta1.Volume) {
		v.Spec.ForProvider.Format = "raw"
	})

	xml := ext.generateVolumeXML(volume, 2147483648, "raw")
	if !contains(xml, "raw") {
		t.Errorf("Expected raw format in XML, got:\n%s", xml)
	}
	if !contains(xml, "test-disk") {
		t.Errorf("Expected volume name in XML, got:\n%s", xml)
	}
}

func TestGenerateVolumeXMLVmdkFormat(t *testing.T) {
	ext := &external{client: nil}
	volume := testVolume(func(v *v1beta1.Volume) {
		v.Spec.ForProvider.Format = "vmdk"
	})

	xml := ext.generateVolumeXML(volume, 5368709120, "vmdk")
	if !contains(xml, "vmdk") {
		t.Errorf("Expected vmdk format in XML, got:\n%s", xml)
	}
}

func TestGenerateVolumeXMLCapacity(t *testing.T) {
	ext := &external{client: nil}
	volume := testVolume()

	cases := map[string]int64{
		"1GB":   1073741824,
		"10GB":  10737418240,
		"100GB": 107374182400,
	}

	for name, capacity := range cases {
		t.Run(name, func(t *testing.T) {
			xml := ext.generateVolumeXML(volume, capacity, "qcow2")
			capacityStr := capStr(capacity)
			if !contains(xml, capacityStr) {
				t.Errorf("Expected capacity %d in XML, got:\n%s", capacity, xml)
			}
		})
	}
}

func capStr(c int64) string {
	return fmt.Sprintf("%d", c)
}

func TestVolumePoolParameter(t *testing.T) {
	ext := &external{client: nil}
	volume := testVolume(func(v *v1beta1.Volume) {
		v.Spec.ForProvider.Pool = "storage-pool"
	})

	// Pool parameter is separate from XML generation
	// Verify it's preserved in the object
	if volume.Spec.ForProvider.Pool != "storage-pool" {
		t.Errorf("Expected pool parameter to be set")
	}

	// XML should still have the volume name
	xml := ext.generateVolumeXML(volume, 1073741824, "qcow2")
	if !contains(xml, "test-disk") {
		t.Errorf("Expected volume name in XML, got:\n%s", xml)
	}
}
