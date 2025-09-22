/*
Copyright 2025 Ross Golder
*/

package clients

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/digitalocean/go-libvirt"
	"github.com/pkg/errors"
)

// VirshClient implements libvirt operations using virsh command-line interface
type VirshClient struct {
	uri string
}

// NewVirshClient creates a new virsh-based libvirt client
func NewVirshClient(uri string) *VirshClient {
	return &VirshClient{
		uri: uri,
	}
}

// Close is a no-op for virsh client since it doesn't maintain persistent connections
func (c *VirshClient) Close() error {
	return nil
}

// execVirsh executes a virsh command with the configured URI
func (c *VirshClient) execVirsh(ctx context.Context, args ...string) ([]byte, error) {
	// Build command with URI
	cmdArgs := []string{"--connect", c.uri}
	cmdArgs = append(cmdArgs, args...)

	cmd := exec.CommandContext(ctx, "virsh", cmdArgs...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("virsh command failed: %v\nstderr: %s", err, stderr.String())
	}

	return stdout.Bytes(), nil
}

// DomainLookupByName looks up a domain by name
func (c *VirshClient) DomainLookupByName(name string) (libvirt.Domain, error) {
	ctx := context.Background()

	// Use 'virsh dominfo' to check if domain exists
	output, err := c.execVirsh(ctx, "dominfo", name)
	if err != nil {
		// Check if domain not found
		if strings.Contains(err.Error(), "Domain not found") ||
		   strings.Contains(err.Error(), "no domain with matching name") {
			return libvirt.Domain{}, errors.New("Domain not found: no domain with matching name")
		}
		return libvirt.Domain{}, err
	}

	// Parse domain info to get ID and UUID
	info := string(output)
	lines := strings.Split(info, "\n")

	domain := libvirt.Domain{
		Name: name,
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Id:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 && parts[1] != "-" {
				if id, err := strconv.Atoi(parts[1]); err == nil {
					domain.ID = int32(id)
				}
			}
		} else if strings.HasPrefix(line, "UUID:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				if uuid, err := StringToUUID(parts[1]); err == nil {
					domain.UUID = uuid
				}
			}
		}
	}

	return domain, nil
}

// DomainGetState gets domain state
func (c *VirshClient) DomainGetState(domain libvirt.Domain, flags uint32) (int32, int32, error) {
	ctx := context.Background()

	output, err := c.execVirsh(ctx, "domstate", domain.Name)
	if err != nil {
		return 0, 0, err
	}

	state := strings.TrimSpace(string(output))
	var domainState int32

	switch state {
	case "running":
		domainState = int32(libvirt.DomainRunning)
	case "shut off":
		domainState = int32(libvirt.DomainShutoff)
	case "paused":
		domainState = int32(libvirt.DomainPaused)
	case "in shutdown":
		domainState = int32(libvirt.DomainShutdown)
	case "blocked":
		domainState = int32(libvirt.DomainBlocked)
	case "crashed":
		domainState = int32(libvirt.DomainCrashed)
	case "pmsuspended":
		domainState = int32(libvirt.DomainPmsuspended)
	default:
		domainState = int32(libvirt.DomainNostate)
	}

	return domainState, 0, nil // reason is always 0 for compatibility
}

