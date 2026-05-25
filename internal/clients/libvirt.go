/*
Copyright 2025 The Crossplane Authors.

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

// Package clients provides a libvirt client for the provider.
package clients

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/digitalocean/go-libvirt"
	"github.com/digitalocean/go-libvirt/socket/dialers"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1 "github.com/rossigee/provider-libvirt/apis/v1beta1"
)

const (
	defaultDialTimeout = 10 * time.Second
)

// Client wraps a go-libvirt connection.
type Client struct {
	lv *libvirt.Libvirt
}

// GetClient returns a connected libvirt.Client from a ProviderConfig.
// The caller is responsible for calling Close() when done.
func GetClient(ctx context.Context, kube client.Client, pc *v1beta1.ProviderConfig) (*Client, error) {
	uri := pc.Spec.URI
	if uri == "" {
		return nil, errors.New("provider config URI is empty")
	}

	conn, err := dialURI(ctx, uri)
	if err != nil {
		return nil, errors.Wrap(err, "cannot dial libvirt URI")
	}

	lv := libvirt.NewWithDialer(dialers.NewAlreadyConnected(conn))
	if err := lv.Connect(); err != nil {
		_ = conn.Close()
		return nil, errors.Wrap(err, "cannot connect to libvirt")
	}

	return &Client{lv: lv}, nil
}

// GetClientFromRef builds a client by looking up the ProviderConfig referenced by name.
func GetClientFromRef(ctx context.Context, kube client.Client, pcName string) (*Client, error) {
	pc := &v1beta1.ProviderConfig{}
	if err := kube.Get(ctx, types.NamespacedName{Name: pcName}, pc); err != nil {
		return nil, errors.Wrap(err, "cannot get ProviderConfig")
	}
	return GetClient(ctx, kube, pc)
}

// Close disconnects the libvirt connection.
func (c *Client) Close() {
	if c.lv != nil {
		_ = c.lv.Disconnect()
	}
}

// Libvirt returns the underlying go-libvirt client for direct API calls.
func (c *Client) Libvirt() *libvirt.Libvirt {
	return c.lv
}

// dialURI establishes a net.Conn to the libvirt daemon described by uri.
// Supported schemes: qemu (unix socket), qemu+tcp (cleartext TCP), qemu+tls (TLS TCP).
// For TLS, the ProviderConfig credentials secret should contain PEM-encoded
// client.crt, client.key, and optionally ca.crt keys.
func dialURI(ctx context.Context, rawURI string) (net.Conn, error) {
	u, err := url.Parse(rawURI)
	if err != nil {
		return nil, errors.Wrap(err, "invalid libvirt URI")
	}

	switch strings.ToLower(u.Scheme) {
	case "qemu", "qemu+unix":
		// Local or unix socket connection — no network needed.
		socketPath := "/var/run/libvirt/libvirt-sock"
		if s := u.Query().Get("socket"); s != "" {
			socketPath = s
		}
		dialer := net.Dialer{Timeout: defaultDialTimeout}
		return dialer.DialContext(ctx, "unix", socketPath)

	case "qemu+tcp":
		host := u.Hostname()
		port := u.Port()
		if port == "" {
			port = "16509"
		}
		dialer := net.Dialer{Timeout: defaultDialTimeout}
		return dialer.DialContext(ctx, "tcp", net.JoinHostPort(host, port))

	case "qemu+tls", "qemu+tls+tcp":
		// TLS requires a future extension to load certs from the ProviderConfig secret.
		// For now, return a clear error so the user knows TLS is not yet implemented.
		return nil, errors.New("qemu+tls connections require TLS certificate support — use qemu+tcp for now or contribute TLS support")

	default:
		return nil, fmt.Errorf("unsupported libvirt URI scheme %q (supported: qemu, qemu+unix, qemu+tcp)", strings.ToLower(u.Scheme))
	}
}

// GetCredentialSecret fetches the credential secret referenced in the ProviderConfig.
func GetCredentialSecret(ctx context.Context, kube client.Client, pc *v1beta1.ProviderConfig) (*corev1.Secret, error) {
	if pc.Spec.Credentials == nil {
		return nil, nil
	}
	if pc.Spec.Credentials.SecretRef == nil {
		return nil, nil
	}
	secret := &corev1.Secret{}
	if err := kube.Get(ctx, types.NamespacedName{
		Name:      pc.Spec.Credentials.SecretRef.Name,
		Namespace: pc.Spec.Credentials.SecretRef.Namespace,
	}, secret); err != nil {
		return nil, errors.Wrap(err, "cannot get credentials secret")
	}
	return secret, nil
}
