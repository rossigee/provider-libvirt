/*
Copyright 2025 Ross Golder
*/

package controller

// Package-level marker for connection retry utilities
// See clients/libvirt.go for RetriableError and ExtractRetriableBackoff helpers
// Controllers should check for RetriableError from GetLibvirtClient and return
// appropriate reconcile results with backoff