// DomainGetInfo gets domain information
func (c *VirshClient) DomainGetInfo(domain libvirt.Domain) (int8, uint64, uint64, uint32, uint64, error) {
	ctx := context.Background()

	output, err := c.execVirsh(ctx, "dominfo", domain.Name)
	if err != nil {
		return 0, 0, 0, 0, 0, err
	}

	info := string(output)
	lines := strings.Split(info, "\n")

	var state int8 = int8(libvirt.DomainNostate)
	var maxMem uint64 = 0
	var memory uint64 = 0
	var nrVirtCpu uint32 = 0
	var cpuTime uint64 = 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "State:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				switch parts[1] {
				case "running":
					state = int8(libvirt.DomainRunning)
				case "shut":
					state = int8(libvirt.DomainShutoff)
				case "paused":
					state = int8(libvirt.DomainPaused)
				case "in":
					state = int8(libvirt.DomainShutdown)
				case "blocked":
					state = int8(libvirt.DomainBlocked)
				case "crashed":
					state = int8(libvirt.DomainCrashed)
				case "pmsuspended":
					state = int8(libvirt.DomainPmsuspended)
				}
			}
		} else if strings.HasPrefix(line, "Max memory:") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				if mem, err := strconv.ParseUint(parts[2], 10, 64); err == nil {
					maxMem = mem * 1024 // Convert from KiB to bytes
				}
			}
		} else if strings.HasPrefix(line, "Used memory:") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				if mem, err := strconv.ParseUint(parts[2], 10, 64); err == nil {
					memory = mem * 1024 // Convert from KiB to bytes
				}
			}
		} else if strings.HasPrefix(line, "CPU(s):") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				if cpus, err := strconv.ParseUint(parts[1], 10, 32); err == nil {
					nrVirtCpu = uint32(cpus)
				}
			}
		} else if strings.HasPrefix(line, "CPU time:") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				// Parse CPU time (format like "123.4s")
				timeStr := parts[2]
				if strings.HasSuffix(timeStr, "s") {
					timeStr = strings.TrimSuffix(timeStr, "s")
					if time, err := strconv.ParseFloat(timeStr, 64); err == nil {
						cpuTime = uint64(time * 1000000000) // Convert to nanoseconds
					}
				}
			}
		}
	}

	return state, maxMem, memory, nrVirtCpu, cpuTime, nil
}

// DomainDefineXML defines a domain from XML
func (c *VirshClient) DomainDefineXML(xml string) (libvirt.Domain, error) {
	ctx := context.Background()

	// Create a temporary file for the XML
	cmd := exec.CommandContext(ctx, "virsh", "--connect", c.uri, "define", "/dev/stdin")
	cmd.Stdin = strings.NewReader(xml)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return libvirt.Domain{}, fmt.Errorf("virsh define failed: %v\nstderr: %s", err, stderr.String())
	}

	// Extract domain name from XML to return domain object
	// This is a simple extraction - in practice you'd use a proper XML parser
	nameStart := strings.Index(xml, "<name>")
	nameEnd := strings.Index(xml, "</name>")
	if nameStart == -1 || nameEnd == -1 || nameEnd <= nameStart {
		return libvirt.Domain{}, errors.New("could not extract domain name from XML")
	}

	domainName := xml[nameStart+6 : nameEnd]
	return c.DomainLookupByName(domainName)
}

// DomainCreate starts a domain
func (c *VirshClient) DomainCreate(domain libvirt.Domain) error {
	ctx := context.Background()
	_, err := c.execVirsh(ctx, "start", domain.Name)
	return err
}

// DomainDestroy forcefully stops a domain
func (c *VirshClient) DomainDestroy(domain libvirt.Domain) error {
	ctx := context.Background()
	_, err := c.execVirsh(ctx, "destroy", domain.Name)
	return err
}

// DomainShutdown gracefully shuts down a domain
func (c *VirshClient) DomainShutdown(domain libvirt.Domain) error {
	ctx := context.Background()
	_, err := c.execVirsh(ctx, "shutdown", domain.Name)
	return err
}

// DomainUndefine undefines a domain
func (c *VirshClient) DomainUndefine(domain libvirt.Domain) error {
	ctx := context.Background()
	_, err := c.execVirsh(ctx, "undefine", domain.Name)
	return err
}

// DomainSetAutostart sets domain autostart
func (c *VirshClient) DomainSetAutostart(domain libvirt.Domain, autostart int32) error {
	ctx := context.Background()

	var cmd string
	if autostart != 0 {
		cmd = "autostart"
	} else {
		cmd = "autostart"
	}

	args := []string{cmd}
	if autostart == 0 {
		args = append(args, "--disable")
	}
	args = append(args, domain.Name)

	_, err := c.execVirsh(ctx, args...)
	return err
}

