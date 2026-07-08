//go:build e2e
// +build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"testing"
	"time"
)

// TestEnvironment manages Docker containers for E2E testing
type TestEnvironment struct {
	t              *testing.T
	containerID    string
	containerImage string
	hostPort       string
	baseURL        string
	client         *http.Client
}

// SetupTestEnvironment starts a mock-libvirtd container
func SetupTestEnvironment(t *testing.T) *TestEnvironment {
	env := &TestEnvironment{
		t:              t,
		containerImage: "ghcr.io/rossigee/mock-libvirtd:latest",
		hostPort:       "8080",
		baseURL:        "http://localhost:8080",
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	// Pull image
	t.Log("Pulling mock-libvirtd image...")
	if err := env.pullImage(); err != nil {
		t.Fatalf("Failed to pull image: %v", err)
	}

	// Start container
	t.Log("Starting mock-libvirtd container...")
	if err := env.startContainer(); err != nil {
		t.Fatalf("Failed to start container: %v", err)
	}

	// Wait for readiness
	t.Log("Waiting for mock-libvirtd to be ready...")
	if err := env.waitReady(); err != nil {
		env.Cleanup()
		t.Fatalf("Container failed to become ready: %v", err)
	}

	t.Log("Test environment ready at " + env.baseURL)
	return env
}

// pullImage pulls the Docker image
func (e *TestEnvironment) pullImage() error {
	cmd := exec.Command("docker", "pull", e.containerImage)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker pull failed: %w: %s", err, stderr.String())
	}
	return nil
}

// startContainer starts the Docker container
func (e *TestEnvironment) startContainer() error {
	containerName := fmt.Sprintf("mock-libvirtd-e2e-%d", time.Now().UnixNano())
	cmd := exec.Command(
		"docker", "run",
		"-d",
		"-p", fmt.Sprintf("%s:8080", e.hostPort),
		"--name", containerName,
		e.containerImage,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker run failed: %w: %s", err, stderr.String())
	}

	e.containerID = string(bytes.TrimSpace(stdout.Bytes()))
	return nil
}

// waitReady polls the health endpoint until ready
func (e *TestEnvironment) waitReady() error {
	maxAttempts := 30
	for i := 0; i < maxAttempts; i++ {
		resp, err := e.client.Get(fmt.Sprintf("%s/health", e.baseURL))
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			return nil // Ready
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("container did not become ready after %d seconds", maxAttempts)
}

// Cleanup stops and removes the container
func (e *TestEnvironment) Cleanup() {
	if e.containerID == "" {
		return
	}

	cmd := exec.Command("docker", "rm", "-f", e.containerID)
	if err := cmd.Run(); err != nil {
		e.t.Logf("Warning: Failed to cleanup container: %v", err)
	}
}

// HTTPRequest makes an HTTP request and returns the response
func (e *TestEnvironment) HTTPRequest(method, path string, body interface{}) (int, interface{}, error) {
	url := fmt.Sprintf("%s%s", e.baseURL, path)

	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}

	var result interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return resp.StatusCode, respBody, nil
	}

	return resp.StatusCode, result, nil
}

// TestDomainLifecycle tests domain create/read/update/delete via HTTP API
func TestDomainLifecycle(t *testing.T) {
	env := SetupTestEnvironment(t)
	defer env.Cleanup()

	domainName := "test-vm"

	t.Run("ListDomains", func(t *testing.T) {
		status, result, err := env.HTTPRequest("GET", "/api/domains", nil)
		if err != nil {
			t.Fatalf("Failed to list domains: %v", err)
		}
		if status != 200 {
			t.Fatalf("Expected 200, got %d: %v", status, result)
		}
		t.Logf("Domains list: %v", result)
	})

	t.Run("CreateDomain", func(t *testing.T) {
		body := map[string]interface{}{
			"name":   domainName,
			"memory": 1073741824,
			"cpus":   2,
		}
		status, result, err := env.HTTPRequest("POST", "/api/domains", body)
		if err != nil {
			t.Fatalf("Failed to create domain: %v", err)
		}
		if status != 200 && status != 201 {
			t.Fatalf("Expected 200/201, got %d: %v", status, result)
		}
		t.Logf("Domain created: %v", result)
	})

	t.Run("GetDomain", func(t *testing.T) {
		status, result, err := env.HTTPRequest("GET", "/api/domains/"+domainName, nil)
		if err != nil {
			t.Fatalf("Failed to get domain: %v", err)
		}
		if status != 200 {
			t.Fatalf("Expected 200, got %d: %v", status, result)
		}
		t.Logf("Domain details: %v", result)
	})

	t.Run("UpdateDomain", func(t *testing.T) {
		body := map[string]interface{}{
			"state": "running",
		}
		status, result, err := env.HTTPRequest("PUT", "/api/domains/"+domainName, body)
		if err != nil {
			t.Fatalf("Failed to update domain: %v", err)
		}
		if status != 200 {
			t.Fatalf("Expected 200, got %d: %v", status, result)
		}
		t.Logf("Domain updated: %v", result)

		// Wait for state transition
		time.Sleep(2 * time.Second)

		// Verify state changed
		status, result, err = env.HTTPRequest("GET", "/api/domains/"+domainName, nil)
		if err != nil {
			t.Fatalf("Failed to verify domain state: %v", err)
		}
		t.Logf("Domain state after update: %v", result)
	})

	t.Run("DeleteDomain", func(t *testing.T) {
		status, result, err := env.HTTPRequest("DELETE", "/api/domains/"+domainName, nil)
		if err != nil {
			t.Fatalf("Failed to delete domain: %v", err)
		}
		if status != 200 && status != 204 {
			t.Fatalf("Expected 200/204, got %d: %v", status, result)
		}
		t.Logf("Domain deleted")
	})
}

