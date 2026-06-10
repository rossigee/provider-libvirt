# Testing Strategy for provider-libvirt

## Overview

Provider-libvirt uses a three-tier testing approach:

1. **Unit Tests** - Test controller business logic (XML generation, state formatting, parameter validation)
2. **E2E Tests** - Test full reconciliation loop against containerized mock service
3. **Integration Tests** - Planned for real libvirtd daemon testing

## Unit Tests (Current Implementation)

### Test Coverage by Controller

Current coverage with 366 test cases across comprehensive test suites:
- **Domain**: 69.6% (XML generation, reference resolution, boot devices, machine type, WWN, console target, WaitForLease)
- **Network**: 79.1% (modes, XML generation, DHCP, IP ranges, DNS, domain config, STP delay, host reservations)
- **StoragePool**: 77.6% (all 8 pool types, path handling, target configuration, STP delay tuning)
- **Volume**: 61.0% (format validation, capacity handling, pool parameters, error paths)
- **Secret**: 36.5% (helper functions, all 4 secret types)

### Running Unit Tests

```bash
# Run all controller unit tests
go test ./internal/controller/... -v

# Run specific controller tests
go test ./internal/controller/domain -v

# With coverage report
go test ./internal/controller/domain -cover -v

# Run with detailed output
go test ./internal/controller/domain -v -run TestGenerateDomainXML
```

### What Unit Tests Cover

**Domain Controller** (42 tests):
- XML generation: type/arch defaults, memory/vCPU configs, console/graphics
- State formatting: all 9 libvirt states (running, shutoff, paused, etc.)
- Boot devices, machine type, WWN support, console target configuration
- WaitForLease network interface support
- Reference resolution: VolumeRef → Volume path, NetworkRef → Network name
- Error paths: DomainLookupByName, DomainDefineXML, DomainCreate, DomainDestroy failures
- Bool conversion: true→1, false→0
- Parameter validation: name, memory, vCPU, type, arch

**Network Controller** (28 tests):
- All network modes: NAT, bridge, routed, isolated
- XML generation with IP ranges, DNS configuration, domain support
- Bridge STP delay configuration with edge cases
- DHCP enable/disable with host reservations
- Multiple IP configurations
- Domain parameter handling
- Error paths: NetworkCreate, NetworkDestroy, NetworkGetXML failures

**StoragePool Controller** (31 tests):
- All 8 pool types: dir, fs, netfs, iscsi, logical, rbd, gluster, zfs
- XML generation with custom paths and pool-specific configurations
- Pool naming and configuration validation
- Target path and device handling
- Autostart and active state management
- Error paths: PoolCreate, PoolDestroy, PoolSetAutostart, PoolUndefine failures

**Volume Controller** (18 tests):
- Format validation: qcow2, raw, vmdk
- Capacity handling: 1GB, 10GB, 100GB
- Size/capacity priority and conversion
- Pool parameter preservation
- Error paths: StorageVolLookupByName, StorageVolCreateXML, StorageVolResize failures

**Secret Controller** (14 tests):
- All 4 secret types: volume, ceph, iscsi, vnc
- Secret name validation
- XML generation for each secret type

### Unit Test Strategy

**What We Test**:
- XML generation for all resource types and parameter combinations
- Reference resolution (VolumeRef/NetworkRef → libvirt resources)
- Parameter validation and state formatting
- Error paths for all major operations (using interface-based mocking)
- Cross-resource linking and dependency handling

**Testing Approach**:
- Interface-based mocking at DomainClient/NetworkClient/etc. level (not CGO types)
- Fake Kubernetes client for testing cross-resource references
- Comprehensive error injection via mock interfaces
- State machine transitions for all lifecycle operations

**What We Don't Test**:
- Full Observe/Create/Update/Delete lifecycle against real libvirt daemon
- Actual libvirt RPC protocol behavior
- CGO-specific edge cases with libvirt C bindings

## E2E Tests

E2E tests use containerized mock-libvirtd service and test the full HTTP API contract.

### Running E2E Tests

```bash
# Requires Docker and 2-5 minutes per test suite
go test ./internal/e2e/... -v -tags e2e

# Run specific lifecycle test
go test ./internal/e2e/... -v -tags e2e -run TestDomainLifecycle

# Run with longer timeout
go test ./internal/e2e/... -v -tags e2e -timeout 10m
```

### E2E Test Coverage

E2E tests verify the HTTP API contract with mock-libvirtd:

**Domain Operations**:
- List domains (GET /api/domains)
- Create domain (POST /api/domains)
- Get domain (GET /api/domains/{name})
- Update domain (PUT /api/domains/{name})
- Delete domain (DELETE /api/domains/{name})

**Network Operations**:
- Create network (POST /api/networks)
- List networks (GET /api/networks)
- Get network (GET /api/networks/{name})
- Delete network (DELETE /api/networks/{name})

**StoragePool Operations**:
- Create pool (POST /api/storage)
- List pools (GET /api/storage)
- Get pool (GET /api/storage/{name})
- Delete pool (DELETE /api/storage/{name})

