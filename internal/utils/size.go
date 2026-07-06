/*
Copyright 2025 Ross Golder
*/

package utils

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/resource"
)

// ParseSize converts Kubernetes quantity strings to bytes
// Uses the standard Kubernetes resource.ParseQuantity function for consistency
// with PersistentVolume, resource limits, and other Kubernetes quantities.
// Supports formats like: 100Gi, 1.5Ti, 512Mi, 2048Ki, 1024, 500M, etc.
// See: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#meaning-of-memory
func ParseSize(sizeStr string) (int64, error) {
	if sizeStr == "" {
		return 0, fmt.Errorf("empty size string")
	}

	// Use Kubernetes' standard quantity parsing
	quantity, err := resource.ParseQuantity(sizeStr)
	if err != nil {
		return 0, fmt.Errorf("invalid quantity format: %s (expected Kubernetes quantity format like 100Gi, 1.5Ti, 512Mi)", sizeStr)
	}

	// Convert to bytes (resource.Quantity uses int64 internally)
	bytes := quantity.Value()

	// Ensure minimum size (1MB = 1,000,000 bytes)
	if bytes < 1000000 {
		return 0, fmt.Errorf("volume size must be at least 1MB, got: %d bytes", bytes)
	}

	return bytes, nil
}

// FormatSize converts bytes to human-readable format
// Uses binary units (1024-based) by default
func FormatSize(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%dB", bytes)
	}

	const unit = 1024
	units := []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB"}

	size := float64(bytes)
	i := 0
	for size >= unit && i < len(units)-1 {
		size /= unit
		i++
	}

	if size == float64(int64(size)) {
		return fmt.Sprintf("%.0f%s", size, units[i])
	}
	return fmt.Sprintf("%.1f%s", size, units[i])
}
