/*
Copyright 2025 Ross Golder
*/

package webhook

import (
	"regexp"
	"strings"
)

// contains checks if a slice contains a specific value
func contains(slice []string, value string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}

// matchesPattern checks if a string matches a regular expression pattern
func matchesPattern(value, pattern string) bool {
	matched, err := regexp.MatchString(pattern, value)
	if err != nil {
		return false
	}
	return matched
}

// splitHostPort splits a host:port string, handling various formats
func splitHostPort(hostPort string) []string {
	// Handle IPv6 addresses with ports: [::1]:8080
	if strings.HasPrefix(hostPort, "[") {
		if idx := strings.LastIndex(hostPort, "]:"); idx >= 0 {
			host := hostPort[1:idx]
			port := hostPort[idx+2:]
			return []string{host, port}
		}
		// IPv6 without port: [::1]
		if strings.HasSuffix(hostPort, "]") {
			return []string{hostPort[1 : len(hostPort)-1]}
		}
	}
	
	// Handle IPv4 addresses with ports: 192.168.1.1:8080
	if idx := strings.LastIndex(hostPort, ":"); idx >= 0 {
		// Make sure this isn't an IPv6 address without brackets
		if strings.Count(hostPort, ":") == 1 {
			host := hostPort[:idx]
			port := hostPort[idx+1:]
			return []string{host, port}
		}
	}
	
	// Host only (IPv4, IPv6 without brackets, or hostname)
	return []string{hostPort}
}