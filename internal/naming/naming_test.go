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

package naming

import (
	"testing"
)

func TestGenerateName(t *testing.T) {
	tests := []struct {
		name           string
		strategy       Strategy
		libvirtName    string
		providerConfig string
		want           string
	}{
		{
			name:           "StrategyNone",
			strategy:       StrategyNone,
			libvirtName:    "my-vm",
			providerConfig: "libvirt-host1",
			want:           "my-vm",
		},
		{
			name:           "StrategyPrefixProvider",
			strategy:       StrategyPrefixProvider,
			libvirtName:    "my-vm",
			providerConfig: "libvirt-host1",
			want:           "libvirt-host1-my-vm",
		},
		{
			name:           "StrategyPrefixHost",
			strategy:       StrategyPrefixHost,
			libvirtName:    "my-vm",
			providerConfig: "libvirt-host1",
			want:           "host1-my-vm",
		},
		{
			name:           "StrategyPrefixHostComplexProvider",
			strategy:       StrategyPrefixHost,
			libvirtName:    "database",
			providerConfig: "provider-prod-01-libvirt",
			want:           "prod-01-database",
		},
		{
			name:           "SanitizeSpecialChars",
			strategy:       StrategyPrefixProvider,
			libvirtName:    "My_VM@2024",
			providerConfig: "libvirt.host1",
			want:           "libvirt-host1-my-vm-2024",
		},
		{
			name:           "TruncateLongName",
			strategy:       StrategyPrefixProvider,
			libvirtName:    "very-long-vm-name-that-exceeds-kubernetes-limits-for-resource-names",
			providerConfig: "libvirt-host1",
			want:           "libvirt-host1-very-long-vm-name-that-exceeds-kubernet-463", // truncated with hash
		},
		{
			name:           "CollapseMultipleHyphens",
			strategy:       StrategyPrefixProvider,
			libvirtName:    "vm---with---many---hyphens",
			providerConfig: "libvirt--host1",
			want:           "libvirt-host1-vm-with-many-hyphens",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGenerator(tt.strategy)
			got := g.GenerateName(tt.libvirtName, tt.providerConfig)
			
			// For hash-based truncation, just check the prefix matches
			if len(tt.want) > 60 && len(got) == 63 {
				if got[:len(got)-4] != tt.want[:len(tt.want)-4] {
					t.Errorf("GenerateName() = %v, want prefix %v", got, tt.want[:len(tt.want)-4])
				}
			} else if got != tt.want {
				t.Errorf("GenerateName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "AlreadyValid",
			input: "valid-name-123",
			want:  "valid-name-123",
		},
		{
			name:  "UpperCase",
			input: "MyVirtualMachine",
			want:  "myvirtualmachine",
		},
		{
			name:  "SpecialCharacters",
			input: "vm@prod#2024!",
			want:  "vm-prod-2024",
		},
		{
			name:  "LeadingTrailingHyphens",
			input: "--vm-name--",
			want:  "vm-name",
		},
		{
			name:  "Underscores",
			input: "vm_name_with_underscores",
			want:  "vm-name-with-underscores",
		},
		{
			name:  "Dots",
			input: "vm.prod.example.com",
			want:  "vm-prod-example-com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeName(tt.input); got != tt.want {
				t.Errorf("sanitizeName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractHostname(t *testing.T) {
	tests := []struct {
		name           string
		providerConfig string
		want           string
	}{
		{
			name:           "StandardPrefix",
			providerConfig: "libvirt-host1",
			want:           "host1",
		},
		{
			name:           "StandardSuffix",
			providerConfig: "host1-libvirt",
			want:           "host1",
		},
		{
			name:           "ProviderPrefix",
			providerConfig: "provider-prod-01",
			want:           "prod-01",
		},
		{
			name:           "ComplexName",
			providerConfig: "libvirt-us-east-1a-provider",
			want:           "us-east-1a",
		},
		{
			name:           "NoPattern",
			providerConfig: "custom-name",
			want:           "custom-name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractHostname(tt.providerConfig); got != tt.want {
				t.Errorf("extractHostname() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseStrategy(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  Strategy
	}{
		{
			name:  "PrefixProvider",
			input: "prefix-provider",
			want:  StrategyPrefixProvider,
		},
		{
			name:  "PrefixHost",
			input: "prefix-host",
			want:  StrategyPrefixHost,
		},
		{
			name:  "Hash",
			input: "hash",
			want:  StrategyHash,
		},
		{
			name:  "None",
			input: "none",
			want:  StrategyNone,
		},
		{
			name:  "Empty",
			input: "",
			want:  StrategyNone,
		},
		{
			name:  "Invalid",
			input: "invalid-strategy",
			want:  StrategyNone,
		},
		{
			name:  "CaseInsensitive",
			input: "PREFIX-PROVIDER",
			want:  StrategyPrefixProvider,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseStrategy(tt.input); got != tt.want {
				t.Errorf("ParseStrategy() = %v, want %v", got, tt.want)
			}
		})
	}
}