// NetworkLookupByName looks up a network by name
func (c *VirshClient) NetworkLookupByName(name string) (libvirt.Network, error) {
	ctx := context.Background()

	// Use 'virsh net-info' to check if network exists
	output, err := c.execVirsh(ctx, "net-info", name)
	if err != nil {
		// Check if network not found
		if strings.Contains(err.Error(), "Network not found") ||
		   strings.Contains(err.Error(), "no network with matching name") {
			return libvirt.Network{}, errors.New("Network not found: no network with matching name")
		}
		return libvirt.Network{}, err
	}

	// Parse network info to get UUID
	info := string(output)
	lines := strings.Split(info, "\n")

	network := libvirt.Network{
		Name: name,
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "UUID:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				if uuid, err := StringToUUID(parts[1]); err == nil {
					network.UUID = uuid
				}
			}
		}
	}

	return network, nil
}

// Version gets libvirt version (placeholder implementation)
func (c *VirshClient) Version() (uint32, error) {
	ctx := context.Background()

	output, err := c.execVirsh(ctx, "version", "--daemon")
	if err != nil {
		return 0, err
	}

	// Parse version from output - this is simplified
	info := string(output)
	lines := strings.Split(info, "\n")

	for _, line := range lines {
		if strings.Contains(line, "libvirt") && strings.Contains(line, "version:") {
			// Extract version number - simplified parsing
			parts := strings.Fields(line)
			for i, part := range parts {
				if part == "version:" && i+1 < len(parts) {
					versionStr := parts[i+1]
					// Convert version string to uint32 (simplified)
					if strings.Contains(versionStr, ".") {
						parts := strings.Split(versionStr, ".")
						if len(parts) >= 2 {
							major, _ := strconv.Atoi(parts[0])
							minor, _ := strconv.Atoi(parts[1])
							return uint32(major*1000000 + minor*1000), nil
						}
					}
				}
			}
		}
	}

	return 11000000, nil // Default to version 11.0.0
}

// Additional network operations needed by the controllers
func (c *VirshClient) NetworkIsActive(net libvirt.Network) (int32, error) {
	ctx := context.Background()

	output, err := c.execVirsh(ctx, "net-info", net.Name)
	if err != nil {
		return 0, err
	}

	info := string(output)
	if strings.Contains(info, "Active:") {
		if strings.Contains(info, "Active: yes") {
			return 1, nil
		}
	}

	return 0, nil
}

func (c *VirshClient) NetworkIsPersistent(net libvirt.Network) (int32, error) {
	ctx := context.Background()

	output, err := c.execVirsh(ctx, "net-info", net.Name)
	if err != nil {
		return 0, err
	}

	info := string(output)
	if strings.Contains(info, "Persistent:") {
		if strings.Contains(info, "Persistent: yes") {
			return 1, nil
		}
	}

	return 0, nil
}

func (c *VirshClient) NetworkGetAutostart(net libvirt.Network) (int32, error) {
	ctx := context.Background()

	output, err := c.execVirsh(ctx, "net-info", net.Name)
	if err != nil {
		return 0, err
	}

	info := string(output)
	if strings.Contains(info, "Autostart:") {
		if strings.Contains(info, "Autostart: yes") {
			return 1, nil
		}
	}

	return 0, nil
}

func (c *VirshClient) NetworkSetAutostart(net libvirt.Network, autostart int32) error {
	ctx := context.Background()

	args := []string{"net-autostart"}
	if autostart == 0 {
		args = append(args, "--disable")
	}
	args = append(args, net.Name)

	_, err := c.execVirsh(ctx, args...)
	return err
}

func (c *VirshClient) NetworkGetXMLDesc(net libvirt.Network, flags uint32) (string, error) {
	ctx := context.Background()

	output, err := c.execVirsh(ctx, "net-dumpxml", net.Name)
	if err != nil {
		return "", err
	}

	return string(output), nil
}

