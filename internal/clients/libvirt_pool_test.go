/*
Copyright 2025 Ross Golder
*/

package clients

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"libvirt.org/go/libvirt"
)

// resetConnPool clears the package-level connection pool so a test starts
// from a known state. The pool is process-global, so tests that use it must
// not run in parallel.
func resetConnPool(t *testing.T) {
	t.Helper()
	connPoolMu.Lock()
	for uri, e := range connPool {
		e.mu.Lock()
		if e.conn != nil {
			_, _ = e.conn.Close()
			e.conn = nil
		}
		e.mu.Unlock()
		delete(connPool, uri)
	}
	connPoolMu.Unlock()
}

// TestAcquireConnectionDoesNotBlockOtherURIs is the regression test for the
// outage where one unreachable hypervisor stalled the whole provider.
//
// The pool used to hold a single package-wide mutex across the connect
// attempt. A connect to a blackholed address blocks for the kernel's TCP
// timeout, so every other URI's reconcile queued behind it and the provider
// advanced at one item per connect timeout. The pool must now serialise only
// against the same URI.
func TestAcquireConnectionDoesNotBlockOtherURIs(t *testing.T) {
	resetConnPool(t)
	t.Cleanup(func() { resetConnPool(t) })

	// The slow URI blocks in its connect for well past the time the other
	// URI needs to complete.
	const slowConnect = 3 * time.Second

	origConnect := newLibvirtConnect
	t.Cleanup(func() { newLibvirtConnect = origConnect })

	releaseOnce := sync.Once{}
	release := make(chan struct{})
	blocked := make(chan struct{}, 1)
	slowDone := make(chan struct{})

	// Releasing the slow connect is a cleanup, registered after the pool
	// reset so it runs first (cleanups are LIFO). The slow connect holds the
	// pool lock while it waits, so the reset would otherwise deadlock
	// against it and the test would hang instead of reporting the failure.
	t.Cleanup(func() {
		releaseOnce.Do(func() { close(release) })
		<-slowDone
	})

	newLibvirtConnect = func(uri string) (*libvirt.Connect, error) {
		if uri == "qemu+tls://slow.invalid/system" {
			blocked <- struct{}{}
			<-release
			return nil, fmt.Errorf("connect to %s timed out", uri)
		}
		return nil, fmt.Errorf("connect to %s failed", uri)
	}

	// Start the slow acquire. It will fail, but it fails only after the
	// connect returns.
	go func() {
		defer close(slowDone)
		_, _ = acquireConnection("qemu+tls://slow.invalid/system")
	}()

	// Wait for the slow connect to be in flight.
	select {
	case <-blocked:
	case <-time.After(5 * time.Second):
		t.Fatal("slow connect never started")
	}

	// A different URI must not be held up by it.
	otherDone := make(chan struct{})
	go func() {
		_, _ = acquireConnection("qemu+tls://other.invalid/system")
		close(otherDone)
	}()

	select {
	case <-otherDone:
		// Good: the other URI was not blocked.
	case <-time.After(slowConnect / 2):
		t.Error("acquireConnection for a different URI was blocked by an unrelated slow connect")
	}
}

// TestConnectWithTimeoutReturnsOnUnreachableHost proves the connect is
// bounded. libvirt's NewConnect has no timeout of its own, so without this
// bound an unreachable hypervisor holds its own reconcile worker for the
// kernel's TCP timeout.
func TestConnectWithTimeoutReturnsOnUnreachableHost(t *testing.T) {
	origConnect := newLibvirtConnect
	t.Cleanup(func() { newLibvirtConnect = origConnect })

	newLibvirtConnect = func(uri string) (*libvirt.Connect, error) {
		time.Sleep(5 * time.Second)
		return nil, fmt.Errorf("connect to %s timed out", uri)
	}

	start := time.Now()
	_, err := connectWithTimeout("qemu+tls://slow.invalid/system", 200*time.Millisecond)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected an error from an overrunning connect")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("connectWithTimeout took %s, expected it to give up promptly", elapsed)
	}
}

// TestConnectWithTimeoutReturnsConnectResult proves a connect that completes
// in time is returned rather than treated as a timeout.
func TestConnectWithTimeoutReturnsConnectResult(t *testing.T) {
	origConnect := newLibvirtConnect
	t.Cleanup(func() { newLibvirtConnect = origConnect })

	// A connect that returns an error in time must surface that error
	// rather than a timeout.
	newLibvirtConnect = func(uri string) (*libvirt.Connect, error) {
		return nil, fmt.Errorf("connect refused")
	}

	_, err := connectWithTimeout("qemu+tls://refused.invalid/system", 5*time.Second)
	if err == nil {
		t.Fatal("expected the connect error to be returned")
	}
	if err.Error() != "connect refused" {
		t.Fatalf("expected the connect error, got %v", err)
	}
}

// TestReleaseConnectionForUnknownURIIsNoOp proves releasing a connection for
// a URI that was never acquired does not create a pool entry.
func TestReleaseConnectionForUnknownURIIsNoOp(t *testing.T) {
	resetConnPool(t)
	t.Cleanup(func() { resetConnPool(t) })

	releaseConnection("qemu+tls://never-acquired.invalid/system")

	connPoolMu.Lock()
	_, ok := connPool["qemu+tls://never-acquired.invalid/system"]
	connPoolMu.Unlock()
	if ok {
		t.Fatal("releaseConnection created a pool entry for a URI that was never acquired")
	}
}
