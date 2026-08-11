/*
Copyright 2025 Ross Golder
*/

package nodedevice

import (
	"context"
	"testing"

	"github.com/rossigee/provider-libvirt/internal/clients"
)

func TestDisconnect(t *testing.T) {
	e := &external{client: &clients.LibvirtClient{}}

	if err := e.Disconnect(context.Background()); err != nil {
		t.Fatalf("Disconnect() returned unexpected error: %v", err)
	}
}