func (c *VirshClient) NetworkDefineXML(xml string) (libvirt.Network, error) {
	ctx := context.Background()

	cmd := exec.CommandContext(ctx, "virsh", "--connect", c.uri, "net-define", "/dev/stdin")
	cmd.Stdin = strings.NewReader(xml)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return libvirt.Network{}, fmt.Errorf("virsh net-define failed: %v\nstderr: %s", err, stderr.String())
	}

	// Extract network name from XML
	nameStart := strings.Index(xml, "<name>")
	nameEnd := strings.Index(xml, "</name>")
	if nameStart == -1 || nameEnd == -1 || nameEnd <= nameStart {
		return libvirt.Network{}, errors.New("could not extract network name from XML")
	}

	networkName := xml[nameStart+6 : nameEnd]
	return c.NetworkLookupByName(networkName)
}

func (c *VirshClient) NetworkCreate(net libvirt.Network) error {
	ctx := context.Background()
	_, err := c.execVirsh(ctx, "net-start", net.Name)
	return err
}

func (c *VirshClient) NetworkDestroy(net libvirt.Network) error {
	ctx := context.Background()
	_, err := c.execVirsh(ctx, "net-destroy", net.Name)
	return err
}

func (c *VirshClient) NetworkUndefine(net libvirt.Network) error {
	ctx := context.Background()
	_, err := c.execVirsh(ctx, "net-undefine", net.Name)
	return err
}

// Storage Pool operations
func (c *VirshClient) StoragePoolLookupByName(name string) (libvirt.StoragePool, error) {
	ctx := context.Background()

	// Use 'virsh pool-info' to check if pool exists
	output, err := c.execVirsh(ctx, "pool-info", name)
	if err != nil {
		// Check if pool not found
		if strings.Contains(err.Error(), "Storage pool not found") ||
		   strings.Contains(err.Error(), "no storage pool with matching name") {
			return libvirt.StoragePool{}, errors.New("Storage pool not found: no storage pool with matching name")
		}
		return libvirt.StoragePool{}, err
	}

	// Parse pool info to get UUID
	info := string(output)
	lines := strings.Split(info, "\n")

	pool := libvirt.StoragePool{
		Name: name,
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "UUID:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				if uuid, err := StringToUUID(parts[1]); err == nil {
					pool.UUID = uuid
				}
			}
		}
	}

	return pool, nil
}

func (c *VirshClient) StoragePoolIsActive(pool libvirt.StoragePool) (int32, error) {
	ctx := context.Background()

	output, err := c.execVirsh(ctx, "pool-info", pool.Name)
	if err != nil {
		return 0, err
	}

	info := string(output)
	if strings.Contains(info, "State:") {
		if strings.Contains(info, "State: running") {
			return 1, nil
		}
	}

	return 0, nil
}

func (c *VirshClient) StoragePoolIsPersistent(pool libvirt.StoragePool) (int32, error) {
	ctx := context.Background()

	output, err := c.execVirsh(ctx, "pool-info", pool.Name)
	if err != nil {
		return 0, err
	}

	info := string(output)
	if strings.Contains(info, "Persistent:") {
		if strings.Contains(info, "Persistent: yes") {
			return 1, nil
		}
	}

	return 0, nil
}

func (c *VirshClient) StoragePoolGetAutostart(pool libvirt.StoragePool) (int32, error) {
	ctx := context.Background()

	output, err := c.execVirsh(ctx, "pool-info", pool.Name)
	if err != nil {
		return 0, err
	}

	info := string(output)
	if strings.Contains(info, "Autostart:") {
		if strings.Contains(info, "Autostart: yes") {
			return 1, nil
		}
	}

	return 0, nil
}

func (c *VirshClient) StoragePoolSetAutostart(pool libvirt.StoragePool, autostart int32) error {
	ctx := context.Background()

	args := []string{"pool-autostart"}
	if autostart == 0 {
		args = append(args, "--disable")
	}
	args = append(args, pool.Name)

	_, err := c.execVirsh(ctx, args...)
	return err
}

