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

// Package naming provides utilities for generating instance-aware resource names
// to prevent collisions when managing multiple libvirt instances.
package naming

import (
	"fmt"
	"strings"
	"regexp"
)

const (
	// LabelLibvirtInstance is the label key for identifying which libvirt instance a resource belongs to
	LabelLibvirtInstance = "libvirt.nourspeed.io/instance"
	
	// LabelLibvirtHost is an alternative label for the libvirt host
	LabelLibvirtHost = "libvirt.nourspeed.io/host"
	
	// AnnotationOriginalName stores the original libvirt resource name
	AnnotationOriginalName = "libvirt.nourspeed.io/original-name"
)

// Strategy defines how to generate Kubernetes resource names
type Strategy string

const (
	// StrategyNone uses the libvirt name as-is (default, may cause collisions)
	StrategyNone Strategy = "none"
	
	// StrategyPrefixProvider prefixes with the provider config name
	StrategyPrefixProvider Strategy = "prefix-provider"
	
	// StrategyPrefixHost prefixes with a sanitized hostname
	StrategyPrefixHost Strategy = "prefix-host"
	
	// StrategyHash appends a hash of the provider config
	StrategyHash Strategy = "hash"
)

// Generator generates instance-aware names
type Generator struct {
	Strategy Strategy
}

// NewGenerator creates a new name generator with the specified strategy
func NewGenerator(strategy Strategy) *Generator {
	return &Generator{Strategy: strategy}
}

// GenerateName creates a Kubernetes-safe name that includes instance information
func (g *Generator) GenerateName(libvirtName, providerConfig string) string {
	switch g.Strategy {
	case StrategyPrefixProvider:
		return g.prefixProviderName(libvirtName, providerConfig)
	case StrategyPrefixHost:
		return g.prefixHostName(libvirtName, providerConfig)
	case StrategyHash:
		return g.appendHash(libvirtName, providerConfig)
	default:
		return libvirtName
	}
}

// prefixProviderName adds the provider config name as a prefix
func (g *Generator) prefixProviderName(libvirtName, providerConfig string) string {
	// Sanitize the provider config name
	prefix := sanitizeName(providerConfig)
	name := sanitizeName(libvirtName)
	
	// Combine with a separator
	combined := fmt.Sprintf("%s-%s", prefix, name)
	
	// Ensure it's within Kubernetes name limits (63 chars)
	return truncateName(combined, 63)
}

// prefixHostName extracts hostname from provider config and uses as prefix
func (g *Generator) prefixHostName(libvirtName, providerConfig string) string {
	// Extract host portion from provider config name
	// e.g., "libvirt-host1" -> "host1"
	host := extractHostname(providerConfig)
	name := sanitizeName(libvirtName)
	
	combined := fmt.Sprintf("%s-%s", host, name)
	return truncateName(combined, 63)
}

// appendHash adds a short hash of the provider config
func (g *Generator) appendHash(libvirtName, providerConfig string) string {
	name := sanitizeName(libvirtName)
	hash := shortHash(providerConfig)
	
	combined := fmt.Sprintf("%s-%s", name, hash)
	return truncateName(combined, 63)
}

// sanitizeName makes a string safe for Kubernetes resource names
func sanitizeName(name string) string {
	// Convert to lowercase
	name = strings.ToLower(name)
	
	// Replace invalid characters with hyphens
	reg := regexp.MustCompile(`[^a-z0-9-]`)
	name = reg.ReplaceAllString(name, "-")
	
	// Remove leading/trailing hyphens
	name = strings.Trim(name, "-")
	
	// Collapse multiple hyphens
	reg = regexp.MustCompile(`-+`)
	name = reg.ReplaceAllString(name, "-")
	
	return name
}

// extractHostname attempts to extract a hostname from a provider config name
func extractHostname(providerConfig string) string {
	// Common patterns:
	// - libvirt-host1 -> host1
	// - libvirt-prod-01 -> prod-01
	// - host1-libvirt -> host1
	
	lower := strings.ToLower(providerConfig)
	
	// Remove common prefixes
	for _, prefix := range []string{"libvirt-", "provider-", "config-"} {
		if strings.HasPrefix(lower, prefix) {
			lower = strings.TrimPrefix(lower, prefix)
		}
	}
	
	// Remove common suffixes
	for _, suffix := range []string{"-libvirt", "-provider", "-config"} {
		if strings.HasSuffix(lower, suffix) {
			lower = strings.TrimSuffix(lower, suffix)
		}
	}
	
	return sanitizeName(lower)
}

// shortHash generates a short hash suitable for resource names
func shortHash(input string) string {
	// Simple hash for demonstration - in production, use a proper hash
	hash := 0
	for _, c := range input {
		hash = (hash << 5) - hash + int(c)
	}
	
	// Convert to base36 and take first 6 chars
	hashStr := fmt.Sprintf("%x", hash)
	if len(hashStr) > 6 {
		hashStr = hashStr[:6]
	}
	
	return hashStr
}

// truncateName ensures a name doesn't exceed the maximum length
func truncateName(name string, maxLen int) string {
	if len(name) <= maxLen {
		return name
	}
	
	// Leave room for a hash suffix (e.g., "-abc")
	if maxLen < 5 {
		// If maxLen is too small, just truncate
		return name[:maxLen]
	}
	
	// Calculate hash of the full name for consistency
	hash := 0
	for i, c := range name {
		hash = hash*31 + int(c) + i
	}
	// Take absolute value and use modulo to get 3 digits
	if hash < 0 {
		hash = -hash
	}
	hashStr := fmt.Sprintf("%03d", hash % 1000) // Always 3 digits with leading zeros
	
	// Truncate name and append hash
	// Reserve 4 characters for "-XXX" suffix
	truncateAt := maxLen - 4
	if truncateAt < 1 {
		truncateAt = 1
	}
	
	result := name[:truncateAt] + "-" + hashStr
	// Ensure we don't exceed maxLen
	if len(result) > maxLen {
		result = result[:maxLen]
	}
	
	return result
}

// ParseStrategy converts a string to a Strategy
func ParseStrategy(s string) Strategy {
	switch strings.ToLower(s) {
	case "prefix-provider":
		return StrategyPrefixProvider
	case "prefix-host":
		return StrategyPrefixHost
	case "hash":
		return StrategyHash
	case "none", "":
		return StrategyNone
	default:
		return StrategyNone
	}
}