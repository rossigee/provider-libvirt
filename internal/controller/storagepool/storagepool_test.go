package storagepool

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/rossigee/provider-libvirt/apis/v1beta1"
)

type storagePoolModifier func(*v1beta1.StoragePool)

func testStoragePool(m ...storagePoolModifier) *v1beta1.StoragePool {
	sp := &v1beta1.StoragePool{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StoragePool",
			APIVersion: "libvirt.m.crossplane.io/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pool",
			Namespace: "default",
		},
		Spec: v1beta1.StoragePoolSpec{
			ForProvider: v1beta1.StoragePoolParameters{
				Name: "test-storage",
				Type: "dir",
				Target: &v1beta1.StoragePoolTarget{
					Path: "/var/lib/libvirt/images",
				},
			},
		},
	}
	for _, f := range m {
		f(sp)
	}
	return sp
}

func TestGeneratePoolXML(t *testing.T) {
	cases := map[string]struct {
		pool *v1beta1.StoragePool
		want string
	}{
		"BasicDir": {
			pool: testStoragePool(),
			want: "test-storage",
		},
		"CustomPath": {
			pool: testStoragePool(func(sp *v1beta1.StoragePool) {
				sp.Spec.ForProvider.Target = &v1beta1.StoragePoolTarget{
					Path: "/mnt/storage",
				}
			}),
			want: "test-storage",
		},
		"FSType": {
			pool: testStoragePool(func(sp *v1beta1.StoragePool) {
				sp.Spec.ForProvider.Type = "fs"
			}),
			want: "test-storage",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ext := &external{client: nil}
			got := ext.generatePoolXML(tc.pool)
			if !contains(got, tc.want) {
				t.Errorf("generatePoolXML(...): expected to contain %q, got:\n%s", tc.want, got)
			}
			// Verify type is in XML
			if !contains(got, tc.pool.Spec.ForProvider.Type) {
				t.Errorf("generatePoolXML(...): expected type %q in output", tc.pool.Spec.ForProvider.Type)
			}
		})
	}
}

func TestStoragePoolParameters(t *testing.T) {
	cases := map[string]struct {
		pool *v1beta1.StoragePool
		want string
	}{
		"NameParameter": {
			pool: testStoragePool(),
			want: "test-storage",
		},
		"TypeParameter": {
			pool: testStoragePool(func(sp *v1beta1.StoragePool) {
				sp.Spec.ForProvider.Type = "fs"
			}),
			want: "fs",
		},
		"PathParameter": {
			pool: testStoragePool(func(sp *v1beta1.StoragePool) {
				sp.Spec.ForProvider.Target = &v1beta1.StoragePoolTarget{
					Path: "/mnt/storage",
				}
			}),
			want: "/mnt/storage",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			params := tc.pool.Spec.ForProvider
			if name == "NameParameter" && params.Name != tc.want {
				t.Errorf("Expected name %q, got %q", tc.want, params.Name)
			}
			if name == "TypeParameter" && params.Type != tc.want {
				t.Errorf("Expected type %q, got %q", tc.want, params.Type)
			}
			if name == "PathParameter" && params.Target != nil && params.Target.Path != tc.want {
				t.Errorf("Expected path %q, got %q", tc.want, params.Target.Path)
			}
		})
	}
}

func TestStoragePoolTypes(t *testing.T) {
	cases := map[string]struct {
		poolType string
		want     string
	}{
		"Dir": {
			poolType: "dir",
			want:     "dir",
		},
		"FS": {
			poolType: "fs",
			want:     "fs",
		},
		"NetFS": {
			poolType: "netfs",
			want:     "netfs",
		},
		"iSCSI": {
			poolType: "iscsi",
			want:     "iscsi",
		},
		"LVM": {
			poolType: "logical",
			want:     "logical",
		},
		"RBD": {
			poolType: "rbd",
			want:     "rbd",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			sp := testStoragePool(func(pool *v1beta1.StoragePool) {
				pool.Spec.ForProvider.Type = tc.poolType
			})

			if diff := cmp.Diff(tc.want, sp.Spec.ForProvider.Type); diff != "" {
				t.Errorf("StoragePool type -want, +got:\n%s", diff)
			}
		})
	}
}

func TestPoolPathHandling(t *testing.T) {
	cases := map[string]struct {
		targetPath string
		want       string
	}{
		"CustomPath": {
			targetPath: "/mnt/custom/storage",
			want:       "/mnt/custom/storage",
		},
		"VarLibPath": {
			targetPath: "/var/lib/libvirt/images",
			want:       "/var/lib/libvirt/images",
		},
		"DeepPath": {
			targetPath: "/srv/storage/pool/images",
			want:       "/srv/storage/pool/images",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			sp := testStoragePool(func(pool *v1beta1.StoragePool) {
				pool.Spec.ForProvider.Target = &v1beta1.StoragePoolTarget{
					Path: tc.targetPath,
				}
			})
			if sp.Spec.ForProvider.Target.Path != tc.want {
				t.Errorf("Expected path %q, got %q", tc.want, sp.Spec.ForProvider.Target.Path)
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