func (c *VirshClient) StoragePoolGetInfo(pool libvirt.StoragePool) (uint8, uint64, uint64, uint64, error) {
	ctx := context.Background()

	output, err := c.execVirsh(ctx, "pool-info", pool.Name)
	if err != nil {
		return 0, 0, 0, 0, err
	}

	info := string(output)
	lines := strings.Split(info, "\n")

	var state uint8 = uint8(libvirt.StoragePoolInactive)
	var capacity uint64 = 0
	var allocation uint64 = 0
	var available uint64 = 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "State:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 && parts[1] == "running" {
				state = uint8(libvirt.StoragePoolRunning)
			}
		} else if strings.HasPrefix(line, "Capacity:") {
			// Parse capacity (format like "123.45 GiB")
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				if cap, err := strconv.ParseFloat(parts[1], 64); err == nil {
					// Convert to bytes (simplified conversion)
					capacity = uint64(cap * 1024 * 1024 * 1024)
				}
			}
		} else if strings.HasPrefix(line, "Allocation:") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				if alloc, err := strconv.ParseFloat(parts[1], 64); err == nil {
					allocation = uint64(alloc * 1024 * 1024 * 1024)
				}
			}
		} else if strings.HasPrefix(line, "Available:") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				if avail, err := strconv.ParseFloat(parts[1], 64); err == nil {
					available = uint64(avail * 1024 * 1024 * 1024)
				}
			}
		}
	}

	return state, capacity, allocation, available, nil
}

func (c *VirshClient) StoragePoolListAllVolumes(pool libvirt.StoragePool, flags int32, maxVols uint32) ([]libvirt.StorageVol, uint32, error) {
	ctx := context.Background()

	output, err := c.execVirsh(ctx, "vol-list", pool.Name)
	if err != nil {
		return []libvirt.StorageVol{}, 0, err
	}

	// Parse volume list (simplified)
	lines := strings.Split(string(output), "\n")
	var volumes []libvirt.StorageVol

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Name") || strings.HasPrefix(line, "----") {
			continue
		}

		// Parse volume name from line
		parts := strings.Fields(line)
		if len(parts) >= 1 {
			volumes = append(volumes, libvirt.StorageVol{
				Name: parts[0],
				Pool: pool.Name,
			})
		}
	}

	return volumes, uint32(len(volumes)), nil
}

func (c *VirshClient) StoragePoolDefineXML(xml string, flags int32) (libvirt.StoragePool, error) {
	ctx := context.Background()

	cmd := exec.CommandContext(ctx, "virsh", "--connect", c.uri, "pool-define", "/dev/stdin")
	cmd.Stdin = strings.NewReader(xml)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return libvirt.StoragePool{}, fmt.Errorf("virsh pool-define failed: %v\nstderr: %s", err, stderr.String())
	}

	// Extract pool name from XML
	nameStart := strings.Index(xml, "<name>")
	nameEnd := strings.Index(xml, "</name>")
	if nameStart == -1 || nameEnd == -1 || nameEnd <= nameStart {
		return libvirt.StoragePool{}, errors.New("could not extract pool name from XML")
	}

	poolName := xml[nameStart+6 : nameEnd]
	return c.StoragePoolLookupByName(poolName)
}

func (c *VirshClient) StoragePoolBuild(pool libvirt.StoragePool, flags int32) error {
	ctx := context.Background()
	_, err := c.execVirsh(ctx, "pool-build", pool.Name)
	return err
}

func (c *VirshClient) StoragePoolCreate(pool libvirt.StoragePool, flags int32) error {
	ctx := context.Background()
	_, err := c.execVirsh(ctx, "pool-start", pool.Name)
	return err
}

func (c *VirshClient) StoragePoolDestroy(pool libvirt.StoragePool) error {
	ctx := context.Background()
	_, err := c.execVirsh(ctx, "pool-destroy", pool.Name)
	return err
}

