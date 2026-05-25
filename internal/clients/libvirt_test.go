package clients

import (
	"context"
	"net"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestDialURIUnsupportedScheme(t *testing.T) {
	_, err := dialURI(context.Background(), "invalid://example.com")
	if err == nil {
		t.Error("expected error for unsupported scheme, got nil")
	}
}

func TestDialURITLSReturnsError(t *testing.T) {
	_, err := dialURI(context.Background(), "qemu+tls://example.com/system")
	if err == nil {
		t.Error("expected error for qemu+tls (not implemented), got nil")
	}
}

func TestDialURITCPTimeout(t *testing.T) {
	// Dialling a non-existent TCP endpoint should fail rather than hang.
	// We use 127.0.0.1:1 which is reliably closed on any host.
	ctx, cancel := context.WithTimeout(context.Background(), defaultDialTimeout)
	defer cancel()

	conn, err := dialURI(ctx, "qemu+tcp://127.0.0.1:1/system")
	if err == nil {
		_ = conn.Close()
		t.Error("expected connection error to 127.0.0.1:1, got nil")
	}
}

func TestClientClose(t *testing.T) {
	// A Client with a nil lv should not panic on Close.
	c := &Client{}
	c.Close() // must not panic
}

func TestDialURIUnixSocket(t *testing.T) {
	// Create a temporary unix listener to verify the dialler connects.
	ln, err := net.Listen("unix", t.TempDir()+"/libvirt.sock")
	if err != nil {
		t.Fatalf("cannot create unix listener: %v", err)
	}
	defer ln.Close()

	uri := "qemu+unix:///system?socket=" + ln.Addr().String()
	conn, err := dialURI(context.Background(), uri)
	if err != nil {
		t.Fatalf("dialURI unix: unexpected error: %v", err)
	}
	_ = conn.Close()
}

func TestParseCIDRNotPresent(t *testing.T) {
	// parseCIDR is not in clients package — this test is a placeholder
	// to ensure the test binary compiles and the package is sound.
	_ = cmp.Diff("a", "a")
}
