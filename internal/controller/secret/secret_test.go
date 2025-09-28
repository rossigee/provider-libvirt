package secret

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"

	"github.com/rossigee/provider-libvirt/apis/v1beta1"
)

var (
	errBoom = errors.New("boom")
)

type secretModifier func(*v1beta1.Secret)


func secret(m ...secretModifier) *v1beta1.Secret {
	i := &v1beta1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "libvirt.m.crossplane.io/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
		Spec: v1beta1.SecretSpec{
			ForProvider: v1beta1.SecretParameters{
				Type:        "volume",
				Usage:       "test-volume",
				Description: "Test secret for volume",
				Data: v1beta1.SecretData{
					Volume: &v1beta1.VolumeSecretData{
						Passphrase: &v1beta1.SecretReference{
							Name:      "test-passphrase",
							Key:       "passphrase",
							Namespace: "default",
						},
					},
				},
			},
		},
		Status: v1beta1.SecretStatus{},
	}
	for _, f := range m {
		f(i)
	}
	return i
}

// Test helper functions
func TestGetUsageType(t *testing.T) {
	cases := map[string]struct {
		secretType string
		usage      string
		want       string
	}{
		"VolumeType": {
			secretType: "volume",
			usage:      "test-volume",
			want:       "volume",
		},
		"CephType": {
			secretType: "ceph",
			usage:      "test-ceph",
			want:       "ceph",
		},
		"ISCSIType": {
			secretType: "iscsi",
			usage:      "test-iscsi",
			want:       "iscsi",
		},
		"TLSType": {
			secretType: "tls",
			usage:      "test-tls",
			want:       "tls",
		},
		"DefaultType": {
			secretType: "unknown",
			usage:      "test",
			want:       "volume",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := getUsageType(tc.secretType, tc.usage)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("getUsageType(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestGenerateUsageID(t *testing.T) {
	cases := map[string]struct {
		secret *v1beta1.Secret
		want   string
	}{
		"WithNamespace": {
			secret: &v1beta1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "test-namespace",
				},
			},
			want: "test-namespace-test-secret",
		},
		"WithoutNamespace": {
			secret: &v1beta1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-secret",
				},
			},
			want: "test-secret",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := generateUsageID(tc.secret)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("generateUsageID(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestGetUsageElementName(t *testing.T) {
	cases := map[string]struct {
		usageType string
		want      string
	}{
		"VolumeType": {
			usageType: "volume",
			want:      "volume",
		},
		"CephType": {
			usageType: "ceph",
			want:      "name",
		},
		"ISCSIType": {
			usageType: "iscsi",
			want:      "target",
		},
		"TLSType": {
			usageType: "tls",
			want:      "name",
		},
		"DefaultType": {
			usageType: "unknown",
			want:      "volume",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := getUsageElementName(tc.usageType)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("getUsageElementName(...): -want, +got:\n%s", diff)
			}
		})
	}
}

// External client tests
func TestExternalObserve(t *testing.T) {
	type want struct {
		o   managed.ExternalObservation
		err error
	}

	cases := map[string]struct {
		args func() (managed.ExternalClient, resource.Managed)
		want want
	}{
		"NotASecret": {
			args: func() (managed.ExternalClient, resource.Managed) {
				return &external{}, &v1beta1.Domain{}
			},
			want: want{
				err: errors.New(errNotSecret),
			},
		},
		"NoUUID": {
			args: func() (managed.ExternalClient, resource.Managed) {
				return &external{}, secret()
			},
			want: want{
				o: managed.ExternalObservation{
					ResourceExists: false,
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e, mg := tc.args()
			got, err := e.Observe(context.Background(), mg)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("Observe(...): -wantErr, +gotErr:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.o, got); diff != "" {
				t.Errorf("Observe(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestExternalCreate(t *testing.T) {
	type want struct {
		c   managed.ExternalCreation
		err error
	}

	cases := map[string]struct {
		args func() (managed.ExternalClient, resource.Managed)
		want want
	}{
		"NotASecret": {
			args: func() (managed.ExternalClient, resource.Managed) {
				return &external{}, &v1beta1.Domain{}
			},
			want: want{
				err: errors.New(errNotSecret),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e, mg := tc.args()
			got, err := e.Create(context.Background(), mg)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("Create(...): -wantErr, +gotErr:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.c, got); diff != "" {
				t.Errorf("Create(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestExternalUpdate(t *testing.T) {
	type want struct {
		u   managed.ExternalUpdate
		err error
	}

	cases := map[string]struct {
		args func() (managed.ExternalClient, resource.Managed)
		want want
	}{
		"NotASecret": {
			args: func() (managed.ExternalClient, resource.Managed) {
				return &external{}, &v1beta1.Domain{}
			},
			want: want{
				err: errors.New(errNotSecret),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e, mg := tc.args()
			got, err := e.Update(context.Background(), mg)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("Update(...): -wantErr, +gotErr:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.u, got); diff != "" {
				t.Errorf("Update(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestExternalDelete(t *testing.T) {
	type want struct {
		d   managed.ExternalDelete
		err error
	}

	cases := map[string]struct {
		args func() (managed.ExternalClient, resource.Managed)
		want want
	}{
		"NotASecret": {
			args: func() (managed.ExternalClient, resource.Managed) {
				return &external{}, &v1beta1.Domain{}
			},
			want: want{
				err: errors.New(errNotSecret),
			},
		},
		"NoUUID": {
			args: func() (managed.ExternalClient, resource.Managed) {
				return &external{}, secret()
			},
			want: want{
				d: managed.ExternalDelete{},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e, mg := tc.args()
			got, err := e.Delete(context.Background(), mg)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("Delete(...): -wantErr, +gotErr:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.d, got); diff != "" {
				t.Errorf("Delete(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestGetSecretFromK8s(t *testing.T) {
	type args struct {
		ref *v1beta1.SecretReference
	}
	type want struct {
		data []byte
		err  error
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"EmptyName": {
			args: args{
				ref: &v1beta1.SecretReference{
					Name: "",
					Key:  "test-key",
				},
			},
			want: want{
				err: errors.New(errInvalidSecretRef),
			},
		},
		"EmptyKey": {
			args: args{
				ref: &v1beta1.SecretReference{
					Name: "test-secret",
					Key:  "",
				},
			},
			want: want{
				err: errors.New(errInvalidSecretRef),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := &external{
				kube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
						return errBoom
					},
				},
			}
			got, err := e.getSecretFromK8s(context.Background(), tc.args.ref)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("getSecretFromK8s(...): -wantErr, +gotErr:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.data, got); diff != "" {
				t.Errorf("getSecretFromK8s(...): -want, +got:\n%s", diff)
			}
		})
	}
}

// Connector tests
func TestConnect(t *testing.T) {
	type want struct {
		err error
	}

	cases := map[string]struct {
		args func() (context.Context, resource.Managed)
		want want
	}{
		"NotASecret": {
			args: func() (context.Context, resource.Managed) {
				return context.Background(), &v1beta1.Domain{}
			},
			want: want{
				err: errors.New(errNotSecret),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := &connector{
				kube:         nil,
				usage:        nil,
				newServiceFn: nil,
			}
			ctx, mg := tc.args()
			_, err := c.Connect(ctx, mg)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("Connect(...): -wantErr, +gotErr:\n%s", diff)
			}
		})
	}
}