// TestNetworkLifecycle tests network create/read/delete via HTTP API
func TestNetworkLifecycle(t *testing.T) {
	env := SetupTestEnvironment(t)
	defer env.Cleanup()

	networkName := "test-net"

	t.Run("CreateNetwork", func(t *testing.T) {
		body := map[string]interface{}{
			"name":   networkName,
			"bridge": "virbr1",
		}
		status, result, err := env.HTTPRequest("POST", "/api/networks", body)
		if err != nil {
			t.Fatalf("Failed to create network: %v", err)
		}
		if status != 200 && status != 201 {
			t.Fatalf("Expected 200/201, got %d: %v", status, result)
		}
		t.Logf("Network created: %v", result)
	})

	t.Run("ListNetworks", func(t *testing.T) {
		status, result, err := env.HTTPRequest("GET", "/api/networks", nil)
		if err != nil {
			t.Fatalf("Failed to list networks: %v", err)
		}
		if status != 200 {
			t.Fatalf("Expected 200, got %d: %v", status, result)
		}
		t.Logf("Networks list: %v", result)
	})

	t.Run("GetNetwork", func(t *testing.T) {
		status, result, err := env.HTTPRequest("GET", "/api/networks/"+networkName, nil)
		if err != nil {
			t.Fatalf("Failed to get network: %v", err)
		}
		if status != 200 {
			t.Fatalf("Expected 200, got %d: %v", status, result)
		}
		t.Logf("Network details: %v", result)
	})

	t.Run("DeleteNetwork", func(t *testing.T) {
		status, result, err := env.HTTPRequest("DELETE", "/api/networks/"+networkName, nil)
		if err != nil {
			t.Fatalf("Failed to delete network: %v", err)
		}
		if status != 200 && status != 204 {
			t.Fatalf("Expected 200/204, got %d: %v", status, result)
		}
		t.Logf("Network deleted")
	})
}

// TestStoragePoolLifecycle tests storage pool create/read/delete via HTTP API
func TestStoragePoolLifecycle(t *testing.T) {
	env := SetupTestEnvironment(t)
	defer env.Cleanup()

	poolName := "test-storage"

	t.Run("CreateStoragePool", func(t *testing.T) {
		body := map[string]interface{}{
			"name":     poolName,
			"type":     "dir",
			"path":     "/tmp/libvirt-storage",
			"capacity": 107374182400, // 100GB
		}
		status, result, err := env.HTTPRequest("POST", "/api/storage", body)
		if err != nil {
			t.Fatalf("Failed to create storage pool: %v", err)
		}
		if status != 200 && status != 201 {
			t.Fatalf("Expected 200/201, got %d: %v", status, result)
		}
		t.Logf("Storage pool created: %v", result)
	})

	t.Run("ListStoragePools", func(t *testing.T) {
		status, result, err := env.HTTPRequest("GET", "/api/storage", nil)
		if err != nil {
			t.Fatalf("Failed to list storage pools: %v", err)
		}
		if status != 200 {
			t.Fatalf("Expected 200, got %d: %v", status, result)
		}
		t.Logf("Storage pools list: %v", result)
	})

	t.Run("GetStoragePool", func(t *testing.T) {
		status, result, err := env.HTTPRequest("GET", "/api/storage/"+poolName, nil)
		if err != nil {
			t.Fatalf("Failed to get storage pool: %v", err)
		}
		if status != 200 {
			t.Fatalf("Expected 200, got %d: %v", status, result)
		}
		t.Logf("Storage pool details: %v", result)
	})

	t.Run("DeleteStoragePool", func(t *testing.T) {
		status, result, err := env.HTTPRequest("DELETE", "/api/storage/"+poolName, nil)
		if err != nil {
			t.Fatalf("Failed to delete storage pool: %v", err)
		}
		if status != 200 && status != 204 {
			t.Fatalf("Expected 200/204, got %d: %v", status, result)
		}
		t.Logf("Storage pool deleted")
	})
}
