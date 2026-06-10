# Testing Strategy for provider-libvirt

## Overview

Provider-libvirt uses a three-tier testing approach:

1. **Unit Tests** - Test controller business logic (XML generation, state formatting, parameter validation)
2. **E2E Tests** - Test full reconciliation loop against containerized mock service
3. **Integration Tests** - Planned for real libvirtd daemon testing

## Unit Tests (Current Implementation)

### Test Coverage by Controller

Current coverage with 125+ test cases:
- **Domain**: 42.2% (XML generation, state formatting, parameters, console/graphics)
- **Network**: 17.4% (modes, XML generation, DHCP, IP ranges)
- **StoragePool**: 9.2% (pool types, path handling, target configuration)
- **Volume**: 2.0% (format validation, capacity handling, pool parameters)

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

**Domain Controller** (26 tests):
- XML generation: type/arch defaults, memory/vCPU configs, console/graphics
- State formatting: all 9 libvirt states (running, shutoff, paused, etc.)
- Bool conversion: true→1, false→0
- Parameter validation: name, memory, vCPU, type, arch

**Network Controller** (18 tests):
- All network modes: NAT, bridge, routed, isolated
- XML generation with IP ranges
- DHCP enable/disable
- Multiple IP configurations
- Domain parameter handling

**StoragePool Controller** (18 tests):
- All pool types: dir, fs, netfs, iscsi, logical, rbd, gluster, zfs
- XML generation with custom paths
- Pool naming and configuration
- Target path handling

**Volume Controller** (15 tests):
- Format validation: qcow2, raw, vmdk
- Capacity handling: 1GB, 10GB, 100GB
- Size/capacity priority
- Pool parameter preservation

### Unit Test Limitations

- Cannot test full Observe/Create/Update/Delete lifecycle (requires libvirt client mock)
- Cannot test error paths from actual libvirt daemon
- XML generation tested, but not libvirt processing of the XML

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

## Current Gaps & Future Work

### Implemented ✅
- 125+ unit tests across 4 controllers
- 42.2% coverage on Domain controller
- E2E framework with real HTTP API validation
- Mock bindings for testing

### Not Yet Implemented
- Full Observe/Create/Update/Delete lifecycle tests (requires libvirt client interface mock)
- Real libvirtd integration tests
- Error path testing (would need libvirt error simulation)
- Volume/StoragePool lifecycle coverage (9-2%)
- Cross-resource reference tests (VolumeRef, NetworkRef)

### Future Enhancements

**Short-term (would improve coverage to >70% on all controllers)**:
1. Create interface mock for clients.LibvirtClient
2. Add Observe/Create/Update/Delete tests with mocks
3. Test error scenarios (pool not found, invalid params)
4. Fix Secret controller test compilation

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