func (c *VirshClient) StoragePoolUndefine(pool libvirt.StoragePool) error {
	ctx := context.Background()
	_, err := c.execVirsh(ctx, "pool-undefine", pool.Name)
	return err
}

// Storage Volume operations
func (c *VirshClient) StorageVolLookupByName(pool libvirt.StoragePool, name string) (libvirt.StorageVol, error) {
	ctx := context.Background()

	// Use 'virsh vol-info' to check if volume exists
	_, err := c.execVirsh(ctx, "vol-info", name, "--pool", pool.Name)
	if err != nil {
		// Check if volume not found
		if strings.Contains(err.Error(), "Volume not found") ||
		   strings.Contains(err.Error(), "no storage vol with matching name") {
			return libvirt.StorageVol{}, errors.New("Volume not found: no storage vol with matching name")
		}
		return libvirt.StorageVol{}, err
	}

	// Parse volume info
	volume := libvirt.StorageVol{
		Name: name,
		Pool: pool.Name,
	}

	return volume, nil
}

func (c *VirshClient) StorageVolGetInfo(vol libvirt.StorageVol) (int8, uint64, uint64, error) {
	ctx := context.Background()

	output, err := c.execVirsh(ctx, "vol-info", vol.Name, "--pool", vol.Pool)
	if err != nil {
		return 0, 0, 0, err
	}

	info := string(output)
	lines := strings.Split(info, "\n")

	var volType int8 = int8(libvirt.StorageVolFile)
	var capacity uint64 = 0
	var allocation uint64 = 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Type:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 && parts[1] == "file" {
				volType = int8(libvirt.StorageVolFile)
			}
		} else if strings.HasPrefix(line, "Capacity:") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				if cap, err := strconv.ParseFloat(parts[1], 64); err == nil {
					capacity = uint64(cap * 1024 * 1024 * 1024)
				}
			}
		} else if strings.HasPrefix(line, "Allocation:") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				if alloc, err := strconv.ParseFloat(parts[1], 64); err == nil {
					allocation = uint64(alloc * 1024 * 1024 * 1024)
				}
			}
		}
	}

	return volType, capacity, allocation, nil
}

func (c *VirshClient) StorageVolGetXMLDesc(vol libvirt.StorageVol, flags int32) (string, error) {
	ctx := context.Background()

	output, err := c.execVirsh(ctx, "vol-dumpxml", vol.Name, "--pool", vol.Pool)
	if err != nil {
		return "", err
	}

	return string(output), nil
}

func (c *VirshClient) StorageVolCreateXML(pool libvirt.StoragePool, xml string, flags libvirt.StorageVolCreateFlags) (libvirt.StorageVol, error) {
	ctx := context.Background()

	cmd := exec.CommandContext(ctx, "virsh", "--connect", c.uri, "vol-create", pool.Name, "/dev/stdin")
	cmd.Stdin = strings.NewReader(xml)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return libvirt.StorageVol{}, fmt.Errorf("virsh vol-create failed: %v\nstderr: %s", err, stderr.String())
	}

	// Extract volume name from XML
	nameStart := strings.Index(xml, "<name>")
	nameEnd := strings.Index(xml, "</name>")
	if nameStart == -1 || nameEnd == -1 || nameEnd <= nameStart {
		return libvirt.StorageVol{}, errors.New("could not extract volume name from XML")
	}

	volumeName := xml[nameStart+6 : nameEnd]
	return c.StorageVolLookupByName(pool, volumeName)
}

func (c *VirshClient) StorageVolDelete(vol libvirt.StorageVol, flags libvirt.StorageVolDeleteFlags) error {
	ctx := context.Background()
	_, err := c.execVirsh(ctx, "vol-delete", vol.Name, "--pool", vol.Pool)
	return err
}

func (c *VirshClient) StorageVolResize(vol libvirt.StorageVol, capacity uint64, flags libvirt.StorageVolResizeFlags) error {
	ctx := context.Background()

	// Convert bytes to human readable format (simple GiB conversion)
	capacityGiB := capacity / (1024 * 1024 * 1024)
	capacityStr := fmt.Sprintf("%dG", capacityGiB)

	_, err := c.execVirsh(ctx, "vol-resize", vol.Name, capacityStr, "--pool", vol.Pool)
	return err
}