### E2E Infrastructure

The test framework provides:

1. **TestEnvironment** - Container lifecycle management
   - Auto-pulls ghcr.io/rossigee/mock-libvirtd:latest
   - Starts container on localhost:8080
   - Health checks until ready
   - Automatic cleanup

2. **HTTPRequest Helper** - Makes REST API calls
   - JSON marshaling/unmarshaling
   - Status code validation
   - Request/response logging

## Mock-libvirtd vs Real libvirtd

### mock-libvirtd (via github.com/rossigee/mock-libvirtd)

**Pros:**
- Fast container startup (~2s)
- In-memory state, no I/O
- HTTP/REST interface for testing
- Good for testing controller HTTP API contract

**Cons:**
- Uses HTTP, not native libvirt RPC protocol
- Cannot test RPC-specific behavior
- No actual VM/domain capability

**Use case:** HTTP API validation, state transitions, error handling

### Real libvirtd

**Pros:**
- Tests actual RPC protocol
- Real VM lifecycle behavior
- Full feature compatibility

**Cons:**
- Slow container startup (~10s)
- Requires KVM/QEMU (heavy)
- Complex setup

**Use case:** Production validation, system integration

## Testing Pyramid

```
    ╔════════════════════════╗
    ║   E2E Tests (5%)       ║  Full stack, mock-libvirtd
    ║   ~/5 test suites      ║
    ╠════════════════════════╣
    ║  Integration Tests (0%)║  Would use real libvirtd
    ║  ~/0 test suites       ║  (not yet implemented)
    ╠════════════════════════╣
    ║  Unit Tests (95%)      ║  XML generation, validation
    ║  ~125 test cases       ║
    ╚════════════════════════╝
```

## Coverage Progress

### Implemented ✅
- **366 unit tests** across 5 controllers (Domain, Network, StoragePool, Volume, Secret)
- **69.6% coverage** on Domain controller
- **79.1% coverage** on Network controller
- **77.6% coverage** on StoragePool controller
- **61.0% coverage** on Volume controller
- **36.5% coverage** on Secret controller
- **Average coverage across core controllers: 71.0%**
- Interface-based mocking at client level (not CGO types)
- Kubernetes fake client for reference resolution tests
- Comprehensive error path testing for all operations
- Reference resolution infrastructure (VolumeRef, NetworkRef)
- E2E framework with real HTTP API validation

### Still To Implement
- Full Observe/Create/Update/Delete lifecycle tests (non-error paths need real/complex mocking)
- Real libvirtd integration tests
- Volume/Secret lifecycle coverage optimization
- CI/CD GitHub Actions integration

### Architecture Decisions

**Interface-Based Mocking** (Adopted in Phase 1):
- Mock at DomainClient/NetworkClient interface level instead of libvirt.Domain types
- Allows comprehensive error path testing without touching CGO bindings
- Enables full coverage of error handling in lifecycle methods
- Maintains clean separation between controller logic and external services

**Reference Resolution** (Implemented in Phase 3):
- Three-tier approach: Direct specification → Reference resolution → Error handling
- VolumeRef resolves to Volume resource → extracts path from status.AtProvider.Path
- NetworkRef resolves to Network resource → extracts name from spec.ForProvider.Name
- Tested with Kubernetes fake client for realistic reference scenarios

### Future Enhancements

**Medium-term**:
1. Real libvirtd container for true RPC testing
2. Parallel E2E test execution
3. Performance benchmarks
4. CI/CD GitHub Actions integration

## Practical Test Execution

### Quick test (15 seconds)
```bash
go test ./internal/controller/... -v
```

### Full test with coverage (2 minutes)
```bash
go test ./internal/controller/... ./internal/e2e/... -v -tags e2e -timeout 5m
```

### Test specific controller
```bash
go test ./internal/controller/domain -v -cover
```

### Test with output file
```bash
go test ./internal/controller/... -v > test-results.txt 2>&1
```

## Troubleshooting

### Unit Test Issues

**Test fails with "client is nil"**
- This is expected for Observe/Create/Update/Delete tests
- Helper methods (generateDomainXML, etc.) don't need client

**Coverage is lower than expected**
- Controller lifecycle methods (Observe, Create, etc.) need integration tests
- Mock client interface not yet implemented

### E2E Test Issues

**"docker: command not found"**
- Install Docker: https://docs.docker.com/install/
- Ensure Docker daemon is running: `systemctl start docker`

**"Container fails to start"**
- Check image: `docker images | grep mock-libvirtd`
- Pull manually: `docker pull ghcr.io/rossigee/mock-libvirtd:latest`
- View logs: `docker logs <container-id>`

**"Connection refused" on localhost:8080**
- Wait for container startup (~2 seconds)
- Check if port is bound: `netstat -tlnp | grep 8080`
- Verify mock-libvirtd started: `docker exec <id> curl http://localhost:8080/health`
