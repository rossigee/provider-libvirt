/*
Copyright 2024 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package domain

import (
	"context"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	domainv1alpha1 "github.com/nourspeed/provider-libvirt/apis/domain/v1alpha1"
	cloudinitv1alpha1 "github.com/nourspeed/provider-libvirt/apis/cloudinit/v1alpha1"
)

func TestValidateDomain(t *testing.T) {
	type args struct {
		domain *domainv1alpha1.Domain
		client client.Client
	}
	type want struct {
		err error
	}

	tests := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ValidDomainNoReferences": {
			reason: "Domain with no cross-references should be valid",
			args: args{
				domain: &domainv1alpha1.Domain{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-domain",
					},
					Spec: domainv1alpha1.DomainSpec{
						ResourceSpec: xpv1.ResourceSpec{
							ProviderConfigReference: &xpv1.Reference{
								Name: "provider-config-1",
							},
						},
						ForProvider: domainv1alpha1.DomainParameters{
							Name: stringPtr("test-domain"),
						},
					},
				},
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil),
				},
			},
			want: want{
				err: nil,
			},
		},
		"ValidDomainWithMatchingCloudinit": {
			reason: "Domain with cloudinit reference using same provider config should be valid",
			args: args{
				domain: &domainv1alpha1.Domain{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-domain",
					},
					Spec: domainv1alpha1.DomainSpec{
						ResourceSpec: xpv1.ResourceSpec{
							ProviderConfigReference: &xpv1.Reference{
								Name: "provider-config-1",
							},
						},
						ForProvider: domainv1alpha1.DomainParameters{
							Name: stringPtr("test-domain"),
							CloudinitRef: &xpv1.Reference{
								Name: "test-cloudinit",
							},
						},
					},
				},
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						if cloudinit, ok := obj.(*cloudinitv1alpha1.Disk); ok {
							*cloudinit = cloudinitv1alpha1.Disk{
								ObjectMeta: metav1.ObjectMeta{
									Name: "test-cloudinit",
								},
								Spec: cloudinitv1alpha1.DiskSpec{
									ResourceSpec: xpv1.ResourceSpec{
										ProviderConfigReference: &xpv1.Reference{
											Name: "provider-config-1",
										},
									},
								},
							}
						}
						return nil
					}),
				},
			},
			want: want{
				err: nil,
			},
		},
		"InvalidDomainMismatchedCloudinit": {
			reason: "Domain with cloudinit reference using different provider config should be invalid",
			args: args{
				domain: &domainv1alpha1.Domain{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-domain",
					},
					Spec: domainv1alpha1.DomainSpec{
						ResourceSpec: xpv1.ResourceSpec{
							ProviderConfigReference: &xpv1.Reference{
								Name: "provider-config-1",
							},
						},
						ForProvider: domainv1alpha1.DomainParameters{
							Name: stringPtr("test-domain"),
							CloudinitRef: &xpv1.Reference{
								Name: "test-cloudinit",
							},
						},
					},
				},
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						if cloudinit, ok := obj.(*cloudinitv1alpha1.Disk); ok {
							*cloudinit = cloudinitv1alpha1.Disk{
								ObjectMeta: metav1.ObjectMeta{
									Name: "test-cloudinit",
								},
								Spec: cloudinitv1alpha1.DiskSpec{
									ResourceSpec: xpv1.ResourceSpec{
										ProviderConfigReference: &xpv1.Reference{
											Name: "provider-config-2", // Different provider config
										},
									},
								},
							}
						}
						return nil
					}),
				},
			},
			want: want{
				err: errors.Wrap(errors.New("cloudinit disk test-cloudinit uses providerConfig provider-config-2, but domain uses providerConfig provider-config-1. Resources must use the same libvirt instance"), "invalid cloudinit reference"),
			},
		},
		"InvalidDomainNoProviderConfig": {
			reason: "Domain without provider config should be invalid",
			args: args{
				domain: &domainv1alpha1.Domain{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-domain",
					},
					Spec: domainv1alpha1.DomainSpec{
						ForProvider: domainv1alpha1.DomainParameters{
							Name: stringPtr("test-domain"),
						},
					},
				},
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil),
				},
			},
			want: want{
				err: errors.New("domain must have a providerConfigRef"),
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			v := &Validator{client: tc.args.client}
			err := v.validateDomain(context.Background(), tc.args.domain)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("%s\nvalidateDomain(): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func stringPtr(s string) *string {
	return &s
}