func (c *VirshClient) StorageVolCreateXMLFrom(pool libvirt.StoragePool, xml string, originalVol libvirt.StorageVol, flags libvirt.StorageVolCreateFlags) (libvirt.StorageVol, error) {
	ctx := context.Background()

	cmd := exec.CommandContext(ctx, "virsh", "--connect", c.uri, "vol-create-from", pool.Name, "/dev/stdin", originalVol.Name)
	cmd.Stdin = strings.NewReader(xml)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return libvirt.StorageVol{}, fmt.Errorf("virsh vol-create-from failed: %v\nstderr: %s", err, stderr.String())
	}

	// Extract volume name from XML
	nameStart := strings.Index(xml, "<name>")
	nameEnd := strings.Index(xml, "</name>")
	if nameStart == -1 || nameEnd == -1 || nameEnd <= nameStart {
		return libvirt.StorageVol{}, errors.New("could not extract volume name from XML")
	}

	volumeName := xml[nameStart+6 : nameEnd]
	return c.StorageVolLookupByName(pool, volumeName)
}

func (c *VirshClient) StorageVolGetPath(vol libvirt.StorageVol) (string, error) {
	ctx := context.Background()

	output, err := c.execVirsh(ctx, "vol-path", vol.Name, "--pool", vol.Pool)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

// Secret operations
func (c *VirshClient) SecretLookupByUUID(uuid [16]byte) (libvirt.Secret, error) {
	ctx := context.Background()
	uuidStr := UUIDToString(uuid)

	// Use 'virsh secret-get-value' to check if secret exists
	_, err := c.execVirsh(ctx, "secret-get-value", uuidStr)
	if err != nil {
		// Check if secret not found
		if strings.Contains(err.Error(), "Secret not found") ||
		   strings.Contains(err.Error(), "no secret with matching uuid") {
			return libvirt.Secret{}, errors.New("Secret not found: no secret with matching uuid")
		}
		return libvirt.Secret{}, err
	}

	return libvirt.Secret{
		UUID: uuid,
	}, nil
}

func (c *VirshClient) SecretDefineXML(xml string, flags int32) (libvirt.Secret, error) {
	ctx := context.Background()

	cmd := exec.CommandContext(ctx, "virsh", "--connect", c.uri, "secret-define", "/dev/stdin")
	cmd.Stdin = strings.NewReader(xml)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return libvirt.Secret{}, fmt.Errorf("virsh secret-define failed: %v\nstderr: %s", err, stderr.String())
	}

	// Extract UUID from output or XML
	output := stdout.String()
	if strings.Contains(output, "Secret") && strings.Contains(output, "created") {
		// Parse UUID from output
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			if strings.Contains(line, "Secret") && strings.Contains(line, "created") {
				parts := strings.Fields(line)
				for _, part := range parts {
					if len(part) == 36 && strings.Count(part, "-") == 4 {
						if uuid, err := StringToUUID(part); err == nil {
							return libvirt.Secret{UUID: uuid}, nil
						}
					}
				}
			}
		}
	}

	return libvirt.Secret{}, errors.New("could not extract secret UUID from output")
}

func (c *VirshClient) SecretSetValue(secret libvirt.Secret, value []byte, flags int32) error {
	ctx := context.Background()
	uuidStr := UUIDToString(secret.UUID)

	cmd := exec.CommandContext(ctx, "virsh", "--connect", c.uri, "secret-set-value", uuidStr, "/dev/stdin")
	cmd.Stdin = bytes.NewReader(value)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return err
}

func (c *VirshClient) SecretUndefine(secret libvirt.Secret) error {
	ctx := context.Background()
	uuidStr := UUIDToString(secret.UUID)
	_, err := c.execVirsh(ctx, "secret-undefine", uuidStr)
	return err
}