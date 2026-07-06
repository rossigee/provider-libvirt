/*
Copyright 2025 Ross Golder
*/

package utils

import (
	"testing"
)

func TestParseSize(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
		wantErr  bool
	}{
		// Basic byte values (Kubernetes standard)
		{"bytes only", "1048576", 1048576, false},

		// Kilobytes (binary only - Kubernetes doesn't support "K" decimal)
		{"kilobytes binary", "1024Ki", 1048576, false}, // 1024Ki = 1MiB, should be OK

		// Megabytes (decimal and binary) - Kubernetes format
		{"megabytes decimal", "100M", 100000000, false},
		{"megabytes binary", "100Mi", 104857600, false},

		// Gigabytes (decimal and binary) - Kubernetes format
		{"gigabytes decimal", "1G", 1000000000, false},
		{"gigabytes binary", "1Gi", 1073741824, false},

		// Terabytes (decimal and binary) - Kubernetes format
		{"terabytes decimal", "1T", 1000000000000, false},
		{"terabytes binary", "1Ti", 1099511627776, false},

		// Decimal values - Kubernetes format
		{"decimal gigabytes", "1.5G", 1500000000, false},
		{"decimal gigabytes binary", "1.5Gi", 1610612736, false},

		// Common realistic sizes - Kubernetes format
		{"20G disk", "20G", 20000000000, false},
		{"100G disk", "100G", 100000000000, false},
		{"1T disk", "1T", 1000000000000, false},

		// Error cases
		{"empty string", "", 0, true},
		{"invalid format", "abc", 0, true},
		{"negative value", "-100G", 0, true},
		{"invalid unit", "100X", 0, true},
		{"invalid with B suffix", "100GB", 0, true},         // 'B' suffix not allowed in Kubernetes
		{"invalid lowercase", "1g", 0, true},                // Lowercase not allowed in Kubernetes
		{"too small", "512", 0, true},                       // Less than 1MB minimum
		{"too small with Ki", "512Ki", 0, true},             // 512Ki < 1MB
		{"just over minimum", "1M", 1000000, false},         // 1M should be ok
		{"binary just over minimum", "1Mi", 1048576, false}, // 1Mi should be ok
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSize(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseSize() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("ParseSize() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		name     string
		bytes    int64
		expected string
	}{
		{"bytes", 512, "512B"},
		{"kilobytes", 1024, "1KiB"},
		{"kilobytes decimal", 1536, "1.5KiB"},
		{"megabytes", 1048576, "1MiB"},
		{"megabytes decimal", 1572864, "1.5MiB"},
		{"gigabytes", 1073741824, "1GiB"},
		{"gigabytes decimal", 1610612736, "1.5GiB"},
		{"terabytes", 1099511627776, "1TiB"},
		{"large size", 21474836480, "20GiB"},    // 20GB in bytes
		{"larger size", 107374182400, "100GiB"}, // 100GB in bytes
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatSize(tt.bytes)
			if got != tt.expected {
				t.Errorf("FormatSize() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// Benchmark tests
func BenchmarkParseSize(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = ParseSize("100G")
	}
}

func BenchmarkFormatSize(b *testing.B) {
	for i := 0; i < b.N; i++ {
		FormatSize(107374182400)
	}
}
