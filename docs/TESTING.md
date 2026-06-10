# Testing Strategy for provider-libvirt

## Overview

Provider-libvirt uses a three-tier testing approach:

1. **Unit Tests** - Mock libvirt bindings, test controller logic in isolation
2. **Integration Tests** - Mock HTTP service, test reconciliation
3. **E2E Tests** - Real/containerized libvirtd, test full stack

## Unit Tests (Current Implementation)

Unit tests use **mock libvirt bindings** that implement the `libvirt.Domain`, `libvirt.Network`, `libvirt.StoragePool`, and `libvirt.StorageVol` interfaces.

### Running Unit Tests

```bash
# Run all unit tests
go test ./internal/controller/... -v

# Run specific controller tests
go test ./internal/controller/domain -v -run TestGenerateDomainXML

# With coverage
go test ./internal/controller/... -cover -v
```

### Using Mock Bindings

Create a mock client in your tests:

```go
import "github.com/rossigee/provider-libvirt/internal/controller/mocks"

func TestMyController(t *testing.T) {
	mockClient := mocks.NewMockLibvirtClient()
	
	// Configure mock behavior
	mockClient.DomainLookupByNameFn = func(name string) (*mocks.MockDomain, error) {
		return &mocks.MockDomain{
			Name:  name,
			State: libvirt.DOMAIN_RUNNING,
			UUID:  "test-uuid",
		}, nil
	}
	
	// Use in tests
	ext := &external{client: mockClient}
	obs, err := ext.Observe(context.Background(), domain)
	// assertions...
}
```

### Test Coverage

Current coverage by controller:
- **Domain**: 29.5% (XML generation, state formatting, parameters)
- **Volume**: 2.0% (high-level helpers, format validation)
- **Network**: 16.3% (mode validation, XML generation)
- **StoragePool**: 9.2% (type validation, path handling)

## Integration Tests (Planned)

Integration tests would use mock-libvirtd HTTP service:

```bash
# Not yet implemented
go test ./internal/integration/... -v
```

## E2E Tests (Framework Ready)

E2E tests use containerized services and test the full reconciliation loop.

### Running E2E Tests

```bash
# Run all E2E tests (requires Docker)
go test ./internal/e2e/... -v -tags e2e

# Run specific test
go test ./internal/e2e/... -v -tags e2e -run TestDomainLifecycle
```

### E2E Infrastructure

The test framework (`internal/e2e/e2e_test.go`) provides:

1. **TestEnvironment** - Manages container lifecycle
   - Pulls docker image
   - Starts mock-libvirtd container
   - Waits for health readiness
   - Cleans up on test completion

2. **Helper Functions**
   - `SetupTestEnvironment(t *testing.T) *TestEnvironment`
   - `(e *TestEnvironment) Cleanup()`
   - `(e *TestEnvironment) LibvirtURI() string`

### E2E Test Structure

```go
func TestDomainLifecycle(t *testing.T) {
	// Setup container
	env := SetupTestEnvironment(t)
	defer env.Cleanup()

	// Get libvirt URI for connection
	uri := env.libvirtURI // "qemu+tcp://localhost:16509/system"

	// Test domain operations
	t.Run("CreateDomain", func(t *testing.T) {
		// Connect to libvirt
		// Create domain via controller
		// Verify state
	})
}
```

## Mock-libvirtd vs Real libvirtd

### mock-libvirtd (via mock-servers)

**Pros:**
- Fast container startup (~2s)
- In-memory state, no I/O
- Realistic state machine with boot delays
- HTTP/REST interface (separate from RPC)

**Cons:**
- Speaks HTTP, not native libvirt RPC
- Requires adapter/bridge to test native RPC protocol
- No actual VM capability

**Use case:** Testing controller business logic, state transitions, error handling

### Real libvirtd

**Pros:**
- Tests actual RPC protocol
- Real VM lifecycle behavior
- Full feature compatibility

**Cons:**
- Slow container startup
- Requires KVM/QEMU host support
- Heavy resource usage

**Use case:** Full E2E validation, CI/CD verification

## Current Limitations

### Unit Tests
- Mock pointers are typed as `(*libvirt.Domain)(nil)` which is a limitation of the libvirt CGO bindings
- Full lifecycle testing would require actual libvirt connection

### E2E Tests
- Framework is ready but tests are stubbed (marked `.Skip()`)
- Need libvirt RPC client implementation to connect from tests
- Tests currently commented with "Requires libvirt RPC client"

## Future Enhancements

### Short-term

1. **Improve Mock Bindings**
   - Add volume operations to MockStoragePool
   - Implement proper mock domain state machine
   - Add mock metric tracking (CPU, memory usage)

2. **Complete E2E Test Implementation**
   - Implement libvirt URI connections in E2E tests
   - Add domain lifecycle assertions
   - Add error case coverage

### Medium-term

1. **Container-based Testing**
   - Option to use real libvirtd in Docker
   - Parallel test execution with isolated containers
   - Performance benchmarks

2. **CI/CD Integration**
   - E2E tests in GitHub Actions
   - Test matrix for different pool types
   - Performance regression detection

## Running Tests in CI/CD

### Local CI-like Setup

```bash
# Run all tests as CI would
make test
make lint
make coverage

# Run E2E tests (requires Docker)
make test-e2e
```

### GitHub Actions

See `.github/workflows/test.yml` for full CI configuration.

## Troubleshooting

### Mock Issues

**"panic: runtime error: invalid memory address or nil pointer dereference"**
- Ensure mock client is properly initialized
- Check that function pointers are assigned before use

**"Cannot use (*libvirt.Domain)(nil)"**
- This is expected due to CGO pointer limitations
- Mock functions accept type-casted nil pointers

### Container Issues

**"docker: command not found"**
- Install Docker: https://docs.docker.com/install/

**"E2E container fails to start"**
- Check Docker daemon is running: `docker ps`
- Check image exists: `docker images | grep mock-libvirtd`
- View logs: `docker logs <container-id>`

### Connection Issues

**"qemu+tcp://localhost:16509 refused"**
- Verify container is running: `docker ps`
- Check port mapping: `docker port <container-id>`
- Test health: `curl http://localhost:16509/health`
